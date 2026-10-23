package auth

import (
	"regexp"
	"testing"
)

func TestAppTokensHaveTheDocumentedShape(t *testing.T) {
	for _, tc := range []struct {
		generate func() (IssuedKey, error)
		parse    func(string) ([32]byte, bool)
		other    func(string) ([32]byte, bool)
		shape    string
	}{
		{GenerateAppAccessToken, ParseAppAccessToken, ParseAppRefreshToken, `^svco_[0-9A-Za-z]{38}$`},
		{GenerateAppRefreshToken, ParseAppRefreshToken, ParseAppAccessToken, `^svcf_[0-9A-Za-z]{38}$`},
	} {
		tok, err := tc.generate()
		if err != nil {
			t.Fatal(err)
		}
		if !regexp.MustCompile(tc.shape).MatchString(tok.Secret) || tok.Hash != TokenHash(tok.Secret) {
			t.Fatalf("shape: %q", tok.Secret)
		}
		if h, ok := tc.parse(tok.Secret); !ok || h != tok.Hash {
			t.Errorf("%q does not parse", tok.Secret)
		}
		if _, ok := tc.other(tok.Secret); ok {
			t.Errorf("%q parses as the other kind", tok.Secret)
		}
		if _, ok := ParsePersonalToken(tok.Secret); ok {
			t.Errorf("%q parses as a personal token", tok.Secret)
		}
	}
}
