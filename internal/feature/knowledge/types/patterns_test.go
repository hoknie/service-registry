package types

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestRefsSelectBranchesAndTagsSeparately(t *testing.T) {
	t.Parallel()
	s := Settings{Refs: []RefPattern{{Pattern: "feat/*", Kind: RefBranch}, {Pattern: "master", Kind: RefBranch}, {Pattern: "v*", Kind: RefTag},
		{Pattern: "v*-release", Kind: RefTag, Release: true}}}
	for name, want := range map[string]bool{"main": true, "master": true, "feat/login": true, "feat/ui/menu": false, "fix/typo": false,
		"v2-preview": false} {
		if got := s.CollectsBranch(name, "main"); got != want {
			t.Errorf("branch %s: %v", name, got)
		}
	}
	for name, want := range map[string]bool{"v2.0.0": true, "v0.1.0-release": true, "master": false, "feat/login": false} {
		if got := s.CollectsTag(name); got != want {
			t.Errorf("tag %s: %v", name, got)
		}
	}
	if !s.ReleaseTag("v0.1.0-release") || s.ReleaseTag("v0.3.0-rc1") || !s.WantsTags() {
		t.Fatal("release patterns")
	}
	if (Settings{Refs: []RefPattern{{Pattern: "x", Kind: RefBranch}}}).WantsTags() {
		t.Fatal("branch patterns want no tags")
	}
}

func TestValidateRefs(t *testing.T) {
	t.Parallel()
	for _, refs := range [][]RefPattern{
		{{Pattern: "main", Kind: RefBranch, Release: true}},
		{{Pattern: "v*", Kind: "commit"}},
		{{Pattern: "v*", Kind: RefTag}, {Pattern: "v*", Kind: RefTag, Release: true}},
		{{Pattern: "v[", Kind: RefTag}},
	} {
		if _, err := ValidateNodeSettings(NodeSettings{Refs: &refs}); !errors.Is(err, InvalidSettings) {
			t.Errorf("%+v: %v", refs, err)
		}
	}
	ok := []RefPattern{{Pattern: "v*", Kind: RefBranch}, {Pattern: "v*", Kind: RefTag, Release: true}}
	if _, err := ValidateNodeSettings(NodeSettings{Refs: &ok}); err != nil {
		t.Fatal(err)
	}
}

func TestTagOrder(t *testing.T) {
	t.Parallel()
	at := time.Unix(100, 0)
	newer := func(a, b string) bool { return CompareTags(TagInfo{a, at}, TagInfo{b, at}) > 0 }
	for _, c := range [][2]string{{"v0.10.0", "v0.9.3"}, {"v0.0.1", "stable"}, {"release-2.4", "release-2.3.9"}, {"v1.2.0", "v1.2"}, {"build10", "build9"}} {
		if !newer(c[0], c[1]) || newer(c[1], c[0]) {
			t.Errorf("%s should be newer than %s", c[0], c[1])
		}
	}
	if CompareTags(TagInfo{"beta", at.Add(time.Second)}, TagInfo{"alpha", at}) <= 0 {
		t.Error("tags without a version follow first sight")
	}
	if got := TagVersion("v0.1.0-release"); !slices.Equal(got, []uint64{0, 1, 0}) {
		t.Errorf("version %v", got)
	}
}

func TestPickTags(t *testing.T) {
	t.Parallel()
	now := time.Unix(1000, 0)
	s := Settings{Refs: []RefPattern{{Pattern: "v*", Kind: RefTag}, {Pattern: "v*-release", Kind: RefTag, Release: true}}}
	found := map[string]string{"v1.0.0": "a", "v1.1.0": "b", "v1.2.0": "c", "other": "d"}
	names := func(tags []Tag) string {
		out := make([]string, len(tags))
		for i, t := range tags {
			out[i] = t.Name
		}
		return strings.Join(out, ",")
	}
	picked, release := PickTags(found, nil, s, 2, now)
	if names(picked) != "v1.2.0,v1.1.0" || release != "" || picked[0].FirstSeen != now {
		t.Fatalf("%v %q", names(picked), release)
	}
	found = map[string]string{"v0.2.0-release": "a", "v0.3.0-rc1": "b"}
	picked, release = PickTags(found, map[string]time.Time{"v0.2.0-release": time.Unix(5, 0)}, s, 1, now)
	if names(picked) != "v0.3.0-rc1,v0.2.0-release" || release != "v0.2.0-release" || picked[1].FirstSeen != time.Unix(5, 0) {
		t.Fatalf("%v %q", names(picked), release)
	}
	if picked, release = PickTags(found, nil, s, 0, now); picked != nil || release != "" {
		t.Fatal("zero limit collects nothing")
	}
}

func TestSelectRef(t *testing.T) {
	t.Parallel()
	str := func(s string) *string { return &s }
	long := strings.Repeat("a", 256)
	for _, c := range [][3]*string{{str("main"), str("v1"), nil}, {nil, str(""), nil}, {nil, str(long), nil}, {nil, nil, str("latest")}} {
		if _, err := SelectRef(c[0], c[1], c[2]); !errors.Is(err, InvalidRef) {
			t.Errorf("%v: %v", c, err)
		}
	}
	if r, err := SelectRef(nil, nil, str("current")); err != nil || !r.Release || r.Kind != RefTag {
		t.Fatalf("%+v %v", r, err)
	}
	if r, err := SelectRef(nil, str("v1.0.0"), nil); err != nil || r.Name != "v1.0.0" || r.Kind != RefTag {
		t.Fatalf("%+v %v", r, err)
	}
	if r, err := SelectRef(nil, nil, nil); err != nil || !r.IsZero() {
		t.Fatalf("%+v %v", r, err)
	}
	if _, err := SelectRef(str(" "), nil, nil); err == nil || errors.Is(err, InvalidRef) {
		t.Fatalf("bad branch: %v", err)
	}
}
