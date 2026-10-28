package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	apisservices "svc-registry/internal/feature/apis/services"
	apistypes "svc-registry/internal/feature/apis/types"
	knowledgeservices "svc-registry/internal/feature/knowledge/services"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/internal/feature/publishing/repositories"
	publishingtypes "svc-registry/internal/feature/publishing/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/auth"
)

type PublicService struct {
	knowledge             *knowledgeservices.KnowledgeServices
	apis                  *apisservices.ApisServices
	publicationRepository *repositories.PublicationRepository
	hosts                 *HostsService
	limiter               *auth.PublicDocsLimiter
}

func (s *PublicService) Allow(client string) error {
	if s.limiter == nil {
		return nil
	}
	if retry, ok := s.limiter.Hit(client, time.Now()); !ok {
		return &apperr.Error{Kind: apperr.PublicRateLimited, RetryAfterSecs: retry}
	}
	return nil
}

type EntryRef struct {
	Root string
	Host *uuid.UUID
}

type PublicNode struct {
	Node    publishingtypes.Node
	HasDocs bool
}

type PublicProject struct {
	Path []string
	Name string
}

type PublicSpace struct {
	Projects  []PublicProject
	Entry     publishingtypes.Node
	EntryHost *string
	Node      publishingtypes.Node
	Ancestors []publishingtypes.Node
	Children  []PublicNode
	Docs      *publishingtypes.PublicDocs
	DocPath   *string
}

func (s *PublicService) entry(ctx context.Context, ref EntryRef) (*publishingtypes.Public, error) {
	var id uuid.UUID
	switch {
	case ref.Host != nil:
		id = *ref.Host
	case ref.Root != "":
		found, err := s.publicationRepository.EntryNode(ctx, ref.Root)
		if err != nil {
			return nil, apperr.Wrap(err)
		}
		if found == nil {
			return nil, apperr.ErrNotFound
		}
		id = *found
	default:
		return nil, apperr.ErrNotFound
	}
	chain, err := s.publicationRepository.Chain(ctx, id)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	if !publishingtypes.IsEntry(chain[0]) || (ref.Host != nil && chain[0].Own.Host == nil) {
		return nil, apperr.ErrNotFound
	}
	subtree, err := s.publicationRepository.Subtree(ctx, id)
	if err != nil || len(subtree) == 0 {
		return nil, apperr.Wrap(errOr(err))
	}
	pub, ok := publishingtypes.BuildPublic(chain, subtree[0], subtree[1:])
	if !ok {
		return nil, apperr.ErrNotFound
	}
	return pub, nil
}

func errOr(err error) error {
	if err != nil {
		return err
	}
	return apperr.ErrNotFound
}

func (s *PublicService) Space(ctx context.Context, ref EntryRef, segments []string) (PublicSpace, error) {
	pub, err := s.entry(ctx, ref)
	if err != nil {
		return PublicSpace{}, err
	}
	node, ancestors, docPath, ok := pub.Walk(segments)
	if !ok {
		return PublicSpace{}, apperr.ErrNotFound
	}
	out := PublicSpace{Entry: pub.Entry, EntryHost: pub.Entry.Own.Host, Node: node, Ancestors: ancestors, Children: []PublicNode{}}
	for _, p := range pub.Projects(pub.Entry.ID) {
		out.Projects = append(out.Projects, PublicProject{Path: pub.PathOf(p.ID), Name: p.Name})
	}
	if docPath != "" {
		out.DocPath = &docPath
	}
	if node.Kind == "project" {
		docs, err := s.versions(ctx, pub, node)
		if err != nil {
			return PublicSpace{}, err
		}
		out.Docs = &docs
		return out, nil
	}
	projects := pub.Projects(node.ID)
	ids := make([]uuid.UUID, len(projects))
	for i, p := range projects {
		ids[i] = p.ID
	}
	has, err := s.knowledge.Ports.ProjectsWithDocs(ctx, ids)
	if err != nil {
		return PublicSpace{}, err
	}
	for _, c := range pub.Children(node.ID) {
		item := PublicNode{Node: c}
		for _, p := range pub.Projects(c.ID) {
			item.HasDocs = item.HasDocs || has[p.ID]
		}
		out.Children = append(out.Children, item)
	}
	return out, nil
}

func (s *PublicService) versions(ctx context.Context, pub *publishingtypes.Public, project publishingtypes.Node) (publishingtypes.PublicDocs, error) {
	all, err := s.knowledge.Ports.PublishedDocs(ctx, project.ID)
	if err != nil {
		return publishingtypes.PublicDocs{}, err
	}
	settings := pub.Settings(project.ID)
	out := publishingtypes.PublicDocs{Branches: []publishingtypes.PublicVersion{}, Tags: []publishingtypes.PublicVersion{}}
	var defaultBranch *publishingtypes.PublicVersion
	for _, b := range all.Branches {
		if b.Snapshot != nil && settings.VersionAllowed(knowledgetypes.RefBranch, b.Name, b.Name == all.Default, false) {
			out.Branches = append(out.Branches, publishingtypes.PublicVersion{Name: b.Name, Kind: knowledgetypes.RefBranch, CollectedAt: b.Snapshot.CollectedAt})
			if b.Name == all.Default {
				defaultBranch = &out.Branches[len(out.Branches)-1]
			}
		}
	}
	for _, t := range all.Tags {
		if t.Snapshot != nil && settings.VersionAllowed(knowledgetypes.RefTag, t.Name, false, t.Release) {
			out.Tags = append(out.Tags, publishingtypes.PublicVersion{Name: t.Name, Kind: knowledgetypes.RefTag, CollectedAt: t.Snapshot.CollectedAt, Release: t.Release})
			if t.Release {
				out.Release = new(t.Name)
				out.Default = &out.Tags[len(out.Tags)-1]
			}
		}
	}
	switch {
	case out.Default != nil:
	case defaultBranch != nil:
		out.Default = defaultBranch
	case len(out.Branches) > 0:
		out.Default = &out.Branches[0]
	case len(out.Tags) > 0:
		out.Default = &out.Tags[0]
	}
	return out, nil
}

