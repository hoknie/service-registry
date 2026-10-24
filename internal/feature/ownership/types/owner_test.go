package types

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	catalogtypes "svc-registry/internal/feature/catalog/types"
)

func TestValidateOwners(t *testing.T) {
	t.Parallel()
	many := make([]PutOwner, MaxOwners+1)
	for i := range many {
		many[i] = PutOwner{SubjectKind: "group", Subject: strings.Repeat("g", i+1)}
	}
	for name, tc := range map[string]struct {
		in   []PutOwner
		none bool
		want error
	}{
		"empty":           {nil, false, nil},
		"none":            {nil, true, InvalidOwners},
		"none and owners": {[]PutOwner{{"group", "Platform"}}, true, InvalidOwners},
		"two":             {[]PutOwner{{"group", "Platform"}, {"user", "bob@example.com"}}, false, nil},
		"same name":       {[]PutOwner{{"group", "Platform"}, {"group", " platform "}}, false, InvalidOwners},
		"other kind":      {[]PutOwner{{"group", "bob"}, {"user", "bob"}}, false, nil},
		"bad kind":        {[]PutOwner{{"team", "x"}}, false, catalogtypes.InvalidSubjectKind},
		"too many":        {many, false, InvalidOwners},
	} {
		_, err := ValidateOwners(tc.in, tc.none)
		if !errors.Is(err, tc.want) && (err != nil || tc.want != nil) {
			t.Errorf("%s: err %v, want %v", name, err, tc.want)
		}
	}
}

func TestQueriesAndSets(t *testing.T) {
	t.Parallel()
	for _, q := range []string{"", "p"} {
		if got, err := ValidateCandidateQuery(q); err != nil || got != q {
			t.Errorf("%q: %q %v", q, got, err)
		}
	}
	if _, err := ValidateCandidateQuery(strings.Repeat("x", QueryMaxLen+1)); !errors.Is(err, catalogtypes.InvalidFilter) {
		t.Errorf("long candidate query: %v", err)
	}
	if q, err := ValidateCandidateQuery(" pla "); err != nil || q != "pla" {
		t.Errorf("pla: %q %v", q, err)
	}
	if _, err := ValidateSuggestQuery(strings.Repeat("x", QueryMaxLen+1)); !errors.Is(err, catalogtypes.InvalidFilter) {
		t.Errorf("long: %v", err)
	}
	if s := SetOf(nil); s.Own || s.Items == nil || len(s.Items) != 0 {
		t.Errorf("empty set %+v", s)
	}
	if s := SetOf([]Owner{{NodeID: uuid.New(), Inherited: true}}); s.Own {
		t.Errorf("inherited set %+v", s)
	}
	if s := SetOf([]Owner{{Inherited: true}, {}}); !s.Own {
		t.Errorf("own set %+v", s)
	}
}

func TestOwnSubjects(t *testing.T) {
	t.Parallel()
	g, u, up := uuid.New(), uuid.New(), uuid.New()
	for name, tc := range map[string]struct {
		in        []ResolvedSubject
		inherited []uuid.UUID
		want      int
		err       error
	}{
		"empty":     {nil, nil, 0, nil},
		"both":      {[]ResolvedSubject{{GroupID: &g}, {UserID: &u}}, nil, 2, nil},
		"inherited": {[]ResolvedSubject{{GroupID: &g}, {UserID: &up}}, []uuid.UUID{up}, 1, nil},
		"unknown":   {[]ResolvedSubject{{GroupID: &g}, {}}, nil, 0, catalogtypes.InvalidUnknownSubject},
		"repeated":  {[]ResolvedSubject{{UserID: &u}, {UserID: &u}}, nil, 0, InvalidOwners},
	} {
		got, err := OwnSubjects(tc.in, tc.inherited)
		if !errors.Is(err, tc.err) && (err != nil || tc.err != nil) {
			t.Errorf("%s: err %v, want %v", name, err, tc.err)
		}
		if err == nil && len(got) != tc.want {
			t.Errorf("%s: %d own, want %d", name, len(got), tc.want)
		}
	}
	if (ResolvedSubject{GroupID: &g, UserID: &u}).ID() != &g {
		t.Error("group id first")
	}
}
