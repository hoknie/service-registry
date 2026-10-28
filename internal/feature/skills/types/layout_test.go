package types

import (
	"slices"
	"testing"
)

func TestRepositoryLinksSplitIntoAddressBranchAndRoot(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		in   string
		want RepoLink
		ok   bool
	}{
		{"https://github.com/apollographql/skills/tree/main/skills", RepoLink{"https://github.com/apollographql/skills", "main", "skills"}, true},
		{"https://github.com/Jeffallan/claude-skills/tree/main", RepoLink{"https://github.com/Jeffallan/claude-skills", "main", ""}, true},
		{"https://gitlab.example.com/platform/agent-skills/-/tree/dev/a/b?ref_type=heads", RepoLink{"https://gitlab.example.com/platform/agent-skills", "dev", "a/b"}, true},
		{"https://codeberg.org/acme/skills/src/branch/main/skills", RepoLink{"https://codeberg.org/acme/skills", "main", "skills"}, true},
		{"https://github.com/acme/skills", RepoLink{"https://github.com/acme/skills", "", ""}, false},
	} {
		got, ok := ParseRepoLink(c.in)
		if ok != c.ok || got != c.want {
			t.Fatalf("%s: %+v %v", c.in, got, ok)
		}
	}
}

func TestRootsLayoutsAndSlugs(t *testing.T) {
	t.Parallel()
	roots, err := CleanRoots([]string{"/skills/", "plugins/sec"})
	if err != nil || !slices.Equal(roots, []string{"skills", "plugins/sec"}) {
		t.Fatalf("roots %v %v", roots, err)
	}
	if _, err := CleanRoots([]string{"a", "a"}); err == nil {
		t.Fatal("repeat accepted")
	}
	if r, _ := CleanRoots(nil); !slices.Equal(r, []string{""}) {
		t.Fatalf("default roots %v", r)
	}
	if _, _, err := CleanLayout(LayoutField, nil); err == nil {
		t.Fatal("field without group_field accepted")
	}
	if l, f, err := CleanLayout("", nil); err != nil || l != LayoutFlat || f != nil {
		t.Fatalf("default layout %q %v %v", l, f, err)
	}
	m := Manifest{Fields: map[string]any{"subdomain": "Digital Forensics", "metadata": map[string]any{"category": []any{"Web", "x"}}}}
	for _, c := range []struct{ layout, field, root, dir, want string }{
		{LayoutField, "subdomain", "skills", "skills/vol", "Digital Forensics"},
		{LayoutField, "metadata.category", "", "vol", "Web"},
		{LayoutField, "missing", "", "vol", ""},
		{LayoutTree, "", "skills", "skills/lang/python-pro", "lang"},
		{LayoutTree, "", "skills", "skills/go", ""},
		{LayoutFlat, "", "skills", "skills/lang/python-pro", ""},
	} {
		if got := Place(c.layout, c.field, c.root, c.dir, m); got != c.want {
			t.Fatalf("%+v: %q", c, got)
		}
	}
	for in, want := range map[string]string{"Digital Forensics": "digital-forensics", "Сетевая безопасность": "setevaya-bezopasnost", "!!": "group"} {
		if got := SlugOf(in); got != want {
			t.Fatalf("%q: %q", in, got)
		}
	}
	dirs := FindSkillDirsIn([]string{"skills/a/SKILL.md", "plugins/sec/b/SKILL.md", "other/c/SKILL.md"}, []string{"skills", "plugins/sec"})
	if !slices.Equal(dirs, []string{"plugins/sec/b", "skills/a"}) || RootOf("plugins/sec/b", []string{"skills", "plugins/sec"}) != "plugins/sec" {
		t.Fatalf("dirs %v", dirs)
	}
	if got := MissingRoots([]string{"skills/a/SKILL.md"}, []string{"skills", "nope"}); !slices.Equal(got, []string{"nope"}) {
		t.Fatalf("missing %v", got)
	}
}
