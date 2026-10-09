package secretbox

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func key(id string, b byte) Key {
	k := Key{ID: id}
	for i := range k.Key {
		k.Key[i] = b
	}
	return k
}

func TestSealOpenRoundTrip(t *testing.T) {
	box := New([]Key{key("k1", 1)})
	aad := AAD("forge_connections", "id-1", "credentials_enc")
	ct, err := box.Seal("glpat-secret", aad)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(ct, "v1.k1.") || strings.Contains(ct, "glpat") {
		t.Fatalf("ciphertext %q", ct)
	}
	if got, err := box.Open(ct, aad); err != nil || got != "glpat-secret" {
		t.Fatalf("%q %v", got, err)
	}
	again, _ := box.Seal("glpat-secret", aad)
	if again == ct {
		t.Error("nonce repeats")
	}
}

func TestCiphertextIsBoundToItsRow(t *testing.T) {
	box := New([]Key{key("k1", 1)})
	ct, _ := box.Seal("x", AAD("forge_connections", "id-1", "credentials_enc"))
	if _, err := box.Open(ct, AAD("forge_connections", "id-2", "credentials_enc")); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("another row: %v", err)
	}
	if _, err := box.Open(ct, AAD("forge_connections", "id-1", "webhook_secret_enc")); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("another column: %v", err)
	}
	prefix := ct[:strings.LastIndexByte(ct, '.')+1]
	raw, _ := base64.RawURLEncoding.DecodeString(ct[len(prefix):])
	raw[len(raw)-1] ^= 1
	tampered := prefix + base64.RawURLEncoding.EncodeToString(raw)
	if _, err := box.Open(tampered, AAD("forge_connections", "id-1", "credentials_enc")); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("tampered: %v", err)
	}
}

func TestRotationDecryptsWithAnyKeyAndSealsWithTheFirst(t *testing.T) {
	aad := AAD("t", "1", "c")
	old := New([]Key{key("k1", 1)})
	ct, _ := old.Seal("s", aad)
	both := New([]Key{key("k2", 2), key("k1", 1)})
	if got, err := both.Open(ct, aad); err != nil || got != "s" {
		t.Fatalf("%q %v", got, err)
	}
	fresh, _ := both.Seal("s", aad)
	if id, _ := KeyID(fresh); id != "k2" {
		t.Fatalf("sealed with %s", id)
	}
	onlyNew := New([]Key{key("k2", 2)})
	if _, err := onlyNew.Open(ct, aad); !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("old ciphertext with new key only: %v", err)
	}
	if got, err := onlyNew.Open(fresh, aad); err != nil || got != "s" {
		t.Fatalf("%q %v", got, err)
	}
}

func TestWithoutKeys(t *testing.T) {
	box := New(nil)
	if box.CanEncrypt() {
		t.Fatal("can encrypt")
	}
	if _, err := box.Seal("x", nil); !errors.Is(err, ErrNoKey) {
		t.Fatal(err)
	}
	if _, err := box.Open("garbage", nil); !errors.Is(err, ErrCorrupt) {
		t.Fatal(err)
	}
}

func TestFingerprint(t *testing.T) {
	if got := Fingerprint("abc"); got != "ba78" {
		t.Fatal(got)
	}
}
