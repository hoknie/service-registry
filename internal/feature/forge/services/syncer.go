package services

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	forgeports "svc-registry/internal/feature/forge/ports"
	forgetypes "svc-registry/internal/feature/forge/types"
	"svc-registry/pkg/ptr"
)

type syncer struct {
	svc      *SyncService
	conn     forgetypes.Connection
	trigger  forgetypes.Trigger
	client   forgeports.ClientPort
	groups   map[string]uuid.UUID
	counts   forgetypes.Counts
	problems []forgetypes.Problem
}

func (s *syncer) res(base forgetypes.RunResult) forgetypes.RunResult {
	base.Counts = s.counts
	base.Problems = s.problems
	return base
}

func (s *syncer) skip(path, code string) {
	s.counts.Skipped++
	if len(s.problems) < forgetypes.MaxProblems {
		s.problems = append(s.problems, forgetypes.Problem{FullPath: path, Code: code})
	}
}

func (s *syncer) parentFor(ctx context.Context, r forgetypes.RemoteRepo) (*uuid.UUID, string, error) {
	parent := s.conn.NodeID
	segs := forgetypes.Subgroups(s.conn.OwnerPath, r.FullPath, s.conn.MirrorSubgroups)
	for i := range segs {
		full := forgetypes.GroupPath(s.conn.OwnerPath, segs, i+1)
		if id, ok := s.groups[strings.ToLower(full)]; ok {
			parent = id
			continue
		}
		slug := forgetypes.Slugify(segs[i])
		folder, err := s.svc.catalog.System.InsertNode(ctx, catalogtypes.NewNode{
			ID: uuid.Must(uuid.NewV7()), Kind: catalogtypes.KindFolder, ParentID: &parent, Slug: slug,
			Name: nodeName(segs[i], slug), Labels: catalogtypes.Labels{},
		})
		if errors.Is(err, catalogtypes.ConflictSlugTaken) {
			return nil, catalogtypes.ConflictSlugTaken.Code, nil
		}
		if err != nil {
			return nil, "", err
		}
		if err := s.svc.repoRepository.LinkGroup(ctx, s.conn.ID, folder.ID, full); err != nil {
			return nil, "", err
		}
		s.groups[strings.ToLower(full)] = folder.ID
		parent = folder.ID
	}
	return &parent, "", nil
}

func (s *syncer) apply(ctx context.Context, t forgetypes.Target) error {
	r := t.Remote
	if r.TooLong() {
		s.skip(forgetypes.ProblemPath(r.FullPath), forgetypes.CodeValueTooLong)
		return nil
	}
	parent, code, err := s.parentFor(ctx, r)
	if err != nil {
		return err
	}
	if code != "" {
		s.skip(r.FullPath, code)
		return nil
	}
	repo := catalogtypes.Repo{Forge: new(s.conn.Kind.Forge()), RepoURL: ptr.NonEmpty(r.WebURL), DefaultBranch: ptr.NonEmpty(r.DefaultBranch)}
	changed := forgetypes.NeedsDetails(t)
	details, err := s.fetchDetails(ctx, r, changed)
	if err != nil {
		return err
	}
	placed := placement{target: t, parent: parent, slug: forgetypes.Slugify(r.Name), repo: repo, details: details, changed: changed}
	if t.Link == nil {
		return s.create(ctx, placed)
	}
	return s.update(ctx, placed)
}

type placement struct {
	target  forgetypes.Target
	parent  *uuid.UUID
	slug    string
	repo    catalogtypes.Repo
	details *forgetypes.Details
	changed bool
}

func (s *syncer) fetchDetails(ctx context.Context, r forgetypes.RemoteRepo, changed bool) (*forgetypes.Details, error) {
	if !changed {
		return nil, nil
	}
	d, err := s.client.Details(ctx, r)
	switch {
	case err == nil:
		return &d, nil
	case forgetypes.StopsSync(err):
		return nil, err
	default:
		s.skipDetails(r.FullPath, err)
		return nil, nil
	}
}

