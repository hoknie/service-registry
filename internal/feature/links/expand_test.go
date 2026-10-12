package links

import (
	"testing"

	"github.com/google/uuid"
)

func logsTemplate(text string) []Template {
	return []Template{{NodeID: uuid.Must(uuid.NewV7()), LinkKey: "logs", KindKey: "logs", Template: &text}}
}

func withDeployments() Context {
	c := apiContext()
	c.Deployments = []Deployment{
		{Service: "api", Environment: "staging", Version: "1.1", Namespace: ptr("api-stg")},
		{Service: "api", Environment: "production", Version: "1.0", Namespace: ptr("api-prod")},
	}
	return c
}

func TestALinkPerCurrentEnvironment(t *testing.T) {
	links := Expand(withDeployments(), logsTemplate("https://logs.example/?ns={namespace}"), "")
	if len(links) != 2 {
		t.Fatalf("%+v", links)
	}
	for i, want := range []struct{ env, url string }{{"production", "https://logs.example/?ns=api-prod"}, {"staging", "https://logs.example/?ns=api-stg"}} {
		l := links[i]
		if *l.Environment != want.env || *l.Service != "api" || l.URL == nil || *l.URL != want.url || len(l.Missing) != 0 {
			t.Errorf("%d: %+v", i, l)
		}
	}
	only := Expand(withDeployments(), logsTemplate("https://logs.example/?ns={namespace}"), "production")
	if len(only) != 1 || *only[0].Environment != "production" {
		t.Fatalf("%+v", only)
	}
}

func TestATemplateWithoutDeploymentVariablesGivesOneLink(t *testing.T) {
	links := Expand(withDeployments(), logsTemplate("https://logs.example/{project.slug}"), "")
	if len(links) != 1 || links[0].Service != nil || links[0].Environment != nil || *links[0].URL != "https://logs.example/api" {
		t.Fatalf("%+v", links)
	}
}

func TestMissingVariableGivesALinkWithoutAddress(t *testing.T) {
	links := Expand(apiContext(), logsTemplate("https://logs.example/?ns={namespace}"), "")
	if len(links) != 1 || links[0].URL != nil || len(links[0].Missing) != 1 || links[0].Missing[0] != "namespace" {
		t.Fatalf("%+v", links)
	}
	c := withDeployments()
	c.Deployments[0].Namespace = nil
	links = Expand(c, logsTemplate("https://logs.example/?ns={namespace}"), "staging")
	if len(links) != 1 || links[0].URL != nil || links[0].Missing[0] != "namespace" {
		t.Fatalf("%+v", links)
	}
}

func TestBranchAndRename(t *testing.T) {
	c := apiContext()
	c.Branch = ptr("feature/x")
	links := Expand(c, logsTemplate("https://ci.example/{project.slug}/tree/{branch|raw}"), "")
	if *links[0].URL != "https://ci.example/api/tree/feature/x" {
		t.Fatal(*links[0].URL)
	}
	c.Path[1] = "core"
	links = Expand(c, logsTemplate("https://x.example/{path|raw}"), "")
	if *links[0].URL != "https://x.example/acme/core/api" {
		t.Fatal(*links[0].URL)
	}
}

func TestExpansionThatIsNotAURLHasNoAddress(t *testing.T) {
	c := withDeployments()
	c.Deployments = c.Deployments[:1]
	c.Deployments[0].URL = ptr("not a url")
	links := Expand(c, logsTemplate("{url|raw}"), "")
	if len(links) != 1 || links[0].URL != nil || len(links[0].Missing) != 0 {
		t.Fatalf("%+v", links)
	}
}

func eventContext() Context {
	c := withDeployments()
	c.Kinds = map[string]KindInfo{
		"logs":    {Position: 10, Icon: "scroll-text"},
		"runbook": {Position: 60, Icon: "book-open-check", IconURL: ptr("https://ci.example/favicon.png")},
	}
	c.Deployments[1].Links = []EventLink{
		{Key: "runbook", Kind: "runbook", URL: "https://ci.example/runs/42", Title: ptr("Пайплайн")},
		{Key: "logs", Kind: "logs", URL: "https://logs.example/event"},
		{Key: "pipeline", Kind: "pipeline", URL: "https://ci.example/p"},
	}
	return c
}

func TestEventLinksJoinAndReplaceTemplateLinksOfTheirPair(t *testing.T) {
	templates := logsTemplate("https://logs.example/?ns={namespace}")
	templates[0].KindPosition, templates[0].KindIcon = 10, "scroll-text"
	got := ExpandProject(eventContext(), templates, "")
	type row struct{ key, env, source, url, icon string }
	var rows []row
	for _, l := range got {
		rows = append(rows, row{l.LinkKey, *l.Environment, l.Source, *l.URL, l.Icon.Kind + ":" + l.Icon.Name})
	}
	want := []row{
		{"logs", "production", SourceEvent, "https://logs.example/event", "builtin:scroll-text"},
		{"logs", "staging", SourceTemplate, "https://logs.example/?ns=api-stg", "builtin:scroll-text"},
		{"runbook", "production", SourceEvent, "https://ci.example/runs/42", "url:book-open-check"},
		{"pipeline", "production", SourceEvent, "https://ci.example/p", "builtin:link"},
	}
	if len(rows) != len(want) {
		t.Fatalf("%+v", rows)
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Errorf("%d: got %+v, want %+v", i, rows[i], want[i])
		}
	}
	if *got[2].Title != "Пайплайн" || got[2].NodeID != eventContext().ProjectID || got[2].Inherited || len(got[2].Missing) != 0 {
		t.Errorf("%+v", got[2])
	}
}

func TestEventLinksFollowTheEnvironmentFilter(t *testing.T) {
	got := ExpandProject(eventContext(), nil, "staging")
	if len(got) != 0 {
		t.Fatalf("%+v", got)
	}
	if got := ExpandProject(withDeployments(), logsTemplate("https://logs.example/{project.slug}"), ""); len(got) != 1 || got[0].Source != SourceTemplate {
		t.Fatalf("%+v", got)
	}
}

func TestBadgesPreferTheEventAddress(t *testing.T) {
	templates := logsTemplate("https://logs.example/{project.slug}")
	templates[0].KindPosition = 10
	badges := Badges(eventContext(), templates)
	if len(badges) != 3 || badges[0].LinkKey != "logs" || *badges[0].URL != "https://logs.example/event" {
		t.Fatalf("%+v", badges)
	}
}

func TestIconsCarryTheKindIconName(t *testing.T) {
	tpl := Template{KindIcon: "chart-line", KindIconURL: ptr("https://grafana.example/favicon.ico")}
	if i := tpl.LinkIcon(); i.Kind != "url" || i.URL != "https://grafana.example/favicon.ico" || i.Name != "chart-line" {
		t.Errorf("%+v", i)
	}
	tpl.IconFile = ptr("a.png")
	if i := tpl.LinkIcon(); i.Kind != "file" || i.Name != "chart-line" {
		t.Errorf("%+v", i)
	}
	if i := (Template{KindIcon: "bug"}).LinkIcon(); i.Kind != "builtin" || i.Name != "bug" {
		t.Errorf("%+v", i)
	}
}
