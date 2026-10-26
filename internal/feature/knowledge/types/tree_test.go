package types

import (
	"reflect"
	"strings"
	"testing"
)

func TestLevelGroupsOneDirectory(t *testing.T) {
	files := []FileInfo{
		{Path: "README.md"},
		{Path: "docs/guide.md"},
		{Path: "docs/new.md", Meta: Meta{Git: GitUntracked}},
		{Path: "docs/api/v1.md"},
		{Path: "drafts/a.md", Meta: Meta{Git: GitIgnored}},
		{Path: "drafts/b/c.md", Meta: Meta{Git: GitIgnored}},
		{Path: "openspec/specs/x/spec.md"},
	}
	root, ok := Level(files, "")
	if !ok {
		t.Fatal("root")
	}
	if got := []TreeDir{{"docs", "docs", 3, GitUntracked}, {"drafts", "drafts", 2, GitIgnored}, {"openspec", "openspec", 1, ""}}; !reflect.DeepEqual(root.Dirs, got) {
		t.Errorf("dirs %+v", root.Dirs)
	}
	if len(root.Files) != 1 || root.Files[0].Path != "README.md" {
		t.Errorf("files %+v", root.Files)
	}
	docs, ok := Level(files, "docs")
	if !ok || len(docs.Files) != 2 || len(docs.Dirs) != 1 || docs.Dirs[0].Path != "docs/api" {
		t.Errorf("docs %+v %v", docs, ok)
	}
	if _, ok := Level(files, "nope"); ok {
		t.Error("missing directory")
	}
	if _, ok := Level(files, "doc"); ok {
		t.Error("a prefix of a name is not a directory")
	}
}

func TestNormalizeDir(t *testing.T) {
	for in, want := range map[string]string{"": "", "/docs/": "docs", "a/b": "a/b"} {
		if got, ok := NormalizeDir(in); !ok || got != want {
			t.Errorf("%q: %q %v", in, got, ok)
		}
	}
	for _, in := range []string{"a//b", "../x", "a/./b"} {
		if _, ok := NormalizeDir(in); ok {
			t.Errorf("%q accepted", in)
		}
	}
}

func TestMarkdownTitle(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("я", 250)
	for in, want := range map[string]string{
		"```bash\n# not a title\n```\n\n# Руководство по **API**\n": "Руководство по API",
		"Intro\n\n## Sub\n":                 "",
		"Title here\n=====\n\ntext":         "Title here",
		"# [Docs](http://x) and `code` #\n": "Docs and code",
		"~~~\n# hidden\n~~~\n":              "",
		"# " + long:                         strings.Repeat("я", 200),
		"#hashtag\n":                        "",
	} {
		if got := MarkdownTitle(in); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
	if !IsMarkdown("docs/A.MD") || IsMarkdown("a.yaml") {
		t.Fatal("markdown detection")
	}
}
