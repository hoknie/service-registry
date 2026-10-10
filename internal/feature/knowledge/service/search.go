package service

import (
	"context"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/feature/catalog"
	"svc-registry/internal/feature/knowledge"
	"svc-registry/internal/platform/apperr"
)

type SearchResult struct {
	Hits []knowledge.Hit
	Next string
}

type SearchModes struct {
	Engine  string
	Modes   []knowledge.Mode
	Default knowledge.Mode
	Pending *int
	Indexed *int
}

func (s *Service) engine() knowledge.Engine {
	if s.searchEngine == nil {
		return knowledge.TextOnly{}
	}
	return s.searchEngine
}

func (s *Service) KnowledgeSearchModes(ctx context.Context) (SearchModes, error) {
	e := s.engine()
	out := SearchModes{Engine: e.Name(), Modes: e.Modes(), Default: e.DefaultMode()}
	if s.embedder != nil && len(out.Modes) > 1 {
		pending, indexed, err := s.docIndex.Counts(ctx, s.embedder.Model())
		if err != nil {
			return SearchModes{}, apperr.Wrap(err)
		}
		out.Pending, out.Indexed = &pending, &indexed
	}
	return out, nil
}

func (s *Service) SearchKnowledge(ctx context.Context, p access.Principal, in knowledge.SearchInput) (SearchResult, error) {
	if in.Branch != nil {
		name, err := catalog.BranchName(*in.Branch)
		if err != nil {
			return SearchResult{}, apperr.Wrap(err)
		}
		in.Branch = &name
	}
	q, err := knowledge.ValidateSearch(in)
	if err != nil {
		return SearchResult{}, apperr.Wrap(err)
	}
	e := s.engine()
	mode := e.DefaultMode()
	if in.Mode != "" {
		m, ok := knowledge.ParseMode(in.Mode)
		if !ok || !knowledge.Offers(e, m) {
			return SearchResult{}, apperr.Wrap(knowledge.InvalidSearchMode)
		}
		mode = m
	} else if !knowledge.Offers(e, mode) {
		mode = knowledge.ModeText
	}
	viewer := knowledge.Viewer{UserID: p.UserID, Superadmin: p.IsSuperadmin}
	var hits []knowledge.Hit
	var more bool
	switch {
	case mode == knowledge.ModeText && !e.Handles(mode):
		hits, more, err = s.docSearch.Search(ctx, viewer, q)
	case mode == knowledge.ModeHybrid && !e.Handles(mode):
		hits, more, err = s.hybrid(ctx, e, viewer, q)
	default:
		var eq knowledge.EngineQuery
		if eq, err = s.engineQuery(ctx, viewer, q, mode); err == nil {
			hits, more, err = e.Search(ctx, eq)
		}
	}
	if err != nil {
		return SearchResult{}, apperr.Wrap(err)
	}
	out := SearchResult{Hits: hits}
	if more {
		out.Next = knowledge.Cursor(q.Offset + q.Limit)
	}
	return out, nil
}

func (s *Service) engineQuery(ctx context.Context, v knowledge.Viewer, q knowledge.Search, mode knowledge.Mode) (knowledge.EngineQuery, error) {
	projects, err := s.docSearch.Readable(ctx, v, q.Project)
	if err != nil {
		return knowledge.EngineQuery{}, err
	}
	eq := knowledge.EngineQuery{Search: q, Mode: mode, Projects: projects}
	if q.Branch == nil && len(projects) > 0 {
		if eq.Branches, err = s.docSearch.DefaultBranches(ctx, projects); err != nil {
			return knowledge.EngineQuery{}, err
		}
	}
	if mode != knowledge.ModeText {
		if s.embedder == nil {
			return knowledge.EngineQuery{}, knowledge.ErrEmbeddingsUnavailable
		}
		vectors, err := s.embedder.Embed(ctx, []string{q.Q})
		if err != nil {
			return knowledge.EngineQuery{}, err
		}
		eq.Vector = vectors[0]
	}
	return eq, nil
}

func (s *Service) hybrid(ctx context.Context, e knowledge.Engine, v knowledge.Viewer, q knowledge.Search) ([]knowledge.Hit, bool, error) {
	window := q
	window.Offset, window.Limit = 0, q.Offset+q.Limit+1
	text, _, err := s.docSearch.Search(ctx, v, window)
	if err != nil {
		return nil, false, err
	}
	eq, err := s.engineQuery(ctx, v, window, knowledge.ModeSemantic)
	if err != nil {
		return nil, false, err
	}
	semantic, _, err := e.Search(ctx, eq)
	if err != nil {
		return nil, false, err
	}
	fused := knowledge.FuseRRF(text, semantic, q.Offset+q.Limit+1)
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
