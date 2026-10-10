package auth

import (
	"regexp"
	"testing"
)

func TestGeneratedTokensHaveTheDocumentedShape(t *testing.T) {
	tok, err := GeneratePersonalToken()
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^svcp_[0-9A-Za-z]{38}$`).MatchString(tok.Secret) || len(tok.Secret) != KeyLen {
		t.Fatalf("shape: %q", tok.Secret)
	}
	if tok.Prefix != tok.Secret[:DisplayPrefixLen] || tok.Hash != TokenHash(tok.Secret) {
		t.Error("prefix or hash")
	}
	if h, ok := ParsePersonalToken(tok.Secret); !ok || h != tok.Hash {
		t.Error("a generated token parses")
	}
}

func TestTokensAndProjectKeysDoNotParseAsEachOther(t *testing.T) {
	tok, _ := GeneratePersonalToken()
	key, _ := GenerateProjectKey()
	if _, ok := ParseProjectKey(tok.Secret); ok {
		t.Error("a token parses as a project key")
	}
	if _, ok := ParsePersonalToken(key.Secret); ok {
		t.Error("a project key parses as a token")
	}
	if _, ok := ParsePersonalToken(TokenPrefix + key.Secret[len(KeyPrefix):]); !ok {
		t.Error("a re-prefixed body is still well-formed")
	}
}

func TestAnySingleCharacterChangeBreaksATokenChecksum(t *testing.T) {
	tok, _ := GeneratePersonalToken()
	for i := len(TokenPrefix); i < KeyLen; i++ {
		b := []byte(tok.Secret)
		if b[i] == 'a' {
			b[i] = 'b'
		} else {
			b[i] = 'a'
		}
		if _, ok := ParsePersonalToken(string(b)); ok {
			t.Errorf("position %d", i)
		}
	}
}
