package source

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	knowledgeports "svc-registry/internal/feature/knowledge/ports"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
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

var all = knowledgetypes.Settings{Include: []string{"**"}}

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
	f := &FactoryClient{Roots: []string{root}, MaxFileBytes: 1024}
	r, _ := f.Open(context.Background(), knowledgetypes.Source{Kind: knowledgetypes.SourceLocalDir, Path: dir}, all, "")
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
	r2, _ := f.Open(context.Background(), knowledgetypes.Source{Kind: knowledgetypes.SourceLocalDir, Path: dir}, all, "")
	again, _, _ := r2.Heads(context.Background())
	if again["local"] != heads["local"] {
		t.Fatal("same content, another fingerprint")
	}
	write(t, filepath.Join(dir, "docs/a.md"), "beta")
	r3, _ := f.Open(context.Background(), knowledgetypes.Source{Kind: knowledgetypes.SourceLocalDir, Path: dir}, all, "")
	changed, _, _ := r3.Heads(context.Background())
	if changed["local"] == heads["local"] {
		t.Fatal("changed content, same fingerprint")
	}
	for path, want := range map[string]error{outside: knowledgetypes.FailNotAllowed, filepath.Join(root, "none"): knowledgetypes.FailSourceNotFound,
		filepath.Join(dir, "out"): knowledgetypes.FailNotAllowed} {
		r, _ := f.Open(context.Background(), knowledgetypes.Source{Kind: knowledgetypes.SourceLocalDir, Path: path}, all, "")
		if _, _, err := r.Heads(context.Background()); !errors.Is(err, want) {
			t.Errorf("%s: %v", path, err)
		}
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
	f := &FactoryClient{Roots: []string{root}}
	r, _ := f.Open(context.Background(), knowledgetypes.Source{Kind: knowledgetypes.SourceLocalGit, Path: dir}, all, "")
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
	rb, _ := f.Open(context.Background(), knowledgetypes.Source{Kind: knowledgetypes.SourceLocalGit, Path: bare}, all, "")
	if heads, _, err := rb.Heads(context.Background()); err != nil || heads["release/1"] == "" && heads["master"] == "" {
		t.Fatal(heads, err)
	}
	plain := filepath.Join(root, "plain")
	write(t, filepath.Join(plain, "x.md"), "x")
	rp, _ := f.Open(context.Background(), knowledgetypes.Source{Kind: knowledgetypes.SourceLocalGit, Path: plain}, all, "")
	if _, _, err := rp.Heads(context.Background()); !errors.Is(err, knowledgetypes.FailNotARepository) {
		t.Fatal(err)
	}
}

func TestLocalGitTagsPointAtCommits(t *testing.T) {
	root := realDir(t)
	dir := filepath.Join(root, "repo")
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	wt, _ := repo.Worktree()
	first := commit(t, wt, dir, map[string]string{"README.md": "one"}, "first")
	second := commit(t, wt, dir, map[string]string{"README.md": "two"}, "second")
	if _, err := repo.CreateTag("v0.1.0", first, nil); err != nil {
		t.Fatal(err)
	}
	sig := &object.Signature{Name: "t", Email: "t@example.com", When: time.Unix(1, 0)}
	if _, err := repo.CreateTag("v0.3.0", second, &git.CreateTagOptions{Tagger: sig, Message: "release"}); err != nil {
		t.Fatal(err)
	}
	f := &FactoryClient{Roots: []string{root}}
	r, _ := f.Open(context.Background(), knowledgetypes.Source{Kind: knowledgetypes.SourceLocalGit, Path: dir}, all, "")
	tags, err := r.Tags(context.Background())
	if err != nil || len(tags) != 2 || tags["v0.1.0"] != first.String() || tags["v0.3.0"] != second.String() {
		t.Fatalf("%v %v", tags, err)
	}
	tree, _, err := r.Tree(context.Background(), "v0.1.0", tags["v0.1.0"])
	if err != nil || len(tree) != 1 {
		t.Fatalf("%+v %v", tree, err)
	}
	d, _ := f.Open(context.Background(), knowledgetypes.Source{Kind: knowledgetypes.SourceLocalDir, Path: dir}, all, "")
	if tags, err := d.Tags(context.Background()); err != nil || len(tags) != 0 {
		t.Fatalf("local dir: %v %v", tags, err)
	}
}

