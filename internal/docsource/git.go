package docsource

import (
	"context"
	"errors"
	"io"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"

	"svc-registry/internal/knowledge"
)

type localGit struct {
	path  string
	roots []string
	repo  *git.Repository
}

func (g *localGit) open() error {
	if g.repo != nil {
		return nil
	}
	resolved, err := Resolve(g.roots, g.path)
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
	g.repo = repo
	return nil
}

func (g *localGit) Heads(context.Context) (map[string]string, string, error) {
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
	return heads, def, nil
}

func (g *localGit) Tree(_ context.Context, _, head string) ([]knowledge.Entry, bool, error) {
	if err := g.open(); err != nil {
		return nil, false, err
	}
	commit, err := g.repo.CommitObject(plumbing.NewHash(head))
	if err != nil {
		return nil, false, knowledge.FailSourceUnreadable
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, false, knowledge.FailSourceUnreadable
	}
	var out []knowledge.Entry
	err = tree.Files().ForEach(func(f *object.File) error {
		if f.Mode == filemode.Regular || f.Mode == filemode.Executable {
			out = append(out, knowledge.Entry{Path: f.Name, BlobSHA: f.Hash.String(), Size: f.Size})
		}
		return nil
	})
	if err != nil {
		return nil, false, knowledge.FailSourceUnreadable
	}
	return out, false, nil
}

func (g *localGit) Read(_ context.Context, e knowledge.Entry, limit int64) ([]byte, error) {
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
