package types

import "testing"

func TestEmailsAreNormalizedAndChecked(t *testing.T) {
	t.Parallel()
	for raw, want := range map[string]string{" Ann@Example.COM ": "ann@example.com", "bob@example.com": "bob@example.com",
		"Ann <ann@example.com>": "", "no-at-sign": "", "": ""} {
		got, ok := NormalizeEmail(raw)
		if got != want || ok != (want != "") {
			t.Errorf("%q: %q %v", raw, got, ok)
		}
	}
}

func TestRolesOnlyGoUp(t *testing.T) {
	t.Parallel()
	if _, ok := ParseRole("owner"); ok {
		t.Fatal("owner is not a role")
	}
	admin, viewer := Admin, Viewer
	if Higher(&admin, Viewer) != Admin || Higher(&viewer, Editor) != Editor || Higher(nil, Viewer) != Viewer {
		t.Fatal("an invitation raises a role and never lowers it")
	}
}

func TestInvitationStates(t *testing.T) {
	t.Parallel()
	at := "2026-10-28T10:00:00Z"
	for want, i := range map[State]Invitation{Pending: {}, Expired: {Expired: true}, Accepted: {AcceptedAt: &at, Expired: true},
		Revoked: {RevokedAt: &at}} {
		if got := i.State(); got != want {
			t.Errorf("%s: %s", want, got)
		}
	}
	if !ValidLocale("zh") || ValidLocale("de") {
		t.Fatal("locales are en, es, ru and zh")
	}
}
