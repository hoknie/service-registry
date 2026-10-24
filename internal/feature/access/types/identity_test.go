package types

import "testing"

func TestLastLoginMethod(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		hasPassword bool
		identities  int64
		want        bool
	}{
		"password and one identity": {hasPassword: true, identities: 1},
		"no password, one identity": {identities: 1, want: true},
		"no password, two":          {identities: 2},
		"no password, none":         {want: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := LastLoginMethod(tc.hasPassword, tc.identities); got != tc.want {
				t.Fatalf("LastLoginMethod = %v, want %v", got, tc.want)
			}
		})
	}
}
