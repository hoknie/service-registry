package tests

import (
	"regexp"
	"testing"
)

var agentTokenShape = regexp.MustCompile(`^svca_[0-9A-Za-z]{38}$`)

func TestAgentClusterLifecycle(t *testing.T) {
	t.Parallel()
	app, k8s := startClusterApp(t)
	root := app.admin()
	c, tokenA := app.agentCluster(root, "edge-1", nil)
	id := idOf(c)
	eq(t, c["mode"], any("agent"))
	eq(t, c["api_url"], nil)
	eq(t, c["credentials"], nil)
	eq(t, c["status"], any("never"))
	eq(t, agentTokenShape.MatchString(tokenA), true, tokenA)
	eq(t, at(c, "agent_token", "prefix"), any(tokenA[:12]))
	eq(t, jsonText(at(c, "agent", "token_prefixes")), `["`+tokenA[:12]+`"]`)
	lacks(t, scalar[string](t, app.db, "SELECT string_agg(row_to_json(t)::text, ',') FROM cluster_agent_tokens t"), tokenA)
	got := app.get(clusterPath(id), root)
	lacks(t, got.text(), tokenA)
	eq(t, got.json(t)["agent_token"], nil)
	eq(t, at(got.json(t), "agent", "version"), nil)

	for _, tt := range []struct {
		body obj
		code string
	}{
		{obj{"name": "x", "environment": "p", "mode": "push"}, "validation.invalid_cluster_mode"},
		{obj{"name": "x", "environment": "p", "mode": "agent", "api_url": "https://k8s.example:6443"}, "validation.invalid_api_url"},
		{obj{"name": "x", "environment": "p", "mode": "agent", "in_cluster": true}, "validation.invalid_cluster_mode"},
	} {
		r := app.send("POST", clustersPath, root, tt.body)
		eq(t, r.status, 400, jsonText(tt.body))
		eq(t, code(t, r), any(tt.code), jsonText(tt.body))
	}

	r := app.call("POST", clusterPath(id)+"/poll", root)
	eq(t, r.status, 409)
	eq(t, code(t, r), any("conflict.cluster_mode_agent"))
	r = app.call("POST", clusterPath(id)+"/test", root)
	eq(t, r.status, 200)
	eq(t, r.json(t)["ok"], any(false))
	eq(t, at(r.json(t), "error", "code"), any("k8s.agent_never_reported"))
	eq(t, len(k8s.Accesses()), 0, "the registry never calls an agent cluster")

	polled := app.cluster(root, "prod", nil)
	r = app.call("POST", clusterPath(idOf(polled))+"/agent-token", root)
	eq(t, r.status, 409)
	eq(t, code(t, r), any("conflict.cluster_mode_poll"))
	eq(t, code(t, app.send("POST", clusterPath(id)+"/agent-token", root, obj{"grace_secs": -1})), any("validation.invalid_grace_period"))

	r = app.send("POST", clusterPath(id)+"/agent-token", root, obj{"grace_secs": 3600})
	eq(t, r.status, 201, r.text())
	tokenB := at(r.json(t), "agent_token", "secret").(string)
	eq(t, app.agentConfig(id, tokenA).status, 200, "the old token works during the grace period")
	eq(t, app.agentConfig(id, tokenB).status, 200)
	eq(t, len(list(t, app.get(clusterPath(id), root).json(t), "agent", "token_prefixes")), 2)
	execSQL(t, app.db, "UPDATE cluster_agent_tokens SET expires_at = now() - interval '1 second' WHERE prefix = $1", tokenA[:12])
	eq(t, app.agentConfig(id, tokenA).status, 401)

	eq(t, app.call("DELETE", clusterPath(id)+"/agent-token", root).status, 204)
	eq(t, app.agentConfig(id, tokenB).status, 401)

	r = app.send("PATCH", clusterPath(id), root, obj{"mode": "poll"})
	eq(t, code(t, r), any("validation.invalid_api_url"))
	r = app.send("PATCH", clusterPath(id), root, obj{"mode": "poll", "api_url": "https://k8s.example:6443",
		"credentials": app.tokenOn(root, "", saToken)})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["mode"], any("poll"))
	eq(t, r.json(t)["agent"], nil)
	eq(t, r.json(t)["agent_token"], nil)
	r = app.send("PATCH", clusterPath(id), root, obj{"mode": "agent"})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["api_url"], nil)
	eq(t, r.json(t)["credentials"], nil)
	tokenC := at(r.json(t), "agent_token", "secret").(string)
	eq(t, app.agentConfig(id, tokenC).status, 200)

	eq(t, app.call("DELETE", clusterPath(id), root).status, 204)
	r = app.agentReport(id, tokenC, agentReportOf())
	eq(t, r.status, 401)
	eq(t, code(t, r), any("auth.invalid_agent_token"))
}

func TestSilentAgentIsAnError(t *testing.T) {
	t.Parallel()
	app, _ := startClusterApp(t)
	root := app.admin()
	project, _ := app.projectWithKey(root, "api")
	c, token := app.agentCluster(root, "edge", nil)
	id := idOf(c)
	ann := map[string]string{"svc-registry.io/project": "api-org/api"}
	eq(t, app.agentReport(id, token, agentReportOf(reportWorkload("u1", "api", "1.0.0", ann))).status, 200)
	eq(t, app.get(clusterPath(id), root).json(t)["status"], any("ok"))

	execSQL(t, app.db, "UPDATE clusters SET agent_reported_at = now() - interval '4 minutes'")
	got := app.get(clusterPath(id), root).json(t)
	eq(t, got["status"], any("error"))
	eq(t, at(got, "last_error", "code"), any("k8s.agent_silent"))
	eq(t, at(got, "agent", "last_report_at") != nil, true)
	activity := app.get(nodePath(project)+"/activity", root)
	eq(t, activity.status, 200, activity.text())
	contains(t, activity.text(), "k8s.agent_silent")

	eq(t, app.agentReport(id, token, agentReportOf(reportWorkload("u1", "api", "1.0.0", ann))).status, 200)
	eq(t, app.get(clusterPath(id), root).json(t)["status"], any("ok"))

	quiet, _ := app.agentCluster(root, "never", nil)
	execSQL(t, app.db, "UPDATE clusters SET updated_at = now() - interval '4 minutes' WHERE id = $1", idOf(quiet))
	eq(t, at(app.get(clusterPath(idOf(quiet)), root).json(t), "last_error", "code"), any("k8s.agent_silent"))
}