func (s *syncer) create(ctx context.Context, pl placement) error {
	r := pl.target.Remote
	node, err := s.svc.catalog.System.InsertNode(ctx, catalogtypes.NewNode{
		ID: uuid.Must(uuid.NewV7()), Kind: catalogtypes.KindProject, ParentID: pl.parent, Slug: pl.slug,
		Name: nodeName(r.Name, pl.slug), Description: description(r.Description), Labels: catalogtypes.Labels{}, Repo: pl.repo,
	})
	if errors.Is(err, catalogtypes.ConflictSlugTaken) {
		s.skip(r.FullPath, catalogtypes.ConflictSlugTaken.Code)
		return nil
	}
	if err != nil {
		return err
	}
	if err := s.svc.repoRepository.Upsert(ctx, forgetypes.RepoRecord{ProjectID: node.ID, ConnectionID: s.conn.ID, Remote: r, Details: pl.details}.Stored()); err != nil {
		_ = s.svc.catalog.System.RemoveNode(context.WithoutCancel(ctx), node.ID)
		return err
	}
	s.counts.Created++
	return s.branches(ctx, node.ID, r, pl.changed)
}

func (s *syncer) update(ctx context.Context, pl placement) error {
	r, t := pl.target.Remote, pl.target
	projectID := t.Link.ProjectID
	node, err := s.svc.catalog.System.FindNode(ctx, projectID)
	if err != nil {
		return err
	}
	if node == nil {
		return nil
	}
	if node.ParentID == nil || *node.ParentID != *pl.parent {
		if _, err := s.svc.catalog.System.PlaceNode(ctx, projectID, pl.parent); err != nil {
			if conflictCode(err) != "" {
				s.skip(r.FullPath, conflictCode(err))
				return nil
			}
			return err
		}
	}
	changes := catalogtypes.NodeChanges{
		Description:   new(description(r.Description)),
		Forge:         catalogtypes.Change[catalogtypes.Forge]{Set: true, Value: pl.repo.Forge},
		RepoURL:       catalogtypes.Change[string]{Set: true, Value: pl.repo.RepoURL},
		DefaultBranch: catalogtypes.Change[string]{Set: true, Value: pl.repo.DefaultBranch},
	}
	if !strings.EqualFold(t.Link.FullPath, r.FullPath) && node.Slug != pl.slug {
		changes.Slug = &pl.slug
		changes.Name = new(nodeName(r.Name, pl.slug))
	}
	if _, err := s.svc.catalog.System.ChangeNode(ctx, projectID, changes); err != nil {
		if conflictCode(err) != "" {
			s.skip(r.FullPath, conflictCode(err))
			return nil
		}
		return err
	}
	if err := s.svc.repoRepository.Upsert(ctx, forgetypes.RepoRecord{ProjectID: projectID, ConnectionID: s.conn.ID, Remote: r, Details: pl.details}.Stored()); err != nil {
		return err
	}
	if t.Adopted {
		if err := s.svc.unlink.adopted(ctx, s.conn.ID, r.ExternalID, projectID); err != nil {
			return err
		}
	}
	s.counts.Updated++
	return s.branches(ctx, projectID, r, pl.changed)
}

func (s *syncer) branches(ctx context.Context, projectID uuid.UUID, r forgetypes.RemoteRepo, changed bool) error {
	if changed {
		keep := forgetypes.BranchFilter(settingsOf(s.conn), r.DefaultBranch)
		list, truncated, err := s.client.ListBranches(ctx, r, keep, int(s.svc.cfg.Branches.SyncMaxPerRepo))
		switch {
		case forgetypes.StopsSync(err):
			return err
		case err != nil:
			s.skipDetails(r.FullPath, err)
		default:
			if truncated && len(s.problems) < forgetypes.MaxProblems {
				s.problems = append(s.problems, forgetypes.Problem{FullPath: r.FullPath, Code: "branches.truncated"})
			}
			moved, err := s.svc.catalog.BranchRecords.SyncForgeBranches(ctx, projectID, list, !truncated)
			if err != nil {
				return err
			}
			if err := s.triggerDocs(ctx, projectID, moved); err != nil {
				return err
			}
		}
	}
	return s.svc.catalog.BranchRecords.SetDefaultBranch(ctx, projectID, ptr.NonEmpty(r.DefaultBranch))
}

func (s *syncer) triggerDocs(ctx context.Context, projectID uuid.UUID, moved bool) error {
	if !moved || s.trigger == forgetypes.TriggerSchedule || s.svc.docs == nil {
		return nil
	}
	event := forgeports.EventForgeManual
	if s.trigger == forgetypes.TriggerWebhook {
		event = forgeports.EventForgeWebhook
	}
	return s.svc.docs.TriggerCollection(ctx, projectID, event)
}

func (s *syncer) skipDetails(path string, err error) {
	if len(s.problems) < forgetypes.MaxProblems {
		code := forgetypes.FailureCode(err)
		if code == "" {
			code = "upstream_error"
		}
		s.problems = append(s.problems, forgetypes.Problem{FullPath: path, Code: code})
	}
}
