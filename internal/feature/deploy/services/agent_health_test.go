package services

import (
	"testing"
	"time"

	deploytypes "svc-registry/internal/feature/deploy/types"
)

func TestAgentHealth(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	h := NewAgentHealth(func() time.Time { return now })
	if !h.Live() || h.Ready() {
		t.Fatalf("at start: live %v, ready %v", h.Live(), h.Ready())
	}
	now = now.Add(9 * time.Minute)
	if !h.Live() {
		t.Fatal("the stall floor is ten minutes")
	}
	now = now.Add(2 * time.Minute)
	if h.Live() {
		t.Fatal("eleven minutes without a step is a stall")
	}
	h.Contact(5 * time.Minute)
	if !h.Live() || !h.Ready() {
		t.Fatal("a contact is a step and makes the agent ready")
	}
	now = now.Add(14 * time.Minute)
	h.Beat(0)
	if !h.Live() || !h.Ready() {
		t.Fatal("a failed step keeps the agent live; the contact is within three intervals")
	}
	now = now.Add(2 * time.Minute)
	if h.Ready() {
		t.Fatal("no contact for more than three intervals")
	}
	if !h.Live() {
		t.Fatal("the loop keeps stepping")
	}
	now = now.Add(13 * time.Minute)
	if !h.Live() {
		t.Fatal("three intervals of five minutes are longer than the floor")
	}
	now = now.Add(time.Minute)
	if h.Live() {
		t.Fatal("more than three intervals without a step")
	}
}

func TestAgentPauseFollowsTheReportGap(t *testing.T) {
	t.Parallel()
	cases := []struct {
		interval, gap int32
		want          time.Duration
	}{
		{0, 0, 60 * time.Second},
		{60, 30, 60 * time.Second},
		{15, 50, 50 * time.Second},
		{0, 21, 21 * time.Second},
	}
	for _, c := range cases {
		if got := pause(deploytypes.AgentConfig{IntervalSecs: c.interval, ReportGapSecs: c.gap}); got != c.want {
			t.Errorf("pause(%d, %d) = %v, want %v", c.interval, c.gap, got, c.want)
		}
	}
}
