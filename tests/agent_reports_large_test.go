package tests

import (
	"fmt"
	"testing"

	deploytypes "svc-registry/internal/feature/deploy/types"
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