func TestLocalGitWorkingTreeOfTheCheckedOutBranch(t *testing.T) {
	ctx := context.Background()
	root := realDir(t)
	dir := filepath.Join(root, "repo")
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	wt, _ := repo.Worktree()
	head := commit(t, wt, dir, map[string]string{"README.md": "one", "docs/old.md": "old", ".gitignore": "docs/build/\n*.tmp.md\n"}, "first")
	if err := wt.Checkout(&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName("release/1"), Create: true, Hash: head}); err != nil {
		t.Fatal(err)
	}
	if err := wt.Checkout(&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName("master")}); err != nil {
		t.Fatal(err)
	}
	f := &FactoryClient{Roots: []string{root}, MaxFileBytes: 1 << 20}
	open := func(working bool) knowledgeports.ReaderPort {
		r, err := f.Open(ctx, knowledgetypes.Source{Kind: knowledgetypes.SourceLocalGit, Path: dir, WorkingTree: working}, all, "")
		if err != nil {
			t.Fatal(err)
		}
		return r
	}

	heads, _, err := open(true).Heads(ctx)
	if err != nil || heads["master"] != head.String() {
		t.Fatalf("a clean working tree keeps the commit: %v %v", heads, err)
	}

	write(t, filepath.Join(dir, "openspec/specs/a/spec.md"), "new spec")
	write(t, filepath.Join(dir, "README.md"), "edited")
	write(t, filepath.Join(dir, "docs/build/out.md"), "ignored dir")
	write(t, filepath.Join(dir, "notes.tmp.md"), "ignored file")
	write(t, filepath.Join(dir, "local.md"), "excluded")
	write(t, filepath.Join(dir, ".git/info/exclude"), "local.md\n")
	write(t, filepath.Join(dir, "sub/.gitignore"), "secret.md\n")
	write(t, filepath.Join(dir, "sub/secret.md"), "nested ignore")
	write(t, filepath.Join(dir, "vendor/mod/.git"), "gitdir: elsewhere")
	write(t, filepath.Join(dir, "vendor/mod/x.md"), "submodule")
	if err := os.Remove(filepath.Join(dir, "docs/old.md")); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.md")
	write(t, outside, "outside")
	if err := os.Symlink(outside, filepath.Join(dir, "escape.md")); err != nil {
		t.Fatal(err)
	}

	r := open(true)
	heads, def, err := r.Heads(ctx)
	if err != nil || def != "master" || !strings.HasPrefix(heads["master"], head.String()+"+worktree:sha256:") || heads["release/1"] != head.String() {
		t.Fatalf("%v %s %v", heads, def, err)
	}
	tree, _, err := r.Tree(ctx, "master", heads["master"])
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, e := range tree {
		paths = append(paths, e.Path)
		if e.Path == "README.md" {
			if c, err := r.Read(ctx, e, 100); err != nil || string(c) != "edited" {
				t.Fatal(string(c), err)
			}
		}
	}
	if got := strings.Join(paths, ","); got != ".gitignore,README.md,openspec/specs/a/spec.md,sub/.gitignore" {
		t.Fatalf("working tree files: %s", got)
	}
	if tree, _, err := r.Tree(ctx, "release/1", heads["release/1"]); err != nil || len(tree) != 3 {
		t.Fatalf("other branches come from commits: %+v %v", tree, err)
	}

	again, _, _ := open(true).Heads(ctx)
	if again["master"] != heads["master"] {
		t.Fatal("an unchanged working tree keeps its head")
	}
	write(t, filepath.Join(dir, "openspec/specs/a/spec.md"), "changed spec")
	if changed, _, _ := open(true).Heads(ctx); changed["master"] == heads["master"] {
		t.Fatal("an edit moves the head")
	}
	if off, _, _ := open(false).Heads(ctx); off["master"] != head.String() {
		t.Fatal("the flag off reads commits", off)
	}

	if err := wt.Checkout(&git.CheckoutOptions{Hash: head, Force: true}); err != nil {
		t.Fatal(err)
	}
	if detached, def, _ := open(true).Heads(ctx); def != "" || detached["master"] != head.String() {
		t.Fatal("a detached HEAD reads commits", detached, def)
	}
}

