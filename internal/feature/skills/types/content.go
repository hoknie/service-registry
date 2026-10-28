package types

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"slices"
	"strings"

	knowledgetypes "svc-registry/internal/feature/knowledge/types"
)

type Limits struct {
	TotalBytes int64
	FileBytes  int64
	Files      int
}

type Entry struct {
	Path string
	Open func() (io.ReadCloser, error)
}

type Blob struct {
	File
	Data []byte
}

type Content struct {
	Files    []Blob
	Manifest Manifest
	Digest   string
	Size     int64
}

func UnpackZip(data []byte, l Limits) (Content, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return Content{}, ArchiveInvalid
	}
	entries := make([]Entry, 0, len(r.File))
	for _, f := range r.File {
		mode := f.Mode()
		if mode.IsDir() || mode&fs.ModeSymlink != 0 || strings.HasSuffix(f.Name, "/") {
			continue
		}
		if f.Flags&0x1 != 0 {
			return Content{}, ArchiveInvalid
		}
		entries = append(entries, Entry{Path: f.Name, Open: f.Open})
	}
	return Build(entries, l)
}

func Build(entries []Entry, l Limits) (Content, error) {
	files := map[string]Entry{}
	for _, e := range entries {
		p, err := CleanPath(e.Path)
		if err != nil {
			return Content{}, err
		}
		if Ignored(p) {
			continue
		}
		if _, dup := files[p]; dup {
			return Content{}, ArchiveInvalid
		}
		files[p] = Entry{Path: p, Open: e.Open}
	}
	files = stripRoot(files)
	if _, ok := files[ManifestFile]; !ok {
		return Content{}, ManifestMissing
	}
	if l.Files > 0 && len(files) > l.Files {
		return Content{}, ContentTooLarge
	}
	return read(files, l)
}

func stripRoot(files map[string]Entry) map[string]Entry {
	if _, ok := files[ManifestFile]; ok || len(files) == 0 {
		return files
	}
	var root string
	for p := range files {
		first, _, found := strings.Cut(p, "/")
		if !found || (root != "" && first != root) {
			return files
		}
		root = first
	}
	if _, ok := files[root+"/"+ManifestFile]; !ok {
		return files
	}
	out := make(map[string]Entry, len(files))
	for p, e := range files {
		rest := strings.TrimPrefix(p, root+"/")
		out[rest] = Entry{Path: rest, Open: e.Open}
	}
	return out
}

func read(files map[string]Entry, l Limits) (Content, error) {
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	out := Content{Files: make([]Blob, 0, len(paths))}
	for _, p := range paths {
		data, err := readLimited(files[p], l.FileBytes)
		if err != nil {
			return Content{}, err
		}
		out.Size += int64(len(data))
		if l.TotalBytes > 0 && out.Size > l.TotalBytes {
			return Content{}, ContentTooLarge
		}
		out.Files = append(out.Files, NewBlob(p, data))
	}
	m, err := ParseManifest(out.Files[slices.IndexFunc(out.Files, func(b Blob) bool { return b.Path == ManifestFile })].Data)
	if err != nil {
		return Content{}, err
	}
	out.Manifest = m
	out.Digest = Digest(out.Files)
	return out, nil
}

func readLimited(e Entry, limit int64) ([]byte, error) {
	rc, err := e.Open()
	if err != nil {
		return nil, ArchiveInvalid
	}
	defer rc.Close()
	src := io.Reader(rc)
	if limit > 0 {
		src = io.LimitReader(rc, limit+1)
	}
	data, err := io.ReadAll(src)
	if err != nil {
		return nil, ArchiveInvalid
	}
	if limit > 0 && int64(len(data)) > limit {
		return nil, ContentTooLarge
	}
	return data, nil
}

func NewBlob(p string, data []byte) Blob {
	sum := sha256.Sum256(data)
	kind, ct := KindOf(p, data)
	b := Blob{File: File{Path: p, Size: int64(len(data)), SHA256: sum[:], Kind: kind, ContentType: ct}, Data: data}
	if kind == KindMarkdown {
		if t := knowledgetypes.MarkdownTitle(string(data)); t != "" {
			b.Title = &t
		}
	}
	return b
}

func Digest(files []Blob) string {
	sorted := slices.Clone(files)
	slices.SortFunc(sorted, func(a, b Blob) int { return strings.Compare(a.Path, b.Path) })
	h := sha256.New()
	for _, f := range sorted {
		_, _ = io.WriteString(h, f.Path+"\x00"+hex.EncodeToString(f.SHA256)+"\n")
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func (c Content) File(p string) (Blob, bool) {
	for _, b := range c.Files {
		if b.Path == p {
			return b, true
		}
	}
	return Blob{}, false
}
