package metrics

import (
	"strconv"
	"time"
)

func (m *Metrics) HTTPRequest(method, route string, status int, d time.Duration) {
	if m == nil {
		return
	}
	method = Method(method)
	m.httpRequests.WithLabelValues(method, route, strconv.Itoa(status)).Inc()
	m.httpDuration.WithLabelValues(method, route).Observe(d.Seconds())
}

func (m *Metrics) HTTPInFlight(delta float64) {
	if m == nil {
		return
	}
	m.httpInFlight.Add(delta)
}

func (m *Metrics) IngestEvent(typ, outcome, code string) {
	if m == nil {
		return
	}
	if outcome != "rejected" {
		code = ""
	}
	m.ingestEvents.WithLabelValues(m.eventType(typ), oneOf(outcome, "accepted", "replayed", "rejected"), Code(code)).Inc()
}

func (m *Metrics) Deployment(source string) {
	if m == nil {
		return
	}
	m.deployments.WithLabelValues(oneOf(source, "event", "cluster")).Inc()
}

func (m *Metrics) AgentReport(accepted bool) {
	if m == nil {
		return
	}
	m.agentReports.WithLabelValues(acceptance(accepted)).Inc()
}

func (m *Metrics) ForgeWebhook(accepted bool) {
	if m == nil {
		return
	}
	m.forgeWebhooks.WithLabelValues(acceptance(accepted)).Inc()
}

func acceptance(ok bool) string {
	if ok {
		return "accepted"
	}
	return "rejected"
}

func (m *Metrics) JobRun(job, outcome string, d time.Duration) {
	if m == nil {
		return
	}
	m.jobRuns.WithLabelValues(job, oneOf(outcome, "succeeded", "failed", "panicked")).Inc()
	m.jobDuration.WithLabelValues(job).Observe(d.Seconds())
}

func (m *Metrics) MCPCall(kind, name string, failed bool, d time.Duration) {
	if m == nil {
		return
	}
	outcome := "ok"
	if failed {
		outcome = "error"
	}
	kind = oneOf(kind, "tool", "resource")
	m.mcpCalls.WithLabelValues(kind, name, outcome).Inc()
	m.mcpDuration.WithLabelValues(kind, name).Observe(d.Seconds())
}

func (m *Metrics) MCPProjectCall(organizationID, projectID string) {
	if m == nil {
		return
	}
	m.mcpProjects.WithLabelValues(organizationID, projectID).Inc()
}

func (m *Metrics) Login(method, outcome string) {
	if m == nil {
		return
	}
	m.logins.WithLabelValues(method, oneOf(outcome, "succeeded", "failed", "throttled")).Inc()
}

func (m *Metrics) Search(mode, source string, failed bool) {
	if m == nil {
		return
	}
	outcome := "ok"
	if failed {
		outcome = "error"
	}
	m.searches.WithLabelValues(oneOf(mode, "text", "semantic", "hybrid"), oneOf(source, "api", "mcp"), outcome).Inc()
}

func (m *Metrics) Outbound(target, result string, d time.Duration) {
	if m == nil {
		return
	}
	m.outbound.WithLabelValues(target, result).Inc()
	m.outboundDuration.WithLabelValues(target).Observe(d.Seconds())
}

func (m *Metrics) MCPGatewayCall(serverID, outcome string, d time.Duration) {
	if m == nil {
		return
	}
	outcome = oneOf(outcome, "ok", "tool_error", "failed")
	m.gatewayCalls.WithLabelValues(serverID, outcome).Inc()
	m.gatewayDuration.WithLabelValues(serverID).Observe(d.Seconds())
}

func (m *Metrics) MCPGatewayCheck(outcome string) {
	if m == nil {
		return
	}
	m.gatewayChecks.WithLabelValues(oneOf(outcome, "ok", "failed")).Inc()
}
