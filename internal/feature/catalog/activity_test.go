package catalog

import (
	"testing"

	"github.com/google/uuid"
)

func TestStateOfPrefersRunningThenQueuedThenFailed(t *testing.T) {
	t.Parallel()
	code := "forge.unauthorized"
	cases := []struct {
		name string
		in   ProcessSignals
		want ProcessState
		code bool
	}{
		{"all", ProcessSignals{Running: true, Queued: true, Failed: true, Code: &code}, StateRunning, false},
		{"queued and failed", ProcessSignals{Queued: true, Failed: true, Code: &code}, StateQueued, false},
		{"failed", ProcessSignals{Failed: true, Code: &code}, StateFailed, true},
		{"none", ProcessSignals{Code: &code}, StateIdle, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			p := ProcessOf(c.in)
			if p.State != c.want || (p.Code != nil) != c.code {
				t.Fatalf("got %s code %v, want %s code %v", p.State, p.Code, c.want, c.code)
			}
		})
	}
}

func TestSummarizeCountsEachProjectOncePerState(t *testing.T) {
	t.Parallel()
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	got := Summarize([]uuid.UUID{a, b, c, a}, map[uuid.UUID][]Process{
		a: {{State: StateRunning}, {State: StateRunning}, {State: StateFailed}},
		b: {{State: StateQueued}},
		c: {{State: StateIdle}},
	})
	if got != (Summary{Running: 1, Queued: 1, Failed: 1}) {
		t.Fatalf("got %+v", got)
	}
}

func TestParseProcessState(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"running", "queued", "failed"} {
		if _, ok := ParseProcessState(v); !ok {
			t.Fatalf("%s refused", v)
		}
	}
	for _, v := range []string{"idle", "", "RUNNING"} {
		if _, ok := ParseProcessState(v); ok {
			t.Fatalf("%s accepted", v)
		}
	}
}
