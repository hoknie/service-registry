package access

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"svc-registry/internal/platform/apperr"
)

func kindOf(err error) apperr.Kind {
	var e *apperr.Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return 0
}

func TestScopesLimitOnlyTokenRequests(t *testing.T) {
	session := Principal{UserID: uuid.Must(uuid.NewV7()), SessionID: uuid.Must(uuid.NewV7())}
	token := func(scopes ...Scope) Principal {
		return Principal{UserID: session.UserID, TokenID: uuid.Must(uuid.NewV7()), Scopes: scopes}
	}
	all := token(AllScopes...)
	tests := []struct {
		name string
		p    Principal
		need Need
		want apperr.Kind
	}{
		{"session needs nothing", session, NeedSession, 0},
		{"session admin route", session, NeedAdmin, 0},
		{"token on a session route", all, NeedSession, apperr.SessionRequired},
		{"any scope reads me", token(ScopeMCP), NeedAny, 0},
		{"read reads", token(ScopeRead), NeedRead, 0},
		{"read does not write", token(ScopeRead), NeedWrite, apperr.InsufficientScope},
		{"write does not read", token(ScopeWrite), NeedRead, apperr.InsufficientScope},
		{"write writes", token(ScopeWrite), NeedWrite, 0},
		{"admin does not read the catalog", token(ScopeAdmin), NeedRead, apperr.InsufficientScope},
		{"admin administers", token(ScopeAdmin), NeedAdmin, 0},
		{"read and write do not administer", token(ScopeRead, ScopeWrite), NeedAdmin, apperr.InsufficientScope},
		{"mcp opens no catalog route", token(ScopeMCP), NeedRead, apperr.InsufficientScope},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := kindOf(RequireScope(tt.p, tt.need)); got != tt.want {
				t.Fatalf("kind %d, want %d", got, tt.want)
			}
		})
	}
}
