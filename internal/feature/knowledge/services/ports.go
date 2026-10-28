package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	catalogports "svc-registry/internal/feature/catalog/ports"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	knowledgeports "svc-registry/internal/feature/knowledge/ports"
	"svc-registry/internal/feature/knowledge/repositories"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/pkg/secretbox"
)

func (s *PortsService) ActivitySignals(ctx context.Context, projects []uuid.UUID, pending, busy bool) (map[uuid.UUID][]catalogtypes.ProcessSignals, error) {
	model := ""
	if s.embedder != nil {
		model = s.embedder.Model()
	}
	return s.collectionRepository.Signals(ctx, projects, s.embedder != nil || s.external != nil, model, pending, busy)
}

type preparedSource struct {
	svc    *PortsService
	source knowledgetypes.NewSource
}

func (p preparedSource) Repo() (*catalogtypes.Hosting, *string, bool) {
	return sourceRepo(p.source.Source)
}

func (p preparedSource) Attach(ctx context.Context, projectID uuid.UUID) error {
	if err := p.svc.sourcesService.checkSecret(ctx, projectID, p.source.SecretID); err != nil {
		return err
	}
	_, err := p.svc.sourcesService.saveSource(ctx, projectID, p.source)
	return err
}

func (s *PortsService) SourceFor(in knowledgetypes.SourceInput) catalogports.SourceFactoryPort {
	return func(projectID uuid.UUID) (catalogports.ProjectSourcePort, error) {
		n, err := s.sourcesService.prepareSource(context.Background(), projectID, in, false)
		if err != nil {
			return nil, err
		}
		return preparedSource{svc: s, source: n}, nil
	}
}

func (s *PortsService) AdoptLegacyCredentials(ctx context.Context, adopt catalogtypes.AdoptFunc) (int, error) {
	rows, err := s.legacySources(ctx)
	if err != nil {
		return 0, err
	}
	for _, row := range rows {
		value := catalogtypes.SecretValue{Ref: row.Ref}
		if row.Ref == nil {
			v, err := s.secrets.Open(*row.Enc, secretbox.AAD(knowledgetypes.SourceTable, row.ProjectID.String(), knowledgetypes.SourceColumnCredentials))
			if err != nil {
				return 0, fmt.Errorf("documentation source %s: %w", row.ProjectID, err)
			}
			value.Value = &v
		}
		secret, err := adopt(ctx, catalogtypes.LegacyCredential{RecordKind: "source", RecordID: row.ProjectID, Scope: row.ParentID,
			Name: row.Slug, Value: value})
		if err != nil {
			return 0, fmt.Errorf("documentation source %s: %w", row.ProjectID, err)
		}
		if err := s.sourceRepository.AdoptSecret(ctx, row.ProjectID, secret); err != nil {
			return 0, err
		}
	}
	return len(rows), nil
}

func (s *PortsService) LegacyCredentialCount(ctx context.Context) (int, error) {
	rows, err := s.legacySources(ctx)
	return len(rows), err
}

func (s *PortsService) legacySources(ctx context.Context) ([]knowledgetypes.LegacySource, error) {
	ok, err := s.sourceRepository.HasLegacyCredentials(ctx)
	if err != nil || !ok {
		return nil, err
	}
	return s.sourceRepository.Legacy(ctx)
}

type IndexInfo struct {
	Engine   knowledgeports.EnginePort
	Model    string
	Enabled  bool
	External bool
}

func (s *PortsService) Indexing() IndexInfo {
	info := IndexInfo{Engine: s.searchEngine, Enabled: s.embedder != nil || s.external != nil, External: s.external != nil}
	if s.embedder != nil {
		info.Model = s.embedder.Model()
	}
	return info
}

func (s *PortsService) SecretUsage(ctx context.Context, secrets []uuid.UUID) (map[uuid.UUID]int64, error) {
	return s.sourceRepository.SecretUsage(ctx, secrets)
}

func (s *PortsService) TriggerCollection(ctx context.Context, projectID uuid.UUID, event string) error {
	settings, err := s.settingsService.effectiveSettings(ctx, projectID)
	if err != nil || settings.Refresh != knowledgetypes.RefreshEvent || !settings.Listens(knowledgetypes.CollectEvent(event)) {
		return err
	}
	return apperr.Wrap(s.collectionRepository.Trigger(ctx, projectID))
}

type PortsService struct {
	documents            *DocumentsService
	snapshotRepository   *repositories.SnapshotRepository
	searchRepository     *repositories.SearchRepository
	embedder             knowledgeports.EmbedderPort
	external             knowledgeports.ExternalIndexPort
	searchEngine         knowledgeports.EnginePort
	secrets              *secretbox.Box
	collectionRepository *repositories.CollectionRepository
	sourceRepository     *repositories.SourceRepository
	sourcesService       *SourcesService
	settingsService      *SettingsService
}

