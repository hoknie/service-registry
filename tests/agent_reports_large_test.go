package tests

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	deploytypes "svc-registry/internal/feature/deploy/types"
	"svc-registry/internal/testsupport"
)

func TestLargeAgentReportIsSavedInBatches(t *testing.T) {
	t.Parallel()
	app, _ := startClusterApp(t)
	root := app.admin()
	project, key := app.projectWithKey(root, "api")
	c, token := app.agentCluster(root, "edge", nil)
	id := idOf(c)
	ann := map[string]string{deploytypes.AnnotationProject: "api-org/api", deploytypes.AnnotationService: "api"}
	eq(t, app.ingest(project, key, deployed("k1", "api", "production", "1.4.2", "2026-10-01T10:00:00Z")).status, 201)

	const many = 2500
	workloads := []obj{reportWorkload("u-api", "api", "1.4.3", ann)}
	for i := range many {
		workloads = append(workloads, reportWorkload(fmt.Sprintf("u%05d", i), fmt.Sprintf("svc-%05d", i), "1", nil))
	}
	workloads = append(workloads, reportWorkload("u00007", "svc-renamed", "2", nil))
	r := app.agentReport(id, token, agentReportOf(workloads...))
	eq(t, r.status, 200, r.text())
	eq(t, jsonText(r.json(t)["accepted"]), fmt.Sprintf(`{"matched":1,"unmatched":%d,"workloads":%d}`, many+1, many+2))
	eq(t, app.count("cluster_workloads"), int64(many+1))
	eq(t, scalar[string](t, app.db, "SELECT name FROM cluster_workloads WHERE uid = 'u00007'"), "svc-renamed")
	eq(t, scalar[string](t, app.db, "SELECT pending_version FROM cluster_workloads WHERE uid = 'u-api'"), "1.4.3")
	eq(t, scalar[string](t, app.db, "SELECT state->'replicas'->>'desired' FROM cluster_workloads WHERE uid = 'u02499'"), "2")
	eq(t, scalar[string](t, app.db, "SELECT state->>'restarts' FROM cluster_workloads WHERE uid = 'u-api'"), "3")

	app.allowReport()
	r = app.agentReport(id, token, agentReportOf(workloads[:1001]...))
	eq(t, r.status, 200, r.text())
	eq(t, scalar[int64](t, app.db, "SELECT count(*) FROM cluster_workloads WHERE gone_at IS NULL"), int64(1001))
	eq(t, scalar[bool](t, app.db, "SELECT gone_at IS NOT NULL FROM cluster_workloads WHERE uid = 'u02499'"), true)
	eq(t, scalar[string](t, app.db, "SELECT pending_version FROM cluster_workloads WHERE uid = 'u-api'"), "1.4.3")
}

func TestAgentReportGapGrowsWithWorkloads(t *testing.T) {
	t.Parallel()
	app, _ := startClusterApp(t)
	root := app.admin()
	c, token := app.agentCluster(root, "edge", nil)
	id := idOf(c)
	for _, g := range []struct{ interval, workloads, per, want int }{{60, 0, 2000, 30}, {15, 0, 2000, 15}, {15, 40001, 2000, 21},
		{15, 100000, 2000, 50}, {15, 100000, 1000, 100}} {
		eq(t, scalar[int32](t, app.db, "SELECT agent_report_gap($1, $2, $3)", g.interval, g.workloads, g.per), int32(g.want))
	}
	contains(t, execErr(t, app.db, "UPDATE clusters SET agent_workloads = 100001"), "clusters_agent_workloads_check")

	eq(t, app.agentConfig(id, token).json(t)["report_gap_secs"], any(float64(30)))
	r := app.agentReport(id, token, agentReportOf(reportWorkload("u1", "api", "1", nil), reportWorkload("u2", "redis", "7", nil)))
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["report_gap_secs"], any(float64(30)))
	eq(t, scalar[int32](t, app.db, "SELECT agent_workloads FROM clusters"), int32(2))
	agent := app.get(clusterPath(id), root).json(t)["agent"].(map[string]any)
	eq(t, agent["report_gap_secs"], any(float64(30)))
	eq(t, agent["report_workloads"], any(float64(2)))

	execSQL(t, app.db, "UPDATE clusters SET interval_secs = 15, agent_workloads = 100000, "+
		"agent_reported_at = now() - interval '30 seconds'")
	cfg := app.agentConfig(id, token).json(t)
	eq(t, cfg["interval_secs"], any(float64(15)))
	eq(t, cfg["report_gap_secs"], any(float64(50)))
	r = app.agentReport(id, token, agentReportOf())
	eq(t, r.status, 429, r.text())
	retry, err := strconv.Atoi(r.header("Retry-After"))
	eq(t, err == nil && retry > 15 && retry <= 20, true, r.header("Retry-After"))

	execSQL(t, app.db, "UPDATE clusters SET agent_reported_at = now() - interval '100 seconds'")
	eq(t, app.get(clusterPath(id), root).json(t)["status"], any("ok"))
	execSQL(t, app.db, "UPDATE clusters SET agent_reported_at = now() - interval '200 seconds'")
	eq(t, at(app.get(clusterPath(id), root).json(t), "last_error", "code"), any("k8s.agent_silent"))

	r = app.agentReport(id, token, agentReportOf(reportWorkload("u1", "api", "1", nil)))
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["report_gap_secs"], any(float64(15)))
}

func TestAgentReportGapFollowsTheConfiguredMultiplier(t *testing.T) {
	t.Parallel()
	app, _ := startClusterApp(t, "K8S_AGENT_WORKLOADS_PER_GAP_SEC", "1000")
	root := app.admin()
	c, token := app.agentCluster(root, "edge", nil)
	id := idOf(c)
	execSQL(t, app.db, "UPDATE clusters SET status = 'ok', interval_secs = 15, agent_workloads = 100000, "+
		"agent_reported_at = now() - interval '60 seconds'")
	eq(t, app.agentConfig(id, token).json(t)["report_gap_secs"], any(float64(100)))
	agent := app.get(clusterPath(id), root).json(t)["agent"].(map[string]any)
	eq(t, agent["report_gap_secs"], any(float64(100)))
	eq(t, agent["report_workloads"], any(float64(100000)))
	r := app.agentReport(id, token, agentReportOf())
	eq(t, r.status, 429, r.text())
	retry, err := strconv.Atoi(r.header("Retry-After"))
	eq(t, err == nil && retry > 35 && retry <= 40, true, r.header("Retry-After"))
	execSQL(t, app.db, "UPDATE clusters SET agent_reported_at = now() - interval '200 seconds'")
	eq(t, app.get(clusterPath(id), root).json(t)["status"], any("ok"))
}

func TestAgentReportGapFunctionRollsBack(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tdb := testsupport.NewTestDB(t)
	m := embedded(t)
	_, err := m.Migrate(ctx, tdb.Pool)
	must(t, err)
	eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM pg_proc WHERE pronamespace = current_schema()::regnamespace AND proname = 'agent_report_gap'"), int64(1))
	eq(t, scalar[int32](t, tdb, "SELECT agent_report_gap(15, 100000, 2000)"), int32(50))
	n, err := rollbackThrough(t, m, tdb, squashMigration)
	must(t, err)
	eq(t, n, migrationsFrom(t, m, squashMigration))
	eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM pg_proc WHERE pronamespace = current_schema()::regnamespace AND proname = 'agent_report_gap'"), int64(0))
}
