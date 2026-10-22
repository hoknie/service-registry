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
	viewer := knowledgetypes.Viewer{UserID: p.UserID, Superadmin: p.IsSuperadmin}
	var hits []knowledgetypes.Hit
	var more bool
	switch {
	case mode == knowledgetypes.ModeText && !e.Handles(mode):
		hits, more, err = s.searchRepository.Search(ctx, viewer, q)
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
	if q.Branch == nil && len(projects) > 0 {
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
	text, _, err := s.searchRepository.Search(ctx, v, window)
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

func offers(e knowledgeports.EnginePort, m knowledgetypes.Mode) bool {
	return slices.Contains(e.Modes(), m)
}

type SearchService struct {
	indexRepository  *repositories.IndexRepository
	searchRepository *repositories.SearchRepository
	embedder         knowledgeports.EmbedderPort
	searchEngine     knowledgeports.EnginePort
}
