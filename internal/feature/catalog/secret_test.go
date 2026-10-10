package catalog

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
		{"value", SecretInput{Name: ptr(" gitlab "), Value: ptr("glpat-1")}, true, true},
		{"ref", SecretInput{Name: ptr("gitlab"), Ref: ptr("env:GITLAB_TOKEN")}, true, true},
		{"file ref", SecretInput{Name: ptr("gitlab"), Ref: ptr("file:/run/secrets/gitlab")}, true, true},
		{"no name", SecretInput{Value: ptr("x")}, true, false},
		{"long name", SecretInput{Name: ptr(strings.Repeat("a", 101)), Value: ptr("x")}, true, false},
		{"long description", SecretInput{Name: ptr("a"), Description: ptr(strings.Repeat("d", 501)), Value: ptr("x")}, true, false},
		{"both", SecretInput{Name: ptr("a"), Value: ptr("x"), Ref: ptr("env:X")}, true, false},
		{"neither", SecretInput{Name: ptr("a")}, true, false},
		{"space in value", SecretInput{Name: ptr("a"), Value: ptr("a b")}, true, false},
		{"relative file", SecretInput{Name: ptr("a"), Ref: ptr("file:secrets/x")}, true, false},
		{"patch description only", SecretInput{Description: ptr("CI token")}, false, true},
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
	v, _ := ValidateSecret(SecretInput{Name: ptr(" gitlab ")}, false)
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
	if _, err := ParseCredentials(&CredentialsInput{SecretID: ptr("nope")}, invalid); !errors.Is(err, invalid) {
		t.Fatalf("bad id: %v", err)
	}
	if _, err := ParseCredentials(nil, invalid); !errors.Is(err, invalid) {
		t.Fatalf("nil: %v", err)
	}
	id, err := ParseCredentials(&CredentialsInput{SecretID: ptr("0199c000-0000-7000-8000-000000000001")}, invalid)
	if err != nil || id.String() != "0199c000-0000-7000-8000-000000000001" {
		t.Fatalf("id: %v %v", id, err)
	}
}
