package deploy

import (
	"testing"
	"time"
)

func TestPendingVersions(t *testing.T) {
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	v := func(s string) *string { return &s }
	if NextPending(v("1.4.2"), v("1.4.2"), nil, now) != nil {
		t.Fatal("same as current")
	}
	if NextPending(v("1.4.2"), nil, nil, now) != nil {
		t.Fatal("no version")
	}
	p := NextPending(v("1.4.2"), v("1.4.3"), nil, now)
	if p == nil || p.Version != "1.4.3" || !p.Since.Equal(now) {
		t.Fatalf("%+v", p)
	}
	later := now.Add(5 * time.Minute)
	if q := NextPending(v("1.4.2"), v("1.4.3"), p, later); q != p {
		t.Fatal("the same version keeps waiting since the first sighting")
	}
	if q := NextPending(v("1.4.2"), v("1.4.4"), p, later); q.Version != "1.4.4" || !q.Since.Equal(later) {
		t.Fatalf("%+v", q)
	}
	if NextPending(v("1.4.3"), v("1.4.3"), p, later) != nil {
		t.Fatal("confirmed by an event")
	}
	if NextPending(nil, v("1.0.0"), nil, now) == nil {
		t.Fatal("no deployment yet")
	}
	if Due(p, now.Add(9*time.Minute), 10*time.Minute) || !Due(p, now.Add(10*time.Minute), 10*time.Minute) || Due(nil, now, 0) {
		t.Fatal("window")
	}
}
