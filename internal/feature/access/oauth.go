package access

import (
	"context"
	"slices"
	"strings"
)

type Refusal string

const (
	RefuseUnavailable     Refusal = "auth.oauth_unavailable"
	RefuseDenied          Refusal = "auth.oauth_denied"
	RefuseState           Refusal = "auth.oauth_state_invalid"
	RefuseFailed          Refusal = "auth.oauth_failed"
	RefuseEmailUnverified Refusal = "auth.oauth_email_unverified"
	RefuseLinkRequired    Refusal = "auth.oauth_link_required"
	RefuseNotProvisioned  Refusal = "auth.oauth_not_provisioned"
	RefuseIdentityTaken   Refusal = "auth.oauth_identity_taken"
	RefuseDisabled        Refusal = "auth.account_disabled"
	RefuseServiceAccount  Refusal = "auth.service_account"
)

func (r Refusal) Error() string { return string(r) }

type Claims struct {
	Subject           string
	Email             string
	EmailVerified     bool
	Name              string
	PreferredUsername string
	Groups            []string
}

type GroupRule struct {
	Value string
	Group string
}

type ProviderPolicy struct {
	Key            string
	AutoProvision  bool
	AllowedDomains []string
	GroupRules     []GroupRule
}

type Outcome int

const (
	SignIn Outcome = iota + 1
	LinkAndSignIn
	Provision
)

func Decide(identityUser, emailUser *User, c Claims, p ProviderPolicy) (Outcome, error) {
	if identityUser != nil {
		if identityUser.IsService {
			return 0, RefuseServiceAccount
		}
		if identityUser.Status != StatusActive {
			return 0, RefuseDisabled
		}
		return SignIn, nil
	}
	if c.Email == "" || !c.EmailVerified {
		return 0, RefuseEmailUnverified
	}
	if emailUser != nil {
		switch {
		case emailUser.IsService:
			return 0, RefuseServiceAccount
		case emailUser.IsSuperadmin:
			return 0, RefuseLinkRequired
		case emailUser.Status != StatusActive:
			return 0, RefuseDisabled
		}
		return LinkAndSignIn, nil
	}
	if !p.AutoProvision || !DomainAllowed(c.Email, p.AllowedDomains) {
		return 0, RefuseNotProvisioned
	}
	return Provision, nil
}

func DomainAllowed(email string, domains []string) bool {
	if len(domains) == 0 {
		return true
	}
	_, domain, ok := strings.Cut(strings.ToLower(email), "@")
	return ok && slices.Contains(domains, domain)
}

func ProvisionedName(c Claims) string {
	local, _, _ := strings.Cut(c.Email, "@")
	for _, candidate := range []string{c.Name, c.PreferredUsername, local} {
		if name, err := ValidateDisplayName(candidate); err == nil {
			return name
		}
	}
	return "user"
}

func GroupsFor(values []string, rules []GroupRule) []string {
	var out []string
	add := func(raw string) {
		name, err := ValidateGroupName(raw)
		if err != nil {
			return
		}
		for _, have := range out {
			if strings.EqualFold(have, name) {
				return
			}
		}
		out = append(out, name)
	}
	for _, v := range values {
		for _, r := range rules {
			switch {
			case r.Value == "*":
				add(v)
			case r.Value == v:
				add(r.Group)
			}
		}
	}
	return out
}

type PasswordLogin string

const (
	PasswordAll         PasswordLogin = "all"
	PasswordSuperadmins PasswordLogin = "superadmins"
	PasswordOff         PasswordLogin = "off"
)

func (m PasswordLogin) Allows(superadmin bool) bool {
	switch m {
	case PasswordOff:
		return false
	case PasswordSuperadmins:
		return superadmin
	}
	return true
}

type OAuthClient interface {
	AuthURL(ctx context.Context, provider, state, nonce, verifier string) (string, error)
	Exchange(ctx context.Context, provider, code, verifier, nonce string) (Claims, error)
}
