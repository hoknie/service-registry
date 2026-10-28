package source

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"

	forgetypes "svc-registry/internal/feature/forge/types"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
)

const unpackFactor = 8

func (r *remote) Snapshot(ctx context.Context, commit string, limit int64) (fs.FS, func(), error) {
	repo, err := r.current(ctx)
	if err != nil {
		return nil, nil, err
	}
	body, err := r.client.Archive(ctx, repo, commit)
	if errors.Is(err, forgetypes.ErrArchiveUnsupported) {
		return nil, nil, knowledgetypes.ErrSnapshotUnsupported
	}
	if err != nil {
		return nil, nil, err
	}
	defer body.Close()
	return Unpack(body, limit)
}

type capped struct {
	r    io.Reader
	left int64
}

func (c *capped) Read(p []byte) (int, error) {
	if c.left <= 0 {
		return 0, knowledgetypes.ErrSnapshotTooLarge
	}
	if int64(len(p)) > c.left {
		p = p[:c.left]
	}
	n, err := c.r.Read(p)
	c.left -= int64(n)
	return n, err
}

func Unpack(body io.Reader, limit int64) (fs.FS, func(), error) {
	dir, err := os.MkdirTemp("", "svcr-snapshot-")
	if err != nil {
		return nil, nil, err
	}
	release := func() { _ = os.RemoveAll(dir) }
	if err := unpack(dir, &capped{r: body, left: limit + 1}, limit*unpackFactor); err != nil {
		release()
		return nil, nil, err
	}
	return os.DirFS(dir), release, nil
}

func unpack(dir string, body io.Reader, budget int64) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	gz, err := gzip.NewReader(body)
	if err != nil {
		return tooLarge(err)
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return tooLarge(err)
		}
		name, ok := entryName(h.Name)
		if !ok || h.Typeflag != tar.TypeReg {
			continue
		}
		if budget -= h.Size; budget < 0 {
			return knowledgetypes.ErrSnapshotTooLarge
		}
		if err := writeEntry(root, name, tr); err != nil {
			return tooLarge(err)
		}
	}
}

func tooLarge(err error) error {
	if errors.Is(err, knowledgetypes.ErrSnapshotTooLarge) {
		return knowledgetypes.ErrSnapshotTooLarge
	}
	return &forgetypes.Upstream{Detail: "repository archive is unreadable: " + err.Error()}
}

func entryName(raw string) (string, bool) {
	_, rest, ok := strings.Cut(strings.TrimPrefix(raw, "./"), "/")
	if !ok || rest == "" {
		return "", false
	}
	clean := path.Clean(rest)
	if clean != rest || strings.HasPrefix(clean, "../") || clean == ".." || path.IsAbs(clean) {
		return "", false
	}
	return clean, true
}

func writeEntry(root *os.Root, name string, r io.Reader) error {
	if dir := path.Dir(name); dir != "." {
		if err := root.MkdirAll(dir, 0o750); err != nil {
			return err
		}
	}
	f, err := root.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
