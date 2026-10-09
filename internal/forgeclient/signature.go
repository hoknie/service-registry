package forgeclient

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"

	"github.com/google/go-github/v92/github"

	domain "svc-registry/internal/forge"
)

func (f *Factory) VerifyDelivery(kind domain.Kind, secret string, header func(string) string, body []byte) bool {
	if secret == "" {
		return false
	}
	switch kind {
	case domain.KindGithub:
		sig := header("X-Hub-Signature-256")
		return sig != "" && github.ValidateSignature(sig, body, []byte(secret)) == nil
	case domain.KindGitea, domain.KindForgejo:
		sig := header("X-Forgejo-Signature")
		if sig == "" {
			sig = header("X-Gitea-Signature")
		}
		got, err := hex.DecodeString(sig)
		if err != nil || len(got) == 0 {
			return false
		}
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		return hmac.Equal(got, mac.Sum(nil))
	case domain.KindGitlab:
		token := header("X-Gitlab-Token")
		return token != "" && subtle.ConstantTimeCompare([]byte(token), []byte(secret)) == 1
	}
	return false
}
