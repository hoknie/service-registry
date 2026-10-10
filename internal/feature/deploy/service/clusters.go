package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/feature/deploy"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/secretbox"
)

func (s *Service) ListClusters(ctx context.Context, p access.Principal) ([]deploy.Cluster, error) {
	if err := access.RequireSuperadmin(p); err != nil {
		return nil, err
	}
	items, err := s.clusters.List(ctx)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	return s.decorateAll(ctx, items)
}

func (s *Service) GetCluster(ctx context.Context, p access.Principal, id uuid.UUID) (deploy.Cluster, error) {
	if err := access.RequireSuperadmin(p); err != nil {
		return deploy.Cluster{}, err
	}
	c, err := s.loadCluster(ctx, id)
	if err != nil {
		return deploy.Cluster{}, err
	}
	return s.decorate(ctx, c)
}

func (s *Service) loadCluster(ctx context.Context, id uuid.UUID) (deploy.Cluster, error) {
	c, err := s.clusters.Get(ctx, id)
	if err != nil {
		return deploy.Cluster{}, apperr.Wrap(err)
	}
	if c == nil {
		return deploy.Cluster{}, apperr.New(apperr.NotFound)
	}
	return *c, nil
}

func (s *Service) CreateCluster(ctx context.Context, p access.Principal, in deploy.ClusterInput) (deploy.Cluster, error) {
	if err := access.RequireSuperadmin(p); err != nil {
		return deploy.Cluster{}, err
	}
	settings, secret, err := deploy.ValidateCluster(deploy.DefaultSettings(), false, in)
	if err != nil {
		return deploy.Cluster{}, apperr.Wrap(err)
	}
	if err := s.globalSecret(ctx, secret); err != nil {
		return deploy.Cluster{}, err
	}
	c, err := s.clusters.Insert(ctx, deploy.NewCluster{ID: uuid.Must(uuid.NewV7()), Settings: settings, SecretID: secret})
	if err != nil {
		return deploy.Cluster{}, apperr.Wrap(err)
	}
	return s.decorate(ctx, c)
}

func (s *Service) UpdateCluster(ctx context.Context, p access.Principal, id uuid.UUID, in deploy.ClusterInput) (deploy.Cluster, error) {
	if err := access.RequireSuperadmin(p); err != nil {
		return deploy.Cluster{}, err
	}
	current, err := s.loadCluster(ctx, id)
	if err != nil {
		return deploy.Cluster{}, err
	}
	settings, secret, err := deploy.ValidateCluster(current.Settings, current.Credentials != nil, in)
	if err != nil {
		return deploy.Cluster{}, apperr.Wrap(err)
	}
	if err := s.globalSecret(ctx, secret); err != nil {
		return deploy.Cluster{}, err
	}
	c, err := s.clusters.Update(ctx, id, deploy.ClusterUpdate{Settings: settings, SecretID: secret})
	if err != nil {
		return deploy.Cluster{}, apperr.Wrap(err)
	}
	if c == nil {
		return deploy.Cluster{}, apperr.New(apperr.NotFound)
	}
	return s.decorate(ctx, *c)
}

func (s *Service) DeleteCluster(ctx context.Context, p access.Principal, id uuid.UUID) error {
	if err := access.RequireSuperadmin(p); err != nil {
		return err
	}
	return apperr.Wrap(s.clusters.Delete(ctx, id))
}

func (s *Service) PollCluster(ctx context.Context, p access.Principal, id uuid.UUID) error {
	if err := access.RequireSuperadmin(p); err != nil {
		return err
	}
	return apperr.Wrap(s.clusters.ScheduleNow(ctx, id))
}

type ClusterTest struct {
	OK      bool
	Version *string
	Missing []string
	Error   *deploy.Failure
}

