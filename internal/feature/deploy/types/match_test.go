package types

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/google/uuid"
)

var (
	apiID    = uuid.MustParse("01890a5d-ac96-774b-bcce-b302099a8057")
	offID    = uuid.MustParse("01890a5d-ac96-774b-bcce-b302099a8058")
	projects = map[string]*ProjectRef{
		"acme/backend/api":     {ID: apiID, Observe: true},
		apiID.String():         {ID: apiID, Observe: true},
		"acme/backend/billing": {ID: offID, Observe: false},
	}
)

func resolve(ref string) (*ProjectRef, error) { return projects[ref], nil }

func deployment(name string, ann map[string]string) Workload {
	return Workload{Kind: KindDeployment, Namespace: "backend", Name: name, Annotations: ann,
		Labels: map[string]string{"app.kubernetes.io/name": name}, Containers: []Container{{Name: "app", Image: "registry.example:5000/acme/" + name + ":1.4.2"}}}
}

func TestMatchWorkload(t *testing.T) {
	rules := []Rule{{Label: "app.kubernetes.io/name", Project: "acme/{namespace}/{label:app.kubernetes.io/name}"}}
	tests := []struct {
		name        string
		w           Workload
		rules       []Rule
		project     *uuid.UUID
		reason      Reason
		service     string
		environment string
	}{
		{"path annotation", deployment("billing-svc", map[string]string{AnnotationProject: "acme/backend/api"}), nil, &apiID, "", "billing-svc", "production"},
		{"uuid annotation and overrides", deployment("x", map[string]string{AnnotationProject: apiID.String(), AnnotationService: " API ", AnnotationEnvironment: "Staging"}), nil, &apiID, "", "api", "staging"},
		{"template annotation", Workload{Kind: KindDeployment, Name: "w", TemplateAnnotations: map[string]string{AnnotationProject: "/acme/backend/api/"}}, nil, &apiID, "", "w", "production"},
		{"unknown project", deployment("x", map[string]string{AnnotationProject: "acme/nope"}), nil, nil, ReasonProjectNotFound, "", ""},
		{"no annotation", deployment("redis", nil), nil, nil, ReasonNoAnnotation, "", ""},
		{"rule", deployment("api", nil), rules, &apiID, "", "api", "production"},
		{"rule without project", deployment("redis", nil), rules, nil, ReasonNoAnnotation, "", ""},
		{"bad service", deployment("x", map[string]string{AnnotationProject: "acme/backend/api", AnnotationService: "-bad-"}), nil, nil, ReasonInvalidService, "", ""},
		{"bad environment", deployment("x", map[string]string{AnnotationProject: "acme/backend/api", AnnotationEnvironment: "a b"}), nil, nil, ReasonInvalidEnvironment, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := MatchWorkload(tt.w, "Production", tt.rules, resolve)
			if err != nil {
				t.Fatal(err)
			}
			if (m.Project == nil) != (tt.project == nil) || (m.Project != nil && m.Project.ID != *tt.project) || m.Reason != tt.reason {
				t.Fatalf("%+v", m)
			}
			if tt.project != nil && (m.Service != tt.service || m.Environment != tt.environment) {
				t.Fatalf("%+v", m)
			}
		})
	}
	m, _ := MatchWorkload(deployment("x", map[string]string{AnnotationProject: "acme/backend/api", AnnotationBranch: "refs/heads/release/1.4",
		AnnotationCommit: "ABCDEF1"}), "production", nil, resolve)
	if *m.Branch != "release/1.4" || *m.Commit != "abcdef1" || *m.Annotation != "acme/backend/api" {
		t.Fatalf("%+v", m)
	}
	failing := func(string) (*ProjectRef, error) { return nil, errors.New("db down") }
	if _, err := MatchWorkload(deployment("x", map[string]string{AnnotationProject: "a"}), "p", nil, failing); err == nil {
		t.Fatal("resolver error lost")
	}
}

func TestVersionOf(t *testing.T) {
	for want, w := range map[string]Workload{
		"2.0.0":      {Labels: map[string]string{LabelVersion: "2.0.0"}, Containers: []Container{{Image: "a:1"}}},
		"2.1.0":      {TemplateLabels: map[string]string{LabelVersion: "2.1.0"}},
		"1.4.2":      {Containers: []Container{{Image: "registry:5000/acme/api:1.4.2@sha256:abc"}}},
		"sha256:abc": {Containers: []Container{{Image: "registry:5000/acme/api@sha256:abc"}}},
	} {
		if got := VersionOf(w); got == nil || *got != want {
			t.Errorf("%s: %v", want, got)
		}
	}
	if VersionOf(Workload{Containers: []Container{{Image: "registry:5000/acme/api"}}}) != nil {
		t.Error("no tag, no digest")
	}
}

func TestLongWorkloadValuesAreBounded(t *testing.T) {
	long := strings.Repeat("v", 256)
	w := deployment("api", map[string]string{AnnotationProject: "missing/" + strings.Repeat("ж", 2000)})
	w.Labels[LabelVersion] = long
	m, err := MatchWorkload(w, "production", nil, resolve)
	if err != nil {
		t.Fatal(err)
	}
	if m.Version != nil {
		t.Errorf("version %q", *m.Version)
	}
	if m.Annotation == nil || utf8.RuneCountInString(*m.Annotation) != MaxAnnotationLen {
		t.Errorf("annotation %v", m.Annotation)
	}
	w.Labels[LabelVersion] = long[:255]
	if m, _ = MatchWorkload(w, "production", nil, resolve); m.Version == nil {
		t.Error("a 255-character version is kept")
	}
}
