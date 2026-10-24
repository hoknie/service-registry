package types

import (
	"slices"
	"testing"

	"github.com/google/uuid"
)

func TestProjectPath(t *testing.T) {
	t.Parallel()
	id := uuid.MustParse("0199a3a4-0000-7000-8000-000000000001")
	for name, c := range map[string]struct {
		in    string
		id    *uuid.UUID
		slugs []string
	}{
		"uuid":           {in: id.String(), id: &id},
		"path lowered":   {in: "ACME/Backend/Billing", slugs: []string{"acme", "backend", "billing"}},
		"single":         {in: "api", slugs: []string{"api"}},
		"empty segments": {in: "a//b", slugs: []string{"a", "", "b"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			gotID, slugs := ProjectPath(c.in)
			if (gotID == nil) != (c.id == nil) || (gotID != nil && *gotID != *c.id) || !slices.Equal(slugs, c.slugs) {
				t.Fatalf("ProjectPath(%q) = %v %v, want %v %v", c.in, gotID, slugs, c.id, c.slugs)
			}
		})
	}
}

func TestSettingsStored(t *testing.T) {
	t.Parallel()
	s := Settings{}.Stored()
	if s.Namespaces == nil || s.Rules == nil {
		t.Fatalf("stored settings keep nil lists: %#v", s)
	}
	kept := Settings{Namespaces: []string{"a"}, Rules: []Rule{{Label: "l", Project: "p"}}}.Stored()
	if len(kept.Namespaces) != 1 || len(kept.Rules) != 1 {
		t.Fatalf("stored settings lost values: %#v", kept)
	}
}

func TestStatusOf(t *testing.T) {
	t.Parallel()
	if StatusOf(nil) != StatusOK || StatusOf(&Failure{Code: FailUnauthorized}) != StatusError {
		t.Fatal("status does not follow the failure")
	}
}

func TestStoredWorkloads(t *testing.T) {
	t.Parallel()
	n := 0
	next := func() uuid.UUID {
		n++
		return uuid.UUID{byte(n)}
	}
	images := []Image{{}}
	rows, uids := StoredWorkloads([]Observation{
		{UID: "a", Name: "first"},
		{UID: "b", Images: images},
		{UID: "a", Name: "last"},
	}, next)
	if !slices.Equal(uids, []string{"b", "a"}) || len(rows) != 2 {
		t.Fatalf("uids %v rows %d, want [b a] and 2 rows", uids, len(rows))
	}
	if rows[1].Name != "last" || rows[1].Images == nil || len(rows[0].Images) != 1 {
		t.Fatalf("rows %#v", rows)
	}
	if rows[0].ID == rows[1].ID || rows[0].ID == uuid.Nil {
		t.Fatalf("ids %v %v", rows[0].ID, rows[1].ID)
	}
	if rows, uids := StoredWorkloads(nil, next); rows == nil || uids == nil {
		t.Fatal("no observations must give empty non-nil slices")
	}
}

func TestFinishedReportOf(t *testing.T) {
	t.Parallel()
	ok := FinishedReportOf("1.0", 3, ReportSummary{})
	if ok.Status != StatusOK || ok.Failure != nil || ok.Workloads != 3 || ok.Version != "1.0" {
		t.Fatalf("ok report %#v", ok)
	}
	failed := FinishedReportOf("1.0", 0, ReportSummary{Error: &ReportError{Code: "k8s.forbidden", Message: " raw "}})
	if failed.Status != StatusError || failed.Failure == nil || failed.Failure.Code != "k8s.forbidden" || failed.Failure.Message != " raw " {
		t.Fatalf("failed report %#v", failed)
	}
}
