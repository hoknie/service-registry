package types

import (
	"reflect"
	"testing"
)

func TestRightsCanonical(t *testing.T) {
	t.Parallel()
	in := Rights{
		SectionUsers:  {ActionUpdate, ActionRead, ActionRead},
		Section("x"):  {ActionRead},
		SectionGroups: {Action("fly")},
	}
	want := Rights{SectionUsers: {ActionRead, ActionUpdate}}
	if got := in.Canonical(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Canonical() = %v, want %v", got, want)
	}
	if got := Rights(nil).Canonical(); got == nil || len(got) != 0 {
		t.Fatalf("Canonical() of nil = %#v", got)
	}
}

func TestGroupNameKeys(t *testing.T) {
	t.Parallel()
	got := GroupNameKeys([]string{"Ops", "SRE-Team", "dev"})
	if want := []string{"ops", "sre-team", "dev"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("GroupNameKeys = %v, want %v", got, want)
	}
	if got := GroupNameKeys(nil); got == nil || len(got) != 0 {
		t.Fatalf("GroupNameKeys(nil) = %#v", got)
	}
}