func (s *PublicService) project(ctx context.Context, ref EntryRef, segments []string, branch, tag, release *string) (*publishingtypes.Public, publishingtypes.Node, knowledgetypes.RefSelector, error) {
	pub, err := s.entry(ctx, ref)
	if err != nil {
		return nil, publishingtypes.Node{}, knowledgetypes.RefSelector{}, err
	}
	node, _, docPath, ok := pub.Walk(segments)
	if !ok || node.Kind != "project" || docPath != "" {
		return nil, publishingtypes.Node{}, knowledgetypes.RefSelector{}, apperr.ErrNotFound
	}
	sel, err := knowledgetypes.SelectRef(branch, tag, release)
	if err != nil {
		return nil, publishingtypes.Node{}, knowledgetypes.RefSelector{}, apperr.Wrap(err)
	}
	docs, err := s.versions(ctx, pub, node)
	if err != nil {
		return nil, publishingtypes.Node{}, knowledgetypes.RefSelector{}, err
	}
	chosen := publishingtypes.Pick(docs, sel)
	if chosen == nil {
		return nil, publishingtypes.Node{}, knowledgetypes.RefSelector{}, apperr.ErrNotFound
	}
	return pub, node, knowledgetypes.RefSelector{Kind: chosen.Kind, Name: chosen.Name}, nil
}

type VersionQuery struct{ Branch, Tag, Release *string }

func (s *PublicService) Tree(ctx context.Context, ref EntryRef, segments []string, v VersionQuery, dir string) (knowledgetypes.Snapshot, knowledgetypes.TreeLevel, error) {
	_, node, sel, err := s.project(ctx, ref, segments, v.Branch, v.Tag, v.Release)
	if err != nil {
		return knowledgetypes.Snapshot{}, knowledgetypes.TreeLevel{}, err
	}
	snap, _, level, err := s.knowledge.Ports.PublishedSnapshot(ctx, node.ID, sel, &dir)
	if err != nil {
		return knowledgetypes.Snapshot{}, knowledgetypes.TreeLevel{}, err
	}
	return snap, *level, nil
}

func (s *PublicService) Files(ctx context.Context, ref EntryRef, segments []string, v VersionQuery) (knowledgetypes.Snapshot, []knowledgetypes.FileInfo, error) {
	_, node, sel, err := s.project(ctx, ref, segments, v.Branch, v.Tag, v.Release)
	if err != nil {
		return knowledgetypes.Snapshot{}, nil, err
	}
	snap, files, _, err := s.knowledge.Ports.PublishedSnapshot(ctx, node.ID, sel, nil)
	return snap, files, err
}

func (s *PublicService) File(ctx context.Context, ref EntryRef, segments []string, v VersionQuery, path string) (knowledgetypes.Snapshot, knowledgetypes.FileContent, error) {
	_, node, sel, err := s.project(ctx, ref, segments, v.Branch, v.Tag, v.Release)
	if err != nil {
		return knowledgetypes.Snapshot{}, knowledgetypes.FileContent{}, err
	}
	return s.knowledge.Ports.PublishedFile(ctx, node.ID, sel, path)
}

func (s *PublicService) API(ctx context.Context, ref EntryRef, segments []string, v VersionQuery, path string) (knowledgetypes.Snapshot, apistypes.Document, error) {
	_, node, sel, err := s.project(ctx, ref, segments, v.Branch, v.Tag, v.Release)
	if err != nil {
		return knowledgetypes.Snapshot{}, apistypes.Document{}, err
	}
	return s.apis.Specs.PublishedAPI(ctx, node.ID, sel, path)
}

type PublicHit struct {
	knowledgetypes.Hit
	ProjectPath []string
}

func (s *PublicService) Search(ctx context.Context, ref EntryRef, segments []string, v VersionQuery, in knowledgetypes.SearchInput) ([]PublicHit, string, error) {
	pub, err := s.entry(ctx, ref)
	if err != nil {
		return nil, "", err
	}
	var targets []knowledgetypes.SearchTarget
	if len(segments) > 0 {
		_, node, sel, err := s.project(ctx, ref, segments, v.Branch, v.Tag, v.Release)
		if err != nil {
			return nil, "", err
		}
		targets = append(targets, knowledgetypes.SearchTarget{Project: node.ID, Key: knowledgetypes.VersionKey(sel.Kind, sel.Name)})
	} else {
		for _, p := range pub.Projects(pub.Entry.ID) {
			docs, err := s.versions(ctx, pub, p)
			if err != nil {
				return nil, "", err
			}
			if docs.Default != nil {
				targets = append(targets, knowledgetypes.SearchTarget{Project: p.ID, Key: knowledgetypes.VersionKey(docs.Default.Kind, docs.Default.Name)})
			}
		}
	}
	res, err := s.knowledge.Ports.PublishedSearch(ctx, targets, in)
	if err != nil {
		return nil, "", err
	}
	out := make([]PublicHit, 0, len(res.Hits))
	for _, h := range res.Hits {
		out = append(out, PublicHit{Hit: h, ProjectPath: pub.PathOf(h.ProjectID)})
	}
	return out, res.Next, nil
}
