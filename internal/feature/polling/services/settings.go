package services

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	pollingports "svc-registry/internal/feature/polling/ports"
	"svc-registry/internal/feature/polling/repositories"
	pollingtypes "svc-registry/internal/feature/polling/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/pkg/pgtx"
)

type Polling struct {
	Effective pollingtypes.Effective
	Own       pollingtypes.NodeSettings
	From      map[string]*pollingtypes.ChainNode
	Defaults  pollingtypes.Defaults
}

type SettingsService struct {
	db                *pgtx.DB
	catalog           *catalogservices.CatalogServices
	pollingRepository *repositories.PollingRepository
	ports             *PortsService
	replanners        []pollingports.ReplanPort
}

func (s *SettingsService) GetPolling(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (Polling, error) {
	if _, err := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermRead, id); err != nil {
		return Polling{}, err
	}
	return s.polling(ctx, id)
}

func (s *SettingsService) PutPolling(ctx context.Context, p accesstypes.Principal, id uuid.UUID, in pollingtypes.NodeSettings) (Polling, error) {
	if _, err := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermWrite, id); err != nil {
		return Polling{}, err
	}
	in, err := pollingtypes.Validate(in)
	if err != nil {
		return Polling{}, apperr.Wrap(err)
	}
	err = repositories.Err(s.db.InTx(ctx, func(ctx context.Context) error {
		if in.IsEmpty() {
			return s.pollingRepository.Delete(ctx, id)
		}
		return s.pollingRepository.Put(ctx, id, in)
	}))
	if err != nil {
		return Polling{}, apperr.Wrap(err)
	}
	for _, r := range s.replanners {
		if err := r.ReplanSubtree(ctx, id); err != nil {
			slog.Warn("polling settings saved, replanning failed", "node", id, "error", err)
		}
	}
	return s.polling(ctx, id)
}

func (s *SettingsService) polling(ctx context.Context, id uuid.UUID) (Polling, error) {
	chain, err := s.pollingRepository.Chain(ctx, id)
	if err != nil {
		return Polling{}, apperr.Wrap(err)
	}
	if len(chain) == 0 {
		return Polling{}, apperr.Wrap(apperr.ErrNotFound)
	}
	defaults := s.ports.Defaults()
	effective, from := pollingtypes.Merge(chain, defaults.TimeoutSecs)
	origin := func(i int) *pollingtypes.ChainNode {
		if i <= 0 {
			return nil
		}
		return &chain[i]
	}
	return Polling{Effective: effective, Own: chain[0].Own, Defaults: defaults, From: map[string]*pollingtypes.ChainNode{
		"interval_secs": origin(from.IntervalSecs), "timeout_secs": origin(from.TimeoutSecs), "mode": origin(from.Mode)}}, nil
}
