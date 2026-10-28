package types

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"svc-registry/internal/platform/apperr"
)

func fieldsOf(t *testing.T, err error) []string {
	t.Helper()
	var e *apperr.Error
	if !errors.As(err, &e) || e.Code != "validation.invalid_mcp_server" {
		t.Fatalf("error %v", err)
	}
	out := make([]string, len(e.Fields))
	for i, f := range e.Fields {
		out[i] = f.Path
	}
	return out
}

func TestCleanServerAcceptsAWellFormedServer(t *testing.T) {
	t.Parallel()
	secret := uuid.New()
	got, err := CleanServer(ServerFields{Slug: "jira", Title: " Jira ", URL: " https://mcp.jira.example/mcp ",
		Tags: []string{"Tracker", "tracker", " "}, Auth: Auth{Kind: AuthHeader, Header: "X-Api-Key", SecretID: &secret}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Jira" || got.URL != "https://mcp.jira.example/mcp" || got.Transport != TransportStreamable ||
		got.Status != StatusDraft || len(got.Tags) != 1 || got.Tags[0] != "tracker" {
		t.Fatalf("cleaned %+v", got)
	}
}

func TestCleanServerNamesEveryBadField(t *testing.T) {
	t.Parallel()
	icon := "http://icons.example/x.png"
	for _, c := range []struct {
		name string
		in   ServerFields
		want string
	}{
		{"slug", ServerFields{Slug: "Jira", Title: "J", URL: "https://x"}, "slug"},
		{"slug too long", ServerFields{Slug: "a234567890123456789012345678901234", Title: "J", URL: "https://x"}, "slug"},
		{"credentials in url", ServerFields{Slug: "j", Title: "J", URL: "https://user:pw@x/mcp"}, "url"},
		{"scheme", ServerFields{Slug: "j", Title: "J", URL: "ftp://x"}, "url"},
		{"icon not https", ServerFields{Slug: "j", Title: "J", URL: "https://x", IconURL: &icon}, "icon_url"},
		{"transport", ServerFields{Slug: "j", Title: "J", URL: "https://x", Transport: "ws"}, "transport"},
		{"bearer without secret", ServerFields{Slug: "j", Title: "J", URL: "https://x", Auth: Auth{Kind: AuthBearer}}, "auth.credentials.secret_id"},
		{"reserved header", ServerFields{Slug: "j", Title: "J", URL: "https://x", Auth: Auth{Kind: AuthHeader, Header: "Mcp-Session-Id"}}, "auth.header"},
		{"bad tag", ServerFields{Slug: "j", Title: "J", URL: "https://x", Tags: []string{"no spaces"}}, "tags"},
		{"empty title", ServerFields{Slug: "j", Title: " ", URL: "https://x"}, "title"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := CleanServer(c.in)
			if got := fieldsOf(t, err); len(got) != 1 || got[0] != c.want {
				t.Fatalf("fields %v", got)
			}
		})
	}
}

func TestStateOfServer(t *testing.T) {
	t.Parallel()
	ok, failed := &Check{Outcome: OutcomeOK}, &Check{Outcome: OutcomeFailed}
	for _, c := range []struct {
		status   Status
		last     *Check
		snapshot bool
		want     State
	}{
		{StatusPublished, nil, false, StateUnknown},
		{StatusPublished, failed, false, StateUnknown},
		{StatusPublished, failed, true, StateFailing},
		{StatusDraft, ok, true, StateOK},
		{StatusDisabled, ok, true, StateDisabled},
	} {
		if got := StateOf(c.status, c.last, c.snapshot); got != c.want {
			t.Fatalf("%v %v %v: %s", c.status, c.last, c.snapshot, got)
		}
	}
}
