package docsource

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"strings"

	"svc-registry/internal/knowledge"
)

type localDir struct {
	path     string
	roots    []string
	settings knowledge.Settings
	maxFile  int64
	entries  []knowledge.Entry
	content  map[string][]byte
}

func blobSHA(content []byte) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(content))
	h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}

func unreadable(err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return knowledge.FailSourceNotFound
	}
	return knowledge.FailSourceUnreadable
}

func (d *localDir) Heads(ctx context.Context) (map[string]string, string, error) {
	resolved, err := Resolve(d.roots, d.path)
	if err != nil {
		return nil, "", err
	}
	if st, err := os.Stat(resolved); err != nil || !st.IsDir() {
		return nil, "", knowledge.FailSourceNotFound
	}
	root, err := os.OpenRoot(resolved)
	if err != nil {
		return nil, "", unreadable(err)
	}
	defer root.Close()
	d.entries, d.content = nil, map[string][]byte{}
	err = fs.WalkDir(root.FS(), ".", func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		switch {
		case e.IsDir() && e.Name() == ".git":
			return fs.SkipDir
		case e.IsDir() || !e.Type().IsRegular():
			return nil
		case !d.settings.Collects(p):
			return nil
		}
		info, err := e.Info()
		if err != nil {
			return err
		}
		if info.Size() > d.maxFile {
			d.entries = append(d.entries, knowledge.Entry{Path: p, BlobSHA: fmt.Sprintf("size:%d", info.Size()), Size: info.Size()})
			return nil
		}
		f, err := root.Open(p)
		if err != nil {
			return err
		}
		content, err := io.ReadAll(io.LimitReader(f, d.maxFile+1))
		f.Close()
		if err != nil {
			return err
		}
		sha := blobSHA(content)
		d.content[sha] = content
		d.entries = append(d.entries, knowledge.Entry{Path: p, BlobSHA: sha, Size: int64(len(content))})
		return nil
	})
	if err != nil {
		if ctx.Err() != nil {
			return nil, "", ctx.Err()
		}
		return nil, "", unreadable(err)
	}
	slices.SortFunc(d.entries, func(a, b knowledge.Entry) int { return strings.Compare(a.Path, b.Path) })
	h := sha256.New()
	for _, e := range d.entries {
		fmt.Fprintf(h, "%s\x00%s\n", e.Path, e.BlobSHA)
	}
	return map[string]string{knowledge.LocalBranch: "sha256:" + hex.EncodeToString(h.Sum(nil))}, knowledge.LocalBranch, nil
}

func (d *localDir) Tree(ctx context.Context, _, _ string) ([]knowledge.Entry, bool, error) {
	if d.entries == nil {
		if _, _, err := d.Heads(ctx); err != nil {
			return nil, false, err
		}
	}
	return d.entries, false, nil
}

func (d *localDir) Read(_ context.Context, e knowledge.Entry, limit int64) ([]byte, error) {
	content, ok := d.content[e.BlobSHA]
	if !ok {
		return nil, knowledge.FailSourceUnreadable
	}
	if int64(len(content)) > limit+1 {
		return content[:limit+1], nil
	}
	return content, nil
}
