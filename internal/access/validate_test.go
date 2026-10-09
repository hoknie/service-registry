package access

import (
	"strings"
	"testing"
)

func TestEmailIsTrimmedAndLowercased(t *testing.T) {
	for raw, want := range map[string]string{"  Ann@Example.COM ": "ann@example.com", "root@localhost": "root@localhost"} {
		if got, err := ValidateEmail(raw); err != nil || got != want {
			t.Errorf("%q: %q %v", raw, got, err)
		}
	}
}

func TestMalformedEmailsAreRejected(t *testing.T) {
	long := strings.Repeat("a", 250) + "@example.com"
	for _, bad := range []string{"ann.example.com", "@example.com", "ann@", "a b@example.com", "a@b@c", "", long} {
		if _, err := ValidateEmail(bad); err != InvalidEmail {
			t.Errorf("%q: %v", bad, err)
		}
	}
}

func TestNamesAreTrimmedAndBounded(t *testing.T) {
	if got, _ := ValidateDisplayName("  Ann  "); got != "Ann" {
		t.Error(got)
	}
	if _, err := ValidateDisplayName("   "); err != InvalidDisplayName {
		t.Error(err)
	}
	if got, err := ValidateDisplayName(strings.Repeat("я", 100)); err != nil || len([]rune(got)) != 100 {
		t.Error(err)
	}
	if _, err := ValidateDisplayName(strings.Repeat("я", 101)); err != InvalidDisplayName {
		t.Error(err)
	}
	if _, err := ValidateGroupName("a\u0007b"); err != InvalidGroupName {
		t.Error(err)
	}
}

func TestPasswordLengthIsCountedInCharacters(t *testing.T) {
	tests := []struct {
		pw   string
		want error
	}{
		{"short", InvalidPasswordTooShort},
		{"пароль12", nil},
		{strings.Repeat("x", 256), nil},
		{strings.Repeat("x", 257), InvalidPasswordTooLong},
	}
	for _, tt := range tests {
		if err := ValidatePassword(tt.pw); err != tt.want {
			t.Errorf("%d chars: %v", len([]rune(tt.pw)), err)
		}
	}
}

func TestStatusValues(t *testing.T) {
	if s, err := ValidateStatus("disabled"); err != nil || s != StatusDisabled {
		t.Error(s, err)
	}
	if _, err := ValidateStatus("banned"); err != InvalidStatus {
		t.Error(err)
	}
}

func TestPaginationDefaultsAndBounds(t *testing.T) {
	p, err := ValidatePage(PageQuery{})
	if err != nil || p != (PageRequest{Limit: 50, Offset: 0}) {
		t.Fatal(p, err)
	}
	q := func(l, o int64) PageQuery { return PageQuery{Limit: &l, Offset: &o} }
	if p, err := ValidatePage(q(200, 7)); err != nil || p != (PageRequest{Limit: 200, Offset: 7}) {
		t.Fatal(p, err)
	}
	for _, c := range [][2]int64{{0, 0}, {201, 0}, {-5, 0}, {10, -1}} {
		if _, err := ValidatePage(q(c[0], c[1])); err != InvalidPagination {
			t.Errorf("%v: %v", c, err)
		}
	}
}
