package links

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func ptr[T any](v T) *T { return &v }

func TestTemplateRecordRules(t *testing.T) {
	node := uuid.Must(uuid.NewV7())
	tpl := "https://grafana.example/d/{project.slug}"
	tests := []struct {
		name    string
		linkKey string
		in      PutTemplate
		want    error
	}{
		{"template", "grafana", PutTemplate{KindKey: "grafana", Template: &tpl}, nil},
		{"disabled", "grafana", PutTemplate{KindKey: "grafana", Disabled: ptr(true)}, nil},
		{"disabled with empty template", "grafana", PutTemplate{KindKey: "grafana", Template: ptr(""), Disabled: ptr(true)}, nil},
		{"disabled with template", "grafana", PutTemplate{KindKey: "grafana", Template: &tpl, Disabled: ptr(true)}, InvalidTemplate},
		{"no template", "grafana", PutTemplate{KindKey: "grafana"}, InvalidTemplate},
		{"bad link key", "Grafana Business", PutTemplate{KindKey: "grafana", Template: &tpl}, InvalidKindKey},
		{"bad kind key", "grafana", PutTemplate{KindKey: "", Template: &tpl}, UnknownKind},
		{"position", "grafana", PutTemplate{KindKey: "grafana", Template: &tpl, Position: ptr(int64(-1))}, InvalidPosition},
		{"unknown variable", "grafana", PutTemplate{KindKey: "grafana", Template: ptr("https://x.example/{team}")}, UnknownVariable},
		{"not a url", "grafana", PutTemplate{KindKey: "grafana", Template: ptr("{path|raw}")}, InvalidLinkURL},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateTemplate(node, tt.linkKey, tt.in)
			if !errors.Is(err, tt.want) && codeOf(err) != codeOf(tt.want) {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
			if err == nil && (got.NodeID != node || got.LinkKey != "grafana" || got.Disabled != (got.Template == nil)) {
				t.Fatalf("%+v", got)
			}
		})
	}
}

func codeOf(err error) string {
	var coded interface{ Code() string }
	if errors.As(err, &coded) {
		return coded.Code()
	}
	return ""
}

func TestNearestTemplateWinsAndDisabledHides(t *testing.T) {
	org, folder, project := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	rec := func(node uuid.UUID, key, kind string, kindPos int32, disabled bool) Template {
		tp := Template{NodeID: node, LinkKey: key, KindKey: kind, KindPosition: kindPos, Disabled: disabled}
		if !disabled {
			tp.Template = ptr("https://" + key + ".example/")
		}
		return tp
	}
	eff := Effective([][]Template{
		{},
		{rec(folder, "grafana", "grafana", 20, false), rec(folder, "sentry", "sentry", 30, true)},
		{rec(org, "grafana", "grafana", 20, false), rec(org, "sentry", "sentry", 30, false),
			rec(org, "grafana-business", "grafana", 20, false), rec(org, "logs", "logs", 10, false)},
	})
	var got []string
	for _, tp := range eff {
		got = append(got, tp.LinkKey+"@"+map[uuid.UUID]string{org: "org", folder: "folder", project: "project"}[tp.NodeID])
		if !tp.Inherited {
			t.Fatalf("%s is not marked inherited", tp.LinkKey)
		}
	}
	want := []string{"logs@org", "grafana@folder", "grafana-business@org", "sentry@folder"}
	if len(got) != len(want) {
		t.Fatalf("%v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%v", got)
		}
	}
	links := Expand(Context{ProjectID: project, ProjectSlug: "api"}, eff, "")
	if len(links) != 3 {
		t.Fatalf("disabled sentry must give no link: %+v", links)
	}
	own := Effective([][]Template{{rec(project, "grafana", "grafana", 20, false)}, {rec(org, "grafana", "grafana", 20, false)}})
	if len(own) != 1 || own[0].Inherited || own[0].NodeID != project {
		t.Fatalf("%+v", own)
	}
}
