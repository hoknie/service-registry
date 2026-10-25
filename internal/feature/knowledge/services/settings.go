package services

import (
	"context"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/feature/knowledge/repositories"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/pgtx"
)

type KnowledgeSettings struct {
	Settings                                                      knowledgetypes.Settings
	Defaults                                                      knowledgetypes.Settings
	Own                                                           knowledgetypes.NodeSettings
	IncludeFrom, ExcludeFrom, BranchesFrom, APIsFrom, RefreshFrom *knowledgetypes.ChainNode
	APIPathsFrom, RefreshEventsFrom                               *knowledgetypes.ChainNode
	IntervalSecs                                                  uint32
}

func (s *SettingsService) effectiveSettings(ctx context.Context, id uuid.UUID) (knowledgetypes.Settings, error) {
	chain, err := s.patternRepository.Chain(ctx, id)
	if err != nil {
		return knowledgetypes.Settings{}, apperr.Wrap(err)
	}
	sc, _ := knowledgetypes.Merge(chain)
	return sc, nil
}

func (s *SettingsService) nodeSettings(ctx context.Context, id uuid.UUID) (KnowledgeSettings, error) {
	chain, err := s.patternRepository.Chain(ctx, id)
	if err != nil {
		return KnowledgeSettings{}, apperr.Wrap(err)
	}
	sc, from := knowledgetypes.Merge(chain)
	origin := func(i int) *knowledgetypes.ChainNode {
		if i <= 0 {
			return nil
		}
		return &chain[i]
	}
	return KnowledgeSettings{Settings: sc, Defaults: knowledgetypes.DefaultSettings(), Own: chain[0].Own, IncludeFrom: origin(from.Include),
		ExcludeFrom: origin(from.Exclude), BranchesFrom: origin(from.Branches), APIsFrom: origin(from.APIs), RefreshFrom: origin(from.Refresh),
		APIPathsFrom: origin(from.APIPaths), RefreshEventsFrom: origin(from.RefreshEvents), IntervalSecs: s.cfg.Knowledge.IntervalSecs}, nil
}

func (s *SettingsService) GetKnowledgeSettings(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (KnowledgeSettings, error) {
	if _, err := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermRead, id); err != nil {
		return KnowledgeSettings{}, err
	}
	return s.nodeSettings(ctx, id)
}

func (s *SettingsService) PutKnowledgeSettings(ctx context.Context, p accesstypes.Principal, id uuid.UUID, in knowledgetypes.NodeSettings) (KnowledgeSettings, error) {
	if _, err := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermWrite, id); err != nil {
		return KnowledgeSettings{}, err
	}
	sc, err := knowledgetypes.ValidateNodeSettings(in)
	if err != nil {
		return KnowledgeSettings{}, apperr.Wrap(err)
	}
	if err := s.putPatterns(ctx, id, sc); err != nil {
		return KnowledgeSettings{}, apperr.Wrap(err)
	}
	return s.nodeSettings(ctx, id)
}

func (s *SettingsService) putPatterns(ctx context.Context, id uuid.UUID, sc knowledgetypes.NodeSettings) error {
	return repositories.Err(s.db.InTx(ctx, func(ctx context.Context) error {
		var err error
		if sc.IsEmpty() {
			err = s.patternRepository.Delete(ctx, id)
		} else {
			err = s.patternRepository.Upsert(ctx, id, sc.Stored())
		}
		if err != nil {
			return err
		}
		return s.collectionRepository.MarkSubtreeDue(ctx, id)
	}))
}

type SettingsService struct {
	cfg                  *config.Config
	db                   *pgtx.DB
	catalog              *catalogservices.CatalogServices
	collectionRepository *repositories.CollectionRepository
	patternRepository    *repositories.PatternRepository
}
