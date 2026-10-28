package types

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

var limits = Limits{TotalBytes: 1 << 20, FileBytes: 64 << 10, Files: 10}

const skillMD = "---\nname: go-testing\ndescription: Tests\n---\n# Go testing\n"

func zipOf(t *testing.T, files map[string]string, method uint16) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, text := range files {
		f, err := w.CreateHeader(&zip.FileHeader{Name: name, Method: method})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(f, text); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func entries(files map[string]string) []Entry {
	var out []Entry
	for name, text := range files {
		out = append(out, Entry{Path: name, Open: func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader(text)), nil }})
	}
	return out
}

func paths(c Content) string {
	var out []string
	for _, f := range c.Files {
		out = append(out, f.Path+":"+string(f.Kind))
	}
	return strings.Join(out, ",")
}

func TestUnpackStripsTheRootFolder(t *testing.T) {
	t.Parallel()
	c, err := UnpackZip(zipOf(t, map[string]string{"go-testing/SKILL.md": skillMD, "go-testing/scripts/run.sh": "#!/bin/sh\n",
		"go-testing/.DS_Store": "x", "__MACOSX/go-testing/._SKILL.md": "x"}, zip.Deflate), limits)
	if err != nil {
		t.Fatal(err)
	}
	if got := paths(c); got != "SKILL.md:markdown,scripts/run.sh:text" {
		t.Fatal(got)
	}
	if c.Manifest.Name != "go-testing" || !strings.HasPrefix(c.Digest, "sha256:") || *c.Files[0].Title != "Go testing" {
		t.Fatalf("%+v", c)
	}
	parts, err := Build(entries(map[string]string{"go-testing/SKILL.md": skillMD, "go-testing/reference.md": "ref"}), limits)
	if err != nil || paths(parts) != "SKILL.md:markdown,reference.md:markdown" {
		t.Fatal(paths(parts), err)
	}
	same, _ := Build(entries(map[string]string{"SKILL.md": skillMD, "scripts/run.sh": "#!/bin/sh\n"}), limits)
	if same.Digest != c.Digest {
		t.Fatal("digest depends on the packaging")
	}
}

func TestUnpackRefusesBadArchives(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		files map[string]string
		want  error
	}{
		"escape":      {map[string]string{"SKILL.md": skillMD, "../../etc/passwd": "x"}, ArchiveInvalid},
		"absolute":    {map[string]string{"SKILL.md": skillMD, "/etc/passwd": "x"}, ArchiveInvalid},
		"backslash":   {map[string]string{"SKILL.md": skillMD, `a\b`: "x"}, ArchiveInvalid},
		"no manifest": {map[string]string{"README.md": "x"}, ManifestMissing},
		"big file":    {map[string]string{"SKILL.md": skillMD, "big.txt": strings.Repeat("a", 64<<10+1)}, ContentTooLarge},
	} {
		if _, err := UnpackZip(zipOf(t, c.files, zip.Store), limits); !errors.Is(err, c.want) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	many := map[string]string{"SKILL.md": skillMD}
	for i := range 10 {
		many[strings.Repeat("f", i+1)] = "x"
	}
	if _, err := Build(entries(many), limits); !errors.Is(err, ContentTooLarge) {
		t.Fatalf("too many files: %v", err)
	}
	bomb := zipOf(t, map[string]string{"SKILL.md": skillMD, "zeros": strings.Repeat("\x00", 2<<20)}, zip.Deflate)
	if len(bomb) > 64<<10 {
		t.Fatalf("bomb not compressed: %d", len(bomb))
	}
	if _, err := UnpackZip(bomb, limits); !errors.Is(err, ContentTooLarge) {
		t.Fatalf("bomb: %v", err)
	}
	if _, err := UnpackZip([]byte("not a zip"), limits); !errors.Is(err, ArchiveInvalid) {
		t.Fatalf("garbage: %v", err)
	}
	dup := []Entry{entries(map[string]string{"SKILL.md": skillMD})[0], entries(map[string]string{"SKILL.md": skillMD})[0]}
	if _, err := Build(dup, limits); !errors.Is(err, ArchiveInvalid) {
		t.Fatalf("duplicate: %v", err)
	}
}

func TestKindOf(t *testing.T) {
	t.Parallel()
	for p, want := range map[string]FileKind{"a.md": KindMarkdown, "a.sh": KindText, "a.bin": KindBinary, "a.png": KindImage, "a.svg": KindImage} {
		data := map[string][]byte{"a.md": []byte("# x"), "a.sh": []byte("echo"), "a.bin": {0, 1, 2},
			"a.png": []byte("\x89PNG\r\n\x1a\n0000"), "a.svg": []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`)}[p]
		if got, _ := KindOf(p, data); got != want {
			t.Fatalf("%s: %s", p, got)
		}
	}
}
