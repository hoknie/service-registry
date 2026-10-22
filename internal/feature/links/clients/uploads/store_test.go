package uploads

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var png, _ = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==")

func TestFormats(t *testing.T) {
	t.Parallel()
	ico := append([]byte{0, 0, 1, 0, 1, 0}, bytes.Repeat([]byte{0}, 32)...)
	webp := append([]byte("RIFF\x24\x00\x00\x00WEBPVP8 "), bytes.Repeat([]byte{0}, 24)...)
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"png", png, "png"},
		{"ico", ico, "ico"},
		{"webp", webp, "webp"},
		{"svg", []byte(`<?xml version="1.0"?><!-- x --><svg xmlns="http://www.w3.org/2000/svg"/>`), "svg"},
		{"html", []byte("<html><body>x</body></html>"), ""},
		{"text", []byte("hello"), ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, err := Detect(c.data)
			if got != c.want || (c.want == "" && !errors.Is(err, ErrFormat)) {
				t.Fatalf("got %q %v", got, err)
			}
		})
	}
}

func TestPutIsContentAddressed(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "uploads")
	s := New(dir)
	id, err := s.Put(png)
	if err != nil || !ValidID(id) || !strings.HasSuffix(id, ".png") {
		t.Fatalf("%q %v", id, err)
	}
	again, err := s.Put(png)
	if err != nil || again != id {
		t.Fatalf("%q %v", again, err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("one file expected, got %d", len(entries))
	}
	f, size, err := s.Open(id)
	if err != nil || size != int64(len(png)) {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(f)
	f.Close()
	if !bytes.Equal(got, png) || ContentType(id) != "image/png" || !s.Exists(id) {
		t.Fatal("round trip")
	}
}

func TestLimitsAndPaths(t *testing.T) {
	t.Parallel()
	s := New(t.TempDir())
	if _, err := s.Put(append(png, bytes.Repeat([]byte{0}, MaxIconBytes)...)); !errors.Is(err, ErrTooLarge) {
		t.Fatal(err)
	}
	for _, id := range []string{"../etc/passwd", strings.Repeat("a", 64) + ".exe", strings.Repeat("A", 64) + ".png", strings.Repeat("a", 64) + ".png"} {
		if _, _, err := s.Open(id); !errors.Is(err, ErrNotFound) {
			t.Fatalf("%s: %v", id, err)
		}
	}
	if _, err := New("").Put(png); !errors.Is(err, ErrDisabled) {
		t.Fatal(err)
	}
}
