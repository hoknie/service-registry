package tests

import (
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func activityPath(id string) string { return nodePath(id) + "/activity" }

func (a *sourcesApp) processes(id string) map[string]obj {
	a.t.Helper()
	r := a.get(activityPath(id), a.admin)
	eq(a.t, r.status, 200, r.text())
	out := map[string]obj{}
	for _, p := range r.json(a.t)["processes"].([]any) {
		m := p.(map[string]any)
		out[m["kind"].(string)] = m
	}
	return out
}

func (a *sourcesApp) failForge(project, code string) {
	a.t.Helper()
	conn, run := uuid.NewString(), uuid.NewString()
	execSQL(a.t, a.db, `INSERT INTO forge_connections (id, node_id, kind, api_url, owner_path, interval_secs, credentials_ref, next_run_at)
		VALUES ($1, $2, 'github', 'https://api.github.example', 'acme', 600, 'env:T', now() + interval '1 hour')`, conn, a.org)
	execSQL(a.t, a.db, `INSERT INTO forge_repositories (project_id, connection_id, external_id, full_path, web_url, visibility)
		VALUES ($1, $2, $3, $4, 'https://github.example/acme/x', 'private')`, project, conn, uuid.NewString(), "acme/"+project)
	execSQL(a.t, a.db, `INSERT INTO forge_sync_runs (id, connection_id, trigger, status, finished_at, error_code)
		VALUES ($1, $2, 'schedule', 'failed', now(), $3)`, run, conn, code)
	execSQL(a.t, a.db, `INSERT INTO knowledge_settings (project_id, next_run_at) VALUES ($1, now() + interval '1 hour')
		ON CONFLICT (project_id) DO UPDATE SET next_run_at = EXCLUDED.next_run_at`, project)
}

func TestCatalogActivityOfACollectedProject(t *testing.T) {
	t.Parallel()
	a := startSources(t)
	eq(t, len(a.processes(a.project)), 0, "a manual project without a source has no processes")

	dir := filepath.Join(a.root, "docs")
	writeFile(t, filepath.Join(dir, "README.md"), "# Docs")
	a.useSource(a.project, "local_dir", dir)
	eq(t, a.processes(a.project)["collect"]["state"], any("queued"), "a new source waits for the job")

	execSQL(t, a.db, "UPDATE knowledge_settings SET lease_until = now() + interval '1 minute' WHERE project_id = $1", a.project)
	eq(t, a.processes(a.project)["collect"]["state"], any("running"))

	execSQL(t, a.db, "UPDATE knowledge_settings SET lease_until = NULL WHERE project_id = $1", a.project)
	if err := removeAll(dir); err != nil {
		t.Fatal(err)
	}
	a.collect()
	collect := a.processes(a.project)["collect"]
	eq(t, collect["state"], any("failed"))
	eq(t, collect["code"], any("source.not_found"))
	eq(t, collect["last_at"] != nil, true)
	eq(t, collect["pending"], nil)

	eq(t, a.send("POST", knowledgePath(a.project)+"/collect", a.admin, nil).status, 202)
	eq(t, a.processes(a.project)["collect"]["state"], any("queued"), "collect now")
}

func TestCatalogActivityCountsFilesWaitingForEmbeddings(t *testing.T) {
	t.Parallel()
	a := startSearch(t, "pgvector")
	writeFile(t, filepath.Join(a.dir, "README.md"), "# One")
	writeFile(t, filepath.Join(a.dir, "guide.md"), "# Two")
	a.useSource(a.project, "local_dir", a.dir, "**/*.md")
	a.collect()
	execSQL(t, a.db, "INSERT INTO knowledge_index_state (project_id, next_run_at) VALUES ($1, now() + interval '1 hour')", a.project)
	index := a.processes(a.project)["index"]
	eq(t, index["state"], any("idle"))
	eq(t, index["pending"], any(float64(2)))
	execSQL(t, a.db, "UPDATE knowledge_index_state SET failure = 'search.embeddings_unavailable' WHERE project_id = $1", a.project)
	index = a.processes(a.project)["index"]
	eq(t, index["state"], any("failed"))
	eq(t, index["code"], any("search.embeddings_unavailable"))
}

func TestCatalogActivityOfForgeAndClusters(t *testing.T) {
	t.Parallel()
	a := startSources(t)
	a.failForge(a.project, "forge.unauthorized")
	forge := a.processes(a.project)["forge"]
	eq(t, forge["state"], any("failed"))
	eq(t, forge["code"], any("forge.unauthorized"))

	cluster := uuid.NewString()
	execSQL(t, a.db, `INSERT INTO clusters (id, name, environment, in_cluster, status, last_error, next_run_at)
		VALUES ($1, 'prod', 'production', true, 'error', '{"code": "cluster.unreachable", "message": "x"}', now() + interval '1 hour')`, cluster)
	execSQL(t, a.db, `INSERT INTO cluster_workloads (id, cluster_id, uid, namespace, kind, name, project_id)
		VALUES ($1, $2, 'u1', 'default', 'Deployment', 'api', $3)`, uuid.NewString(), cluster, a.project)
	clusters := a.processes(a.project)["clusters"]
	eq(t, clusters["state"], any("failed"))
	eq(t, clusters["code"], any("cluster.unreachable"))
	execSQL(t, a.db, "UPDATE clusters SET enabled = false WHERE id = $1", cluster)
	_, ok := a.processes(a.project)["clusters"]
	eq(t, ok, false, "a disabled cluster is not polled")
}

func TestCatalogActivitySummaryOfAFolder(t *testing.T) {
	t.Parallel()
	a := startSources(t)
	backend := a.nodeID(a.admin, "folder", a.org, "backend")
	billing := a.nodeID(a.admin, "project", backend, "billing")
	users := a.nodeID(a.admin, "project", backend, "users")
	dir := filepath.Join(a.root, "billing")
	writeFile(t, filepath.Join(dir, "README.md"), "# Billing")
	a.useSource(billing, "local_dir", dir)
	execSQL(t, a.db, "UPDATE knowledge_settings SET lease_until = now() + interval '1 minute' WHERE project_id = $1", billing)
	a.failForge(users, "forge.unauthorized")

	r := a.get(activityPath(backend), a.admin)
	eq(t, r.status, 200, r.text())
	eq(t, jsonText(r.json(t)["summary"]), `{"failed":1,"queued":0,"running":1}`)
	eq(t, at(a.get(activityPath(a.org), a.admin).json(t), "summary", "failed"), any(float64(1)))

	a.user("viewer@example.com", false)
	viewer := a.session("viewer@example.com", password)
	a.grant(a.admin, billing, "user", "viewer@example.com", "viewer")
	r = a.get(activityPath(backend), viewer)
	eq(t, r.status, 200, r.text())
	eq(t, jsonText(r.json(t)["summary"]), `{"failed":0,"queued":0,"running":0}`, "navigation only")
	eq(t, a.get(activityPath(users), viewer).status, 404)
	eq(t, len(a.get(activityPath(billing), viewer).json(t)["processes"].([]any)), 1)

	token := a.pat(viewer, "read")
	eq(t, a.bearer("GET", activityPath(billing), token, nil).status, 200)
	eq(t, a.bearer("GET", activityPath(billing), a.pat(viewer, "write"), nil).status, 403)
}