func TestLocalGitIgnoredFilesMatchingTheIncludePatterns(t *testing.T) {
	ctx := context.Background()
	root := realDir(t)
	dir := filepath.Join(root, "repo")
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	wt, _ := repo.Worktree()
	head := commit(t, wt, dir, map[string]string{"README.md": "# Repo", ".gitignore": "*.md\n/target/\n"}, "first")
	write(t, filepath.Join(dir, "openspec/specs/a/spec.md"), "ignored spec")
	write(t, filepath.Join(dir, "other.md"), "ignored root file")
	write(t, filepath.Join(dir, "target/notes.md"), "build output")
	if err := os.Chmod(filepath.Join(dir, "target"), 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(dir, "target"), 0o755) })
	settings := knowledgetypes.Settings{Include: []string{"openspec/**/*.md", "*.md"}}
	f := &FactoryClient{Roots: []string{root}, MaxFileBytes: 1 << 20}
	open := func(include bool) knowledgeports.ReaderPort {
		src := knowledgetypes.Source{Kind: knowledgetypes.SourceLocalGit, Path: dir, WorkingTree: true, IncludeIgnored: include}
		r, err := f.Open(ctx, src, settings, "")
		if err != nil {
			t.Fatal(err)
		}
		return r
	}

	plain := open(false)
	committed, _, err := plain.Tree(ctx, "master", head.String())
	if err != nil || len(committed) != 2 {
		t.Fatalf("README.md is committed despite *.md: %v %v", committed, err)
	}
	heads, _, err := plain.Heads(ctx)
	if err != nil || heads["master"] != head.String() {
		t.Fatalf("a committed file under an ignore rule is not ignored, the rest stays out: %v %v", heads, err)
	}

	r := open(true)
	heads, _, err = r.Heads(ctx)
	if err != nil || !strings.HasPrefix(heads["master"], head.String()+"+worktree:sha256:") {
		t.Fatalf("ignored target/ must not be walked: %v %v", heads, err)
	}
	tree, _, err := r.Tree(ctx, "master", heads["master"])
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, e := range tree {
		paths = append(paths, e.Path)
	}
	if got := strings.Join(paths, ","); got != "README.md,openspec/specs/a/spec.md,other.md" {
		t.Fatalf("got %s", got)
	}
}

func TestLocalGitMarksFilesOutsideTheIndex(t *testing.T) {
	ctx := context.Background()
	root := realDir(t)
	dir := filepath.Join(root, "repo")
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	wt, _ := repo.Worktree()
	commit(t, wt, dir, map[string]string{"README.md": "# Repo", ".gitignore": "drafts/\n"}, "first")
	write(t, filepath.Join(dir, "docs/new.md"), "untracked")
	write(t, filepath.Join(dir, "docs/staged.md"), "staged")
	if _, err := wt.Add("docs/staged.md"); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "drafts/idea.md"), "ignored")
	settings := knowledgetypes.Settings{Include: []string{"*.md", "docs/**", "drafts/**"}}
	f := &FactoryClient{Roots: []string{root}, MaxFileBytes: 1 << 20}
	r, err := f.Open(ctx, knowledgetypes.Source{Kind: knowledgetypes.SourceLocalGit, Path: dir, WorkingTree: true, IncludeIgnored: true}, settings, "")
	if err != nil {
		t.Fatal(err)
	}
	heads, _, err := r.Heads(ctx)
	if err != nil {
		t.Fatal(err)
	}
	tree, _, err := r.Tree(ctx, "master", heads["master"])
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]knowledgetypes.GitStatus{}
	for _, e := range tree {
		got[e.Path] = e.Git
	}
	want := map[string]knowledgetypes.GitStatus{
		"README.md":      "",
		"docs/new.md":    knowledgetypes.GitUntracked,
		"docs/staged.md": "",
		"drafts/idea.md": knowledgetypes.GitIgnored,
	}
	for p, s := range want {
		if g, ok := got[p]; !ok || g != s {
			t.Errorf("%s: got %q (present %v), want %q", p, g, ok, s)
		}
	}

	files, err := knowledgetypes.Collect(tree, settings, knowledgetypes.Limits{MaxFiles: 10, MaxFileBytes: 1 << 20, MaxSnapshotBytes: 1 << 20},
		func(e knowledgetypes.Entry, limit int64) ([]byte, error) { return r.Read(ctx, e, limit) })
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if file.Meta.Git != want[file.Path] {
			t.Errorf("collected %s: got %q, want %q", file.Path, file.Meta.Git, want[file.Path])
		}
	}
}
