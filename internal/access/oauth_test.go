package access

import (
	"errors"
	"reflect"
	"testing"
)

func TestDecideLogin(t *testing.T) {
	active := &User{Email: "ann@example.com", Status: StatusActive}
	disabled := &User{Email: "bob@example.com", Status: StatusDisabled}
	super := &User{Email: "root@example.com", Status: StatusActive, IsSuperadmin: true}
	verified := Claims{Subject: "s", Email: "ann@example.com", EmailVerified: true}
	open := ProviderPolicy{Key: "corp", AutoProvision: true, AllowedDomains: []string{"example.com"}}
	tests := []struct {
		name         string
		identity, by *User
		claims       Claims
		policy       ProviderPolicy
		want         Outcome
		refusal      error
	}{
		{"known identity", active, nil, Claims{}, ProviderPolicy{}, SignIn, nil},
		{"known identity disabled", disabled, nil, Claims{}, ProviderPolicy{}, 0, RefuseDisabled},
		{"unverified email", nil, active, Claims{Email: "ann@example.com"}, open, 0, RefuseEmailUnverified},
		{"no email", nil, nil, Claims{EmailVerified: true}, open, 0, RefuseEmailUnverified},
		{"link by email", nil, active, verified, ProviderPolicy{}, LinkAndSignIn, nil},
		{"superadmin needs manual link", nil, super, verified, open, 0, RefuseLinkRequired},
		{"disabled by email", nil, disabled, verified, open, 0, RefuseDisabled},
		{"provision", nil, nil, Claims{Email: "new@Example.com", EmailVerified: true}, open, Provision, nil},
		{"provision off", nil, nil, verified, ProviderPolicy{}, 0, RefuseNotProvisioned},
		{"foreign domain", nil, nil, Claims{Email: "x@other.example", EmailVerified: true}, open, 0, RefuseNotProvisioned},
		{"any domain", nil, nil, Claims{Email: "x@other.example", EmailVerified: true}, ProviderPolicy{AutoProvision: true}, Provision, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Decide(tt.identity, tt.by, tt.claims, tt.policy)
			if got != tt.want || !errors.Is(err, tt.refusal) {
				t.Fatalf("got %v %v", got, err)
			}
		})
	}
}

func TestGroupsFor(t *testing.T) {
	rules := []GroupRule{{Value: "platform-team", Group: "Platform"}, {Value: "*"}}
	got := GroupsFor([]string{"platform-team", "sre", "SRE", "  "}, rules)
	if !reflect.DeepEqual(got, []string{"Platform", "platform-team", "sre"}) {
		t.Fatal(got)
	}
	if got := GroupsFor([]string{"x"}, []GroupRule{{Value: "y", Group: "Y"}}); got != nil {
		t.Fatal(got)
	}
}

func TestProvisionedNameAndModes(t *testing.T) {
	if n := ProvisionedName(Claims{Name: "  ", PreferredUsername: "ann", Email: "a@x"}); n != "ann" {
		t.Fatal(n)
	}
	if n := ProvisionedName(Claims{Email: "bob@x"}); n != "bob" {
		t.Fatal(n)
	}
	if !PasswordAll.Allows(false) || PasswordSuperadmins.Allows(false) || !PasswordSuperadmins.Allows(true) || PasswordOff.Allows(true) {
		t.Fatal("modes")
	}
	if OAuthSource("corp") != "oauth:corp" || OAuthMethod("corp") != "oauth:corp" {
		t.Fatal("names")
	}
}
