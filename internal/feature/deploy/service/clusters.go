package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/feature/deploy"
	"svc-registry/internal/feature/forge"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/secretbox"
)

func (s *Service) ListClusters(ctx context.Context, p access.Principal) ([]deploy.Cluster, error) {
	if err := access.RequireSuperadmin(p); err != nil {
		return nil, err
	}
	items, err := s.clusters.List(ctx)
	return items, apperr.Wrap(err)
}

func (s *Service) GetCluster(ctx context.Context, p access.Principal, id uuid.UUID) (deploy.Cluster, error) {
	if err := access.RequireSuperadmin(p); err != nil {
		return deploy.Cluster{}, err
	}
	return s.loadCluster(ctx, id)
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

func (s *Service) sealCredentials(id uuid.UUID, c forge.ValidCredentials) (forge.StoredCredentials, error) {
	if c.Ref != nil {
		return forge.StoredCredentials{Ref: c.Ref}, nil
	}
	if !s.secrets.CanEncrypt() {
		return forge.StoredCredentials{}, apperr.Wrap(forge.ConflictSecretsKeyMissing)
	}
	enc, err := s.secrets.Seal(*c.Token, secretbox.AAD(deploy.Table, id.String(), deploy.ColumnCredentials))
	if err != nil {
		return forge.StoredCredentials{}, apperr.Internalf("encrypt: %v", err)
	}
	fp := secretbox.Fingerprint(*c.Token)
	return forge.StoredCredentials{Enc: &enc, Fingerprint: &fp}, nil
}

func (s *Service) CreateCluster(ctx context.Context, p access.Principal, in deploy.ClusterInput) (deploy.Cluster, error) {
	if err := access.RequireSuperadmin(p); err != nil {
		return deploy.Cluster{}, err
	}
	settings, creds, err := deploy.ValidateCluster(deploy.DefaultSettings(), false, in)
	if err != nil {
		return deploy.Cluster{}, apperr.Wrap(err)
	}
	id := uuid.Must(uuid.NewV7())
	var stored forge.StoredCredentials
	if creds != nil {
		if stored, err = s.sealCredentials(id, *creds); err != nil {
			return deploy.Cluster{}, err
		}
	}
	c, err := s.clusters.Insert(ctx, deploy.NewCluster{ID: id, Settings: settings, Credentials: stored})
	return c, apperr.Wrap(err)
}

func (s *Service) UpdateCluster(ctx context.Context, p access.Principal, id uuid.UUID, in deploy.ClusterInput) (deploy.Cluster, error) {
	if err := access.RequireSuperadmin(p); err != nil {
		return deploy.Cluster{}, err
	}
	current, err := s.loadCluster(ctx, id)
	if err != nil {
		return deploy.Cluster{}, err
	}
	settings, creds, err := deploy.ValidateCluster(current.Settings, current.Credentials != nil, in)
	if err != nil {
		return deploy.Cluster{}, apperr.Wrap(err)
	}
	u := deploy.ClusterUpdate{Settings: settings}
	if creds != nil {
		stored, err := s.sealCredentials(id, *creds)
		if err != nil {
			return deploy.Cluster{}, err
		}
		u.Credentials = &stored
	}
	c, err := s.clusters.Update(ctx, id, u)
	if err != nil {
		return deploy.Cluster{}, apperr.Wrap(err)
	}
	if c == nil {
		return deploy.Cluster{}, apperr.New(apperr.NotFound)
	}
	return *c, nil
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
