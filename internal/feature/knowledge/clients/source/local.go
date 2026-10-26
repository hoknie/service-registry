package source

import (
	"context"
	"crypto/sha1" // #nosec G505 -- git object ids are SHA-1 by the git format
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"strings"

	knowledgetypes "svc-registry/internal/feature/knowledge/types"
)

type localDir struct {
	path     string
	roots    []string
	settings knowledgetypes.Settings
	maxFile  int64
	entries  []knowledgetypes.Entry
	content  map[string][]byte
}

func blobSHA(content []byte) string {
	h := sha1.New() // #nosec G401 -- git object ids are SHA-1 by the git format
	fmt.Fprintf(h, "blob %d\x00", len(content))
	h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}

func unreadable(err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return knowledgetypes.FailSourceNotFound
	}
	return knowledgetypes.FailSourceUnreadable
}

func (d *localDir) Heads(ctx context.Context) (map[string]string, string, error) {
	resolved, err := knowledgetypes.ResolveLocalPath(d.roots, d.path)
	if err != nil {
		return nil, "", err
	}
	if st, err := os.Stat(resolved); err != nil || !st.IsDir() {
		return nil, "", knowledgetypes.FailSourceNotFound
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
		case !d.settings.Reads(p):
			return nil
		}
		info, err := e.Info()
		if err != nil {
			return err
		}
		if info.Size() > d.maxFile {
			d.entries = append(d.entries, knowledgetypes.Entry{Path: p, BlobSHA: fmt.Sprintf("size:%d", info.Size()), Size: info.Size()})
			return nil
		}
		f, err := root.Open(p)
		if err != nil {
			return err
		}
		content, err := io.ReadAll(io.LimitReader(f, d.maxFile+1))
		_ = f.Close()
		if err != nil {
			return err
		}
		sha := blobSHA(content)
		d.content[sha] = content
		d.entries = append(d.entries, knowledgetypes.Entry{Path: p, BlobSHA: sha, Size: int64(len(content))})
		return nil
	})
	if err != nil {
		if ctx.Err() != nil {
			return nil, "", ctx.Err()
		}
		return nil, "", unreadable(err)
	}
	slices.SortFunc(d.entries, func(a, b knowledgetypes.Entry) int { return strings.Compare(a.Path, b.Path) })
	h := sha256.New()
	for _, e := range d.entries {
		fmt.Fprintf(h, "%s\x00%s\n", e.Path, e.BlobSHA)
	}
	return map[string]string{knowledgetypes.LocalBranch: "sha256:" + hex.EncodeToString(h.Sum(nil))}, knowledgetypes.LocalBranch, nil
}

func (d *localDir) Tags(context.Context) (map[string]string, error) { return map[string]string{}, nil }

func (d *localDir) Tree(ctx context.Context, _, _ string) ([]knowledgetypes.Entry, bool, error) {
	if d.entries == nil {
		if _, _, err := d.Heads(ctx); err != nil {
			return nil, false, err
		}
	}
	return d.entries, false, nil
}

func (d *localDir) Read(_ context.Context, e knowledgetypes.Entry, limit int64) ([]byte, error) {
	content, ok := d.content[e.BlobSHA]
	if !ok {
		return nil, knowledgetypes.FailSourceUnreadable
	}
	if int64(len(content)) > limit+1 {
		return content[:limit+1], nil
	}
	return content, nil
}
