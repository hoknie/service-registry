package services

import (
	"context"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/feature/gateway/repositories"
	gatewaytypes "svc-registry/internal/feature/gateway/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/metrics"
)

type PortsService struct{ parts }

func (s *PortsService) SecretUsage(ctx context.Context, secrets []uuid.UUID) (map[uuid.UUID]int64, error) {
	out, err := s.servers.UsingSecrets(ctx, secrets)
	return out, apperr.Wrap(err)
}

func (s *PortsService) VisibleServers(ctx context.Context, owner accesstypes.Principal, ids []uuid.UUID) (bool, error) {
	if len(ids) == 0 {
		return true, nil
	}
	read, write, err := s.access.visible(ctx, owner)
	if err != nil {
		return false, err
	}
	found, err := s.servers.List(ctx, repositories.ServerFilter{Read: read, Write: write, IDs: ids})
	if err != nil {
		return false, apperr.Wrap(err)
	}
	return len(found) == len(ids), nil
}

func (s *PortsService) DescribeServers(ctx context.Context, ids []uuid.UUID) ([]accesstypes.ServerRef, error) {
	if len(ids) == 0 {
		return []accesstypes.ServerRef{}, nil
	}
	refs, err := s.organizations.Refs(ctx, ids)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	out := make([]accesstypes.ServerRef, len(refs))
	for i, r := range refs {
		out[i] = accesstypes.ServerRef{ID: r.ID, Slug: r.Slug, Title: r.Title, OrganizationID: r.OrganizationID}
	}
	return out, nil
}

func (s *PortsService) PruneCalls(ctx context.Context) (int64, error) {
	n, err := s.calls.Prune(ctx, int(s.cfg.CallLogRetention().Seconds()))
	return n, apperr.Wrap(err)
}

var metricShapes = map[string]metrics.Shape{
	"servers": {Name: "svcr_mcp_gateway_servers", Labels: []string{"state"}},
	"info":    {Name: "svcr_mcp_gateway_server_info", Labels: []string{"server_id", "slug"}},
}

func (s *PortsService) MetricSamples(ctx context.Context) ([]metrics.Sample, error) {
	counts, err := s.checks.MetricCounts(ctx)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	return metrics.FromPairs(metrics.Zero(counts, "servers:ok", "servers:failing", "servers:unknown", "servers:disabled"), metricShapes), nil
}

func (s *PortsService) ListServers(ctx context.Context, owner accesstypes.Principal) ([]accesstypes.ServerRef, error) {
	read, write, err := s.access.visible(ctx, owner)
	if err != nil {
		return nil, err
	}
	found, err := s.servers.List(ctx, repositories.ServerFilter{Read: read, Write: write})
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	out := make([]accesstypes.ServerRef, 0, len(found))
	for _, srv := range found {
		if srv.Status != gatewaytypes.StatusDisabled {
			out = append(out, accesstypes.ServerRef{ID: srv.ID, Slug: srv.Slug, Title: srv.Title, OrganizationID: srv.OrganizationID})
		}
	}
	return out, nil
}
