package types

import (
	"net/url"
	"strings"

	"github.com/google/uuid"

	"svc-registry/internal/platform/apperr"
)

const (
	ProxyDirect      = "direct"
	ProxyEnvironment = "environment"
	ProxyCustom      = "proxy"

	OwnerOrganization = "organization"
	OwnerFolder       = "folder"
	OwnerServer       = "server"
)

type Proxy struct {
	Mode       string
	URL        *string
	NoProxy    *string
	SecretID   *uuid.UUID
	SecretName *string
	SecretFrom *Ref
}

type ProxyOwner struct {
	Kind string
	ID   uuid.UUID
	Name string
}

type ProxyLink struct {
	Owner ProxyOwner
	Proxy *Proxy
}

type ProxySetting struct {
	Own       *Proxy
	Effective Proxy
	From      *ProxyOwner
}

func CleanProxy(p Proxy) (Proxy, error) {
	var bad []apperr.Field
	switch p.Mode {
	case ProxyDirect, ProxyEnvironment:
		if p.URL != nil || p.NoProxy != nil || p.SecretID != nil {
			bad = append(bad, apperr.Field{Path: "mode", Code: "fields_not_allowed"})
		}
	case ProxyCustom:
		if p.URL == nil || !validProxyURL(*p.URL) {
			bad = append(bad, apperr.Field{Path: "url", Code: "invalid"})
		}
		if p.NoProxy != nil {
			v := strings.TrimSpace(*p.NoProxy)
			if len(v) > 2000 {
				bad = append(bad, apperr.Field{Path: "no_proxy", Code: "too_long"})
			}
			if v == "" {
				p.NoProxy = nil
			} else {
				p.NoProxy = &v
			}
		}
	default:
		bad = append(bad, apperr.Field{Path: "mode", Code: "invalid"})
	}
	if len(bad) > 0 {
		return Proxy{}, InvalidProxy(bad...)
	}
	return p, nil
}

func validProxyURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.User != nil || u.Hostname() == "" || u.Port() == "" || len(raw) > 2000 {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && (u.Path == "" || u.Path == "/") && u.RawQuery == ""
}

func NearestProxy(chain []ProxyLink, useEnvironment bool) (Proxy, *ProxyOwner) {
	for _, l := range chain {
		if l.Proxy != nil {
			owner := l.Owner
			return *l.Proxy, &owner
		}
	}
	if useEnvironment {
		return Proxy{Mode: ProxyEnvironment}, nil
	}
	return Proxy{Mode: ProxyDirect}, nil
}
