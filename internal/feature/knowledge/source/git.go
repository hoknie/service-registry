package source

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"

	"svc-registry/internal/feature/knowledge"
)

type localGit struct {
	path           string
	roots          []string
	settings       knowledge.Settings
	maxFile        int64
	workingTree    bool
	includeIgnored bool
	dir            string
	repo           *git.Repository
	wtHead         string
	wt             worktree
}

func (g *localGit) open() error {
	if g.repo != nil {
		return nil
	}
	resolved, err := knowledge.ResolveLocalPath(g.roots, g.path)
	if err != nil {
		return err
	}
	repo, err := git.PlainOpenWithOptions(resolved, &git.PlainOpenOptions{DetectDotGit: false})
	switch {
	case errors.Is(err, git.ErrRepositoryNotExists):
		return knowledge.FailNotARepository
	case err != nil:
		return unreadable(err)
	}
	g.repo, g.dir = repo, resolved
	return nil
}

func (g *localGit) checkedOut(heads map[string]string, def string) bool {
	if !g.workingTree || def == "" || heads[def] == "" {
		return false
	}
	_, err := g.repo.Worktree()
	return err == nil
}

func (g *localGit) commitEntries(head string) ([]knowledge.Entry, error) {
	commit, err := g.repo.CommitObject(plumbing.NewHash(head))
	if err != nil {
		return nil, knowledge.FailSourceUnreadable
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, knowledge.FailSourceUnreadable
	}
	var out []knowledge.Entry
	err = tree.Files().ForEach(func(f *object.File) error {
		if f.Mode == filemode.Regular || f.Mode == filemode.Executable {
			out = append(out, knowledge.Entry{Path: f.Name, BlobSHA: f.Hash.String(), Size: f.Size})
		}
		return nil
	})
	if err != nil {
		return nil, knowledge.FailSourceUnreadable
	}
	return out, nil
}

func (g *localGit) Heads(ctx context.Context) (map[string]string, string, error) {
	if err := g.open(); err != nil {
		return nil, "", err
	}
	iter, err := g.repo.Branches()
	if err != nil {
		return nil, "", knowledge.FailSourceUnreadable
	}
	heads := map[string]string{}
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		heads[ref.Name().Short()] = ref.Hash().String()
		return nil
	})
	if err != nil {
		return nil, "", knowledge.FailSourceUnreadable
	}
	def := ""
	if head, err := g.repo.Reference(plumbing.HEAD, false); err == nil && head.Type() == plumbing.SymbolicReference {
		def = head.Target().Short()
	}
	g.wtHead = ""
	if g.checkedOut(heads, def) {
		committed, err := g.commitEntries(heads[def])
		if err != nil {
			return nil, "", err
		}
		tracked := make(map[string]bool, len(committed))
		for _, e := range committed {
			tracked[e.Path] = true
		}
		wt, err := readWorktree(ctx, g.dir, g.settings, g.maxFile, worktreeOptions{includeIgnored: g.includeIgnored, tracked: tracked})
		if err != nil {
			return nil, "", err
		}
		if !wt.sameAs(committed, g.settings) {
			g.wt = wt
			g.wtHead = heads[def] + worktreeMark + wt.fingerprint()
			heads[def] = g.wtHead
		}
	}
	return heads, def, nil
}

func (g *localGit) Tree(ctx context.Context, _, head string) ([]knowledge.Entry, bool, error) {
	if err := g.open(); err != nil {
		return nil, false, err
	}
	if strings.Contains(head, worktreeMark) {
		if g.wtHead == "" {
			if _, _, err := g.Heads(ctx); err != nil {
				return nil, false, err
			}
		}
		if head != g.wtHead {
			return nil, false, knowledge.FailSourceUnreadable
		}
		return g.wt.entries, false, nil
	}
	out, err := g.commitEntries(head)
	return out, false, err
}

func (g *localGit) Read(_ context.Context, e knowledge.Entry, limit int64) ([]byte, error) {
	if content, ok := g.wt.content[e.BlobSHA]; ok {
		return content, nil
	}
	if err := g.open(); err != nil {
		return nil, err
	}
	blob, err := g.repo.BlobObject(plumbing.NewHash(e.BlobSHA))
	if err != nil {
		return nil, knowledge.FailSourceUnreadable
	}
	r, err := blob.Reader()
	if err != nil {
		return nil, knowledge.FailSourceUnreadable
	}
	defer r.Close()
	content, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, knowledge.FailSourceUnreadable
	}
	return content, nil
}
