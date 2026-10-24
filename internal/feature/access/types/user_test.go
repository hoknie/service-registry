package types

import (
	"testing"

	"github.com/google/uuid"
)

func TestUserChangesRules(t *testing.T) {
	t.Parallel()
	disabled, active := StatusDisabled, StatusActive
	yes, no := true, false
	tests := map[string]struct {
		in                          UserChanges
		disables, demotes, services bool
	}{
		"empty":        {in: UserChanges{}},
		"disable":      {in: UserChanges{Status: &disabled}, disables: true},
		"activate":     {in: UserChanges{Status: &active}},
		"demote":       {in: UserChanges{IsSuperadmin: &no}, demotes: true},
		"promote":      {in: UserChanges{IsSuperadmin: &yes}},
		"make service": {in: UserChanges{IsService: &yes}, services: true},
		"make person":  {in: UserChanges{IsService: &no}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := tc.in.Disables(); got != tc.disables {
				t.Errorf("Disables() = %v, want %v", got, tc.disables)
			}
			if got := tc.in.Demotes(); got != tc.demotes {
				t.Errorf("Demotes() = %v, want %v", got, tc.demotes)
			}
			if got := tc.in.MakesService(); got != tc.services {
				t.Errorf("MakesService() = %v, want %v", got, tc.services)
			}
		})
	}
}

func TestLastSuperadmin(t *testing.T) {
	t.Parallel()
	a, b := uuid.New(), uuid.New()
	tests := map[string]struct {
		admins []uuid.UUID
		id     uuid.UUID
		want   bool
	}{
		"only one is the user": {admins: []uuid.UUID{a}, id: a, want: true},
		"only one is another":  {admins: []uuid.UUID{b}, id: a},
		"two admins":           {admins: []uuid.UUID{a, b}, id: a},
		"none":                 {id: a},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := LastSuperadmin(tc.admins, tc.id); got != tc.want {
				t.Fatalf("LastSuperadmin = %v, want %v", got, tc.want)
			}
		})
	}
}
