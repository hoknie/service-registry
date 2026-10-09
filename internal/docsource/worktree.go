package docsource

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"

	"svc-registry/internal/knowledge"
)

const worktreeMark = "+worktree:sha256:"

type worktree struct {
	entries []knowledge.Entry
	content map[string][]byte
}

func streamBlobSHA(head []byte, size int64, rest io.Reader) (string, error) {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", size)
	h.Write(head)
	if _, err := io.Copy(h, rest); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func readWorktree(ctx context.Context, dir string, settings knowledge.Settings, maxFile int64) (worktree, error) {
	patterns, _ := gitignore.ReadPatterns(osfs.New(dir, osfs.WithBoundOS()), nil)
	ignored := gitignore.NewMatcher(patterns)
	root, err := os.OpenRoot(dir)
	if err != nil {
		return worktree{}, unreadable(err)
	}
	defer root.Close()
	w := worktree{content: map[string][]byte{}}
	err = fs.WalkDir(root.FS(), ".", func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if p == "." {
			return nil
		}
		parts := strings.Split(p, "/")
		if e.IsDir() {
			if e.Name() == ".git" || ignored.Match(parts, true) {
				return fs.SkipDir
			}
			if _, err := root.Lstat(path.Join(p, ".git")); err == nil {
				return fs.SkipDir
			}
			return nil
		}
		if !e.Type().IsRegular() || ignored.Match(parts, false) || !settings.Collects(p) {
			return nil
		}
		f, err := root.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			return err
		}
		head, err := io.ReadAll(io.LimitReader(f, maxFile+1))
		if err != nil {
			return err
		}
		if int64(len(head)) <= maxFile {
			sha := blobSHA(head)
			w.content[sha] = head
			w.entries = append(w.entries, knowledge.Entry{Path: p, BlobSHA: sha, Size: int64(len(head))})
			return nil
		}
		sha, err := streamBlobSHA(head, info.Size(), f)
		if err != nil {
			return err
		}
		w.entries = append(w.entries, knowledge.Entry{Path: p, BlobSHA: sha, Size: info.Size()})
		return nil
	})
	if err != nil {
		if ctx.Err() != nil {
			return worktree{}, ctx.Err()
		}
		return worktree{}, unreadable(err)
	}
	slices.SortFunc(w.entries, func(a, b knowledge.Entry) int { return strings.Compare(a.Path, b.Path) })
	return w, nil
}

func (w worktree) sameAs(commit []knowledge.Entry, settings knowledge.Settings) bool {
	var kept []knowledge.Entry
	for _, e := range commit {
		if settings.Collects(e.Path) {
			kept = append(kept, e)
		}
	}
	slices.SortFunc(kept, func(a, b knowledge.Entry) int { return strings.Compare(a.Path, b.Path) })
	return slices.EqualFunc(kept, w.entries, func(a, b knowledge.Entry) bool { return a.Path == b.Path && a.BlobSHA == b.BlobSHA })
}

func (w worktree) fingerprint() string {
	var b bytes.Buffer
	for _, e := range w.entries {
		fmt.Fprintf(&b, "%s\x00%s\n", e.Path, e.BlobSHA)
	}
	sum := sha256.Sum256(b.Bytes())
	return hex.EncodeToString(sum[:])
}