func (s *Service) TestCluster(ctx context.Context, p access.Principal, id uuid.UUID) (ClusterTest, error) {
	if err := access.RequireSuperadmin(p); err != nil {
		return ClusterTest{}, err
	}
	c, err := s.loadCluster(ctx, id)
	if err != nil {
		return ClusterTest{}, err
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

func (s *Service) UnmatchedWorkloads(ctx context.Context, p access.Principal, id uuid.UUID, q access.PageQuery) (access.Page[deploy.Unmatched], error) {
	if err := access.RequireSuperadmin(p); err != nil {
		return access.Page[deploy.Unmatched]{}, err
	}
	page, err := access.ValidatePage(q)
	if err != nil {
		return access.Page[deploy.Unmatched]{}, apperr.Wrap(err)
	}
	if _, err := s.loadCluster(ctx, id); err != nil {
		return access.Page[deploy.Unmatched]{}, err
	}
	items, total, err := s.workloads.Unmatched(ctx, id, page.Limit, page.Offset)
	if err != nil {
		return access.Page[deploy.Unmatched]{}, apperr.Wrap(err)
	}
	return access.Page[deploy.Unmatched]{Items: items, Total: total, Limit: page.Limit, Offset: page.Offset}, nil
}

func (s *Service) clusterClient(ctx context.Context, c deploy.Cluster) (deploy.ClusterClient, error) {
	a := deploy.Access{InCluster: c.InCluster}
	if !c.InCluster {
		a.APIURL = *c.APIURL
		if c.CAPEM != nil {
			a.CAPEM = []byte(*c.CAPEM)
		}
		stored, err := s.clusters.Stored(ctx, c.ID)
		if err != nil {
			return nil, apperr.Wrap(err)
		}
		switch {
		case stored == nil:
			return nil, apperr.New(apperr.NotFound)
		case stored.SecretID != nil:
			token, err := s.catalog.ResolveSecret(ctx, *stored.SecretID)
			if err != nil {
				return nil, &deploy.Failure{Code: deploy.FailUnauthorized, Message: "secret cannot be read"}
			}
			a.Token = token
		case stored.Ref != nil:
			token, err := config.ResolveSecretRef(*stored.Ref)
			if err != nil {
				return nil, &deploy.Failure{Code: deploy.FailUnauthorized, Message: "credentials reference: " + err.Error()}
			}
			a.Token = token
		case stored.Enc != nil:
			token, err := s.secrets.Open(*stored.Enc, secretbox.AAD(deploy.Table, c.ID.String(), deploy.ColumnCredentials))
			if err != nil {
				return nil, &deploy.Failure{Code: deploy.FailUnauthorized, Message: "stored token cannot be decrypted"}
			}
			a.Token = token
		}
	}
	return s.k8s.Client(a)
}

func asFailure(err error) *deploy.Failure {
	var f *deploy.Failure
	if errors.As(err, &f) {
		return f
	}
	return nil
}

func (s *Service) PrepareSecretRotation(ctx context.Context) (func(context.Context) error, int, error) {
	clusters, err := s.clusters.Secrets(ctx)
	if err != nil {
		return nil, 0, err
	}
	plain := make([]string, len(clusters))
	for i, c := range clusters {
		v, err := s.secrets.Open(c.Enc, secretbox.AAD(deploy.Table, c.ID.String(), deploy.ColumnCredentials))
		if err != nil {
			return nil, 0, err
		}
		plain[i] = v
	}
	apply := func(ctx context.Context) error {
		for i, c := range clusters {
			enc, err := s.secrets.Seal(plain[i], secretbox.AAD(deploy.Table, c.ID.String(), deploy.ColumnCredentials))
			if err != nil {
				return err
			}
			if err := s.clusters.ReplaceSecret(ctx, c.ID, enc); err != nil {
				return err
			}
		}
		return nil
	}
	return apply, len(clusters), nil
}

func (s *Service) globalSecret(ctx context.Context, secret *uuid.UUID) error {
	if secret == nil {
		return nil
	}
	_, err := s.catalog.SecretFor(ctx, nil, *secret)
	return err
}

func (s *Service) decorate(ctx context.Context, c deploy.Cluster) (deploy.Cluster, error) {
	items, err := s.decorateAll(ctx, []deploy.Cluster{c})
	if err != nil {
		return deploy.Cluster{}, err
	}
	return items[0], nil
}

func (s *Service) decorateAll(ctx context.Context, items []deploy.Cluster) ([]deploy.Cluster, error) {
	var ids []uuid.UUID
	for _, c := range items {
		if c.Credentials != nil && c.Credentials.SecretID != nil {
			ids = append(ids, *c.Credentials.SecretID)
		}
	}
	refs, err := s.catalog.SecretRefs(ctx, ids)
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

func (s *Service) SecretUsage(ctx context.Context, secrets []uuid.UUID) (map[uuid.UUID]int64, error) {
	return s.clusters.SecretUsage(ctx, secrets)
}
