package metrics

import (
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	events           *prometheus.Registry
	snapshot         *snapshot
	eventTypes       map[string]bool
	httpRequests     *prometheus.CounterVec
	httpDuration     *prometheus.HistogramVec
	httpInFlight     prometheus.Gauge
	ingestEvents     *prometheus.CounterVec
	deployments      *prometheus.CounterVec
	agentReports     *prometheus.CounterVec
	forgeWebhooks    *prometheus.CounterVec
	jobRuns          *prometheus.CounterVec
	jobDuration      *prometheus.HistogramVec
	mcpCalls         *prometheus.CounterVec
	mcpDuration      *prometheus.HistogramVec
	mcpProjects      *prometheus.CounterVec
	logins           *prometheus.CounterVec
	searches         *prometheus.CounterVec
	outbound         *prometheus.CounterVec
	outboundDuration *prometheus.HistogramVec
	sampleErrors     *prometheus.CounterVec
	gatewayCalls     *prometheus.CounterVec
	gatewayDuration  *prometheus.HistogramVec
	gatewayChecks    *prometheus.CounterVec
}

func New(version string, eventTypes []string) *Metrics {
	m := &Metrics{events: prometheus.NewRegistry(), snapshot: newSnapshot(), eventTypes: map[string]bool{}}
	for _, t := range eventTypes {
		m.eventTypes[t] = true
	}
	f := factory{m.events}
	m.httpRequests = f.counter("svcr_http_requests_total", "HTTP requests handled.", "method", "route", "status")
	m.httpDuration = f.histogram("svcr_http_request_duration_seconds", "Time to answer an HTTP request.",
		[]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}, "method", "route")
	m.httpInFlight = f.gauge("svcr_http_requests_in_flight", "HTTP requests being handled now.").(prometheus.Gauge)
	m.ingestEvents = f.counter("svcr_ingest_events_total", "Ingest requests by event type and outcome.", "type", "outcome", "code")
	m.deployments = f.counter("svcr_deployments_total", "Deployments recorded.", "source")
	m.agentReports = f.counter("svcr_agent_reports_total", "Cluster agent reports.", "outcome")
	m.forgeWebhooks = f.counter("svcr_forge_webhooks_total", "Forge webhook requests.", "outcome")
	m.jobRuns = f.counter("svcr_job_runs_total", "Background job runs.", "job", "outcome")
	m.jobDuration = f.histogram("svcr_job_run_duration_seconds", "Duration of background job runs.",
		[]float64{0.1, 0.5, 1, 5, 15, 60, 300, 900}, "job")
	m.mcpCalls = f.counter("svcr_mcp_calls_total", "MCP tool calls and resource reads.", "kind", "name", "outcome")
	m.mcpDuration = f.histogram("svcr_mcp_call_duration_seconds", "Duration of MCP calls.",
		[]float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}, "kind", "name")
	m.mcpProjects = f.counter("svcr_mcp_project_calls_total", "MCP calls addressed to a readable project.", "organization_id", "project_id")
	m.logins = f.counter("svcr_logins_total", "Sign-in attempts.", "method", "outcome")
	m.searches = f.counter("svcr_knowledge_searches_total", "Documentation searches.", "mode", "source", "outcome")
	m.outbound = f.counter("svcr_outbound_requests_total", "Outgoing requests to external systems.", "target", "result")
	m.outboundDuration = f.histogram("svcr_outbound_request_duration_seconds", "Duration of outgoing requests.",
		[]float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30}, "target")
	m.gatewayCalls = f.counter("svcr_mcp_gateway_calls_total", "MCP gateway tool calls by server and outcome.", "server_id", "outcome")
	m.gatewayDuration = f.histogram("svcr_mcp_gateway_call_duration_seconds", "Duration of MCP gateway tool calls.",
		[]float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60}, "server_id")
	m.gatewayChecks = f.counter("svcr_mcp_gateway_checks_total", "Checks of MCP gateway servers.", "outcome")
	m.sampleErrors = f.counter("svcr_metrics_sample_errors_total", "Failed samples by source.", "source")
	f.gauge("svcr_build_info", "Version of the running binary.", "version", "go_version").(*prometheus.GaugeVec).
		WithLabelValues(version, runtime.Version()).Set(1)
	f.gauge("svcr_start_time_seconds", "Start time of the process, Unix seconds.").(prometheus.Gauge).
		Set(float64(time.Now().Unix()))
	return m
}

type factory struct{ r *prometheus.Registry }

func (f factory) counter(name, help string, labels ...string) *prometheus.CounterVec {
	v := prometheus.NewCounterVec(prometheus.CounterOpts{Name: name, Help: help}, labels)
	f.r.MustRegister(v)
	return v
}

func (f factory) histogram(name, help string, buckets []float64, labels ...string) *prometheus.HistogramVec {
	v := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: name, Help: help, Buckets: buckets}, labels)
	f.r.MustRegister(v)
	return v
}

func (f factory) gauge(name, help string, labels ...string) prometheus.Collector {
	if len(labels) == 0 {
		g := prometheus.NewGauge(prometheus.GaugeOpts{Name: name, Help: help})
		f.r.MustRegister(g)
		return g
	}
	v := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: name, Help: help}, labels)
	f.r.MustRegister(v)
	return v
}
