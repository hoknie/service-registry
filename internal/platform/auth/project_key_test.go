package auth

import (
	"crypto/sha256"
	"strings"
	"testing"
)

const earlyIssuedKey = "svcr_REDPLXwGkQFzFXi0gGXG2FH426xUTFmv0T5t9j"

func TestGeneratedKeysHaveTheDocumentedShape(t *testing.T) {
	k, err := GenerateProjectKey()
	if err != nil {
		t.Fatal(err)
	}
	if len(k.Secret) != KeyLen || KeyLen != 43 || !strings.HasPrefix(k.Secret, "svcr_") {
		t.Fatalf("shape: %q", k.Secret)
	}
	for _, c := range k.Secret[5:] {
		if !strings.ContainsRune("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz", c) {
			t.Fatalf("not base62: %q", k.Secret)
		}
	}
	if k.Prefix != k.Secret[:DisplayPrefixLen] || k.Hash != TokenHash(k.Secret) {
		t.Error("prefix or hash")
	}
	if h, ok := ParseProjectKey(k.Secret); !ok || h != k.Hash {
		t.Error("a generated key parses")
	}
	other, _ := GenerateProjectKey()
	if other.Secret == k.Secret {
		t.Error("keys repeat")
	}
}

func TestAnEarlierIssuedKeyVerifies(t *testing.T) {
	h, ok := ParseProjectKey(earlyIssuedKey)
	if !ok {
		t.Fatal("rejected")
	}
	if h != sha256.Sum256([]byte(earlyIssuedKey)) {
		t.Error("hash is not SHA-256 of the key")
	}
	if earlyIssuedKey[:DisplayPrefixLen] != "svcr_REDPLXw" {
		t.Error("display prefix")
	}
}

func TestAnySingleCharacterChangeBreaksTheChecksum(t *testing.T) {
	k, _ := GenerateProjectKey()
	for i := 5; i < KeyLen; i++ {
		b := []byte(k.Secret)
		if b[i] == 'a' {
			b[i] = 'b'
		} else {
			b[i] = 'a'
		}
		if _, ok := ParseProjectKey(string(b)); ok {
			t.Errorf("position %d", i)
		}
	}
}

func TestGarbageIsRejected(t *testing.T) {
	k, _ := GenerateProjectKey()
	for _, bad := range []string{"", "svcr_", "ghp_xxx", "svcr_short", "svcr_" + strings.Repeat("-", 38),
		k.Secret + " ", strings.Replace(k.Secret, "svcr_", "SVCR_", 1)} {
		if _, ok := ParseProjectKey(bad); ok {
			t.Errorf("%q accepted", bad)
		}
	}
}
