package types

import (
	"os"
	"testing"
)

func snapshot(t *testing.T, layout map[string]string) (*Files, map[string]string) {
	t.Helper()
	contents := map[string]string{}
	paths := []string{}
	for p, file := range layout {
		b, err := os.ReadFile("testdata/" + file)
		if err != nil {
			t.Fatal(err)
		}
		contents[p] = string(b)
		paths = append(paths, p)
	}
	loads := 0
	return NewFiles(paths, func(ps []string) (map[string]string, error) {
		loads++
		out := map[string]string{}
		for _, p := range ps {
			out[p] = contents[p]
		}
		return out, nil
	}, nil), contents
}

func codes(d Document) map[string]int {
	out := map[string]int{}
	for _, p := range d.Problems {
		out[p.Code]++
	}
	return out
}

func parseOpenAPI(t *testing.T, layout map[string]string, p string) Document {
	t.Helper()
	files, contents := snapshot(t, layout)
	f, v, ok := Detect(p, contents[p])
	if !ok {
		t.Fatalf("%s is not detected", p)
	}
	d, err := ParseOpenAPI(p, contents[p], f, v, files)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestOpenAPI30(t *testing.T) {
	t.Parallel()
	d := parseOpenAPI(t, map[string]string{"api/openapi.yaml": "petstore30.yaml", "api/schemas/pet.yaml": "pet.yaml",
		"api/schemas/owner.yaml": "owner.yaml"}, "api/openapi.yaml")
	o := d.OpenAPI
	if o.Info.Title != "Petstore" || len(o.Servers) != 1 || len(o.Operations) != 3 {
		t.Fatalf("model = %+v", o)
	}
	if got := []string{o.Tags[0].Name, o.Tags[1].Name}; got[0] != "pets" || got[1] != "store" {
		t.Fatalf("tags = %v", got)
	}
	post := o.Operations[1]
	if post.Key != "POST /pets" || post.RequestBody == nil || post.RequestBody.Content[0].Schema["ref"] != "Pet" {
		t.Fatalf("post = %+v", post)
	}
	get := o.Operations[2]
	if !get.Deprecated || len(get.Parameters) != 1 || get.Parameters[0].Name != "id" || !get.Parameters[0].Required {
		t.Fatalf("get one = %+v", get)
	}
	ok := get.Responses[0].Content[0].Schema
	props, _ := ok["properties"].(map[string]any)
	if owner, _ := props["owner"].(Schema); owner["format"] != "email" {
		t.Fatalf("external ref is not resolved: %+v", ok)
	}
	if r := get.Responses[1].Content[0].Schema; r["unresolved"] != true || r["ref"] != "https://example.com/common.yaml#/Error" {
		t.Fatalf("remote ref = %+v", r)
	}
	if r := get.Responses[2]; r.Status != "default" || r.Content[0].Schema["unresolved"] != true {
		t.Fatalf("missing ref = %+v", r)
	}
	if c := codes(d); c[CodeRefUnresolved] != 2 || len(c) != 1 {
		t.Fatalf("problems = %+v", d.Problems)
	}
	if len(o.Schemas) != 1 || o.Schemas[0].Schema["required"] == nil {
		t.Fatalf("schemas = %+v", o.Schemas)
	}
}

func TestOpenAPI31AndSwagger(t *testing.T) {
	t.Parallel()
	d := parseOpenAPI(t, map[string]string{"openapi.json": "petstore31.json"}, "openapi.json")
	if *d.SpecVersion != "3.1.0" || len(d.Problems) != 0 || d.OpenAPI.Operations[0].Responses[0].Content[0].Schema["type"] == nil {
		t.Fatalf("3.1 = %+v", d)
	}
	s := parseOpenAPI(t, map[string]string{"swagger.yaml": "swagger.yaml"}, "swagger.yaml")
	o := s.OpenAPI
	if s.Format != FormatSwagger || len(o.Servers) != 1 || o.Servers[0].URL != "https://api.example/v1" {
		t.Fatalf("servers = %+v", o.Servers)
	}
	op := o.Operations[0]
	if op.RequestBody == nil || op.RequestBody.Content[0].MediaType != "application/json" || op.RequestBody.Content[0].Schema["ref"] != "Pet" {
		t.Fatalf("body = %+v", op.RequestBody)
	}
	if len(op.Parameters) != 1 || op.Parameters[0].In != "header" || op.Responses[0].Content[0].Schema["ref"] != "Pet" || o.Schemas[0].Name != "Pet" {
		t.Fatalf("swagger = %+v", o)
	}
}

func TestOpenAPIInvalidStructure(t *testing.T) {
	t.Parallel()
	d := parseOpenAPI(t, map[string]string{"api/openapi.yaml": "badpaths.yaml"}, "api/openapi.yaml")
	if len(d.OpenAPI.Operations) != 0 || len(d.Problems) != 1 || d.Problems[0].Code != CodeInvalidDocument || d.Problems[0].Line == nil || *d.Problems[0].Line != 4 {
		t.Fatalf("problems = %+v", d.Problems)
	}
	if s := d.Summary(); *s.Operations != 0 || s.Problems != 1 {
		t.Fatalf("summary = %+v", s)
	}
}

func TestDetect(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		path, content string
		format        Format
		ok            bool
	}{
		{"api/openapi.yaml", "openapi: 3.1.0\n", FormatOpenAPI, true},
		{"API.JSON", `{"openapi":"3.0.1"}`, FormatOpenAPI, true},
		{"swagger.json", `{"swagger":"2.0"}`, FormatSwagger, true},
		{"deploy/values.yaml", "replicas: 2\n", "", false},
		{"old.yaml", "openapi: 2.0\n", "", false},
		{"broken.yaml", "openapi: [\n", "", false},
		{"list.json", `[1,2]`, "", false},
		{"pets.proto", "", FormatProto, true},
		{"README.md", "openapi: 3.0.0", "", false},
	} {
		f, _, ok := Detect(c.path, c.content)
		if f != c.format || ok != c.ok {
			t.Errorf("Detect(%q) = %q, %v", c.path, f, ok)
		}
	}
	for p, want := range map[string]bool{"openapi.json": true, "api/Swagger.yml": true, "pets.proto": true, "big.yaml": false} {
		if _, ok := DetectSkipped(p); ok != want {
			t.Errorf("DetectSkipped(%q) = %v", p, ok)
		}
	}
}

func TestSnapshotPaths(t *testing.T) {
	t.Parallel()
	files := NewFiles([]string{"proto/pets/v1/pets.proto", "third_party/google/api/annotations.proto", "a/x.proto", "b/x.proto"}, nil, nil)
	for imp, want := range map[string]string{"proto/pets/v1/pets.proto": "proto/pets/v1/pets.proto",
		"google/api/annotations.proto": "third_party/google/api/annotations.proto", "x.proto": "", "missing.proto": ""} {
		if got, _ := files.ResolveImport(imp); got != want {
			t.Errorf("ResolveImport(%q) = %q", imp, got)
		}
	}
	for target, want := range map[string]string{"./schemas/pet.yaml": "api/schemas/pet.yaml", "../common.yaml": "common.yaml",
		"../../out.yaml": "", "https://x/y.yaml": "", "/abs.yaml": ""} {
		if got, _ := ResolveRef("api/openapi.yaml", target); got != want {
			t.Errorf("ResolveRef(%q) = %q", target, got)
		}
	}
}
