package local

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"svc-registry/internal/platform/apperr"
)

func TestLocalStoreKeepsObjectsByKey(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "skills")
	s := New(dir)
	sum := sha256.Sum256([]byte("hello"))
	key := hex.EncodeToString(sum[:])
	if has, err := s.Has(ctx, key); err != nil || has {
		t.Fatal(has, err)
	}
	for range 2 {
		if err := s.Put(ctx, key, 5, strings.NewReader("hello")); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, key[:2], key[2:])); err != nil {
		t.Fatal(err)
	}
	rc, err := s.Open(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(data) != "hello" {
		t.Fatal(string(data))
	}
	if err := s.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Open(ctx, key); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatal(err)
	}
	if err := s.Put(ctx, "../../etc/passwd", 1, strings.NewReader("x")); err == nil {
		t.Fatal("bad key accepted")
	}
}
