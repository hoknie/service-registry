package services

import (
	"context"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/feature/deploy/repositories"
	deploytypes "svc-registry/internal/feature/deploy/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/auth"
)

type IssuedAgentToken struct {
	Token  deploytypes.AgentToken
	Secret string
}

func (s *AgentTokensService) RotateAgentToken(ctx context.Context, p accesstypes.Principal, id uuid.UUID, graceSecs *int64) (IssuedAgentToken, error) {
	if err := s.agentCluster(ctx, p, id); err != nil {
		return IssuedAgentToken{}, err
	}
	grace, err := catalogtypes.ValidateGrace(graceSecs)
	if err != nil {
		return IssuedAgentToken{}, apperr.Wrap(err)
	}
	return s.issue(ctx, id, grace)
}

func (s *AgentTokensService) RevokeAgentTokens(ctx context.Context, p accesstypes.Principal, id uuid.UUID) error {
	if err := s.agentCluster(ctx, p, id); err != nil {
		return err
	}
	return apperr.Wrap(s.agentTokenRepository.RevokeAll(ctx, id))
}

func (s *AgentTokensService) agentCluster(ctx context.Context, p accesstypes.Principal, id uuid.UUID) error {
	if err := accesstypes.RequireSuperadmin(p); err != nil {
		return err
	}
	c, err := s.clusterRepository.Get(ctx, id)
	switch {
	case err != nil:
		return apperr.Wrap(err)
	case c == nil:
		return apperr.New(apperr.NotFound)
	case c.Mode != deploytypes.ModeAgent:
		return deploytypes.ConflictClusterModePoll
	}
	return nil
}

func (s *AgentTokensService) issue(ctx context.Context, clusterID uuid.UUID, graceSecs uint64) (IssuedAgentToken, error) {
	fresh, err := auth.GenerateAgentToken()
	if err != nil {
		return IssuedAgentToken{}, apperr.Internalf("no randomness: %v", err)
	}
	tok, err := s.agentTokenRepository.Issue(ctx, deploytypes.NewAgentToken{
		ID: uuid.Must(uuid.NewV7()), ClusterID: clusterID, Prefix: fresh.Prefix, Hash: fresh.Hash,
	}, graceSecs)
	if err != nil {
		return IssuedAgentToken{}, apperr.Wrap(err)
	}
	return IssuedAgentToken{Token: tok, Secret: fresh.Secret}, nil
}

type AgentTokensService struct {
	agentTokenRepository *repositories.AgentTokenRepository
	clusterRepository    *repositories.ClusterRepository
}
