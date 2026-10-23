package services

import (
	"context"
	"errors"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogservices "svc-registry/internal/feature/catalog/services"
	deployports "svc-registry/internal/feature/deploy/ports"
	"svc-registry/internal/feature/deploy/repositories"
	deploytypes "svc-registry/internal/feature/deploy/types"
	topologyservices "svc-registry/internal/feature/topology/services"
	"svc-registry/internal/platform/apperr"
	"svc-registry/pkg/pgtx"
	"svc-registry/pkg/secretbox"
)

func (s *ClustersService) ListClusters(ctx context.Context, p accesstypes.Principal) ([]deploytypes.Cluster, error) {
	if err := accesstypes.RequireSuperadmin(p); err != nil {
		return nil, err
	}
	items, err := s.clusterRepository.List(ctx)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	return s.decorateAll(ctx, items)
}

func (s *ClustersService) GetCluster(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (deploytypes.Cluster, error) {
	if err := accesstypes.RequireSuperadmin(p); err != nil {
		return deploytypes.Cluster{}, err
	}
	c, err := s.loadCluster(ctx, id)
	if err != nil {
		return deploytypes.Cluster{}, err
	}
	return s.decorate(ctx, c)
}

func (s *ClustersService) loadCluster(ctx context.Context, id uuid.UUID) (deploytypes.Cluster, error) {
	c, err := s.clusterRepository.Get(ctx, id)
	if err != nil {
		return deploytypes.Cluster{}, apperr.Wrap(err)
	}
	if c == nil {
		return deploytypes.Cluster{}, apperr.New(apperr.NotFound)
	}
	return *c, nil
}

func (s *ClustersService) CreateCluster(ctx context.Context, p accesstypes.Principal, in deploytypes.ClusterInput) (deploytypes.Cluster, *IssuedAgentToken, error) {
	if err := accesstypes.RequireSuperadmin(p); err != nil {
		return deploytypes.Cluster{}, nil, err
	}
	settings, secret, err := deploytypes.ValidateCluster(deploytypes.DefaultSettings(), false, in)
	if err != nil {
		return deploytypes.Cluster{}, nil, apperr.Wrap(err)
	}
	if err := s.globalSecret(ctx, secret); err != nil {
		return deploytypes.Cluster{}, nil, err
	}
	id := uuid.Must(uuid.NewV7())
	issued, err := s.saveCluster(ctx, deploytypes.ModePoll, settings.Mode, id, func(ctx context.Context) error {
		_, err := s.clusterRepository.Insert(ctx, deploytypes.NewCluster{ID: id, Settings: settings, SecretID: secret})
		return err
	})
	if err != nil {
		return deploytypes.Cluster{}, nil, err
	}
	c, err := s.loadCluster(ctx, id)
	if err != nil {
		return deploytypes.Cluster{}, nil, err
	}
	c, err = s.decorate(ctx, c)
	return c, issued, err
}

func (s *ClustersService) UpdateCluster(ctx context.Context, p accesstypes.Principal, id uuid.UUID, in deploytypes.ClusterInput) (deploytypes.Cluster, *IssuedAgentToken, error) {
	if err := accesstypes.RequireSuperadmin(p); err != nil {
		return deploytypes.Cluster{}, nil, err
	}
	current, err := s.loadCluster(ctx, id)
	if err != nil {
		return deploytypes.Cluster{}, nil, err
	}
	settings, secret, err := deploytypes.ValidateCluster(current.Settings, current.Credentials != nil, in)
	if err != nil {
		return deploytypes.Cluster{}, nil, apperr.Wrap(err)
	}
	if err := s.globalSecret(ctx, secret); err != nil {
		return deploytypes.Cluster{}, nil, err
	}
	issued, err := s.saveCluster(ctx, current.Mode, settings.Mode, id, func(ctx context.Context) error {
		c, err := s.clusterRepository.Update(ctx, id, deploytypes.ClusterUpdate{Settings: settings, SecretID: secret})
		if err == nil && c == nil {
			return apperr.New(apperr.NotFound)
		}
		return err
	})
	if err != nil {
		return deploytypes.Cluster{}, nil, err
	}
	c, err := s.loadCluster(ctx, id)
	if err != nil {
		return deploytypes.Cluster{}, nil, err
	}
	c, err = s.decorate(ctx, c)
	return c, issued, err
}

func (s *ClustersService) saveCluster(ctx context.Context, from, to deploytypes.Mode, id uuid.UUID, save func(context.Context) error) (*IssuedAgentToken, error) {
	var issued *IssuedAgentToken
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		if err := save(ctx); err != nil {
			return err
		}
		switch {
		case to == deploytypes.ModeAgent && from != to:
			tok, err := s.tokens.issue(ctx, id, 0)
			issued = &tok
			return err
		case to == deploytypes.ModePoll && from == deploytypes.ModeAgent:
			return s.tokens.agentTokenRepository.RevokeAll(ctx, id)
		}
		return nil
	})
	return issued, apperr.Wrap(err)
}

func (s *ClustersService) DeleteCluster(ctx context.Context, p accesstypes.Principal, id uuid.UUID) error {
	if err := accesstypes.RequireSuperadmin(p); err != nil {
		return err
	}
	return apperr.Wrap(s.db.InTx(ctx, func(ctx context.Context) error {
		if err := s.topology.Records.ClearCluster(ctx, id); err != nil {
			return err
		}
		return s.clusterRepository.Delete(ctx, id)
	}))
}

