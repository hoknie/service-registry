package types

import (
	"errors"
	"reflect"
	"testing"
)

func TestFindSkillDirs(t *testing.T) {
	t.Parallel()
	paths := []string{"README.md", "skills/go-testing/SKILL.md", "skills/go-testing/nested/SKILL.md", "skills/sql/SKILL.md",
		"other/x/SKILL.md", "skills/go-testing/reference.md"}
	if got := FindSkillDirs(paths, "skills"); !reflect.DeepEqual(got, []string{"skills/go-testing", "skills/sql"}) {
		t.Fatal(got)
	}
	if got := FindSkillDirs([]string{"SKILL.md", "a/SKILL.md"}, ""); !reflect.DeepEqual(got, []string{""}) {
		t.Fatal(got)
	}
	if got := FilesUnder(paths, "skills/go-testing"); len(got) != 3 {
		t.Fatal(got)
	}
}

func TestTagPatterns(t *testing.T) {
	t.Parallel()
	p, err := ParseTagPattern("{name}/v{version}")
	if err != nil {
		t.Fatal(err)
	}
	if name, v, ok := p.Match("go-testing/v1.1.0"); !ok || name != "go-testing" || v != "1.1.0" {
		t.Fatal(name, v, ok)
	}
	if _, _, ok := p.Match("go-testing/v1.1"); ok {
		t.Fatal("not semver matched")
	}
	all, _ := ParseTagPattern(DefaultTagPattern)
	if _, v, ok := all.Match("v2.0.0-rc.1"); !ok || v != "2.0.0-rc.1" || !all.ForAll() {
		t.Fatal(v, ok)
	}
	for _, bad := range []string{"release", "{version}-{version}", "v{version}{x}", ""} {
		if _, err := ParseTagPattern(bad); !errors.Is(err, InvalidTagPattern) {
			t.Fatalf("%q: %v", bad, err)
		}
	}
}
