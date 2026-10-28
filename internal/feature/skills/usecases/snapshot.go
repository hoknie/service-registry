package usecases

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"

	knowledgeports "svc-registry/internal/feature/knowledge/ports"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
)

type tree struct {
	paths   []string
	size    map[string]int64
	open    func(p string, limit int64) (io.ReadCloser, error)
	release func()
}

func readTree(ctx context.Context, reader knowledgeports.ReaderPort, branch, commit string, limit int64) (tree, bool, error) {
	files, release, err := reader.Snapshot(ctx, commit, limit)
	if errors.Is(err, knowledgetypes.ErrSnapshotUnsupported) {
		return listTree(ctx, reader, branch, commit)
	}
	if err != nil {
		return tree{}, false, err
	}
	t := tree{size: map[string]int64{}, release: release}
	err = fs.WalkDir(files, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		t.paths = append(t.paths, p)
		t.size[p] = info.Size()
		return nil
	})
	if err != nil {
		release()
		return tree{}, false, err
	}
	t.open = func(p string, _ int64) (io.ReadCloser, error) { return files.Open(p) }
	return t, false, nil
}

func listTree(ctx context.Context, reader knowledgeports.ReaderPort, branch, commit string) (tree, bool, error) {
	entries, truncated, err := reader.Tree(ctx, branch, commit)
	if err != nil {
		return tree{}, false, err
	}
	byPath := make(map[string]knowledgetypes.Entry, len(entries))
	t := tree{size: map[string]int64{}, release: func() {}}
	for _, e := range entries {
		byPath[e.Path] = e
		t.paths = append(t.paths, e.Path)
		t.size[e.Path] = e.Size
	}
	t.open = func(p string, limit int64) (io.ReadCloser, error) {
		data, err := reader.Read(ctx, byPath[p], limit)
		if err != nil {
			return nil, err
		}
		return io.NopCloser(bytes.NewReader(data)), nil
	}
	return t, truncated, nil
}