func (s *ClustersService) PollCluster(ctx context.Context, p accesstypes.Principal, id uuid.UUID) error {
	if err := accesstypes.RequireSuperadmin(p); err != nil {
		return err
	}
	c, err := s.loadCluster(ctx, id)
	if err != nil {
		return err
	}
	if c.Mode == deploytypes.ModeAgent {
		return deploytypes.ConflictClusterModeAgent
	}
	return apperr.Wrap(s.clusterRepository.ScheduleNow(ctx, id))
}

type ClusterTest struct {
	OK      bool
	Version *string
	Missing []string
	Error   *deploytypes.Failure
}

func (s *ClustersService) TestCluster(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (ClusterTest, error) {
	if err := accesstypes.RequireSuperadmin(p); err != nil {
		return ClusterTest{}, err
	}
	c, err := s.loadCluster(ctx, id)
	if err != nil {
		return ClusterTest{}, err
	}
	if c.Agent != nil {
		return reportedTest(c.Agent.Report), nil
	}
	client, err := s.clusterClient(ctx, c)
	if f := asFailure(err); f != nil {
		return ClusterTest{Missing: []string{}, Error: f}, nil
	} else if err != nil {
		return ClusterTest{}, err
	}
	version, err := client.Version(ctx)
	if f := asFailure(err); f != nil {
		return ClusterTest{Missing: []string{}, Error: f}, nil
	}
	missing, err := client.Missing(ctx, c.Namespaces)
	if f := asFailure(err); f != nil {
		return ClusterTest{Version: &version, Missing: []string{}, Error: f}, nil
	}
	return ClusterTest{OK: true, Version: &version, Missing: missing}, nil
}

func reportedTest(r *deploytypes.ReportSummary) ClusterTest {
	if r == nil {
		return ClusterTest{Missing: []string{}, Error: &deploytypes.Failure{Code: deploytypes.FailAgentNeverReported,
			Message: "the agent has not reported yet"}}
	}
	return ClusterTest{OK: r.Error == nil, Version: r.KubernetesVersion, Missing: r.Missing, Error: r.Error.Failure()}
}

func (s *ClustersService) UnmatchedWorkloads(ctx context.Context, p accesstypes.Principal, id uuid.UUID, q accesstypes.PageQuery) (accesstypes.Page[deploytypes.Unmatched], error) {
	if err := accesstypes.RequireSuperadmin(p); err != nil {
		return accesstypes.Page[deploytypes.Unmatched]{}, err
	}
	page, err := accesstypes.ValidatePage(q)
	if err != nil {
		return accesstypes.Page[deploytypes.Unmatched]{}, apperr.Wrap(err)
	}
	if _, err := s.loadCluster(ctx, id); err != nil {
		return accesstypes.Page[deploytypes.Unmatched]{}, err
	}
	items, total, err := s.workloadRepository.Unmatched(ctx, id, page.Limit, page.Offset)
	if err != nil {
		return accesstypes.Page[deploytypes.Unmatched]{}, apperr.Wrap(err)
	}
	return accesstypes.Page[deploytypes.Unmatched]{Items: items, Total: total, Limit: page.Limit, Offset: page.Offset}, nil
}

func (s *ClustersService) clusterClient(ctx context.Context, c deploytypes.Cluster) (deployports.ClusterClientPort, error) {
	a := deploytypes.Access{InCluster: c.InCluster}
	if !c.InCluster {
		a.APIURL = *c.APIURL
		if c.CAPEM != nil {
			a.CAPEM = []byte(*c.CAPEM)
		}
		stored, err := s.clusterRepository.Stored(ctx, c.ID)
		if err != nil {
			return nil, apperr.Wrap(err)
		}
		switch {
		case stored == nil:
			return nil, apperr.New(apperr.NotFound)
		case stored.SecretID != nil:
			token, err := s.catalog.SecretResolver.ResolveSecret(ctx, *stored.SecretID)
			if err != nil {
				return nil, &deploytypes.Failure{Code: deploytypes.FailUnauthorized, Message: "secret cannot be read"}
			}
			a.Token = token
		}
	}
	return s.k8s.Client(a)
}

func asFailure(err error) *deploytypes.Failure {
	if f, ok := errors.AsType[*deploytypes.Failure](err); ok {
		return f
	}
	return nil
}

func (s *ClustersService) globalSecret(ctx context.Context, secret *uuid.UUID) error {
	if secret == nil {
		return nil
	}
	_, err := s.catalog.SecretResolver.SecretFor(ctx, nil, *secret)
	return err
}

func (s *ClustersService) decorate(ctx context.Context, c deploytypes.Cluster) (deploytypes.Cluster, error) {
	items, err := s.decorateAll(ctx, []deploytypes.Cluster{c})
	if err != nil {
		return deploytypes.Cluster{}, err
	}
	return items[0], nil
}

func (s *ClustersService) decorateAll(ctx context.Context, items []deploytypes.Cluster) ([]deploytypes.Cluster, error) {
	var ids []uuid.UUID
	for _, c := range items {
		if c.Credentials != nil && c.Credentials.SecretID != nil {
			ids = append(ids, *c.Credentials.SecretID)
		}
	}
	refs, err := s.catalog.SecretResolver.SecretRefs(ctx, ids)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	for i := range items {
		if cr := items[i].Credentials; cr != nil && cr.SecretID != nil {
			if ref, ok := refs[*cr.SecretID]; ok {
				cr.Secret = &ref
			}
		}
	}
	return items, nil
}

type ClustersService struct {
	catalog            *catalogservices.CatalogServices
	clusterRepository  *repositories.ClusterRepository
	db                 *pgtx.DB
	tokens             *AgentTokensService
	k8s                deployports.ClientFactoryPort
	secrets            *secretbox.Box
	topology           *topologyservices.TopologyServices
	workloadRepository *repositories.WorkloadRepository
}
