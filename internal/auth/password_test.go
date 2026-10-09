package auth

import (
	"strings"
	"testing"

	"github.com/alexedwards/argon2id"

	"svc-registry/internal/config"
)

func hasher(m, t, p uint32) *PasswordHasher {
	return NewPasswordHasher(config.PasswordHashConfig{MemoryKiB: m, Iterations: t, Parallelism: p})
}

func TestHashIsPHCAndVerifies(t *testing.T) {
	h := hasher(1024, 1, 1)
	phcStr, err := h.Hash("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(phcStr, "$argon2id$v=19$m=1024,t=1,p=1$") || strings.Contains(phcStr, "correct horse") {
		t.Fatal(phcStr)
	}
	if !h.Verify("correct horse battery", phcStr) || h.Verify("correct horse batterx", phcStr) {
		t.Fatal("verify")
	}
	_, salt, key, err := argon2id.DecodeHash(phcStr)
	if err != nil || len(salt) != 16 || len(key) != 32 {
		t.Fatal(salt, key, err)
	}
}

func TestSamePasswordGetsDifferentSalts(t *testing.T) {
	h := hasher(1024, 1, 1)
	a, _ := h.Hash("same password")
	b, _ := h.Hash("same password")
	if a == b {
		t.Fatal("same hash")
	}
}

func TestVerificationUsesTheParametersInTheHash(t *testing.T) {
	old, _ := hasher(2048, 2, 2).Hash("long enough pw")
	if !hasher(1024, 1, 1).Verify("long enough pw", old) {
		t.Fatal("old parameters")
	}
}

func TestReferenceImplementationVectorVerifies(t *testing.T) {
	const reference = "$argon2id$v=19$m=1024,t=2,p=1$c29tZXNhbHQ$7FfsnA6vUe7qLpL/3Kqc3uR48ZJyFbUVt7jWZlf0Htk"
	h := hasher(19_456, 2, 1)
	if _, salt, _, _ := argon2id.DecodeHash(reference); string(salt) != "somesalt" {
		t.Fatal("salt")
	}
	if !h.Verify("password", reference) || h.Verify("Password", reference) {
		t.Fatal("reference vector")
	}
}

func TestMalformedHashesNeverVerify(t *testing.T) {
	h := hasher(1024, 1, 1)
	for _, bad := range []string{
		"",
		"plain",
		"$argon2i$v=19$m=1024,t=1,p=1$c2FsdHNhbHQ$aGFzaGhhc2g",
		"$argon2id$v=16$m=1024,t=1,p=1$c2FsdHNhbHQ$aGFzaGhhc2g",
		"$argon2id$v=19$m=x,t=1,p=1$c2FsdHNhbHQ$aGFzaGhhc2g",
		"$argon2id$v=19$m=1024,t=1,p=1$!!$aGFzaGhhc2g",
		"$argon2id$v=19$m=1024,t=1,p=1$c2FsdHNhbHQ$aGFzaGhhc2g$extra",
		"$argon2id$v=19$m=1024,t=0,p=1$c2FsdHNhbHQ$aGFzaGhhc2g",
		"$argon2id$v=19$m=1024,t=1$c2FsdHNhbHQ$aGFzaGhhc2g",
	} {
		if h.Verify("password", bad) {
			t.Errorf("%q verified", bad)
		}
	}
}

func TestDummyVerificationCountsAsAVerification(t *testing.T) {
	h := hasher(1024, 1, 1)
	before := h.Verifications()
	h.VerifyDummy("anything at all")
	if h.Verifications() != before+1 {
		t.Fatal(h.Verifications())
	}
}
