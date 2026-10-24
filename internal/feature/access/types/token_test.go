package types

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestValidTokensAreNormalized(t *testing.T) {
	got, err := ValidateCreateToken(CreateToken{Name: "  ci  ", Scopes: []string{"mcp", "read"}, ExpiresInDays: new(int64(90))}, Grantor{}, 365)
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
		{"blank name", CreateToken{Name: "  ", Scopes: []string{"read"}, ExpiresInDays: new(int64(1))}, false, InvalidTokenName},
		{"long name", CreateToken{Name: strings.Repeat("я", 101), Scopes: []string{"read"}, ExpiresInDays: new(int64(1))}, false, InvalidTokenName},
		{"no scopes", CreateToken{Name: "t", ExpiresInDays: new(int64(1))}, false, InvalidScopes},
		{"unknown scope", CreateToken{Name: "t", Scopes: []string{"read", "deploy"}, ExpiresInDays: new(int64(1))}, false, InvalidScopes},
		{"repeated scope", CreateToken{Name: "t", Scopes: []string{"read", "read"}, ExpiresInDays: new(int64(1))}, false, InvalidScopes},
		{"removed admin scope", CreateToken{Name: "t", Scopes: []string{"admin"}, ExpiresInDays: new(int64(1))}, true, InvalidScopes},
		{"global scope of a user", CreateToken{Name: "t", Scopes: []string{"read", "users:read"}, ExpiresInDays: new(int64(1))}, false, InvalidScopes},
		{"catalog root of a user", CreateToken{Name: "t", Scopes: []string{"catalog.root"}, ExpiresInDays: new(int64(1))}, false, InvalidScopes},
		{"global scopes of a superadmin", CreateToken{Name: "t", Scopes: []string{"settings:write", "users:read", "catalog.root"}, ExpiresInDays: new(int64(1))}, true, nil},
		{"undivided global scope", CreateToken{Name: "t", Scopes: []string{"users"}, ExpiresInDays: new(int64(1))}, true, InvalidScopes},
		{"zero days", CreateToken{Name: "t", Scopes: []string{"read"}, ExpiresInDays: new(int64(0))}, true, InvalidTokenLifetime},
		{"one day", CreateToken{Name: "t", Scopes: []string{"read"}, ExpiresInDays: new(int64(1))}, false, nil},
		{"max days", CreateToken{Name: "t", Scopes: []string{"read"}, ExpiresInDays: new(int64(365))}, false, nil},
		{"over max", CreateToken{Name: "t", Scopes: []string{"read"}, ExpiresInDays: new(int64(366))}, true, InvalidTokenLifetime},
		{"no expiry of a user", CreateToken{Name: "t", Scopes: []string{"read"}}, false, InvalidTokenLifetime},
		{"no expiry of a superadmin", CreateToken{Name: "t", Scopes: []string{"read"}}, true, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateCreateToken(tt.in, Grantor{Superadmin: tt.superadmin}, 365)
			if err != tt.want {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
			if err == nil && tt.in.ExpiresInDays == nil && got.ExpiresInDays != nil {
				t.Error("no expiry kept")
			}
		})
	}
}

func TestTokenNodeRules(t *testing.T) {
	a, b := "0190b6f0-0000-7000-8000-000000000001", "0190b6f0-0000-7000-8000-000000000002"
	many := make([]string, TokenMaxNodes+1)
	for i := range many {
		many[i] = fmt.Sprintf("0190b6f0-0000-7000-8000-%012d", i+1)
	}
	tests := []struct {
		name   string
		nodes  *[]string
		scopes []string
		want   error
		count  int
	}{
		{"absent", nil, []string{"read"}, nil, 0},
		{"two", &[]string{a, b}, []string{"read", "write"}, nil, 2},
		{"twenty", new(many[:TokenMaxNodes]), []string{"mcp"}, nil, TokenMaxNodes},
		{"empty", &[]string{}, []string{"read"}, InvalidTokenNodes, 0},
		{"too many", &many, []string{"read"}, InvalidTokenNodes, 0},
		{"repeated", &[]string{a, a}, []string{"read"}, InvalidTokenNodes, 0},
		{"not an id", &[]string{"acme"}, []string{"read"}, InvalidTokenNodes, 0},
		{"with a global scope", &[]string{a}, []string{"read", "clusters:read"}, InvalidTokenNodes, 0},
		{"with catalog root", &[]string{a}, []string{"write", "catalog.root"}, nil, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateCreateToken(CreateToken{Name: "t", Scopes: tt.scopes, ExpiresInDays: new(int64(1)), Nodes: tt.nodes}, Grantor{Superadmin: true}, 365)
			if err != tt.want {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
			if len(got.Nodes) != tt.count || (tt.nodes == nil && got.Nodes != nil) {
				t.Fatalf("nodes %v", got.Nodes)
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
	if _, err := ValidateTokenQuery(TokenQuery{Page: PageQuery{Limit: new(int64(0))}}); err != InvalidPagination {
		t.Fatal(err)
	}
}
