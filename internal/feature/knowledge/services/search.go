package services

import (
	"context"
	"slices"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	knowledgeports "svc-registry/internal/feature/knowledge/ports"
	"svc-registry/internal/feature/knowledge/repositories"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/metrics"
)

type SearchResult struct {
	Hits []knowledgetypes.Hit
	Next string
}

type SearchModes struct {
	Engine  string
	Modes   []knowledgetypes.Mode
	Default knowledgetypes.Mode
	Pending *int
	Indexed *int
}

func (s *SearchService) engine() knowledgeports.EnginePort {
	if s.searchEngine == nil {
		return knowledgetypes.TextOnly{}
	}
	return s.searchEngine
}

func (s *SearchService) DefaultSearchMode() knowledgetypes.Mode { return s.engine().DefaultMode() }

func (s *SearchService) KnowledgeSearchModes(ctx context.Context) (SearchModes, error) {
	e := s.engine()
	out := SearchModes{Engine: e.Name(), Modes: e.Modes(), Default: e.DefaultMode()}
	if s.embedder != nil && len(out.Modes) > 1 {
		pending, indexed, err := s.indexRepository.Counts(ctx, s.embedder.Model())
		if err != nil {
			return SearchModes{}, apperr.Wrap(err)
		}
		out.Pending, out.Indexed = &pending, &indexed
	}
	return out, nil
}

func (s *SearchService) SearchKnowledge(ctx context.Context, p accesstypes.Principal, in knowledgetypes.SearchInput) (SearchResult, error) {
	if in.Branch != nil {
		name, err := catalogtypes.BranchName(*in.Branch)
		if err != nil {
			return SearchResult{}, apperr.Wrap(err)
		}
		in.Branch = &name
	}
	q, err := knowledgetypes.ValidateSearch(in)
	if err != nil {
		return SearchResult{}, apperr.Wrap(err)
	}
	e := s.engine()
	mode := e.DefaultMode()
	if in.Mode != "" {
		m, ok := knowledgetypes.ParseMode(in.Mode)
		if !ok || !offers(e, m) {
			return SearchResult{}, apperr.Wrap(knowledgetypes.InvalidSearchMode)
		}
		mode = m
	} else if !offers(e, mode) {
		mode = knowledgetypes.ModeText
	}
	viewer := knowledgetypes.Viewer{UserID: p.UserID, Nodes: p.Nodes, Superadmin: p.IsSuperadmin, Admin: p.CatalogAdmin}
	var hits []knowledgetypes.Hit
	var more bool
	switch {
	case mode == knowledgetypes.ModeText && !e.Handles(mode):
		hits, more, err = s.textSearch(ctx, viewer, q)
	case mode == knowledgetypes.ModeHybrid && !e.Handles(mode):
		hits, more, err = s.hybrid(ctx, e, viewer, q)
	default:
		var eq knowledgetypes.EngineQuery
		if eq, err = s.engineQuery(ctx, viewer, q, mode); err == nil {
			hits, more, err = e.Search(ctx, eq)
		}
	}
	if err != nil {
		return SearchResult{}, apperr.Wrap(err)
	}
	for i := range hits {
		hits[i].RefKind, hits[i].Branch = knowledgetypes.SplitVersionKey(hits[i].Branch)
	}
	out := SearchResult{Hits: hits}
	if more {
		out.Next = knowledgetypes.Cursor(q.Offset + q.Limit)
	}
	return out, nil
}

