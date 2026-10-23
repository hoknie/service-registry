package types

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateSecret(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		in     SecretInput
		create bool
		ok     bool
	}{
		{"value", SecretInput{Name: new(" gitlab "), Value: new("glpat-1")}, true, true},
		{"ref", SecretInput{Name: new("gitlab"), Ref: new("env:GITLAB_TOKEN")}, true, true},
		{"file ref", SecretInput{Name: new("gitlab"), Ref: new("file:/run/secrets/gitlab")}, true, true},
		{"no name", SecretInput{Value: new("x")}, true, false},
		{"long name", SecretInput{Name: new(strings.Repeat("a", 101)), Value: new("x")}, true, false},
		{"long description", SecretInput{Name: new("a"), Description: new(strings.Repeat("d", 501)), Value: new("x")}, true, false},
		{"both", SecretInput{Name: new("a"), Value: new("x"), Ref: new("env:X")}, true, false},
		{"neither", SecretInput{Name: new("a")}, true, false},
		{"space in value", SecretInput{Name: new("a"), Value: new("a b")}, true, false},
		{"relative file", SecretInput{Name: new("a"), Ref: new("file:secrets/x")}, true, false},
		{"patch description only", SecretInput{Description: new("CI token")}, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			v, err := ValidateSecret(c.in, c.create)
			if c.ok != (err == nil) {
				t.Fatalf("ValidateSecret = %+v, %v; want ok=%v", v, err, c.ok)
			}
			if err != nil && !errors.Is(err, InvalidSecret) {
				t.Fatalf("error = %v, want InvalidSecret", err)
			}
		})
	}
	v, _ := ValidateSecret(SecretInput{Name: new(" gitlab ")}, false)
	if *v.Name != "gitlab" {
		t.Fatalf("name = %q, want trimmed", *v.Name)
	}
}

func TestParseCredentials(t *testing.T) {
	t.Parallel()
	invalid := errors.New("invalid")
	if _, err := ParseCredentials(&CredentialsInput{Inline: true}, invalid); !errors.Is(err, InvalidCredentialsInline) {
		t.Fatalf("inline: %v", err)
	}
	if _, err := ParseCredentials(&CredentialsInput{SecretID: new("nope")}, invalid); !errors.Is(err, invalid) {
		t.Fatalf("bad id: %v", err)
	}
	if _, err := ParseCredentials(nil, invalid); !errors.Is(err, invalid) {
		t.Fatalf("nil: %v", err)
	}
	id, err := ParseCredentials(&CredentialsInput{SecretID: new("0199c000-0000-7000-8000-000000000001")}, invalid)
	if err != nil || id.String() != "0199c000-0000-7000-8000-000000000001" {
		t.Fatalf("id: %v %v", id, err)
	}
}

func TestAdoptedSecretName(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("a", 120)
	cases := []struct {
		name  string
		base  string
		taken []string
		want  string
	}{
		{"free", "forge-acme", nil, "forge-acme"},
		{"taken", "forge-acme", []string{"forge-acme"}, "forge-acme-2"},
		{"taken case-insensitive", "forge-acme", []string{"FORGE-ACME", "forge-acme-2"}, "forge-acme-3"},
		{"control characters", "source-a\tb", nil, "source-a-b"},
		{"empty", "  ", nil, "secret"},
		{"long", long, nil, strings.Repeat("a", 100)},
		{"long taken", long, []string{strings.Repeat("a", 100)}, strings.Repeat("a", 98) + "-2"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			taken := map[string]bool{}
			for _, n := range c.taken {
				taken[strings.ToLower(n)] = true
			}
			if got := AdoptedSecretName(c.base, taken); got != c.want {
				t.Fatalf("AdoptedSecretName(%q) = %q, want %q", c.base, got, c.want)
			}
		})
	}
}
