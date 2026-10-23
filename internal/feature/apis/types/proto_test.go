package types

import (
	"testing"
)

func parseProto(t *testing.T, layout map[string]string, p string) Document {
	t.Helper()
	files, contents := snapshot(t, layout)
	d, err := ParseProto(t.Context(), p, contents[p], files)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestProtoLinked(t *testing.T) {
	t.Parallel()
	d := parseProto(t, map[string]string{"proto/pets/v1/pets.proto": "pets.proto",
		"third_party/google/api/annotations.proto": "annotations.proto", "third_party/google/api/http.proto": "http.proto"},
		"proto/pets/v1/pets.proto")
	if len(d.Problems) != 0 {
		t.Fatalf("problems = %+v", d.Problems)
	}
	p := d.Proto
	if *d.SpecVersion != "proto3" || *p.Package != "pets.v1" || p.Options["go_package"] != "example.com/pets/v1;petsv1" {
		t.Fatalf("file = %+v", p)
	}
	svc := p.Services[0]
	if svc.FullName != "pets.v1.PetService" || *svc.Description != "Pets service." || len(svc.Methods) != 3 {
		t.Fatalf("service = %+v", svc)
	}
	get, create, watch := svc.Methods[0], svc.Methods[1], svc.Methods[2]
	if get.HTTP == nil || get.HTTP.Method != "GET" || get.HTTP.Path != "/v1/pets/{id}" || get.HTTP.Body != nil || *get.Description != "Gets a pet." {
		t.Fatalf("get = %+v", get)
	}
	if create.HTTP == nil || create.HTTP.Body == nil || *create.HTTP.Body != "*" {
		t.Fatalf("create = %+v", create.HTTP)
	}
	if watch.ClientStreaming || !watch.ServerStreaming || watch.Output != ".pets.v1.Event" {
		t.Fatalf("watch = %+v", watch)
	}
	pet := p.Messages[0]
	if pet.FullName != "pets.v1.Pet" || *pet.Description != "A pet." || len(pet.Fields) != 9 {
		t.Fatalf("pet = %+v", pet)
	}
	f := map[string]Field{}
	for _, x := range pet.Fields {
		f[x.Name] = x
	}
	if *f["id"].Description != "Identifier." || f["id"].Label != nil || *f["tags"].Label != "repeated" || f["counts"].Map == nil ||
		f["counts"].Map.Value != "int32" || *f["nick"].Label != "optional" || *f["dog"].Oneof != "kind" || f["born"].Type != ".google.protobuf.Timestamp" {
		t.Fatalf("fields = %+v", f)
	}
	names := map[string]bool{}
	for _, m := range p.Messages {
		names[m.FullName] = true
	}
	if !names["pets.v1.Pet.Toy"] || len(p.Messages) != 5 || p.Enums[0].FullName != "pets.v1.Status" || *p.Enums[0].Values[0].Description != "Unknown." {
		t.Fatalf("messages = %v, enums = %+v", names, p.Enums)
	}
	if s := d.Summary(); *s.Services != 1 || *s.Methods != 3 || *s.Messages != 5 || *s.Title != "pets.v1" {
		t.Fatalf("summary = %+v", s)
	}
}

func TestProtoImportMissingAndBroken(t *testing.T) {
	t.Parallel()
	d := parseProto(t, map[string]string{"shop/v1/item.proto": "money.proto"}, "shop/v1/item.proto")
	if c := codes(d); c[CodeImportNotFound] != 1 || len(d.Problems) != 1 {
		t.Fatalf("problems = %+v", d.Problems)
	}
	if d.Proto.Messages[0].Fields[0].Type != "common.v1.Money" {
		t.Fatalf("field = %+v", d.Proto.Messages[0].Fields[0])
	}
	b := parseProto(t, map[string]string{"broken.proto": "broken.proto"}, "broken.proto")
	if len(b.Problems) != 1 || b.Problems[0].Code != CodeInvalidDocument || b.Problems[0].Line == nil || *b.Problems[0].Line != 3 || len(b.Proto.Messages) != 0 {
		t.Fatalf("broken = %+v", b.Problems)
	}
	n := parseProto(t, map[string]string{"proto/pets/v1/pets.proto": "pets.proto"}, "proto/pets/v1/pets.proto")
	if c := codes(n); c[CodeImportNotFound] != 1 || n.Proto.Services[0].Methods[0].HTTP == nil || n.Proto.Services[0].Methods[2].Output != "Event" {
		t.Fatalf("unlinked = %+v / %+v", n.Problems, n.Proto.Services[0].Methods)
	}
}

func TestProtoTypeFilesPointToImportedDefinitions(t *testing.T) {
	t.Parallel()
	d := parseProto(t, map[string]string{"shop/v1/item.proto": "money.proto", "proto/common/v1/money.proto": "common_money.proto"},
		"shop/v1/item.proto")
	if len(d.Problems) != 0 {
		t.Fatalf("problems = %+v", d.Problems)
	}
	if d.Proto.Messages[0].Fields[0].Type != ".common.v1.Money" || len(d.Proto.TypeFiles) != 1 ||
		d.Proto.TypeFiles["common.v1.Money"] != "proto/common/v1/money.proto" {
		t.Fatalf("type files = %+v", d.Proto.TypeFiles)
	}
	local := parseProto(t, map[string]string{"proto/pets/v1/pets.proto": "pets.proto",
		"third_party/google/api/annotations.proto": "annotations.proto", "third_party/google/api/http.proto": "http.proto"},
		"proto/pets/v1/pets.proto")
	if len(local.Proto.TypeFiles) != 0 {
		t.Fatalf("well-known and local types are not listed: %+v", local.Proto.TypeFiles)
	}
}

func TestVendored(t *testing.T) {
	t.Parallel()
	for p, want := range map[string]bool{
		"third_party/google/api/http.proto": true, "proto/google/type/date.proto": true, "vendor/x/openapi.yaml": true,
		"web/node_modules/pkg/openapi.json": true, "api/third_party/a.proto": true, "proto/buf/validate/validate.proto": true,
		"proto/pets/v1/pets.proto": false, "api/openapi.yaml": false, "docs/google/openapi.yaml": false,
	} {
		if got := Vendored(p); got != want {
			t.Errorf("%s: %v", p, got)
		}
	}
}
