package access

import (
	"reflect"
	"strings"
	"testing"
)

func days(n int64) *int64 { return &n }

func TestValidTokensAreNormalized(t *testing.T) {
	got, err := ValidateCreateToken(CreateToken{Name: "  ci  ", Scopes: []string{"mcp", "read"}, ExpiresInDays: days(90)}, false, 365)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "ci" || !reflect.DeepEqual(got.Scopes, Scopes{ScopeRead, ScopeMCP}) || got.ExpiresInDays == nil || *got.ExpiresInDays != 90 {
		t.Fatalf("%+v", got)
	}
}

func TestTokenRules(t *testing.T) {
	tests := []struct {
		name       string
		in         CreateToken
		superadmin bool
		want       error
	}{
		{"blank name", CreateToken{Name: "  ", Scopes: []string{"read"}, ExpiresInDays: days(1)}, false, InvalidTokenName},
		{"long name", CreateToken{Name: strings.Repeat("я", 101), Scopes: []string{"read"}, ExpiresInDays: days(1)}, false, InvalidTokenName},
		{"no scopes", CreateToken{Name: "t", ExpiresInDays: days(1)}, false, InvalidScopes},
		{"unknown scope", CreateToken{Name: "t", Scopes: []string{"read", "deploy"}, ExpiresInDays: days(1)}, false, InvalidScopes},
		{"repeated scope", CreateToken{Name: "t", Scopes: []string{"read", "read"}, ExpiresInDays: days(1)}, false, InvalidScopes},
		{"admin of a user", CreateToken{Name: "t", Scopes: []string{"admin"}, ExpiresInDays: days(1)}, false, InvalidScopes},
		{"admin of a superadmin", CreateToken{Name: "t", Scopes: []string{"admin"}, ExpiresInDays: days(1)}, true, nil},
		{"zero days", CreateToken{Name: "t", Scopes: []string{"read"}, ExpiresInDays: days(0)}, true, InvalidTokenLifetime},
		{"one day", CreateToken{Name: "t", Scopes: []string{"read"}, ExpiresInDays: days(1)}, false, nil},
		{"max days", CreateToken{Name: "t", Scopes: []string{"read"}, ExpiresInDays: days(365)}, false, nil},
		{"over max", CreateToken{Name: "t", Scopes: []string{"read"}, ExpiresInDays: days(366)}, true, InvalidTokenLifetime},
		{"no expiry of a user", CreateToken{Name: "t", Scopes: []string{"read"}}, false, InvalidTokenLifetime},
		{"no expiry of a superadmin", CreateToken{Name: "t", Scopes: []string{"read"}}, true, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateCreateToken(tt.in, tt.superadmin, 365)
			if err != tt.want {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
			if err == nil && tt.in.ExpiresInDays == nil && got.ExpiresInDays != nil {
				t.Error("no expiry kept")
			}
		})
	}
}

func TestTokenQueryPrefixIsBounded(t *testing.T) {
	ok, long := "svcp_Ab3xY9q", "svcp_Ab3xY9qZ"
	if f, err := ValidateTokenQuery(TokenQuery{Prefix: &ok}); err != nil || f.Prefix != ok || f.Page.Limit != PageDefaultLimit {
		t.Fatalf("%+v %v", f, err)
	}
	if _, err := ValidateTokenQuery(TokenQuery{Prefix: &long}); err != InvalidTokenPrefix {
		t.Fatal(err)
	}
	zero := int64(0)
	if _, err := ValidateTokenQuery(TokenQuery{Page: PageQuery{Limit: &zero}}); err != InvalidPagination {
		t.Fatal(err)
	}
}
