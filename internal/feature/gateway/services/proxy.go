package services

import (
	"context"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/feature/gateway/repositories"
	gatewaytypes "svc-registry/internal/feature/gateway/types"
	"svc-registry/internal/platform/apperr"
)

type ProxyService struct{ parts }

func (s *ProxyService) place(ctx context.Context, p accesstypes.Principal, kind string, id uuid.UUID) (repositories.ProxyStart, gatewaytypes.Scope, error) {
	switch kind {
	case gatewaytypes.OwnerOrganization:
		scope := gatewaytypes.Scope{OrganizationID: &id}
		ok, err := s.folders.IsOrganization(ctx, id)
		if err != nil {
			return repositories.ProxyStart{}, scope, apperr.Wrap(err)
		}
		if !ok || s.access.canRead(ctx, p, scope) != nil {
			return repositories.ProxyStart{}, scope, apperr.New(apperr.NotFound)
		}
		return repositories.ProxyStart{Organization: &id}, scope, s.access.canManage(ctx, p, scope)
	case gatewaytypes.OwnerFolder:
		chain, err := folderChain(ctx, s.parts, id)
		if err != nil {
			return repositories.ProxyStart{}, gatewaytypes.Scope{}, err
		}
		scope := gatewaytypes.Scope{OrganizationID: chain[0].OrganizationID}
		if err := s.access.canRead(ctx, p, scope); err != nil {
			return repositories.ProxyStart{}, scope, err
		}
		return repositories.ProxyStart{Folder: &id}, scope, s.access.canWrite(ctx, p, scope, accesstypes.ActionUpdate)
	}
	srv, err := writeServer(ctx, s.parts, p, id, accesstypes.ActionUpdate)
	return repositories.ProxyStart{Server: &id}, srv.Scope(), err
}

func (s *ProxyService) Proxy(ctx context.Context, p accesstypes.Principal, kind string, id uuid.UUID) (gatewaytypes.ProxySetting, error) {
	start, _, err := s.place(ctx, p, kind, id)
	if err != nil {
		return gatewaytypes.ProxySetting{}, err
	}
	return s.setting(ctx, start)
}

func (s *ProxyService) setting(ctx context.Context, start repositories.ProxyStart) (gatewaytypes.ProxySetting, error) {
	chain, err := s.proxies.Chain(ctx, start)
	if err != nil {
		return gatewaytypes.ProxySetting{}, apperr.Wrap(err)
	}
	out := gatewaytypes.ProxySetting{}
	if len(chain) > 0 {
		out.Own = chain[0].Proxy
	}
	out.Effective, out.From = gatewaytypes.NearestProxy(chain, s.cfg.UseProxy)
	return out, nil
}

func (s *ProxyService) PutProxy(ctx context.Context, p accesstypes.Principal, kind string, id uuid.UUID, in gatewaytypes.Proxy) (gatewaytypes.ProxySetting, error) {
	start, scope, err := s.place(ctx, p, kind, id)
	if err != nil {
		return gatewaytypes.ProxySetting{}, err
	}
	proxy, err := gatewaytypes.CleanProxy(in)
	if err != nil {
		return gatewaytypes.ProxySetting{}, apperr.Wrap(err)
	}
	if proxy.SecretID != nil {
		if _, err := s.catalog.SecretResolver.SecretFor(ctx, scope.OrganizationID, *proxy.SecretID); err != nil {
			return gatewaytypes.ProxySetting{}, err
		}
	}
	rowID, err := uuid.NewV7()
	if err != nil {
		return gatewaytypes.ProxySetting{}, apperr.Wrap(err)
	}
	err = s.db.InTx(ctx, func(ctx context.Context) error {
		if _, err := s.proxies.Delete(ctx, start); err != nil {
			return err
		}
		if err := s.proxies.Insert(ctx, rowID, start, proxy); err != nil {
			return err
		}
		return s.proxies.Requeue(ctx, start)
	})
	if err != nil {
		return gatewaytypes.ProxySetting{}, apperr.Wrap(repositories.Err(err))
	}
	return s.setting(ctx, start)
}

func (s *ProxyService) DeleteProxy(ctx context.Context, p accesstypes.Principal, kind string, id uuid.UUID) error {
	start, _, err := s.place(ctx, p, kind, id)
	if err != nil {
		return err
	}
	return apperr.Wrap(s.db.InTx(ctx, func(ctx context.Context) error {
		removed, err := s.proxies.Delete(ctx, start)
		if err != nil || !removed {
			return err
		}
		return s.proxies.Requeue(ctx, start)
	}))
}
