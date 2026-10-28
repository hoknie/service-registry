package source

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io/fs"
	"strings"
	"testing"

	knowledgetypes "svc-registry/internal/feature/knowledge/types"
)

func tarball(t *testing.T, files map[string]string, links ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	for _, l := range links {
		if err := tw.WriteHeader(&tar.Header{Name: l, Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestUnpackStripsThePrefixAndSkipsUnsafeEntries(t *testing.T) {
	t.Parallel()
	data := tarball(t, map[string]string{
		"repo-abc/skills/a/SKILL.md": "# a",
		"repo-abc/../escape.txt":     "no",
		"repo-abc/x/../../up.txt":    "no",
		"top.txt":                    "no",
	}, "repo-abc/skills/link")
	files, release, err := Unpack(bytes.NewReader(data), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	var got []string
	if err := fs.WalkDir(files, ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			got = append(got, p)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "skills/a/SKILL.md" {
		t.Fatalf("files = %v", got)
	}
}

func TestUnpackRefusesALargeArchive(t *testing.T) {
	t.Parallel()
	data := tarball(t, map[string]string{"r/big.txt": strings.Repeat("x", 1<<16)})
	if _, _, err := Unpack(bytes.NewReader(data), 64); !errors.Is(err, knowledgetypes.ErrSnapshotTooLarge) {
		t.Fatalf("err = %v", err)
	}
	bomb := tarball(t, map[string]string{"r/zero.txt": string(make([]byte, 1<<20))})
	if _, _, err := Unpack(bytes.NewReader(bomb), int64(len(bomb))); !errors.Is(err, knowledgetypes.ErrSnapshotTooLarge) {
		t.Fatalf("bomb err = %v", err)
	}
}
