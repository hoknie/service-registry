package services

import (
	"context"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	gatewaytypes "svc-registry/internal/feature/gateway/types"
	"svc-registry/internal/feature/gateway/usecases"
	"svc-registry/internal/platform/apperr"
)

type ChecksService struct {
	parts
	check *usecases.CheckServerUseCase
}

func (s *ChecksService) CheckNow(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (gatewaytypes.Server, error) {
	srv, err := writeServer(ctx, s.parts, p, id, accesstypes.ActionUpdate)
	if err != nil {
		return gatewaytypes.Server{}, err
	}
	taken, err := s.checks.TakeNow(ctx, id, int(s.cfg.CallTimeoutSecs)+30)
	if err != nil {
		return gatewaytypes.Server{}, apperr.Wrap(err)
	}
	if !taken {
		return gatewaytypes.Server{}, gatewaytypes.CheckRateLimited
	}
	if err := s.run(ctx, srv); err != nil {
		return gatewaytypes.Server{}, apperr.Wrap(err)
	}
	return readServer(ctx, s.parts, p, id)
}

func (s *ChecksService) ClaimChecks(ctx context.Context, limit, leaseSecs int) ([]uuid.UUID, error) {
	ids, err := s.checks.Claim(ctx, limit, leaseSecs)
	return ids, apperr.Wrap(err)
}

func (s *ChecksService) ExtendCheck(ctx context.Context, id uuid.UUID, leaseSecs int) error {
	return apperr.Wrap(s.checks.Extend(ctx, id, leaseSecs))
}

func (s *ChecksService) RunCheck(ctx context.Context, id uuid.UUID) error {
	srv, err := s.servers.Get(ctx, id)
	if err != nil || srv == nil {
		if err == nil {
			return nil
		}
		return apperr.Wrap(err)
	}
	if srv.Status == gatewaytypes.StatusDisabled {
		return apperr.Wrap(s.checks.Release(ctx, id))
	}
	return apperr.Wrap(s.run(ctx, *srv))
}

func (s *ChecksService) run(ctx context.Context, srv gatewaytypes.Server) error {
	b, err := s.targets.of(ctx, srv)
	_, err = s.check.Run(ctx, usecases.CheckInput{ServerID: srv.ID, Target: b.target, TargetErr: err, Secrets: b.secrets})
	return err
}
