package links

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func apiContext() Context {
	return Context{
		ProjectID:   uuid.MustParse("01890a5d-ac96-774b-bcce-b302099a8057"),
		ProjectSlug: "api",
		ProjectName: "Billing API",
		Path:        []string{"acme", "backend", "api"},
		Labels:      map[string]string{"team": "Платежи"},
		Vars:        map[string]string{"grafana_org": "7"},
	}
}

func expandText(t *testing.T, template string, c Context) (string, []string) {
	t.Helper()
	p, err := Parse(template)
	if err != nil {
		t.Fatalf("%q: %v", template, err)
	}
	return p.Expand(c.values(nil))
}

func TestSubstitution(t *testing.T) {
	tests := []struct{ template, want string }{
		{"https://grafana.example/d/svc?var-path={path}&var-name={project.name}",
			"https://grafana.example/d/svc?var-path=acme%2Fbackend%2Fapi&var-name=Billing%20API"},
		{"https://logs.example/{path|raw}/{project.name|lower}", "https://logs.example/acme/backend/api/billing%20api"},
		{`https://x.example/?q={{"a":"{project.slug}"}}`, `https://x.example/?q={"a":"api"}`},
		{"https://x.example/{labels.team}", "https://x.example/%D0%9F%D0%BB%D0%B0%D1%82%D0%B5%D0%B6%D0%B8"},
		{"https://x.example/{labels.team|lower|raw}", "https://x.example/платежи"},
		{"https://x.example/{node.1}/{node.3|url}/{vars.grafana_org}", "https://x.example/acme/api/7"},
		{"https://x.example/{project.id}", "https://x.example/01890a5d-ac96-774b-bcce-b302099a8057"},
		{"https://x.example/a~b_c.d-e{{}}", "https://x.example/a~b_c.d-e{}"},
	}
	for _, tt := range tests {
		got, missing := expandText(t, tt.template, apiContext())
		if got != tt.want || len(missing) != 0 {
			t.Errorf("%q:\n got %q %v\nwant %q", tt.template, got, missing, tt.want)
		}
	}
}

func TestEscapeKeepsOnlyUnreserved(t *testing.T) {
	if got := Escape("a/b c+d?e=f&g#h~i.j_k-l"); got != "a%2Fb%20c%2Bd%3Fe%3Df%26g%23h~i.j_k-l" {
		t.Fatal(got)
	}
}

func TestMissingValuesAreListedOnceAndSorted(t *testing.T) {
	_, missing := expandText(t, "https://x.example/{namespace}/{labels.nope}/{node.4}/{namespace}/{repo.full_path}/{branch}", apiContext())
	want := []string{"branch", "labels.nope", "namespace", "node.4", "repo.full_path"}
	if len(missing) != len(want) {
		t.Fatalf("%v", missing)
	}
	for i := range want {
		if missing[i] != want[i] {
			t.Fatalf("%v", missing)
		}
	}
}

func TestTemplateErrors(t *testing.T) {
	tests := []struct {
		template string
		want     Invalid
		pos      int
	}{
		{"https://x.example/{team}", UnknownVariable, 20},
		{"https://x.example/{labels}", UnknownVariable, 20},
		{"https://x.example/{node.0}", UnknownVariable, 20},
		{"https://x.example/{node.x}", UnknownVariable, 20},
		{"https://x.example/{}", InvalidTemplate, 20},
		{"https://x.example/{project..slug}", InvalidTemplate, 20},
		{"https://x.example/{Project}", InvalidTemplate, 20},
		{"https://x.example/{ path }", InvalidTemplate, 20},
		{"https://x.example/{path|upper}", InvalidTemplate, 25},
		{"https://x.example/{path|}", InvalidTemplate, 25},
		{"https://x.example/{path", InvalidTemplate, 19},
		{"https://x.example/path}", InvalidTemplate, 23},
		{"https://пример.example/}", InvalidTemplate, 24},
		{"https://x.example/\t", InvalidTemplate, 19},
		{"", InvalidTemplate, 1},
	}
	for _, tt := range tests {
		_, err := Parse(tt.template)
		var te *TemplateError
		if !errors.As(err, &te) || te.Invalid != tt.want || te.Pos != tt.pos {
			t.Errorf("%q: %v (want %s at %d)", tt.template, err, tt.want.Code(), tt.pos)
		}
	}
}

func TestCheckOnSave(t *testing.T) {
	for template, want := range map[string]string{
		"https://grafana.example/d/{project.slug}": "",
		"http://grafana.example:3000/":             "",
		"{url|raw}/metrics":                        "",
		"{path|raw}":                               "validation.invalid_link_url",
		"ftp://x.example/{path}":                   "validation.invalid_link_url",
		"https:///{path}":                          "validation.invalid_link_url",
		"https://x.example/{team}":                 "validation.unknown_variable",
		"https://x.example/{path|bold}":            "validation.invalid_template",
	} {
		if got := codeOf(CheckTemplate(template)); got != want {
			t.Errorf("%q: %q, want %q", template, got, want)
		}
	}
}