func (s *DocumentsService) tagStates(ctx context.Context, kp knowledgetypes.Project, collecting bool) ([]knowledgetypes.BranchState, *string, error) {
	tags, err := s.tagRepository.List(ctx, kp.ID)
	if err != nil {
		return nil, nil, apperr.Wrap(err)
	}
	knowledgetypes.SortTags(tags, func(t knowledgetypes.StoredTag) knowledgetypes.TagInfo {
		return knowledgetypes.TagInfo{Name: t.Name, FirstSeen: t.FirstSeen}
	})
	out := make([]knowledgetypes.BranchState, 0, len(tags))
	var release *string
	for _, t := range tags {
		bs := knowledgetypes.BranchState{Name: t.Name, HeadSHA: new(t.Commit), Release: t.Release}
		if err := s.fillState(ctx, kp, knowledgetypes.RefTag, &bs); err != nil {
			return nil, nil, err
		}
		bs.Pending = collecting && (kp.Force || bs.Last == nil || bs.Last.Commit != t.Commit || bs.Last.Status == knowledgetypes.StatusFailed)
		if t.Release {
			release = new(t.Name)
		}
		out = append(out, bs)
	}
	return out, release, nil
}

func (s *DocumentsService) titles(ctx context.Context, snapshot uuid.UUID, level *knowledgetypes.TreeLevel) error {
	paths := level.MarkdownPaths()
	if len(paths) == 0 {
		return nil
	}
	contents, err := s.snapshotRepository.Contents(ctx, snapshot, paths)
	if err != nil {
		return apperr.Wrap(err)
	}
	level.SetTitles(contents)
	return nil
}

type PublishedDocs struct {
	Default  string
	Branches []knowledgetypes.BranchState
	Tags     []knowledgetypes.BranchState
	Release  *string
}

func (s *PortsService) publicProject(ctx context.Context, id uuid.UUID) (knowledgetypes.Project, error) {
	kp, err := s.collectionRepository.Project(ctx, id)
	if err != nil {
		return knowledgetypes.Project{}, apperr.Wrap(err)
	}
	kp.Settings, err = s.settingsService.effectiveSettings(ctx, id)
	return kp, err
}

func (s *PortsService) PublishedDocs(ctx context.Context, project uuid.UUID) (PublishedDocs, error) {
	kp, err := s.publicProject(ctx, project)
	if err != nil {
		return PublishedDocs{}, err
	}
	d, err := s.documents.documentationOf(ctx, kp)
	if err != nil {
		return PublishedDocs{}, err
	}
	o, err := s.documents.overview(ctx, kp)
	if err != nil {
		return PublishedDocs{}, err
	}
	return PublishedDocs{Default: d.Default, Branches: o.Branches, Tags: o.Tags, Release: o.Release}, nil
}

func (s *PortsService) PublishedSnapshot(ctx context.Context, project uuid.UUID, sel knowledgetypes.RefSelector, rawDir *string) (knowledgetypes.Snapshot, []knowledgetypes.FileInfo, *knowledgetypes.TreeLevel, error) {
	snap, files, err := s.documents.snapshotFiles(ctx, project, sel, knowledgetypes.Ref{})
	if err != nil {
		return knowledgetypes.Snapshot{}, nil, nil, apperr.Wrap(err)
	}
	if rawDir == nil {
		return snap, files, nil, nil
	}
	dir, ok := knowledgetypes.NormalizeDir(*rawDir)
	if !ok {
		return knowledgetypes.Snapshot{}, nil, nil, apperr.New(apperr.NotFound)
	}
	level, found := knowledgetypes.Level(files, dir)
	if !found {
		return knowledgetypes.Snapshot{}, nil, nil, apperr.New(apperr.NotFound)
	}
	return snap, files, &level, s.documents.titles(ctx, snap.ID, &level)
}

func (s *PortsService) PublishedFile(ctx context.Context, project uuid.UUID, sel knowledgetypes.RefSelector, path string) (knowledgetypes.Snapshot, knowledgetypes.FileContent, error) {
	snap, f, err := s.documents.snapshotFile(ctx, project, sel, knowledgetypes.Ref{}, path)
	return snap, f, apperr.Wrap(err)
}

func (s *PortsService) ProjectsWithDocs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	out, err := s.snapshotRepository.ProjectsWithDocs(ctx, ids)
	return out, apperr.Wrap(err)
}

func (s *PortsService) PublishedSearch(ctx context.Context, targets []knowledgetypes.SearchTarget, in knowledgetypes.SearchInput) (SearchResult, error) {
	q, err := knowledgetypes.ValidateSearch(in)
	if err != nil {
		return SearchResult{}, apperr.Wrap(err)
	}
	if len(targets) == 0 {
		return SearchResult{Hits: []knowledgetypes.Hit{}}, nil
	}
	hits, err := s.searchRepository.SearchIn(ctx, targets, q.Probe(), knowledgetypes.NewTextQuery(q.Q))
	if err != nil {
		return SearchResult{}, apperr.Wrap(err)
	}
	hits, more := q.Page(hits)
	for i := range hits {
		hits[i].RefKind, hits[i].Branch = knowledgetypes.SplitVersionKey(hits[i].Branch)
	}
	out := SearchResult{Hits: hits}
	if more {
		out.Next = knowledgetypes.Cursor(q.Offset + q.Limit)
	}
	return out, nil
}
