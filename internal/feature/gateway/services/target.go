package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/google/uuid"

	catalogservices "svc-registry/internal/feature/catalog/services"
	gatewayports "svc-registry/internal/feature/gateway/ports"
	"svc-registry/internal/feature/gateway/repositories"
	gatewaytypes "svc-registry/internal/feature/gateway/types"
	"svc-registry/internal/platform/config"
)

type targets struct {
	proxies *repositories.ProxyRepository
	catalog *catalogservices.CatalogServices
	cfg     config.MCPGatewayConfig
}

type built struct {
	target  gatewayports.Target
	secrets []string
}

func (t *targets) of(ctx context.Context, s gatewaytypes.Server) (built, error) {
	out := built{target: gatewayports.Target{ServerID: s.ID, URL: s.URL, Transport: s.Transport, Timeout: t.cfg.CallTimeout(),
		MaxBytes: t.cfg.MaxResultBytes}}
	if s.Auth.SecretID != nil {
		value, err := t.secret(ctx, *s.Auth.SecretID)
		if err != nil {
			return out, err
		}
		out.secrets = append(out.secrets, value)
		switch s.Auth.Kind {
		case gatewaytypes.AuthBearer:
			out.target.AuthHeader, out.target.AuthValue = "Authorization", "Bearer "+value
		case gatewaytypes.AuthHeader:
			out.target.AuthHeader, out.target.AuthValue = s.Auth.Header, value
		}
	}
	chain, err := t.proxies.Chain(ctx, repositories.ProxyStart{Server: &s.ID})
	if err != nil {
		return out, err
	}
	proxy, _ := gatewaytypes.NearestProxy(chain, t.cfg.UseProxy)
	out.target.Proxy = proxy
	if proxy.SecretID != nil {
		value, err := t.secret(ctx, *proxy.SecretID)
		if err != nil {
			return out, err
		}
		out.secrets = append(out.secrets, value)
		out.target.ProxyAuth = value
	}
	out.target.Version = version(out.target)
	return out, nil
}

func (t *targets) secret(ctx context.Context, id uuid.UUID) (string, error) {
	value, err := t.catalog.SecretResolver.ResolveSecret(ctx, id)
	if err != nil || value == "" {
		return "", gatewaytypes.Fail(gatewaytypes.FailSecretUnavailable, "the secret cannot be read")
	}
	return strings.TrimSpace(value), nil
}

func version(t gatewayports.Target) string {
	h := sha256.New()
	for _, part := range []string{t.URL, t.Transport, t.AuthHeader, t.AuthValue, t.Proxy.Mode, deref(t.Proxy.URL),
		deref(t.Proxy.NoProxy), t.ProxyAuth} {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
