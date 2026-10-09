package apikey

import (
	"hash/crc32"
	"strings"
	"testing"
)

func TestGeneratedSecretsHaveTheDocumentedShape(t *testing.T) {
	k, err := Generate("abcd_")
	if err != nil {
		t.Fatal(err)
	}
	if len(k.Secret) != Len("abcd_") || !strings.HasPrefix(k.Secret, "abcd_") {
		t.Fatalf("shape: %q", k.Secret)
	}
	for _, c := range k.Secret[5:] {
		if !strings.ContainsRune(base62, c) {
			t.Fatalf("not base62: %q", k.Secret)
		}
	}
	if k.Prefix != k.Secret[:DisplayPrefixLen] || k.Hash != Hash(k.Secret) {
		t.Error("prefix or hash")
	}
	if h, ok := Parse("abcd_", k.Secret); !ok || h != k.Hash {
		t.Error("a generated secret parses")
	}
	if _, ok := Parse("other_", k.Secret); ok {
		t.Error("another prefix parses")
	}
	broken := k.Secret[:len(k.Secret)-1] + "!"
	if _, ok := Parse("abcd_", broken); ok {
		t.Error("a broken checksum parses")
	}
}

func TestChecksumIsCRC32InBase62(t *testing.T) {
	if got := Checksum([]byte("123456789")); string(got[:]) != "3jZRME" {
		t.Errorf("got %s", got)
	}
	body := []byte("AbCdEfGhIjKlMnOpQrStUvWxYz012345")
	n, want := crc32.ChecksumIEEE(body), [CheckLen]byte{}
	for i := CheckLen - 1; i >= 0; i-- {
		want[i] = base62[n%62]
		n /= 62
	}
	if Checksum(body) != want {
		t.Error("differs from hash/crc32")
	}
}
