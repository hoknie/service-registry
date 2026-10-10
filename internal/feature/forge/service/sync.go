package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"svc-registry/internal/feature/catalog"
	"svc-registry/internal/feature/forge"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/secretbox"
)

const syncLeaseSecs = 120

func (s *Service) ClaimForgeRuns(ctx context.Context, limit, leaseSecs int) ([]forge.Claimed, error) {
	return s.connections.Claim(ctx, limit, leaseSecs)
}

func (s *Service) ExtendForgeRun(ctx context.Context, c forge.Claimed, leaseSecs int) error {
	return s.connections.Extend(ctx, c.ID, leaseSecs)
}

func (s *Service) RunClaimedSync(ctx context.Context, c forge.Claimed) {
	if _, err := s.runSync(ctx, c.ID, c.Trigger); err != nil {
		slog.Error("forge sync failed", "connection", c.ID, "error", err)
	}
}

func (s *Service) RunSyncNow(ctx context.Context, id uuid.UUID) (forge.Run, error) {
	conn, err := s.connections.Find(ctx, id)
	if err != nil {
		return forge.Run{}, apperr.Wrap(err)
	}
	if conn == nil {
		return forge.Run{}, apperr.New(apperr.NotFound)
	}
	ok, err := s.connections.ClaimOne(ctx, id, syncLeaseSecs)
	if err != nil {
		return forge.Run{}, apperr.Wrap(err)
	}
	if !ok {
		return forge.Run{}, ErrSyncBusy
	}
	done := make(chan struct{})
	defer close(done)
	go func() {
		t := time.NewTicker(syncLeaseSecs / 3 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				_ = s.connections.Extend(context.WithoutCancel(ctx), id, syncLeaseSecs)
			}
		}
	}()
	return s.runSync(ctx, id, forge.TriggerCLI)
}

var ErrSyncBusy = errors.New("sync already running")

func (s *Service) runSync(ctx context.Context, id uuid.UUID, trigger forge.Trigger) (forge.Run, error) {
	keep := context.WithoutCancel(ctx)
	run, err := s.runs.Start(keep, forge.NewRun{ID: uuid.Must(uuid.NewV7()), ConnectionID: id, Trigger: trigger})
	if err != nil {
		_ = s.connections.Release(keep, id, nil)
		return forge.Run{}, err
	}
	conn, err := s.connections.Find(keep, id)
	if err != nil || conn == nil {
		return run, err
	}
	res, next := s.synchronize(ctx, *conn)
	if err := s.runs.Finish(keep, run.ID, res, int(s.cfg.Jobs.ForgeRunsKept)); err != nil {
		return run, err
	}
	if err := s.connections.Release(keep, id, next); err != nil {
		return run, err
	}
	run.Status, run.Counts, run.ErrorCode, run.ErrorMessage, run.Problems = res.Status, res.Counts, res.ErrorCode, res.ErrorMessage, res.Problems
	return run, nil
}

func failedWith(res forge.RunResult, err error) (forge.RunResult, *time.Time) {
	code := forge.FailureCode(err)
	msg := err.Error()
	if code == "" {
		code = "internal"
		msg = "internal error"
		slog.Error("forge sync internal error", "error", err)
	}
	res.ErrorCode, res.ErrorMessage = &code, &msg
	if errors.Is(err, forge.ErrInterrupted) {
		res.Status = forge.RunFailed
		now := time.Now()
		return res, &now
	}
	var limited *forge.RateLimited
	if errors.As(err, &limited) {
		res.Status = forge.RunRateLimited
		if !limited.Reset.IsZero() {
			reset := limited.Reset
			return res, &reset
		}
		return res, nil
	}
	res.Status = forge.RunFailed
	return res, nil
}

func (s *Service) clientFor(ctx context.Context, conn forge.Connection) (forge.Client, error) {
	sec, err := s.connections.Secrets(ctx, conn.ID)
	if err != nil {
		return nil, err
	}
	if sec == nil {
		return nil, forge.ErrNotFound
	}
	var token string
	switch {
	case sec.CredentialsSecretID != nil:
		token, err = s.catalog.ResolveSecret(ctx, *sec.CredentialsSecretID)
	case sec.CredentialsRef != nil:
		token, err = config.ResolveSecretRef(*sec.CredentialsRef)
	case sec.CredentialsEnc != nil:
		token, err = s.secrets.Open(*sec.CredentialsEnc, secretbox.AAD(forge.Table, conn.ID.String(), forge.ColumnCredentials))
	default:
		err = errors.New("no credentials")
	}
	if err != nil {
		slog.Warn("forge credentials unavailable", "connection", conn.ID, "error", err)
		return nil, forge.ErrCredentialsUnavailable
	}
	return s.clients.New(forge.Endpoint{Kind: conn.Kind, APIURL: conn.APIURL, Owner: conn.OwnerPath, Token: token})
}

