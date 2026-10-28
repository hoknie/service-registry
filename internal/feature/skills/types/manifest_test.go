package types

import (
	"errors"
	"reflect"
	"testing"

	"svc-registry/internal/platform/apperr"
)

func TestParseManifestReadsMetadata(t *testing.T) {
	t.Parallel()
	m, err := ParseManifest([]byte("\ufeff---\r\nname: go-testing\r\ndescription: Table-driven tests\r\nallowed-tools: Bash Read\r\n" +
		"metadata:\r\n  title: Go testing\r\n  tags: [go, Testing]\r\n  author: Platform team\r\n  version: v1.2.0+build.5\r\n  image: assets/cover.png\r\n---\r\n# Go\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := Manifest{Name: "go-testing", Description: "Table-driven tests", AllowedTools: []string{"Bash", "Read"}, Title: "Go testing",
		Tags: []string{"go", "testing"}, Author: "Platform team", Image: "assets/cover.png", Version: "1.2.0"}
	if !reflect.DeepEqual(m, want) {
		t.Fatalf("got %+v", m)
	}
	if m, err = ParseManifest([]byte("---\nname: x\ndescription: d\nmetadata:\n  tags: \"go, sql\"\n---\n")); err != nil || !reflect.DeepEqual(m.Tags, []string{"go", "sql"}) {
		t.Fatalf("tags as a string: %v %v", m.Tags, err)
	}
}

func TestParseManifestRefusesBadFields(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		text   string
		fields []apperr.Field
	}{
		"bad name":       {"---\nname: Go Testing\ndescription: d\n---\n", []apperr.Field{{Path: "name", Code: "invalid"}}},
		"no description": {"---\nname: x\n---\n", []apperr.Field{{Path: "description", Code: "required"}}},
		"no frontmatter": {"# Title\n", []apperr.Field{{Path: "frontmatter", Code: "missing"}}},
		"bad version":    {"---\nname: x\ndescription: d\nmetadata:\n  version: one\n---\n", []apperr.Field{{Path: "metadata.version", Code: "invalid"}}},
		"bad yaml":       {"---\nname: [x\n---\n", []apperr.Field{{Path: "frontmatter", Code: "invalid"}}},
	} {
		_, err := ParseManifest([]byte(c.text))
		var e *apperr.Error
		if !errors.As(err, &e) || e.Code != "validation.skill_manifest_invalid" || !reflect.DeepEqual(e.Fields, c.fields) {
			t.Fatalf("%s: %v %+v", name, err, e)
		}
	}
}
