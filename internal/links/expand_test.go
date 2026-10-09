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