func settingsOf(c forge.Connection) forge.Settings {
	return forge.Settings{Kind: c.Kind, APIURL: c.APIURL, OwnerPath: c.OwnerPath, MirrorSubgroups: c.MirrorSubgroups,
		IncludeArchived: c.IncludeArchived, IncludeForks: c.IncludeForks, NameInclude: c.NameInclude,
		NameExclude: c.NameExclude, BranchInclude: c.BranchInclude, IntervalSecs: c.IntervalSecs}
}

func (s *Service) remoteState(ctx context.Context, conn forge.Connection, client forge.Client) ([]forge.Target, []forge.Link, error) {
	all, err := client.ListRepos(ctx)
	if err != nil {
		return nil, nil, err
	}
	settings := settingsOf(conn)
	var kept []forge.RemoteRepo
	for _, r := range all {
		if forge.Keep(settings, r) {
			kept = append(kept, r)
		}
	}
	links, err := s.repositories.Links(ctx, conn.ID)
	if err != nil {
		return nil, nil, err
	}
	targets, orphans := forge.Plan(kept, links)
	return targets, orphans, nil
}

func (s *Service) synchronize(ctx context.Context, conn forge.Connection) (forge.RunResult, *time.Time) {
	res := forge.RunResult{Status: forge.RunSucceeded}
	failed := func(res forge.RunResult, err error) (forge.RunResult, *time.Time) {
		if ctx.Err() != nil {
			err = forge.ErrInterrupted
		}
		return failedWith(res, err)
	}
	client, err := s.clientFor(ctx, conn)
	if err != nil {
		return failed(res, err)
	}
	targets, orphans, err := s.remoteState(ctx, conn, client)
	if err != nil {
		return failed(res, err)
	}
	groups, err := s.repositories.Groups(ctx, conn.ID)
	if err != nil {
		return failed(res, err)
	}
	sc := &syncer{svc: s, conn: conn, client: client, groups: groups}
	for _, t := range targets {
		if err := ctx.Err(); err != nil {
			return failed(sc.res(res), forge.ErrInterrupted)
		}
		if err := sc.apply(ctx, t); err != nil {
			return failed(sc.res(res), err)
		}
	}
	ids := make([]uuid.UUID, 0, len(orphans))
	for _, o := range orphans {
		ids = append(ids, o.ProjectID)
	}
	if err := s.repositories.Orphan(context.WithoutCancel(ctx), ids); err != nil {
		return failed(sc.res(res), err)
	}
	sc.counts.Orphaned = len(ids)
	return sc.res(res), nil
}

type syncer struct {
	svc      *Service
	conn     forge.Connection
	client   forge.Client
	groups   map[string]uuid.UUID
	counts   forge.Counts
	problems []forge.Problem
}

func (s *syncer) res(base forge.RunResult) forge.RunResult {
	base.Counts = s.counts
	base.Problems = s.problems
	return base
}

func (s *syncer) skip(path, code string) {
	s.counts.Skipped++
	if len(s.problems) < forge.MaxProblems {
		s.problems = append(s.problems, forge.Problem{FullPath: path, Code: code})
	}
}

func (s *syncer) parentFor(ctx context.Context, r forge.RemoteRepo) (*uuid.UUID, string, error) {
	parent := s.conn.NodeID
	segs := forge.Subgroups(s.conn.OwnerPath, r.FullPath, s.conn.MirrorSubgroups)
	for i := range segs {
		full := forge.GroupPath(s.conn.OwnerPath, segs, i+1)
		if id, ok := s.groups[strings.ToLower(full)]; ok {
			parent = id
			continue
		}
		slug := forge.Slugify(segs[i])
		folder, err := s.svc.catalog.InsertNode(ctx, catalog.NewNode{
			ID: uuid.Must(uuid.NewV7()), Kind: catalog.KindFolder, ParentID: &parent, Slug: slug,
			Name: nodeName(segs[i], slug), Labels: catalog.Labels{},
		})
		if errors.Is(err, catalog.ConflictSlugTaken) {
			return nil, catalog.ConflictSlugTaken.Code(), nil
		}
		if err != nil {
			return nil, "", err
		}
		if err := s.svc.repositories.LinkGroup(ctx, s.conn.ID, folder.ID, full); err != nil {
			return nil, "", err
		}
		s.groups[strings.ToLower(full)] = folder.ID
		parent = folder.ID
	}
	return &parent, "", nil
}

