package auth

import (
	"crypto/rand"
	"encoding/base64"

	"svc-registry/pkg/apikey"
)

type SessionToken struct {
	Token string
	Hash  [32]byte
}

func NewSessionToken() (SessionToken, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return SessionToken{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	return SessionToken{Token: token, Hash: TokenHash(token)}, nil
}

func TokenHash(token string) [32]byte { return apikey.Hash(token) }
