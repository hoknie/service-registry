package config

import (
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

type OAuthConfig struct {
	PasswordLogin string          `env:"AUTH_PASSWORD_LOGIN" envDefault:"all"`
	ProvidersRaw  string          `env:"OAUTH_PROVIDERS"`
	Providers     []OAuthProvider `env:"-"`
}

const (
	OAuthOIDC   = "oidc"
	OAuthGitLab = "gitlab"
)

type OAuthProvider struct {
	Key            string
	Kind           string
	Issuer         string
	ClientID       string
	ClientSecret   string
	DisplayName    string
	Scopes         []string
	AutoProvision  bool
	AllowedDomains []string
	GroupsClaim    string
	GroupMap       []GroupRule
}

type GroupRule struct {
	Value string
	Group string
}

func (p OAuthProvider) ResolveSecret() (string, error) {
	if strings.HasPrefix(p.ClientSecret, "env:") || strings.HasPrefix(p.ClientSecret, "file:") {
		return ResolveSecretRef(p.ClientSecret)
	}
	return p.ClientSecret, nil
}

var providerKeyRe = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,30}[a-z0-9])?$`)

func (c *OAuthConfig) check() Errors {
	switch c.PasswordLogin = strings.TrimSpace(c.PasswordLogin); c.PasswordLogin {
	case "all", "superadmins", "off":
		return nil
	}
	return Errors{{Var: "AUTH_PASSWORD_LOGIN", Reason: `must be "all", "superadmins" or "off"`}}
}

func (c *OAuthConfig) checkEnv(vars Env) Errors {
	var errs Errors
	get := func(name string) string { return strings.TrimSpace(vars[name]) }
	c.Providers = nil
	seen := map[string]bool{}
	for _, raw := range strings.Split(c.ProvidersRaw, ",") {
		key := strings.TrimSpace(raw)
		if key == "" {
			continue
		}
		if !providerKeyRe.MatchString(key) || seen[key] {
			errs = append(errs, &Error{Var: "OAUTH_PROVIDERS", Reason: "must be distinct keys of 1 to 32 characters a-z, 0-9, '-'"})
			continue
		}
		seen[key] = true
		prefix := "OAUTH_" + strings.ToUpper(strings.ReplaceAll(key, "-", "_")) + "_"
		p := OAuthProvider{Key: key, Kind: get(prefix + "KIND"), Issuer: strings.TrimSuffix(get(prefix+"ISSUER"), "/"),
			ClientID: get(prefix + "CLIENT_ID"), ClientSecret: get(prefix + "CLIENT_SECRET"),
			DisplayName: get(prefix + "DISPLAY_NAME"), GroupsClaim: get(prefix + "GROUPS_CLAIM")}
		switch p.Kind {
		case OAuthOIDC:
		case OAuthGitLab:
			if p.Issuer == "" {
				p.Issuer = "https://gitlab.com"
			}
		default:
			errs = append(errs, &Error{Var: prefix + "KIND", Reason: `must be "oidc" or "gitlab"`})
		}
		if !issuerOK(p.Issuer) {
			errs = append(errs, &Error{Var: prefix + "ISSUER", Reason: "must be an https URL (http only for localhost)"})
		}
		if p.ClientID == "" {
			errs = append(errs, &Error{Var: prefix + "CLIENT_ID", Reason: "is required"})
		}
		if p.ClientSecret == "" {
			errs = append(errs, &Error{Var: prefix + "CLIENT_SECRET", Reason: "is required"})
		}
		if p.DisplayName == "" {
			p.DisplayName = key
		}
		if n := len([]rune(p.DisplayName)); n > 64 {
			errs = append(errs, &Error{Var: prefix + "DISPLAY_NAME", Reason: "must be at most 64 characters"})
		}
		p.Scopes = fields(get(prefix + "SCOPES"))
		if len(p.Scopes) == 0 {
			p.Scopes = []string{"openid", "profile", "email"}
			if p.Kind == OAuthGitLab {
				p.Scopes = append(p.Scopes, "read_user")
			}
		}
		if !slices.Contains(p.Scopes, "openid") {
			errs = append(errs, &Error{Var: prefix + "SCOPES", Reason: `must include "openid"`})
		}
		switch v := strings.ToLower(get(prefix + "AUTO_PROVISION")); v {
		case "", "false", "0":
		case "true", "1":
			p.AutoProvision = true
		default:
			errs = append(errs, &Error{Var: prefix + "AUTO_PROVISION", Reason: "must be true or false"})
		}
		for _, d := range fields(get(prefix + "ALLOWED_DOMAINS")) {
			p.AllowedDomains = append(p.AllowedDomains, strings.ToLower(strings.TrimPrefix(d, "@")))
		}
		if p.GroupsClaim == "" {
			p.GroupsClaim = "groups"
			if p.Kind == OAuthGitLab {
				p.GroupsClaim = "groups_direct"
			}
		}
		rules, ok := groupRules(get(prefix + "GROUP_MAP"))
		if !ok {
			errs = append(errs, &Error{Var: prefix + "GROUP_MAP", Reason: `must be rules "value=>group" separated by ';' (or "*")`})
		}
		p.GroupMap = rules
		c.Providers = append(c.Providers, p)
	}
	if c.PasswordLogin == "off" && len(c.Providers) == 0 && len(errs) == 0 {
		errs = append(errs, &Error{Var: "AUTH_PASSWORD_LOGIN", Reason: `"off" needs at least one provider in OAUTH_PROVIDERS`})
	}
	return errs
}

func issuerOK(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	return u.Scheme == "https" || (u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"))
}

func fields(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
}

func groupRules(raw string) ([]GroupRule, bool) {
	var out []GroupRule
	for _, part := range strings.Split(raw, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if part == "*" {
			out = append(out, GroupRule{Value: "*"})
			continue
		}
		value, group, ok := strings.Cut(part, "=>")
		value, group = strings.TrimSpace(value), strings.TrimSpace(group)
		if !ok || value == "" || group == "" {
			return nil, false
		}
		out = append(out, GroupRule{Value: value, Group: group})
	}
	return out, true
}

func (p OAuthProvider) String() string { return fmt.Sprintf("%s (%s %s)", p.Key, p.Kind, p.Issuer) }