func (s *SearchService) engineQuery(ctx context.Context, v knowledgetypes.Viewer, q knowledgetypes.Search, mode knowledgetypes.Mode) (knowledgetypes.EngineQuery, error) {
	projects, err := s.searchRepository.Readable(ctx, v, q.Project)
	if err != nil {
		return knowledgetypes.EngineQuery{}, err
	}
	eq := knowledgetypes.EngineQuery{Search: q, Mode: mode, Projects: projects}
	switch {
	case q.Release && len(projects) > 0:
		if eq.Branches, err = s.tagRepository.Releases(ctx, projects); err != nil {
			return knowledgetypes.EngineQuery{}, err
		}
	case q.Branch == nil && len(projects) > 0:
		if eq.Branches, err = s.searchRepository.DefaultBranches(ctx, projects); err != nil {
			return knowledgetypes.EngineQuery{}, err
		}
	}
	if mode != knowledgetypes.ModeText {
		if s.embedder == nil {
			return knowledgetypes.EngineQuery{}, knowledgetypes.ErrEmbeddingsUnavailable
		}
		vectors, err := s.embedder.Embed(ctx, []string{q.Q})
		if err != nil {
			return knowledgetypes.EngineQuery{}, err
		}
		eq.Vector = vectors[0]
	}
	return eq, nil
}

func (s *SearchService) hybrid(ctx context.Context, e knowledgeports.EnginePort, v knowledgetypes.Viewer, q knowledgetypes.Search) ([]knowledgetypes.Hit, bool, error) {
	window := q
	window.Offset, window.Limit = 0, q.Offset+q.Limit+1
	text, _, err := s.textSearch(ctx, v, window)
	if err != nil {
		return nil, false, err
	}
	eq, err := s.engineQuery(ctx, v, window, knowledgetypes.ModeSemantic)
	if err != nil {
		return nil, false, err
	}
	semantic, _, err := e.Search(ctx, eq)
	if err != nil {
		return nil, false, err
	}
	fused := knowledgetypes.FuseRRF(text, semantic, q.Offset+q.Limit+1)
	if q.Offset >= len(fused) {
		return nil, false, nil
	}
	page := fused[q.Offset:]
	more := len(page) > q.Limit
	if more {
		page = page[:q.Limit]
	}
	return page, more, nil
}

func (s *SearchService) textSearch(ctx context.Context, v knowledgetypes.Viewer, q knowledgetypes.Search) ([]knowledgetypes.Hit, bool, error) {
	hits, err := s.searchRepository.Search(ctx, v, q.Probe(), knowledgetypes.NewTextQuery(q.Q))
	if err != nil {
		return nil, false, err
	}
	hits, more := q.Page(hits)
	return hits, more, nil
}

func offers(e knowledgeports.EnginePort, m knowledgetypes.Mode) bool {
	return slices.Contains(e.Modes(), m)
}

type SearchService struct {
	tagRepository        *repositories.TagRepository
	collectionRepository *repositories.CollectionRepository
	indexRepository      *repositories.IndexRepository
	searchRepository     *repositories.SearchRepository
	embedder             knowledgeports.EmbedderPort
	searchEngine         knowledgeports.EnginePort
}

var metricShapes = map[string]metrics.Shape{
	"snapshots": {Name: "svcr_knowledge_snapshots"},
	"pending":   {Name: "svcr_knowledge_files_pending"},
	"covered":   {Name: "svcr_projects_covered", Labels: []string{"aspect"}},
	"queue":     {Name: "svcr_job_queue", Labels: []string{"job", "state"}},
}

func (s *SearchService) MetricSamples(ctx context.Context) ([]metrics.Sample, error) {
	counts, err := s.collectionRepository.MetricCounts(ctx)
	if err != nil {
		return nil, err
	}
	if s.embedder != nil {
		pending, _, err := s.indexRepository.Counts(ctx, s.embedder.Model())
		if err != nil {
			return nil, err
		}
		counts["pending"] = int64(pending)
	}
	return metrics.FromPairs(metrics.Zero(counts, "covered:docs"), metricShapes), nil
}

func (s *SearchService) QueueSamples(ctx context.Context) ([]metrics.Sample, error) {
	counts, err := s.collectionRepository.QueueCounts(ctx)
	if err != nil {
		return nil, err
	}
	metrics.Zero(counts, "queue:knowledge_collection/due", "queue:knowledge_collection/leased",
		"queue:knowledge_index/due", "queue:knowledge_index/leased")
	return metrics.FromPairs(counts, metricShapes), nil
}
