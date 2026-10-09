package docsource

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"svc-registry/internal/knowledge"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func realDir(t *testing.T) string {
	t.Helper()
	d, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return d
}

var all = knowledge.Settings{Include: []string{"**"}}

func TestLocalDirReadsInsideTheRootOnly(t *testing.T) {
	root := realDir(t)
	outside := realDir(t)
	dir := filepath.Join(root, "api")
	write(t, filepath.Join(dir, "README.md"), "# API")
	write(t, filepath.Join(dir, "docs/a.md"), "alpha")
	write(t, filepath.Join(dir, ".git/config"), "[core]")
	write(t, filepath.Join(outside, "secret.md"), "secret")
	if err := os.Symlink(filepath.Join(outside, "secret.md"), filepath.Join(dir, "docs/link.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "out")); err != nil {
		t.Fatal(err)
	}
	f := &Factory{Roots: []string{root}, MaxFileBytes: 1024}
	r, _ := f.Open(context.Background(), knowledge.Source{Kind: knowledge.SourceLocalDir, Path: dir}, all, "")
	heads, def, err := r.Heads(context.Background())
	if err != nil || def != "local" {
		t.Fatal(err, def)
	}
	tree, _, _ := r.Tree(context.Background(), "local", heads["local"])
	var paths []string
	for _, e := range tree {
		paths = append(paths, e.Path)
	}
	if len(paths) != 2 || paths[0] != "README.md" || paths[1] != "docs/a.md" {
		t.Fatalf("%v", paths)
	}
	content, err := r.Read(context.Background(), tree[1], 1024)
	if err != nil || string(content) != "alpha" {
		t.Fatal(string(content), err)
	}
	r2, _ := f.Open(context.Background(), knowledge.Source{Kind: knowledge.SourceLocalDir, Path: dir}, all, "")
	again, _, _ := r2.Heads(context.Background())
	if again["local"] != heads["local"] {
		t.Fatal("same content, another fingerprint")
	}
	write(t, filepath.Join(dir, "docs/a.md"), "beta")
	r3, _ := f.Open(context.Background(), knowledge.Source{Kind: knowledge.SourceLocalDir, Path: dir}, all, "")
	changed, _, _ := r3.Heads(context.Background())
	if changed["local"] == heads["local"] {
		t.Fatal("changed content, same fingerprint")
	}
	for path, want := range map[string]error{outside: knowledge.FailNotAllowed, filepath.Join(root, "none"): knowledge.FailSourceNotFound,
		filepath.Join(dir, "out"): knowledge.FailNotAllowed} {
		r, _ := f.Open(context.Background(), knowledge.Source{Kind: knowledge.SourceLocalDir, Path: path}, all, "")
		if _, _, err := r.Heads(context.Background()); !errors.Is(err, want) {
			t.Errorf("%s: %v", path, err)
		}
	}
}

func TestAllowed(t *testing.T) {
	root := realDir(t)
	if err := Allowed(nil, root); !errors.Is(err, knowledge.ConflictLocalDisabled) {
		t.Fatal(err)
	}
	if err := Allowed([]string{root}, filepath.Join(root, "later/api")); err != nil {
		t.Fatal(err)
	}
	if err := Allowed([]string{root}, root+"-other/x"); !errors.Is(err, knowledge.InvalidPath) {
		t.Fatal(err)
	}
}

func commit(t *testing.T, wt *git.Worktree, dir string, files map[string]string, msg string) plumbing.Hash {
	t.Helper()
	for p, c := range files {
		write(t, filepath.Join(dir, p), c)
		if _, err := wt.Add(p); err != nil {
			t.Fatal(err)
		}
	}
	h, err := wt.Commit(msg, &git.CommitOptions{Author: &object.Signature{Name: "t", Email: "t@example.com", When: time.Now()}})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestLocalGitReadsBranchesAndCommits(t *testing.T) {
	root := realDir(t)
	dir := filepath.Join(root, "repo")
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	wt, _ := repo.Worktree()
	first := commit(t, wt, dir, map[string]string{"README.md": "one", "docs/a.md": "a"}, "first")
	if err := os.Symlink("README.md", filepath.Join(dir, "link.md")); err != nil {
		t.Fatal(err)
	}
	wt.Add("link.md")
	head := commit(t, wt, dir, map[string]string{"README.md": "two"}, "second")
	if err := wt.Checkout(&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName("release/1"), Create: true, Hash: first}); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "README.md"), "uncommitted")
	f := &Factory{Roots: []string{root}}
	r, _ := f.Open(context.Background(), knowledge.Source{Kind: knowledge.SourceLocalGit, Path: dir}, all, "")
	heads, def, err := r.Heads(context.Background())
	if err != nil || heads["master"] != head.String() || heads["release/1"] != first.String() || def != "release/1" {
		t.Fatalf("%v %s %v", heads, def, err)
	}
	tree, _, err := r.Tree(context.Background(), "master", head.String())
	if err != nil || len(tree) != 2 {
		t.Fatalf("%+v %v", tree, err)
	}
	for _, e := range tree {
		if e.Path == "README.md" {
			c, err := r.Read(context.Background(), e, 100)
			if err != nil || string(c) != "two" {
				t.Fatal(string(c), err)
			}
		}
	}
	bare := filepath.Join(root, "bare.git")
	if _, err := git.PlainClone(bare, true, &git.CloneOptions{URL: dir}); err != nil {
		t.Fatal(err)
	}
	rb, _ := f.Open(context.Background(), knowledge.Source{Kind: knowledge.SourceLocalGit, Path: bare}, all, "")
	if heads, _, err := rb.Heads(context.Background()); err != nil || heads["release/1"] == "" && heads["master"] == "" {
		t.Fatal(heads, err)
	}
	plain := filepath.Join(root, "plain")
	write(t, filepath.Join(plain, "x.md"), "x")
	rp, _ := f.Open(context.Background(), knowledge.Source{Kind: knowledge.SourceLocalGit, Path: plain}, all, "")
	if _, _, err := rp.Heads(context.Background()); !errors.Is(err, knowledge.FailNotARepository) {
		t.Fatal(err)
	}
}
