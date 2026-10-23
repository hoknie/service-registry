package tests

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"svc-registry/internal/testsupport"
)

func TestKnowledgeRefreshColumns(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tdb := testsupport.NewTestDB(t)
	m := embedded(t)
	_, err := m.Migrate(ctx, tdb.Pool)
	must(t, err)
	org, folder, project := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, slug, name) VALUES ($1, 'organization', 'o', 'O')", org)
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, parent_id, slug, name) VALUES ($1, 'folder', $2, 'f', 'F')", folder, org)
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, parent_id, slug, name) VALUES ($1, 'project', $2, 'p', 'P')", project, folder)
	execSQL(t, tdb, "INSERT INTO knowledge_patterns (node_id, refresh) VALUES ($1, 'event')", org)
	execSQL(t, tdb, "INSERT INTO knowledge_patterns (node_id, exclude, refresh) VALUES ($1, '{tmp/**}', 'interval')", folder)
	contains(t, execErr(t, tdb, "INSERT INTO knowledge_patterns (node_id, refresh) VALUES ($1, 'hourly')", project), "knowledge_patterns_refresh_check")
	execSQL(t, tdb, "INSERT INTO knowledge_settings (project_id, next_run_at, triggered_at) VALUES ($1, NULL, now())", project)
	scan := "INSERT INTO knowledge_scans (id, project_id, kind, trigger, status, started_at, finished_at, last_started_at) " +
		"VALUES ($1, $2, 'collect', $3, 'ok', now(), now(), now())"
	execSQL(t, tdb, scan, uuid.Must(uuid.NewV7()), project, "event")
	contains(t, execErr(t, tdb, scan, uuid.Must(uuid.NewV7()), project, "webhook"), "knowledge_scans_trigger_check")

	n, err := rollbackThrough(t, m, tdb, "alter_knowledge_patterns_refresh")
	must(t, err)
	eq(t, n, migrationsFrom(t, m, "alter_knowledge_patterns_refresh"))
	eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM knowledge_patterns"), int64(1), "a row holding only refresh is dropped")
	eq(t, scalar[bool](t, tdb, "SELECT exclude = '{tmp/**}' FROM knowledge_patterns WHERE node_id = $1", folder), true)
	eq(t, scalar[bool](t, tdb, "SELECT next_run_at IS NOT NULL FROM knowledge_settings WHERE project_id = $1", project), true)
	eq(t, scalar[string](t, tdb, "SELECT trigger FROM knowledge_scans WHERE project_id = $1", project), "schedule")
	eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM information_schema.columns WHERE table_schema = current_schema() "+
		"AND (table_name, column_name) IN (('knowledge_patterns', 'refresh'), ('knowledge_settings', 'triggered_at'))"), int64(0))
	_, err = m.Migrate(ctx, tdb.Pool)
	must(t, err)
}