func (s *syncer) apply(ctx context.Context, t forge.Target) error {
	r := t.Remote
	parent, code, err := s.parentFor(ctx, r)
	if err != nil {
		return err
	}
	if code != "" {
		s.skip(r.FullPath, code)
		return nil
	}
	slug := forge.Slugify(r.Name)
	repo := catalog.Repo{RepoURL: optional(r.WebURL), DefaultBranch: optional(r.DefaultBranch)}
	f := s.conn.Kind.Forge()
	repo.Forge = &f
	var details *forge.Details
	changed := forge.NeedsDetails(t)
	if changed {
		d, err := s.client.Details(ctx, r)
		var limited *forge.RateLimited
		switch {
		case err == nil:
			details = &d
		case errors.As(err, &limited), errors.Is(err, forge.ErrInterrupted), errors.Is(err, forge.ErrUnauthorized):
			return err
		default:
			s.skipDetails(r.FullPath, err)
		}
	}
	if t.Link == nil {
		node, err := s.svc.catalog.InsertNode(ctx, catalog.NewNode{
			ID: uuid.Must(uuid.NewV7()), Kind: catalog.KindProject, ParentID: parent, Slug: slug,
			Name: nodeName(r.Name, slug), Description: description(r.Description), Labels: catalog.Labels{}, Repo: repo,
		})
		if errors.Is(err, catalog.ConflictSlugTaken) {
			s.skip(r.FullPath, catalog.ConflictSlugTaken.Code())
			return nil
		}
		if err != nil {
			return err
		}
		if err := s.svc.repositories.Upsert(ctx, forge.RepoRecord{ProjectID: node.ID, ConnectionID: s.conn.ID, Remote: r, Details: details}); err != nil {
			_ = s.svc.catalog.RemoveNode(context.WithoutCancel(ctx), node.ID)
			return err
		}
		s.counts.Created++
		return s.branches(ctx, node.ID, r, changed)
	}
	projectID := t.Link.ProjectID
	node, err := s.svc.catalog.FindNode(ctx, projectID)
	if err != nil {
		return err
	}
	if node == nil {
		return nil
	}
	if node.ParentID == nil || *node.ParentID != *parent {
		if _, err := s.svc.catalog.PlaceNode(ctx, projectID, parent); err != nil {
			if conflictCode(err) != "" {
				s.skip(r.FullPath, conflictCode(err))
				return nil
			}
			return err
		}
	}
	changes := catalog.NodeChanges{
		Description:   ptrTo(description(r.Description)),
		Forge:         catalog.Change[catalog.Forge]{Set: true, Value: repo.Forge},
		RepoURL:       catalog.Change[string]{Set: true, Value: repo.RepoURL},
		DefaultBranch: catalog.Change[string]{Set: true, Value: repo.DefaultBranch},
	}
	if !strings.EqualFold(t.Link.FullPath, r.FullPath) && node.Slug != slug {
		changes.Slug = &slug
		changes.Name = ptrTo(nodeName(r.Name, slug))
	}
	if _, err := s.svc.catalog.ChangeNode(ctx, projectID, changes); err != nil {
		if conflictCode(err) != "" {
			s.skip(r.FullPath, conflictCode(err))
			return nil
		}
		return err
	}
	if err := s.svc.repositories.Upsert(ctx, forge.RepoRecord{ProjectID: projectID, ConnectionID: s.conn.ID, Remote: r, Details: details}); err != nil {
		return err
	}
	s.counts.Updated++
	return s.branches(ctx, projectID, r, changed)
}

func (s *syncer) branches(ctx context.Context, projectID uuid.UUID, r forge.RemoteRepo, changed bool) error {
	if changed {
		keep := forge.BranchFilter(settingsOf(s.conn), r.DefaultBranch)
		list, truncated, err := s.client.ListBranches(ctx, r, keep, int(s.svc.cfg.Branches.SyncMaxPerRepo))
		var limited *forge.RateLimited
		switch {
		case errors.As(err, &limited), errors.Is(err, forge.ErrInterrupted), errors.Is(err, forge.ErrUnauthorized):
			return err
		case err != nil:
			s.skipDetails(r.FullPath, err)
		default:
			if truncated && len(s.problems) < forge.MaxProblems {
				s.problems = append(s.problems, forge.Problem{FullPath: r.FullPath, Code: "branches.truncated"})
			}
			if err := s.svc.catalog.SyncForgeBranches(ctx, projectID, list, !truncated); err != nil {
				return err
			}
		}
	}
	return s.svc.catalog.SetDefaultBranch(ctx, projectID, optional(r.DefaultBranch))
}

func (s *syncer) skipDetails(path string, err error) {
	if len(s.problems) < forge.MaxProblems {
		code := forge.FailureCode(err)
		if code == "" {
			code = "upstream_error"
		}
		s.problems = append(s.problems, forge.Problem{FullPath: path, Code: code})
	}
}

func conflictCode(err error) string {
	var c catalog.Conflict
	if errors.As(err, &c) {
		return c.Code()
	}
	return ""
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func ptrTo[T any](v T) *T { return &v }

func nodeName(name, slug string) string {
	clean := strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, name))
	if utf8.RuneCountInString(clean) > 100 {
		clean = string([]rune(clean)[:100])
	}
	if clean == "" {
		return slug
	}
	return clean
}

func description(d string) string {
	clean := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, d)
	if utf8.RuneCountInString(clean) > 2000 {
		clean = string([]rune(clean)[:2000])
	}
	return clean
}
