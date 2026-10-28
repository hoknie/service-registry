package metrics_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"svc-registry/internal/platform/metrics"
)

func scrape(t *testing.T, m *metrics.Metrics) string {
	t.Helper()
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

func has(t *testing.T, body, line string) {
	t.Helper()
	if !strings.Contains(body, line) {
		t.Fatalf("missing %q in\n%s", line, body)
	}
}

func lacks(t *testing.T, body, part string) {
	t.Helper()
	if strings.Contains(body, part) {
		t.Fatalf("unexpected %q in\n%s", part, body)
	}
}

func TestNilMetricsRecordNothing(t *testing.T) {
	t.Parallel()
	var m *metrics.Metrics
	m.HTTPRequest("GET", "/x", 200, time.Millisecond)
	m.HTTPInFlight(1)
	m.IngestEvent("service.deployed", "accepted", "")
	m.Deployment("event")
	m.AgentReport(true)
	m.RepoSyncWebhook(false)
	m.JobRun("repo_sync", "failed", time.Second)
	m.MCPCall("tool", "get_project", false, time.Millisecond)
	m.MCPProjectCall("o", "p")
	m.Login("password", "failed")
	m.Search("text", "api", false)
	m.Outbound("hosting", "2xx", time.Millisecond)
	if m.RoundTripper("hosting", http.DefaultTransport) != http.DefaultTransport {
		t.Fatal("nil metrics must not wrap transports")
	}
}

func TestEventsAreRecordedWithBoundedLabels(t *testing.T) {
	t.Parallel()
	m := metrics.New("1.2.3", []string{"service.deployed"})
	m.IngestEvent("service.deployed", "accepted", "ignored")
	m.IngestEvent("custom.whatever", "rejected", "ingest.invalid_event")
	m.IngestEvent("service.deployed", "rejected", "Bad key: svcr_secret")
	m.HTTPRequest("PROPFIND", "web", 405, time.Millisecond)
	m.Search("fuzzy", "mcp", true)
	body := scrape(t, m)
	has(t, body, `svcr_build_info{go_version="go`)
	has(t, body, `version="1.2.3"} 1`)
	has(t, body, `svcr_ingest_events_total{code="",outcome="accepted",type="service.deployed"} 1`)
	has(t, body, `svcr_ingest_events_total{code="ingest.invalid_event",outcome="rejected",type="unknown"} 1`)
	has(t, body, `svcr_ingest_events_total{code="other",outcome="rejected",type="service.deployed"} 1`)
	has(t, body, `svcr_http_requests_total{method="other",route="web",status="405"} 1`)
	has(t, body, `svcr_knowledge_searches_total{mode="other",outcome="error",source="mcp"} 1`)
	lacks(t, body, "svcr_secret")
	lacks(t, body, "custom.whatever")
}

func TestScrapeBeforeSampleHasNoRuntimeFamilies(t *testing.T) {
	t.Parallel()
	m := metrics.New("dev", nil)
	body := scrape(t, m)
	lacks(t, body, "go_goroutines")
	lacks(t, body, "process_")
	lacks(t, body, "svcr_metrics_sample_timestamp_seconds")
	m.Sampler(time.Minute, metrics.RuntimeSource()).SampleNow(context.Background())
	body = scrape(t, m)
	has(t, body, "go_goroutines")
	has(t, body, `svcr_metrics_sample_timestamp_seconds{source="runtime"}`)
}

func TestSamplerKeepsLastGoodSnapshot(t *testing.T) {
	t.Parallel()
	m := metrics.New("dev", nil)
	var calls atomic.Int32
	var fail atomic.Bool
	src := metrics.SampleSource("catalog", func(context.Context) ([]metrics.Sample, error) {
		calls.Add(1)
		if fail.Load() {
			return nil, errors.New("database is down")
		}
		return []metrics.Sample{{Name: "svcr_catalog_nodes", Labels: map[string]string{"kind": "project"}, Value: 12}}, nil
	})
	s := m.Sampler(time.Minute, src)
	s.SampleNow(context.Background())
	first := scrape(t, m)
	has(t, first, `svcr_catalog_nodes{kind="project"} 12`)
	for range 100 {
		scrape(t, m)
	}
	if calls.Load() != 1 {
		t.Fatalf("scrapes called the source: %d", calls.Load())
	}
	fail.Store(true)
	s.SampleNow(context.Background())
	second := scrape(t, m)
	has(t, second, `svcr_catalog_nodes{kind="project"} 12`)
	has(t, second, `svcr_metrics_sample_errors_total{source="catalog"} 1`)
	if stamp(first) != stamp(second) {
		t.Fatalf("timestamp moved after a failure: %s vs %s", stamp(first), stamp(second))
	}
}

func stamp(body string) string {
	for line := range strings.SplitSeq(body, "\n") {
		if strings.HasPrefix(line, `svcr_metrics_sample_timestamp_seconds{source="catalog"}`) {
			return line
		}
	}
	return ""
}

func TestSamplerTimesOutAndStops(t *testing.T) {
	t.Parallel()
	m := metrics.New("dev", nil)
	slow := metrics.SampleSource("catalog", func(ctx context.Context) ([]metrics.Sample, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	m.Sampler(20*time.Millisecond, slow).SampleNow(context.Background())
	has(t, scrape(t, m), `svcr_metrics_sample_errors_total{source="catalog"} 1`)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { m.Sampler(time.Hour, metrics.RuntimeSource()).Serve(ctx); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("sampler did not stop")
	}
}

func TestPoolSource(t *testing.T) {
	t.Parallel()
	m := metrics.New("dev", nil)
	pool := metrics.PoolSource(func() metrics.PoolStat {
		return metrics.PoolStat{Acquired: 2, Idle: 3, Max: 10, Acquires: 40, AcquireWait: 1500 * time.Millisecond}
	})
	m.Sampler(time.Minute, pool).SampleNow(context.Background())
	body := scrape(t, m)
	has(t, body, "svcr_db_pool_max_connections 10")
	has(t, body, `svcr_db_pool_connections{state="idle"} 3`)
	has(t, body, "svcr_db_pool_acquires_total 40")
	has(t, body, "svcr_db_pool_acquire_wait_seconds_total 1.5")
}

func TestRoundTripperCountsResults(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bad" {
			w.WriteHeader(http.StatusBadGateway)
		}
	}))
	defer srv.Close()
	m := metrics.New("dev", nil)
	c := m.Client("hosting", srv.Client())
	for _, p := range []string{"/ok", "/bad"} {
		res, err := c.Get(srv.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, res.Body)
		_ = res.Body.Close()
	}
	if _, err := c.Get("http://127.0.0.1:1/"); err == nil {
		t.Fatal("expected a refused connection")
	}
	body := scrape(t, m)
	has(t, body, `svcr_outbound_requests_total{result="2xx",target="hosting"} 1`)
	has(t, body, `svcr_outbound_requests_total{result="5xx",target="hosting"} 1`)
	has(t, body, `svcr_outbound_requests_total{result="error",target="hosting"} 1`)
	lacks(t, body, "127.0.0.1")
}
