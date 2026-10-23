package auth

import (
	"regexp"
	"testing"
)

func TestAgentTokensHaveTheirOwnPrefix(t *testing.T) {
	t.Parallel()
	tok, err := GenerateAgentToken()
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^svca_[0-9A-Za-z]{38}$`).MatchString(tok.Secret) {
		t.Fatalf("shape: %q", tok.Secret)
	}
	if tok.Prefix != tok.Secret[:DisplayPrefixLen] {
		t.Error("prefix")
	}
	if h, ok := ParseAgentToken(tok.Secret); !ok || h != tok.Hash {
		t.Error("a generated token parses")
	}
	key, _ := GenerateProjectKey()
	pat, _ := GeneratePersonalToken()
	for name, other := range map[string]string{"project key": key.Secret, "personal token": pat.Secret} {
		if _, ok := ParseAgentToken(other); ok {
			t.Errorf("%s parsed as an agent token", name)
		}
	}
	if _, ok := ParseProjectKey(tok.Secret); ok {
		t.Error("agent token parsed as a project key")
	}
	if _, ok := ParsePersonalToken(tok.Secret); ok {
		t.Error("agent token parsed as a personal token")
	}
}
