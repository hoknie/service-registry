package types

import (
	"errors"
	"reflect"
	"testing"
)

func TestResolveVersion(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		form, manifest string
		existing       []string
		want           string
		explicit       bool
	}{
		{"", "", nil, "0.1.0", false},
		{"", "1.2.0", nil, "1.2.0", true},
		{"", "", []string{"1.2.0"}, "1.2.1", false},
		{"v2.0.0", "1.0.0", []string{"1.2.0"}, "2.0.0", true},
		{"", "", []string{"1.4.0", "2.0.0-rc.1"}, "2.0.0", false},
		{"", "", []string{"2.0.0", "1.4.0"}, "2.0.1", false},
	} {
		got, explicit, err := ResolveVersion(c.form, c.manifest, c.existing)
		if err != nil || got != c.want || explicit != c.explicit {
			t.Fatalf("%+v: %s %v %v", c, got, explicit, err)
		}
	}
	if _, _, err := ResolveVersion("1.2", "", nil); !errors.Is(err, InvalidVersion) {
		t.Fatal(err)
	}
}

func versions(vs ...string) []Version {
	out := make([]Version, len(vs))
	for i, v := range vs {
		out[i] = Version{Version: v}
	}
	return out
}

func available(vs []Version) []string {
	var out []string
	for _, v := range vs {
		if v.Available {
			out = append(out, v.Version)
		}
	}
	return out
}

func TestPolicyMergeAndAvailability(t *testing.T) {
	t.Parallel()
	org, skill := Policy{Range: new("^1")}, Policy{Range: new("^2")}
	e := Merge(&skill, &org)
	if e.Range != "^2" || e.From["range"] != FromSkill || e.From["prerelease"] != FromDefault {
		t.Fatalf("%+v", e)
	}
	e = Merge(nil, &org)
	vs := e.Mark(versions("1.3.0", "2.0.0", "1.4.0-rc.1"), true)
	if got := available(vs); !reflect.DeepEqual(got, []string{"1.3.0"}) {
		t.Fatal(got)
	}
	e = Merge(&Policy{Prerelease: new(true)}, &org)
	if got := available(e.Mark(versions("1.3.0", "2.0.0", "1.4.0-rc.1"), true)); !reflect.DeepEqual(got, []string{"1.4.0-rc.1", "1.3.0"}) {
		t.Fatal(got)
	}
	e = Merge(&Policy{Exclude: &[]string{"2.0.0"}}, nil)
	vs = e.Mark(versions("1.3.0", "2.0.0"), true)
	if got := available(vs); !reflect.DeepEqual(got, []string{"1.3.0"}) {
		t.Fatal(got)
	}
	if got := available(e.Mark(versions("1.3.0"), false)); got != nil {
		t.Fatal("a draft has available versions")
	}
}

func TestPickVersions(t *testing.T) {
	t.Parallel()
	e := Merge(nil, nil)
	vs := e.Mark(versions("1.3.0", "2.0.0", "1.4.1"), true)
	for selector, want := range map[string]string{"": "2.0.0", "^1": "1.4.1", "1.3.0": "1.3.0", "~1.3": "1.3.0"} {
		got, err := Pick(vs, selector, false)
		if err != nil || got == nil || got.Version != want {
			t.Fatalf("%q: %+v %v", selector, got, err)
		}
	}
	if got, _ := Pick(vs, "3.0.0", false); got != nil {
		t.Fatal("missing version picked")
	}
	if _, err := Pick(vs, "^^1", false); !errors.Is(err, InvalidVersion) {
		t.Fatal(err)
	}
	if _, err := ValidatePolicy(Policy{Range: new("^^1")}); !errors.Is(err, InvalidRange) {
		t.Fatal(err)
	}
}
