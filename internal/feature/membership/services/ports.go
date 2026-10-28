package services

import (
	"context"

	"github.com/google/uuid"

	membershiptypes "svc-registry/internal/feature/membership/types"
	"svc-registry/internal/feature/membership/usecases"
	"svc-registry/internal/platform/apperr"
)

type PortsService struct {
	parts
	accept *usecases.AcceptInvitationUseCase
}

func (s *PortsService) PendingInvitation(ctx context.Context, hash [32]byte) (string, bool, error) {
	inv, err := s.invitations.ByHash(ctx, hash)
	if err != nil {
		return "", false, apperr.Wrap(err)
	}
	if inv == nil || inv.State() != membershiptypes.Pending {
		return "", false, nil
	}
	return inv.Email, true, nil
}

func (s *PortsService) AcceptInvitation(ctx context.Context, hash [32]byte, user uuid.UUID, email string) error {
	_, err := s.accept.Run(ctx, hash, usecases.Acceptor{ID: user, Email: email})
	return apperr.Wrap(err)
}
