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

	knowledgetypes "svc-registry/internal/feature/knowledge/types"
)

type localGit struct {
	path           string
	roots          []string
	settings       knowledgetypes.Settings
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
	resolved, err := knowledgetypes.ResolveLocalPath(g.roots, g.path)
	if err != nil {
		return err
	}
	repo, err := git.PlainOpenWithOptions(resolved, &git.PlainOpenOptions{DetectDotGit: false})
	switch {
	case errors.Is(err, git.ErrRepositoryNotExists):
		return knowledgetypes.FailNotARepository
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

func (g *localGit) commitEntries(head string) ([]knowledgetypes.Entry, error) {
	commit, err := g.repo.CommitObject(plumbing.NewHash(head))
	if err != nil {
		return nil, knowledgetypes.FailSourceUnreadable
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, knowledgetypes.FailSourceUnreadable
	}
	var out []knowledgetypes.Entry
	err = tree.Files().ForEach(func(f *object.File) error {
		if f.Mode == filemode.Regular || f.Mode == filemode.Executable {
			out = append(out, knowledgetypes.Entry{Path: f.Name, BlobSHA: f.Hash.String(), Size: f.Size})
		}
		return nil
	})
	if err != nil {
		return nil, knowledgetypes.FailSourceUnreadable
	}
	return out, nil
}

func (g *localGit) Heads(ctx context.Context) (map[string]string, string, error) {
	if err := g.open(); err != nil {
		return nil, "", err
	}
	iter, err := g.repo.Branches()
	if err != nil {
		return nil, "", knowledgetypes.FailSourceUnreadable
	}
	heads := map[string]string{}
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		heads[ref.Name().Short()] = ref.Hash().String()
		return nil
	})
	if err != nil {
		return nil, "", knowledgetypes.FailSourceUnreadable
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
		tracked := g.indexed()
		if tracked == nil {
			tracked = make(map[string]bool, len(committed))
			for _, e := range committed {
				tracked[e.Path] = true
			}
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

func (g *localGit) Tags(context.Context) (map[string]string, error) {
	if err := g.open(); err != nil {
		return nil, err
	}
	iter, err := g.repo.Tags()
	if err != nil {
		return nil, knowledgetypes.FailSourceUnreadable
	}
	tags := map[string]string{}
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		hash := ref.Hash()
		if tag, err := g.repo.TagObject(hash); err == nil {
			commit, err := tag.Commit()
			if err != nil {
				return nil
			}
			hash = commit.Hash
		}
		tags[ref.Name().Short()] = hash.String()
		return nil
	})
	if err != nil {
		return nil, knowledgetypes.FailSourceUnreadable
	}
	return tags, nil
}

func (g *localGit) Tree(ctx context.Context, _, head string) ([]knowledgetypes.Entry, bool, error) {
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
			return nil, false, knowledgetypes.FailSourceUnreadable
		}
		return g.wt.entries, false, nil
	}
	out, err := g.commitEntries(head)
	return out, false, err
}

func (g *localGit) Read(_ context.Context, e knowledgetypes.Entry, limit int64) ([]byte, error) {
	if content, ok := g.wt.content[e.BlobSHA]; ok {
		return content, nil
	}
	if err := g.open(); err != nil {
		return nil, err
	}
	blob, err := g.repo.BlobObject(plumbing.NewHash(e.BlobSHA))
	if err != nil {
		return nil, knowledgetypes.FailSourceUnreadable
	}
	r, err := blob.Reader()
	if err != nil {
		return nil, knowledgetypes.FailSourceUnreadable
	}
	defer r.Close()
	content, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, knowledgetypes.FailSourceUnreadable
	}
	return content, nil
}

func (g *localGit) indexed() map[string]bool {
	idx, err := g.repo.Storer.Index()
	if err != nil {
		return nil
	}
	out := make(map[string]bool, len(idx.Entries))
	for _, e := range idx.Entries {
		out[e.Name] = true
	}
	return out
}
