package tests

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"encoding/base64"
	"strconv"
	"strings"
	"testing"

	deploytypes "svc-registry/internal/feature/deploy/types"
)

func (a *testApp) agentCluster(cookie, name string, extra obj) (obj, string) {
	a.t.Helper()
	body := obj{"name": name, "environment": "production", "mode": "agent"}
	for k, v := range extra {
		body[k] = v
	}
	r := a.send("POST", clustersPath, cookie, body)
	if r.status != 201 {
		a.t.Fatalf("agent cluster: %d %s", r.status, r.body)
	}
	c := r.json(a.t)
	return c, at(c, "agent_token", "secret").(string)
}

func gzipped(t testing.TB, body string) string {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func (a *testApp) agentReport(cluster, token string, report any, extra ...string) reply {
	a.t.Helper()
	headers := append([]string{"content-type", "application/json", "content-encoding", "gzip", "authorization", "Bearer " + token}, extra...)
	return request(a.t, a.addr, "POST", clusterIngest(cluster)+"/reports", gzipped(a.t, jsonText(report)), headers...)
}

func (a *testApp) agentConfig(cluster, token string) reply {
	return request(a.t, a.addr, "GET", clusterIngest(cluster)+"/config", "", "authorization", "Bearer "+token)
}

func (a *testApp) allowReport() {
	a.t.Helper()
	execSQL(a.t, a.db, "UPDATE clusters SET agent_reported_at = agent_reported_at - interval '1 hour'")
}

func clusterIngest(id string) string { return "/api/v1/ingest/clusters/" + id }

func reportWorkload(uid, name, image string, ann map[string]string) obj {
	return obj{"uid": uid, "kind": "Deployment", "namespace": "backend", "name": name, "annotations": ann,
		"labels": obj{"app.kubernetes.io/name": name}, "selector": "app=" + name,
		"containers": []obj{{"name": "app", "image": "registry.example/acme/" + name + ":" + image}},
		"generation": 1, "observed_generation": 1, "replicas": obj{"desired": 2, "ready": 2, "updated": 2},
		"conditions": []obj{}, "pods": []obj{{"containers": []obj{{"name": "app", "image_id": "x@sha256:aa", "restarts": 3}}}}}
}

func agentReportOf(workloads ...obj) obj {
	if workloads == nil {
		workloads = []obj{}
	}
	return obj{"agent_version": "0.2.0", "kubernetes_version": "v1.31.2", "missing": []string{"pods:list"}, "error": nil,
		"workloads": workloads}
}

func TestAgentAuthentication(t *testing.T) {
	t.Parallel()
	app, _ := startClusterApp(t)
	root := app.admin()
	a, tokenA := app.agentCluster(root, "edge-a", nil)
	b, _ := app.agentCluster(root, "edge-b", nil)
	_, key := app.projectWithKey(root, "api")
	polled := app.cluster(root, "prod", nil)
	refused := func(r reply, context string) {
		t.Helper()
		eq(t, r.status, 401, context, r.text())
		eq(t, code(t, r), any("auth.invalid_agent_token"), context)
		eq(t, r.header("WWW-Authenticate"), "Bearer", context)
	}
	refused(request(t, app.addr, "POST", clusterIngest(idOf(a))+"/reports", "{}", "content-type", "application/json"), "no header")
	refused(app.agentReport(idOf(a), key, agentReportOf()), "project key")
	refused(app.agentReport(idOf(b), tokenA, agentReportOf()), "token of another cluster")
	refused(app.agentReport("not-a-uuid", tokenA, agentReportOf()), "not a uuid")
	refused(app.agentReport(idOf(polled), tokenA, agentReportOf()), "polled cluster")
	refused(app.agentReport(idOf(a), tokenA[:len(tokenA)-1]+"x", agentReportOf()), "bad checksum")
	refused(app.agentReport(idOf(a), app.pat(root, "clusters:write"), agentReportOf()), "personal token")
	refused(request(t, app.addr, "POST", clusterIngest(idOf(a))+"/reports", "{}", "content-type", "application/json",
		"sec-fetch-site", "cross-site", "cookie", root), "cross-site without a token")
	refused(request(t, app.addr, "GET", clusterIngest(idOf(a))+"/config", "", "cookie", root), "session cookie")

	cfg := app.agentConfig(idOf(a), tokenA)
	eq(t, cfg.status, 200, cfg.text())
	got := cfg.json(t)
	eq(t, at(got, "cluster", "id"), a["id"])
	eq(t, at(got, "cluster", "name"), any("edge-a"))
	eq(t, got["enabled"], any(true))
	eq(t, got["interval_secs"], any(float64(60)))
	eq(t, jsonText(got["namespaces"]), `[]`)
	eq(t, scalar[bool](t, app.db, "SELECT last_used_at IS NOT NULL FROM cluster_agent_tokens WHERE cluster_id = $1", idOf(a)), true)
}

func TestAgentReportsAreObservedLikePolls(t *testing.T) {
	t.Parallel()
	app, _ := startClusterApp(t)
	root := app.admin()
	project, key := app.projectWithKey(root, "api")
	c, token := app.agentCluster(root, "edge", nil)
	id := idOf(c)
	ann := map[string]string{deploytypes.AnnotationProject: "api-org/api", deploytypes.AnnotationService: "api"}
	eq(t, app.ingest(project, key, deployed("k1", "api", "production", "1.4.2", "2026-10-01T10:00:00Z")).status, 201)

	r := app.agentReport(id, token, agentReportOf(reportWorkload("u1", "api", "1.4.3", ann), reportWorkload("u2", "redis", "7", nil)))
	eq(t, r.status, 200, r.text())
	eq(t, jsonText(r.json(t)["accepted"]), `{"matched":1,"unmatched":1,"workloads":2}`)
	eq(t, r.json(t)["interval_secs"], any(float64(60)))
	got := app.get(clusterPath(id), root).json(t)
	eq(t, got["status"], any("ok"))
	eq(t, got["workloads"], any(float64(1)))
	eq(t, at(got, "agent", "version"), any("0.2.0"))
	eq(t, at(got, "agent", "last_report_at") != nil, true)
	eq(t, got["last_polled_at"], at(got, "agent", "last_report_at"))
	eq(t, scalar[string](t, app.db, "SELECT state->>'restarts' FROM cluster_workloads WHERE uid = 'u1'"), "3")
	eq(t, scalar[string](t, app.db, "SELECT pending_version FROM cluster_workloads WHERE uid = 'u1'"), "1.4.3")

	test := app.call("POST", clusterPath(id)+"/test", root)
	eq(t, test.status, 200, test.text())
	eq(t, jsonText(test.json(t)), `{"missing":["pods:list"],"ok":true,"version":"v1.31.2"}`)

	r = app.agentReport(id, token, agentReportOf(reportWorkload("u1", "api", "1.4.3", ann)))
	eq(t, r.status, 429, r.text())
	eq(t, code(t, r), any("ingest.rate_limited"))
	retry, err := strconv.Atoi(r.header("Retry-After"))
	eq(t, err == nil && retry >= 1 && retry <= 30, true, r.header("Retry-After"))

	execSQL(t, app.db, "UPDATE cluster_workloads SET pending_since = pending_since - interval '11 minutes'")
	app.allowReport()
	r = app.agentReport(id, token, agentReportOf(reportWorkload("u1", "api", "1.4.3", ann)))
	eq(t, r.status, 200, r.text())
	first := list(t, app.get(nodePath(project)+"/deployments", root).json(t), "items")[0].(obj)
	eq(t, first["version"], any("1.4.3"))
	eq(t, first["source"], any("cluster"))
	eq(t, first["cluster"], any("edge"))
	eq(t, scalar[bool](t, app.db, "SELECT gone_at IS NOT NULL FROM cluster_workloads WHERE uid = 'u2'"), true)

	app.allowReport()
	r = app.agentReport(id, token, obj{"agent_version": "0.2.0", "error": obj{"code": "k8s.forbidden", "message": "denied"}, "workloads": []obj{}})
	eq(t, r.status, 200, r.text())
	got = app.get(clusterPath(id), root).json(t)
	eq(t, got["status"], any("error"))
	eq(t, at(got, "last_error", "code"), any("k8s.forbidden"))
	eq(t, got["workloads"], any(float64(1)), "observations stay")
	eq(t, at(app.call("POST", clusterPath(id)+"/test", root).json(t), "error", "code"), any("k8s.forbidden"))

	eq(t, app.send("PATCH", nodePath(project), root, obj{"cluster_observation": false}).status, 200)
	app.allowReport()
	eq(t, app.agentReport(id, token, agentReportOf(reportWorkload("u1", "api", "1.4.3", ann))).status, 200)
	eq(t, scalar[int64](t, app.db, "SELECT count(*) FROM cluster_workloads WHERE project_id = $1", project), int64(0))
}

func TestAgentReportRefusals(t *testing.T) {
	t.Parallel()
	app, _ := startClusterApp(t, "K8S_AGENT_REPORT_MAX_BYTES", "1048576")
	root := app.admin()
	c, token := app.agentCluster(root, "edge", obj{"interval_secs": 60})
	id := idOf(c)
	path := clusterIngest(id) + "/reports"
	auth := []string{"authorization", "Bearer " + token}
	invalid := func(r reply, context string) {
		t.Helper()
		eq(t, r.status, 400, context, r.text())
		eq(t, code(t, r), any("validation.invalid_body"), context)
	}
	invalid(request(t, app.addr, "POST", path, "not gzip", append(auth, "content-type", "application/json", "content-encoding", "gzip")...), "bad gzip")
	invalid(request(t, app.addr, "POST", path, jsonText(agentReportOf()), append(auth, "content-type", "text/plain")...), "content type")
	invalid(app.agentReport(id, token, obj{"workloads": []obj{}}), "no agent version")
	invalid(app.agentReport(id, token, agentReportOf(obj{"uid": "u", "kind": "Job", "namespace": "n", "name": "x"})), "unknown kind")
	invalid(app.agentReport(id, token, obj{"agent_version": 5}), "wrong type")

	zipped := gzipped(t, jsonText(agentReportOf()))
	invalid(request(t, app.addr, "POST", path, zipped[:len(zipped)-6], append(auth, "content-type", "application/json", "content-encoding", "gzip")...), "cut gzip")
	invalid(request(t, app.addr, "POST", path, jsonText(agentReportOf())+" {}", append(auth, "content-type", "application/json")...), "data after the object")
	invalid(request(t, app.addr, "POST", path, " [1]", append(auth, "content-type", "application/json")...), "not an object")
	eq(t, app.count("cluster_workloads"), int64(0))

	huge := `{"agent_version":"0.2.0","workloads":[],"pad":"` + strings.Repeat("a", 1<<20) + `"}`
	r := request(t, app.addr, "POST", path, gzipped(t, huge), append(auth, "content-type", "application/json", "content-encoding", "gzip")...)
	eq(t, r.status, 413, r.text())
	eq(t, code(t, r), any("ingest.payload_too_large"))
	r = request(t, app.addr, "POST", path, gzipped(t, huge), "content-type", "application/json", "content-encoding", "gzip")
	eq(t, r.status, 401, "the token is checked before the size")
	r = request(t, app.addr, "POST", path, huge, append(auth, "content-type", "application/json")...)
	eq(t, r.status, 413, "the limit applies to a body sent without gzip too: ", r.text())
	status, body, _ := announceAt(t, app.addr, path, "Bearer "+token, deploytypes.ReportWireLimit+1)
	eq(t, status, 413, body)
	contains(t, body, `"code":"ingest.payload_too_large"`)
	status, body, _ = announceAt(t, app.addr, path, "", deploytypes.ReportWireLimit+1)
	eq(t, status, 401, body)
	eq(t, app.count("cluster_workloads"), int64(0))

	execSQL(t, app.db, "UPDATE clusters SET lease_until = now() + interval '1 minute'")
	r = app.agentReport(id, token, agentReportOf())
	eq(t, r.status, 409, r.text())
	eq(t, code(t, r), any("conflict.agent_report_in_progress"))
	execSQL(t, app.db, "UPDATE clusters SET lease_until = NULL")

	eq(t, app.send("PATCH", clusterPath(id), root, obj{"enabled": false, "namespaces": []string{"payments"}}).status, 200)
	r = app.agentReport(id, token, agentReportOf())
	eq(t, r.status, 409, r.text())
	eq(t, code(t, r), any("conflict.cluster_disabled"))
	eq(t, app.agentConfig(id, token).json(t)["enabled"], any(false))
	eq(t, app.send("PATCH", clusterPath(id), root, obj{"enabled": true}).status, 200)
	r = app.agentReport(id, token, agentReportOf())
	eq(t, r.status, 200, r.text())
	eq(t, jsonText(r.json(t)["namespaces"]), `["payments"]`)
}

func TestAgentReportOverTheServerBodyLimit(t *testing.T) {
	t.Parallel()
	app, _ := startClusterApp(t)
	root := app.admin()
	c, token := app.agentCluster(root, "edge", nil)
	noise := make([]byte, 3<<20)
	if _, err := rand.Read(noise); err != nil {
		t.Fatal(err)
	}
	report := agentReportOf(reportWorkload("u1", "api", "1.4.2", nil))
	report["pad"] = base64.StdEncoding.EncodeToString(noise)
	body := gzipped(t, jsonText(report))
	if len(body) <= 2<<20 {
		t.Fatalf("the compressed report is only %d bytes", len(body))
	}
	r := request(t, app.addr, "POST", clusterIngest(idOf(c))+"/reports", body,
		"content-type", "application/json", "content-encoding", "gzip", "authorization", "Bearer "+token)
	eq(t, r.status, 200, r.text())
	eq(t, at(r.json(t), "accepted", "workloads"), any(float64(1)))
	eq(t, app.count("cluster_workloads"), int64(1))
}
