package service

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/platform/auth"
)

func (s *Service) VerifyProjectKey(ctx context.Context, projectID uuid.UUID, presented string) (*uuid.UUID, error) {
	hash, ok := auth.ParseProjectKey(presented)
	if !ok {
		return nil, nil
	}
	return s.projectKeys.Verify(ctx, projectID, hash)
}
