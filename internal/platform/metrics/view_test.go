package metrics_test

import (
	"context"
	"math"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"svc-registry/internal/platform/metrics"
)

func sampled(t *testing.T, calls *atomic.Int32) *metrics.Metrics {
	t.Helper()
	m := metrics.New("1.2.3", nil)
	counted := metrics.SampleSource("catalog", func(context.Context) ([]metrics.Sample, error) {
		calls.Add(1)
		return []metrics.Sample{{Name: "svcr_catalog_nodes", Labels: map[string]string{"kind": "project"}, Value: 3}}, nil
	})
	m.Sampler(time.Minute, metrics.RuntimeSource(), counted).SampleNow(context.Background())
	m.HTTPRequest("GET", "/api/health", 200, 3*time.Millisecond)
	m.HTTPRequest("GET", "/api/health", 200, 3*time.Millisecond)
	return m
}

func TestViewReadsReadyValues(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	m := sampled(t, &calls)
	var v metrics.View
	for range 10 {
		var err error
		if v, err = m.View("host-a", 60); err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("view must not sample, calls = %d", calls.Load())
	}
	if v.Instance != "host-a" || v.Version != "1.2.3" || v.GoVersion == "" || v.RefreshSecs != 60 || v.StartedAt.IsZero() {
		t.Fatalf("header %+v", v)
	}
	if _, ok := v.SampledAt["runtime"]; !ok {
		t.Fatalf("sampled_at %v", v.SampledAt)
	}
	if g := v.Runtime["go_goroutines"]; g == nil || *g <= 0 {
		t.Fatalf("goroutines %v", g)
	}
	if v.Runtime["go_gc_duration_seconds_count"] == nil {
		t.Fatal("gc count missing")
	}
	if _, ok := v.Runtime["process_max_fds"]; !ok {
		t.Fatal("every runtime name must be present, null when absent")
	}
	found := false
	for _, f := range v.Families {
		if f.Name == "svcr_http_requests_total" && f.Type == "counter" && f.Series == 1 {
			found = true
		}
	}
	if !found {
		t.Fatalf("families %+v", v.Families)
	}
}

func TestViewWithoutSamplesHasNullRuntime(t *testing.T) {
	t.Parallel()
	v, err := metrics.New("1", nil).View("h", 60)
	if err != nil {
		t.Fatal(err)
	}
	if v.Runtime["go_goroutines"] != nil || len(v.SampledAt) != 0 {
		t.Fatalf("runtime %v sampled %v", v.Runtime, v.SampledAt)
	}
}

func TestFamilyMatchesPage(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	m := sampled(t, &calls)
	body := scrape(t, m)
	f, ok, err := m.Family("svcr_http_requests_total")
	if err != nil || !ok || len(f.Rows) != 1 {
		t.Fatalf("family %+v %v %v", f, ok, err)
	}
	row := f.Rows[0]
	if row.Labels["route"] != "/api/health" || row.Value == nil {
		t.Fatalf("row %+v", row)
	}
	has(t, body, `svcr_http_requests_total{method="GET",route="/api/health",status="200"} `+strconv.FormatFloat(*row.Value, 'g', -1, 64))
	h, ok, _ := m.Family("svcr_http_request_duration_seconds")
	if !ok || h.Type != "histogram" || len(h.Rows) != 1 {
		t.Fatalf("histogram %+v", h)
	}
	last := h.Rows[0].Buckets[len(h.Rows[0].Buckets)-1]
	if !math.IsInf(last.Le, 1) || last.Count != *h.Rows[0].Count || *h.Rows[0].Count != 2 {
		t.Fatalf("buckets %+v", h.Rows[0].Buckets)
	}
	s, ok, _ := m.Family("go_gc_duration_seconds")
	if !ok || s.Type != "summary" || s.Rows[0].Count == nil || len(s.Rows[0].Quantiles) == 0 {
		t.Fatalf("summary %+v", s)
	}
	if _, ok, _ := m.Family("no_such_metric"); ok {
		t.Fatal("unknown family found")
	}
	if calls.Load() != 1 {
		t.Fatalf("family must not sample, calls = %d", calls.Load())
	}
}
