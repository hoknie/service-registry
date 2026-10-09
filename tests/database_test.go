package tests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"svc-registry/internal/postgres"
	"svc-registry/internal/testsupport"
)

func fixtureMigrator(*testing.T) postgres.Migrator {
	return postgres.NewMigrator(os.DirFS("fixtures/migrations"))
}

func embedded(*testing.T) postgres.Migrator { return postgres.Embedded() }

func tablesAre(t *testing.T, tdb *testsupport.TestDB, want ...string) {
	t.Helper()
	got := tdb.Tables(t)
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tables %v, want %v", got, want)
	}
}

func embeddedTables(t *testing.T, tdb *testsupport.TestDB) []string {
	var out []string
	for _, n := range tdb.Tables(t) {
		if n != postgres.Ledger {
			out = append(out, n)
		}
	}
	return out
}

func TestReadyIsOKWhenTheDatabaseAnswers(t *testing.T) {
	t.Parallel()
	tdb := testsupport.NewTestDB(t)
	app := spawnApp(t, tdb.URL, testsupport.MissingDist(t).Dir)
	r := request(t, app, "GET", "/api/ready", "")
	eq(t, r.status, 200)
	eq(t, r.text(), `{"status":"ok"}`)
}

func TestMigrateAppliesPendingOnceAndRollbackRevertsNewestFirst(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tdb := testsupport.NewTestDB(t)
	m := fixtureMigrator(t)
	count := func(n int, err error) int { must(t, err); return n }

	eq(t, count(m.Migrate(ctx, tdb.Pool)), 2)
	tablesAre(t, tdb, "gadgets", "schema_migrations", "widgets")
	eq(t, count(m.Migrate(ctx, tdb.Pool)), 0, "idempotent")

	eq(t, count(m.Rollback(ctx, tdb.Pool, 1)), 1)
	tablesAre(t, tdb, "schema_migrations", "widgets")
	eq(t, count(m.Rollback(ctx, tdb.Pool, 5)), 1, "only what is applied")
	tablesAre(t, tdb, "schema_migrations")
	eq(t, count(m.Rollback(ctx, tdb.Pool, 1)), 0)
	eq(t, count(m.Migrate(ctx, tdb.Pool)), 2, "migrate → rollback → migrate")
}

func TestRollbackOfAnEmptySchemaRevertsNothing(t *testing.T) {
	t.Parallel()
	tdb := testsupport.NewTestDB(t)
	n, err := fixtureMigrator(t).Rollback(context.Background(), tdb.Pool, 3)
	must(t, err)
	eq(t, n, 0)
	tablesAre(t, tdb, "schema_migrations")
}

func TestEmbeddedMigrationsApplyThroughTheBinary(t *testing.T) {
	t.Parallel()
	tdb := testsupport.NewTestDB(t)
	code, out, errOut := run(t, []string{"db:migrate"}, "", "DATABASE_URL", tdb.URL)
	eq(t, code, 0, errOut)
	contains(t, out, "applied 46 migration(s)")
	versions, err := embedded(t).Versions()
	must(t, err)
	eq(t, len(versions), 46)
	_, out, _ = run(t, []string{"db:migrate"}, "", "DATABASE_URL", tdb.URL)
	contains(t, out, "applied 0 migration(s)")
	code, out, errOut = run(t, []string{"db:rollback", "--count=4"}, "", "DATABASE_URL", tdb.URL)
	eq(t, code, 0, errOut)
	contains(t, out, "reverted 4 migration(s)")
}

func TestEveryEmbeddedMigrationHasBothDirections(t *testing.T) {
	versions, err := embedded(t).Versions()
	must(t, err)
	entries, err := os.ReadDir("../migrations")
	must(t, err)
	sql := map[string]bool{}
	for _, e := range entries {
		sql[e.Name()] = true
	}
	for _, v := range versions {
		up, down := 0, 0
		for name := range sql {
			if strings.HasPrefix(name, fmt.Sprint(v)+"_") {
				switch {
				case strings.HasSuffix(name, ".up.sql"):
					up++
				case strings.HasSuffix(name, ".down.sql"):
					down++
				}
			}
		}
		eq(t, up == 1 && down == 1, true, v)
	}
	eq(t, versions[0], uint(20261007180001))
}

func TestMigrateFailsCleanlyWhenTheDatabaseIsDown(t *testing.T) {
	code, _, errOut := run(t, []string{"db:migrate"}, "", "DATABASE_URL", deadDB, "DATABASE_ACQUIRE_TIMEOUT_SECS", "1")
	eq(t, code, 1)
	contains(t, errOut, "migration failed")
}

func TestAFailedMigrationLeavesTheSchemaDirtyAndStopsLaterRuns(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tdb := testsupport.NewTestDB(t)
	dir := t.TempDir()
	for name, text := range map[string]string{
		"1_ok.up.sql": "CREATE TABLE ok (id int)", "1_ok.down.sql": "DROP TABLE ok",
		"2_bad.up.sql": "CREATE TABLE bad (id int); SELECT broken", "2_bad.down.sql": "DROP TABLE bad",
	} {
		must(t, os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600))
	}
	m := postgres.NewMigrator(os.DirFS(dir))
	_, err := m.Migrate(ctx, tdb.Pool)
	if err == nil {
		t.Fatal("broken migration applied")
	}
	tablesAre(t, tdb, "ok", "schema_migrations")
	_, err = m.Migrate(ctx, tdb.Pool)
	if err == nil || !strings.Contains(err.Error(), "dirty") {
		t.Fatalf("want a dirty-schema error, got %v", err)
	}
}

func TestTLSRequiredAgainstAServerWithoutTLSIsUnavailable(t *testing.T) {
	t.Parallel()
	tdb := testsupport.NewTestDB(t)
	app := spawnApp(t, tdb.URL+"&sslmode=require", testsupport.MissingDist(t).Dir)
	eq(t, request(t, app, "GET", "/api/ready", "").status, 503)
}

func TestEmbeddedTablesMigrateAndRollBackCompletely(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tdb := testsupport.NewTestDB(t)
	m := embedded(t)
	n, err := m.Migrate(ctx, tdb.Pool)
	must(t, err)
	eq(t, n, 46)
	knowledge := []string{"knowledge_blobs", "knowledge_files", "knowledge_settings", "knowledge_snapshots"}
	oauth := []string{"oauth_login_states", "user_identities"}
	deploy := []string{"cluster_workloads", "clusters", "environments"}
	links := []string{"link_checks", "link_kinds", "link_targets", "link_templates", "node_vars"}
	forge := []string{"forge_connections", "forge_groups", "forge_repositories", "forge_sync_runs"}
	all := []string{"group_members", "groups", "nodes", "personal_access_tokens", "project_events", "project_keys",
		"role_bindings", "service_deployments", "service_environments", "sessions", "users"}
	withBranches := append(append([]string{"branches"}, forge...), all...)
	withLinks := append(append([]string{}, links...), withBranches...)
	slices.Sort(withLinks)
	withDeploy := append(append([]string{}, deploy...), withLinks...)
	slices.Sort(withDeploy)
	withOAuth := append(append([]string{}, oauth...), withDeploy...)
	slices.Sort(withOAuth)
	withKnowledge := append(append([]string{}, knowledge...), withOAuth...)
	slices.Sort(withKnowledge)
	withSources := append([]string{"knowledge_sources"}, withKnowledge...)
	slices.Sort(withSources)
	withPatterns := append([]string{"knowledge_patterns"}, withSources...)
	slices.Sort(withPatterns)
	everything := append([]string{"knowledge_embeddings", "knowledge_index_state"}, withPatterns...)
	slices.Sort(everything)
	withScans := append([]string{"knowledge_scans"}, everything...)
	slices.Sort(withScans)
	eq(t, strings.Join(embeddedTables(t, tdb), ","), strings.Join(withScans, ","))
	n, err = m.Rollback(ctx, tdb.Pool, 1)
	must(t, err)
	eq(t, n, 1)
	eq(t, strings.Join(embeddedTables(t, tdb), ","), strings.Join(withScans, ","))
	n, err = m.Rollback(ctx, tdb.Pool, 1)
	must(t, err)
	eq(t, n, 1)
	eq(t, strings.Join(embeddedTables(t, tdb), ","), strings.Join(withScans, ","))
	n, err = m.Rollback(ctx, tdb.Pool, 1)
	must(t, err)
	eq(t, n, 1)
	eq(t, strings.Join(embeddedTables(t, tdb), ","), strings.Join(everything, ","))
	n, err = m.Rollback(ctx, tdb.Pool, 1)
	must(t, err)
	eq(t, n, 1)
	eq(t, strings.Join(embeddedTables(t, tdb), ","), strings.Join(everything, ","), "the time function has no table")
	eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace "+
		"WHERE p.proname = 'rfc3339' AND n.nspname = current_schema()"), int64(0))
	n, err = m.Rollback(ctx, tdb.Pool, 1)
	must(t, err)
	eq(t, n, 1)
	eq(t, strings.Join(embeddedTables(t, tdb), ","), strings.Join(withPatterns, ","))
	n, err = m.Rollback(ctx, tdb.Pool, 1)
	must(t, err)
	eq(t, n, 1)
	eq(t, strings.Join(embeddedTables(t, tdb), ","), strings.Join(withPatterns, ","))
	n, err = m.Rollback(ctx, tdb.Pool, 1)
	must(t, err)
	eq(t, n, 1)
	eq(t, strings.Join(embeddedTables(t, tdb), ","), strings.Join(withSources, ","))
	n, err = m.Rollback(ctx, tdb.Pool, 1)
	must(t, err)
	eq(t, n, 1)
	eq(t, strings.Join(embeddedTables(t, tdb), ","), strings.Join(withKnowledge, ","))
	n, err = m.Rollback(ctx, tdb.Pool, 4)
	must(t, err)
	eq(t, n, 4)
	eq(t, strings.Join(embeddedTables(t, tdb), ","), strings.Join(withOAuth, ","))
	n, err = m.Rollback(ctx, tdb.Pool, 5)
	must(t, err)
	eq(t, n, 5)
	eq(t, strings.Join(embeddedTables(t, tdb), ","), strings.Join(withDeploy, ","))
	eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM information_schema.columns WHERE table_schema = current_schema() "+
		"AND ((table_name = 'group_members' AND column_name = 'source') OR (table_name = 'sessions' AND column_name = 'method'))"), int64(0))
	eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM link_kinds"), int64(7), "initial kinds")
	eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM environments"), int64(3), "initial environments")
	n, err = m.Rollback(ctx, tdb.Pool, 6)
	must(t, err)
	eq(t, n, 6)
	eq(t, strings.Join(embeddedTables(t, tdb), ","), strings.Join(withLinks, ","))
	eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM information_schema.columns WHERE table_schema = current_schema() "+
		"AND column_name IN ('cluster_observation', 'source')"), int64(0), "altered columns gone")
	n, err = m.Rollback(ctx, tdb.Pool, 5)
	must(t, err)
	eq(t, n, 5)
	eq(t, strings.Join(embeddedTables(t, tdb), ","), strings.Join(withBranches, ","))
	n, err = m.Rollback(ctx, tdb.Pool, 2)
	must(t, err)
	eq(t, n, 2)
	eq(t, strings.Join(embeddedTables(t, tdb), ","), strings.Join(append(append([]string{}, forge...), all...), ","))
	n, err = m.Rollback(ctx, tdb.Pool, 5)
	must(t, err)
	eq(t, n, 5)
	eq(t, strings.Join(embeddedTables(t, tdb), ","), strings.Join(all, ","))
	n, err = m.Rollback(ctx, tdb.Pool, 1)
	must(t, err)
	eq(t, n, 1)
	eq(t, strings.Join(embeddedTables(t, tdb), ","), strings.Join(append(all[:3:3], all[4:]...), ","))
	n, err = m.Rollback(ctx, tdb.Pool, 3)
	must(t, err)
	eq(t, n, 3)
	eq(t, strings.Join(embeddedTables(t, tdb), ","), "group_members,groups,nodes,project_keys,role_bindings,sessions,users")
	n, err = m.Rollback(ctx, tdb.Pool, 3)
	must(t, err)
	eq(t, n, 3)
	eq(t, strings.Join(embeddedTables(t, tdb), ","), "group_members,groups,sessions,users")
	n, err = m.Rollback(ctx, tdb.Pool, 46)
	must(t, err)
	eq(t, n, 4)
	eq(t, len(embeddedTables(t, tdb)), 0)
	n, err = m.Migrate(ctx, tdb.Pool)
	must(t, err)
	eq(t, n, 46)
}

func execErr(t *testing.T, tdb *testsupport.TestDB, query string, args ...any) string {
	t.Helper()
	_, err := tdb.Pool.Exec(context.Background(), query, args...)
	if err == nil {
		t.Fatalf("%s: expected an error", query)
	}
	return err.Error()
}

func TestSchemaConventionsHoldForAccessTables(t *testing.T) {
	t.Parallel()
	tdb := migrated(t)
	contains(t, execErr(t, tdb, "INSERT INTO groups (name) VALUES ('x')"), "null value")
	contains(t, execErr(t, tdb, "INSERT INTO users (id, email, display_name, password_hash, status) "+
		"VALUES (gen_random_uuid(), 'a@example.com', 'A', 'h', 'banned')"), "users_status_check")

	insertUser := "INSERT INTO users (id, email, display_name, password_hash) VALUES ($1, $2, 'A', 'h')"
	user := uuid.Must(uuid.NewV7())
	execSQL(t, tdb, insertUser, user, "a@example.com")
	contains(t, execErr(t, tdb, insertUser, uuid.Must(uuid.NewV7()), "A@EXAMPLE.com"), "users_email_lower_key")

	group, node := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	execSQL(t, tdb, "INSERT INTO groups (id, name) VALUES ($1, 'g')", group)
	execSQL(t, tdb, "INSERT INTO group_members (id, group_id, user_id) VALUES ($1, $2, $3)", uuid.Must(uuid.NewV7()), group, user)
	execSQL(t, tdb, "INSERT INTO sessions (id, user_id, token_hash, expires_at) "+
		"VALUES ($1, $2, decode(repeat('ab', 32), 'hex'), now() + interval '1 day')", uuid.Must(uuid.NewV7()), user)
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, slug, name) VALUES ($1, 'organization', 'o', 'O')", node)
	execSQL(t, tdb, "INSERT INTO role_bindings (id, node_id, user_id, role) VALUES ($1, $2, $3, 'viewer')", uuid.Must(uuid.NewV7()), node, user)
	execSQL(t, tdb, insertToken, uuid.Must(uuid.NewV7()), user, "{read}")
	execSQL(t, tdb, "DELETE FROM users WHERE id = $1", user)
	for _, table := range []string{"sessions", "group_members", "role_bindings", "personal_access_tokens"} {
		eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM "+table), int64(0), table)
	}
}

func TestCatalogConstraintsHoldWithoutTheService(t *testing.T) {
	t.Parallel()
	tdb := migrated(t)
	insert := "INSERT INTO nodes (id, kind, parent_id, slug, name) VALUES ($1, $2, $3, $4, 'N')"
	org, folder := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	execSQL(t, tdb, insert, org, "organization", nil, "o")
	execSQL(t, tdb, insert, folder, "folder", org, "f")
	contains(t, execErr(t, tdb, insert, uuid.Must(uuid.NewV7()), "organization", nil, "o"), "nodes_parent_slug_key")
	contains(t, execErr(t, tdb, "DELETE FROM nodes WHERE id = $1", org), "nodes_parent_id_fkey")
	contains(t, execErr(t, tdb, "UPDATE nodes SET repo_url = 'https://x' WHERE id = $1", folder), "nodes_repo_fields_check")
	contains(t, execErr(t, tdb, "INSERT INTO role_bindings (id, node_id, role) VALUES ($1, $2, 'viewer')", uuid.Must(uuid.NewV7()), org), "role_bindings_subject_check")
}

func TestIngestConstraintsHoldWithoutTheService(t *testing.T) {
	t.Parallel()
	tdb := migrated(t)
	org, project := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, slug, name) VALUES ($1, 'organization', 'o', 'O')", org)
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, parent_id, slug, name) VALUES ($1, 'project', $2, 'p', 'P')", project, org)
	insertEvent := "INSERT INTO project_events (id, project_id, type, version, idempotency_key, occurred_at, payload) " +
		"VALUES ($1, $2, 'service.deployed', 1, 'k-1', now(), '{}'::jsonb)"
	event := uuid.Must(uuid.NewV7())
	execSQL(t, tdb, insertEvent, event, project)
	contains(t, execErr(t, tdb, insertEvent, uuid.Must(uuid.NewV7()), project), "project_events_idempotency_key")
	contains(t, execErr(t, tdb, "INSERT INTO project_events (id, project_id, type, version, idempotency_key, occurred_at, payload) "+
		"VALUES ($1, $2, 'build.finished', 1, 'k-2', now(), '{}'::jsonb)", uuid.Must(uuid.NewV7()), project), "project_events_type_check")

	deployment := uuid.Must(uuid.NewV7())
	execSQL(t, tdb, "INSERT INTO service_deployments (id, project_id, event_id, service, environment, version, occurred_at) "+
		"VALUES ($1, $2, $3, 'api', 'production', '1.0.0', now())", deployment, project, event)
	execSQL(t, tdb, "DELETE FROM project_events WHERE id = $1", event)
	eventID := scalar[*uuid.UUID](t, tdb, "SELECT event_id FROM service_deployments WHERE id = $1", deployment)
	eq(t, eventID == nil, true, "the deployment outlives its raw event")
}

const insertToken = "INSERT INTO personal_access_tokens (id, user_id, name, prefix, token_hash, scopes) " +
	"VALUES ($1, $2, 't', 'svcp_0000000', sha256(convert_to(gen_random_uuid()::text, 'UTF8')), $3::text[])"

func TestPersonalAccessTokenConstraintsHoldWithoutTheService(t *testing.T) {
	t.Parallel()
	tdb := migrated(t)
	user := uuid.Must(uuid.NewV7())
	execSQL(t, tdb, "INSERT INTO users (id, email, display_name, password_hash) VALUES ($1, 'a@example.com', 'A', 'h')", user)
	execSQL(t, tdb, insertToken, uuid.Must(uuid.NewV7()), user, "{read,write,admin,mcp}")
	contains(t, execErr(t, tdb, insertToken, uuid.Must(uuid.NewV7()), user, "{deploy}"), "personal_access_tokens_scopes_check")
	contains(t, execErr(t, tdb, insertToken, uuid.Must(uuid.NewV7()), user, "{}"), "personal_access_tokens_scopes_check")
	contains(t, execErr(t, tdb, "INSERT INTO personal_access_tokens (id, user_id, name, prefix, token_hash, scopes) "+
		"VALUES ($1, $2, 't', 'p', '\\x00', '{read}')", uuid.Must(uuid.NewV7()), user), "personal_access_tokens_token_hash_len_check")
	expires := scalar[*string](t, tdb, "SELECT expires_at::text FROM personal_access_tokens LIMIT 1")
	eq(t, expires == nil, true, "no expiry by default")
}

func TestForgeConstraintsHoldWithoutTheService(t *testing.T) {
	t.Parallel()
	tdb := migrated(t)
	org, project, conn := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, slug, name) VALUES ($1, 'organization', 'o', 'O')", org)
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, parent_id, slug, name, forge) VALUES ($1, 'project', $2, 'p', 'P', 'gitea')", project, org)
	insertConn := "INSERT INTO forge_connections (id, node_id, kind, api_url, owner_path, interval_secs, credentials_enc, credentials_ref) " +
		"VALUES ($1, $2, 'github', 'https://api.github.com', $3, 900, $4, $5)"
	contains(t, execErr(t, tdb, insertConn, conn, org, "acme", "v1.k.x", "env:X"), "forge_connections_credentials_check")
	contains(t, execErr(t, tdb, insertConn, conn, org, "acme", nil, nil), "forge_connections_credentials_check")
	execSQL(t, tdb, insertConn, conn, org, "acme", nil, "env:X")
	contains(t, execErr(t, tdb, insertConn, uuid.Must(uuid.NewV7()), org, "ACME", nil, "env:Y"), "forge_connections_owner_key")
	execSQL(t, tdb, "INSERT INTO forge_repositories (project_id, connection_id, external_id, full_path, web_url, visibility) "+
		"VALUES ($1, $2, '1', 'acme/p', 'https://github.com/acme/p', 'public')", project, conn)
	execSQL(t, tdb, "INSERT INTO forge_sync_runs (id, connection_id, trigger, status) VALUES ($1, $2, 'manual', 'running')", uuid.Must(uuid.NewV7()), conn)
	execSQL(t, tdb, "DELETE FROM forge_connections WHERE id = $1", conn)
	for _, table := range []string{"forge_repositories", "forge_sync_runs"} {
		eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM "+table), int64(0), table)
	}
	eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM nodes"), int64(2), "nodes stay")
}

func TestBranchesTableHoldsItsRulesAndBackfillsDefaults(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tdb := testsupport.NewTestDB(t)
	m := embedded(t)
	_, err := m.Migrate(ctx, tdb.Pool)
	must(t, err)
	_, err = m.Rollback(ctx, tdb.Pool, 30)
	must(t, err)
	org, project, other := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, slug, name) VALUES ($1, 'organization', 'o', 'O')", org)
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, parent_id, slug, name, default_branch) VALUES ($1, 'project', $2, 'p', 'P', 'main')", project, org)
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, parent_id, slug, name) VALUES ($1, 'project', $2, 'q', 'Q')", other, org)
	n, err := m.Migrate(ctx, tdb.Pool)
	must(t, err)
	eq(t, n, 30)
	eq(t, scalar[string](t, tdb, "SELECT name FROM branches WHERE project_id = $1 AND is_default", project), "main")
	eq(t, scalar[string](t, tdb, "SELECT substr(id::text, 15, 1) FROM branches"), "7", "UUID v7")
	eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM branches"), int64(1))
	eq(t, scalar[string](t, tdb, "SELECT column_default FROM information_schema.columns "+
		"WHERE table_schema = current_schema() AND table_name = 'forge_connections' AND column_name = 'branch_include'"), "'{}'::text[]")

	insert := "INSERT INTO branches (id, project_id, name, is_default, sources, head_sha) VALUES ($1, $2, $3, $4, $5::text[], $6)"
	contains(t, execErr(t, tdb, insert, uuid.Must(uuid.NewV7()), project, "dev", true, "{ingest}", nil), "branches_default_key")
	contains(t, execErr(t, tdb, insert, uuid.Must(uuid.NewV7()), project, "dev", false, "{svn}", nil), "branches_sources_check")
	contains(t, execErr(t, tdb, insert, uuid.Must(uuid.NewV7()), project, "dev", false, "{ingest}", "XYZ"), "branches_head_sha_check")
	contains(t, execErr(t, tdb, insert, uuid.Must(uuid.NewV7()), project, "main", false, "{ingest}", nil), "branches_name_key")
	execSQL(t, tdb, "DELETE FROM nodes WHERE id = $1", project)
	eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM branches"), int64(0), "cascade")
}

func TestLinkTablesHoldTheirRulesWithoutTheService(t *testing.T) {
	t.Parallel()
	tdb := migrated(t)
	eq(t, scalar[string](t, tdb, "SELECT string_agg(key, ',' ORDER BY position) FROM link_kinds"),
		"logs,grafana,sentry,alertmanager,traces,runbook,docs")
	eq(t, scalar[string](t, tdb, "SELECT names->>'ru' FROM link_kinds WHERE key = 'logs'"), "Логи")
	eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM link_kinds WHERE substr(id::text, 15, 1) <> '7'"), int64(0), "UUID v7")
	insertKind := "INSERT INTO link_kinds (id, key, names, icon) VALUES ($1, $2, $3::jsonb, $4)"
	names := `{"en":"A","es":"A","ru":"A","zh":"A"}`
	contains(t, execErr(t, tdb, insertKind, uuid.Must(uuid.NewV7()), "x", `{"en":"A","ru":"A"}`, "link"), "link_kinds_names_check")
	contains(t, execErr(t, tdb, insertKind, uuid.Must(uuid.NewV7()), "x", `{"en":"A","es":"A","ru":"A","zh":"A","de":"A"}`, "link"), "link_kinds_names_check")
	contains(t, execErr(t, tdb, insertKind, uuid.Must(uuid.NewV7()), "x", names, "rocket"), "link_kinds_icon_check")
	contains(t, execErr(t, tdb, insertKind, uuid.Must(uuid.NewV7()), "logs", names, "link"), "link_kinds_key_key")

	org := uuid.Must(uuid.NewV7())
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, slug, name) VALUES ($1, 'organization', 'o', 'O')", org)
	insertTemplate := "INSERT INTO link_templates (id, node_id, kind_id, link_key, template, disabled) " +
		"VALUES ($1, $2, (SELECT id FROM link_kinds WHERE key = 'grafana'), $3, $4, $5)"
	contains(t, execErr(t, tdb, insertTemplate, uuid.Must(uuid.NewV7()), org, "g", nil, false), "link_templates_template_check")
	contains(t, execErr(t, tdb, insertTemplate, uuid.Must(uuid.NewV7()), org, "g", "https://x", true), "link_templates_template_check")
	execSQL(t, tdb, insertTemplate, uuid.Must(uuid.NewV7()), org, "grafana", "https://x", false)
	contains(t, execErr(t, tdb, insertTemplate, uuid.Must(uuid.NewV7()), org, "grafana", nil, true), "link_templates_link_key_key")
	contains(t, execErr(t, tdb, "DELETE FROM link_kinds WHERE key = 'grafana'"), "link_templates_kind_id_fkey")
	execSQL(t, tdb, "INSERT INTO node_vars (id, node_id, key, value) VALUES ($1, $2, 'a', '1')", uuid.Must(uuid.NewV7()), org)
	execSQL(t, tdb, "DELETE FROM nodes WHERE id = $1", org)
	for _, table := range []string{"link_templates", "node_vars"} {
		eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM "+table), int64(0), table)
	}

	target := uuid.Must(uuid.NewV7())
	execSQL(t, tdb, "INSERT INTO link_targets (id, url) VALUES ($1, 'https://x.example/')", target)
	contains(t, execErr(t, tdb, "INSERT INTO link_targets (id, url) VALUES ($1, 'https://x.example/')", uuid.Must(uuid.NewV7())), "link_targets_url_key")
	insertCheck := "INSERT INTO link_checks (id, target_id, status, duration_ms) VALUES ($1, $2, $3, 1)"
	execSQL(t, tdb, insertCheck, uuid.Must(uuid.NewV7()), target, "ok")
	contains(t, execErr(t, tdb, insertCheck, uuid.Must(uuid.NewV7()), target, "fine"), "link_checks_status_check")
	execSQL(t, tdb, "DELETE FROM link_targets WHERE id = $1", target)
	eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM link_checks"), int64(0), "cascade")
}

func TestClusterTablesHoldTheirRulesWithoutTheService(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tdb := migrated(t)
	eq(t, scalar[string](t, tdb, "SELECT string_agg(key, ',' ORDER BY position) FROM environments"), "production,staging,development")
	eq(t, scalar[string](t, tdb, "SELECT names->>'ru' FROM environments WHERE key = 'production'"), "Продакшн")
	contains(t, execErr(t, tdb, "INSERT INTO environments (id, key, names) VALUES ($1, 'qa', '{\"en\":\"QA\"}')", uuid.Must(uuid.NewV7())), "environments_names_check")

	insertCluster := "INSERT INTO clusters (id, name, environment, in_cluster, api_url, credentials_enc, credentials_ref, interval_secs) " +
		"VALUES ($1, $2, 'production', $3, $4, $5, $6, $7)"
	cluster := uuid.Must(uuid.NewV7())
	contains(t, execErr(t, tdb, insertCluster, cluster, "a", true, "https://k8s", nil, nil, 60), "clusters_credentials_check")
	contains(t, execErr(t, tdb, insertCluster, cluster, "a", false, "https://k8s", "v1.k.x", "env:X", 60), "clusters_credentials_check")
	contains(t, execErr(t, tdb, insertCluster, cluster, "a", false, "https://k8s", nil, nil, 60), "clusters_credentials_check")
	contains(t, execErr(t, tdb, insertCluster, cluster, "a", false, "https://k8s", nil, "env:X", 14), "clusters_interval_check")
	execSQL(t, tdb, insertCluster, cluster, "Prod", false, "https://k8s", nil, "env:X", 60)
	contains(t, execErr(t, tdb, insertCluster, uuid.Must(uuid.NewV7()), "prod", true, nil, nil, nil, 60), "clusters_name_key")

	org, project := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, slug, name) VALUES ($1, 'organization', 'o', 'O')", org)
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, parent_id, slug, name) VALUES ($1, 'project', $2, 'p', 'P')", project, org)
	eq(t, scalar[bool](t, tdb, "SELECT cluster_observation FROM nodes WHERE id = $1", project), true)
	insertWorkload := "INSERT INTO cluster_workloads (id, cluster_id, uid, namespace, kind, name, project_id, reason) VALUES ($1, $2, $3, 'ns', $4, 'w', $5, $6)"
	contains(t, execErr(t, tdb, insertWorkload, uuid.Must(uuid.NewV7()), cluster, "u0", "Job", project, nil), "cluster_workloads_kind_check")
	contains(t, execErr(t, tdb, insertWorkload, uuid.Must(uuid.NewV7()), cluster, "u0", "Deployment", nil, nil), "cluster_workloads_match_check")
	execSQL(t, tdb, insertWorkload, uuid.Must(uuid.NewV7()), cluster, "u1", "Deployment", project, nil)
	execSQL(t, tdb, insertWorkload, uuid.Must(uuid.NewV7()), cluster, "u2", "CronJob", nil, "no_annotation")
	contains(t, execErr(t, tdb, insertWorkload, uuid.Must(uuid.NewV7()), cluster, "u1", "Deployment", project, nil), "cluster_workloads_uid_key")
	execSQL(t, tdb, "DELETE FROM nodes WHERE id = $1", project)
	eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM cluster_workloads"), int64(1), "project cascade")
	execSQL(t, tdb, "DELETE FROM clusters WHERE id = $1", cluster)
	eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM cluster_workloads"), int64(0), "cluster cascade")

	project = uuid.Must(uuid.NewV7())
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, parent_id, slug, name) VALUES ($1, 'project', $2, 'q', 'Q')", project, org)
	execSQL(t, tdb, "INSERT INTO service_deployments (id, project_id, service, environment, version, occurred_at, source) "+
		"VALUES ($1, $2, 'api', 'production', '1.0.0', now(), 'cluster')", uuid.Must(uuid.NewV7()), project)
	contains(t, execErr(t, tdb, "INSERT INTO service_deployments (id, project_id, service, environment, version, occurred_at, source) "+
		"VALUES ($1, $2, 'api', 'production', '1.0.0', now(), 'manual')", uuid.Must(uuid.NewV7()), project), "service_deployments_source_check")
	execSQL(t, tdb, "INSERT INTO branches (id, project_id, name, sources) VALUES ($1, $2, 'main', '{cluster}')", uuid.Must(uuid.NewV7()), project)
	m := embedded(t)
	if _, err := m.Rollback(ctx, tdb.Pool, 18); err == nil {
		t.Fatal("branches rollback with a cluster source succeeded")
	}
}

func TestDeploymentsFromClustersBlockTheSourceRollback(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tdb := migrated(t)
	org, project := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, slug, name) VALUES ($1, 'organization', 'o', 'O')", org)
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, parent_id, slug, name) VALUES ($1, 'project', $2, 'p', 'P')", project, org)
	execSQL(t, tdb, "INSERT INTO service_deployments (id, project_id, service, environment, version, occurred_at, source) "+
		"VALUES ($1, $2, 'api', 'production', '1.0.0', now(), 'cluster')", uuid.Must(uuid.NewV7()), project)
	if _, err := embedded(t).Rollback(ctx, tdb.Pool, 19); err == nil {
		t.Fatal("rollback of service_deployments.source succeeded")
	}
	eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM service_deployments WHERE source = 'cluster'"), int64(1), "nothing lost")
}

func TestProviderLoginTablesHoldTheirRules(t *testing.T) {
	t.Parallel()
	tdb := migrated(t)
	user := uuid.Must(uuid.NewV7())
	execSQL(t, tdb, "INSERT INTO users (id, email, display_name, password_hash) VALUES ($1, 'a@example.com', 'A', NULL)", user)
	insertIdentity := "INSERT INTO user_identities (id, user_id, provider, subject) VALUES ($1, $2, 'corp', 'sub-1')"
	execSQL(t, tdb, insertIdentity, uuid.Must(uuid.NewV7()), user)
	contains(t, execErr(t, tdb, insertIdentity, uuid.Must(uuid.NewV7()), user), "user_identities_subject_key")
	execSQL(t, tdb, "INSERT INTO oauth_login_states (id, browser_hash, provider, state, nonce, code_verifier, expires_at) "+
		"VALUES ($1, sha256('b'::bytea), 'corp', 's', 'n', 'v', now() + interval '10 minutes')", uuid.Must(uuid.NewV7()))
	contains(t, execErr(t, tdb, "INSERT INTO oauth_login_states (id, browser_hash, provider, state, nonce, code_verifier, expires_at) "+
		"VALUES ($1, '\\x00', 'corp', 's2', 'n', 'v', now())", uuid.Must(uuid.NewV7())), "oauth_login_states_browser_hash_len_check")
	group := uuid.Must(uuid.NewV7())
	execSQL(t, tdb, "INSERT INTO groups (id, name) VALUES ($1, 'g')", group)
	eq(t, scalar[string](t, tdb, "SELECT column_default FROM information_schema.columns WHERE table_schema = current_schema() "+
		"AND table_name = 'sessions' AND column_name = 'method'"), "'password'::text")
	contains(t, execErr(t, tdb, "INSERT INTO group_members (id, group_id, user_id, source) VALUES ($1, $2, $3, 'idp')",
		uuid.Must(uuid.NewV7()), group, user), "group_members_source_check")
	execSQL(t, tdb, "INSERT INTO group_members (id, group_id, user_id, source) VALUES ($1, $2, $3, 'oauth:corp')", uuid.Must(uuid.NewV7()), group, user)
	if _, err := embedded(t).Rollback(context.Background(), tdb.Pool, 14); err == nil {
		t.Fatal("rollback with managed memberships succeeded")
	}
}

func TestUsersWithoutPasswordBlockTheRollback(t *testing.T) {
	t.Parallel()
	tdb := migrated(t)
	execSQL(t, tdb, "INSERT INTO users (id, email, display_name, password_hash) VALUES ($1, 'a@example.com', 'A', NULL)", uuid.Must(uuid.NewV7()))
	if _, err := embedded(t).Rollback(context.Background(), tdb.Pool, 15); err == nil {
		t.Fatal("rollback with users without a password succeeded")
	}
}

func TestKnowledgeTablesHoldTheirRules(t *testing.T) {
	t.Parallel()
	tdb := migrated(t)
	org, project := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, slug, name) VALUES ($1, 'organization', 'o', 'O')", org)
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, parent_id, slug, name) VALUES ($1, 'project', $2, 'p', 'P')", project, org)
	execSQL(t, tdb, "INSERT INTO knowledge_settings (project_id) VALUES ($1)", project)
	execSQL(t, tdb, "INSERT INTO knowledge_blobs (sha256, content, bytes) VALUES (sha256('hello world'::bytea), 'hello world', 11)")
	eq(t, scalar[bool](t, tdb, "SELECT tsv @@ to_tsquery('simple', 'world') FROM knowledge_blobs"), true, "indexed")
	contains(t, execErr(t, tdb, "INSERT INTO knowledge_blobs (sha256, content, bytes) VALUES ('\\x00', 'x', 1)"), "knowledge_blobs_sha256_len_check")
	snapshot := uuid.Must(uuid.NewV7())
	execSQL(t, tdb, "INSERT INTO knowledge_snapshots (id, project_id, branch, commit_sha, status) VALUES ($1, $2, 'main', 'abc', 'ok')", snapshot, project)
	contains(t, execErr(t, tdb, "INSERT INTO knowledge_snapshots (id, project_id, branch, commit_sha, status) VALUES ($1, $2, 'main', 'abc', 'done')",
		uuid.Must(uuid.NewV7()), project), "knowledge_snapshots_status_check")
	insertFile := "INSERT INTO knowledge_files (id, snapshot_id, path, git_blob_sha, sha256, bytes, skip_reason, kind) " +
		"VALUES ($1, $2, $3, 'g', $4, 11, $5, 'doc')"
	hello := scalar[[]byte](t, tdb, "SELECT sha256 FROM knowledge_blobs")
	execSQL(t, tdb, insertFile, uuid.Must(uuid.NewV7()), snapshot, "README.md", hello, nil)
	execSQL(t, tdb, insertFile, uuid.Must(uuid.NewV7()), snapshot, "big.md", nil, "too_large")
	contains(t, execErr(t, tdb, insertFile, uuid.Must(uuid.NewV7()), snapshot, "none.md", nil, nil), "knowledge_files_content_check")
	contains(t, execErr(t, tdb, insertFile, uuid.Must(uuid.NewV7()), snapshot, "README.md", hello, nil), "knowledge_files_path_key")
	contains(t, execErr(t, tdb, "DELETE FROM knowledge_blobs"), "knowledge_files_sha256_fkey")
	execSQL(t, tdb, "DELETE FROM nodes WHERE id = $1", project)
	for _, table := range []string{"knowledge_settings", "knowledge_snapshots", "knowledge_files"} {
		eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM "+table), int64(0), table)
	}
	eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM knowledge_blobs"), int64(1))
}

func TestKnowledgeSourcesHoldTheirRules(t *testing.T) {
	t.Parallel()
	tdb := migrated(t)
	org, project := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, slug, name) VALUES ($1, 'organization', 'o', 'O')", org)
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, parent_id, slug, name) VALUES ($1, 'project', $2, 'p', 'P')", project, org)
	contains(t, execErr(t, tdb, "INSERT INTO knowledge_sources (project_id, kind) VALUES ($1, 'local_dir')", project), "knowledge_sources_fields_check")
	contains(t, execErr(t, tdb, "INSERT INTO knowledge_sources (project_id, kind, forge, url, api_url, path) "+
		"VALUES ($1, 'remote', 'github', 'https://github.com/a/b', 'https://api.github.com', '/srv')", project), "knowledge_sources_fields_check")
	contains(t, execErr(t, tdb, "INSERT INTO knowledge_sources (project_id, kind, path) VALUES ($1, 'svn', '/srv')", project), "knowledge_sources_kind_check")
	execSQL(t, tdb, "INSERT INTO knowledge_sources (project_id, kind, path) VALUES ($1, 'local_git', '/srv/repos/p')", project)
	execSQL(t, tdb, "DELETE FROM nodes WHERE id = $1", project)
	eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM knowledge_sources"), int64(0), "cascade")
}

func TestKnowledgePatternsMoveAndHoldTheirRules(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tdb := testsupport.NewTestDB(t)
	m := embedded(t)
	_, err := m.Migrate(ctx, tdb.Pool)
	must(t, err)
	_, err = m.Rollback(ctx, tdb.Pool, 7)
	must(t, err)
	org, docs, plain := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, slug, name) VALUES ($1, 'organization', 'o', 'O')", org)
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, parent_id, slug, name) VALUES ($1, 'project', $2, 'd', 'D')", docs, org)
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, parent_id, slug, name) VALUES ($1, 'project', $2, 'p', 'P')", plain, org)
	execSQL(t, tdb, "INSERT INTO knowledge_settings (project_id, include, branches) VALUES ($1, '{docs/**}', '{release/*}')", docs)
	execSQL(t, tdb, "INSERT INTO knowledge_settings (project_id) VALUES ($1)", plain)
	_, err = m.Migrate(ctx, tdb.Pool)
	must(t, err)
	eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM knowledge_patterns"), int64(1), "defaults are not moved")
	eq(t, scalar[bool](t, tdb, "SELECT include_set AND include = '{docs/**}' AND exclude IS NULL AND branches = '{release/*}' FROM knowledge_patterns WHERE node_id = $1", docs), true)
	eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM information_schema.columns WHERE table_schema = current_schema() "+
		"AND table_name = 'knowledge_settings' AND column_name IN ('include', 'exclude', 'branches')"), int64(0))
	contains(t, execErr(t, tdb, "INSERT INTO knowledge_patterns (node_id, include_set, include) VALUES ($1, false, '{x}')", org), "knowledge_patterns_include_check")
	execSQL(t, tdb, "INSERT INTO knowledge_patterns (node_id, exclude) VALUES ($1, '{tmp/**}')", org)
	_, err = m.Rollback(ctx, tdb.Pool, 7)
	must(t, err)
	eq(t, scalar[bool](t, tdb, "SELECT include = '{docs/**}' AND exclude = '{}' AND branches = '{release/*}' FROM knowledge_settings WHERE project_id = $1", docs), true)
	eq(t, scalar[bool](t, tdb, "SELECT include IS NULL AND exclude = '{}' FROM knowledge_settings WHERE project_id = $1", plain), true)
	_, err = m.Migrate(ctx, tdb.Pool)
	must(t, err)
	execSQL(t, tdb, "DELETE FROM nodes WHERE id = $1", docs)
	eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM knowledge_patterns"), int64(0), "cascade")
}

func TestKnowledgeIndexHoldsStemsAndExactForms(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tdb := testsupport.NewTestDB(t)
	m := embedded(t)
	_, err := m.Migrate(ctx, tdb.Pool)
	must(t, err)
	_, err = m.Rollback(ctx, tdb.Pool, 6)
	must(t, err)
	execSQL(t, tdb, "INSERT INTO knowledge_blobs (sha256, content, bytes) VALUES (sha256('a'::bytea), 'deployments', 11)")
	eq(t, scalar[bool](t, tdb, "SELECT tsv @@ to_tsquery('russian', 'deploy') FROM knowledge_blobs"), false, "exact forms only")
	_, err = m.Migrate(ctx, tdb.Pool)
	must(t, err)
	eq(t, scalar[bool](t, tdb, "SELECT tsv @@ to_tsquery('russian', 'deploy') FROM knowledge_blobs"), true, "rebuilt")
	execSQL(t, tdb, "INSERT INTO knowledge_blobs (sha256, content, bytes) VALUES (sha256('b'::bytea), 'ветка по умолчанию', 33)")
	branch := "SELECT tsv @@ to_tsquery($1::regconfig, $2) FROM knowledge_blobs WHERE content LIKE 'ветка%'"
	eq(t, scalar[bool](t, tdb, branch, "russian", "ветки"), true, "stem")
	eq(t, scalar[bool](t, tdb, branch, "simple", "по"), true, "exact stop word")
	_, err = m.Rollback(ctx, tdb.Pool, 6)
	must(t, err)
	eq(t, scalar[bool](t, tdb, branch, "russian", "ветки"), false, "rolled back")
	eq(t, scalar[bool](t, tdb, branch, "simple", "ветка"), true)
}

func TestKnowledgeEmbeddingsGoWithTheirContent(t *testing.T) {
	t.Parallel()
	tdb := migrated(t)
	org, project := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, slug, name) VALUES ($1, 'organization', 'o', 'O')", org)
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, parent_id, slug, name) VALUES ($1, 'project', $2, 'p', 'P')", project, org)
	execSQL(t, tdb, "INSERT INTO knowledge_blobs (sha256, content, bytes) VALUES (sha256('x'::bytea), 'hello', 5)")
	execSQL(t, tdb, "INSERT INTO knowledge_embeddings (sha256, model, ord, chunk_start, chunk_end, vector) "+
		"SELECT sha256, 'm', 0, 0, 5, '{0.1,0.2}' FROM knowledge_blobs")
	contains(t, execErr(t, tdb, "INSERT INTO knowledge_embeddings (sha256, model, ord, chunk_start, chunk_end, vector) "+
		"SELECT sha256, 'm', 1, 5, 5, '{0.1}' FROM knowledge_blobs"), "knowledge_embeddings_range_check")
	execSQL(t, tdb, "INSERT INTO knowledge_index_state (project_id) VALUES ($1)", project)
	execSQL(t, tdb, "DELETE FROM knowledge_blobs")
	eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM knowledge_embeddings"), int64(0), "cascade")
	execSQL(t, tdb, "DELETE FROM nodes WHERE id = $1", project)
	eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM knowledge_index_state"), int64(0), "cascade")
	eq(t, scalar[bool](t, tdb, "SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'vector')"), true, "pgvector in the test image")
}

func TestRFC3339FunctionFormatsInUTC(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tdb := testsupport.NewTestDB(t)
	m := embedded(t)
	_, err := m.Migrate(ctx, tdb.Pool)
	must(t, err)
	eq(t, scalar[string](t, tdb, "SELECT rfc3339('2025-06-01 12:00:00.75+03'::timestamptz)"), "2025-06-01T09:00:00Z")
	eq(t, scalar[bool](t, tdb, "SELECT rfc3339(NULL::timestamptz) IS NULL"), true)
	eq(t, scalar[bool](t, tdb, "SELECT rfc3339(now()) = to_char(now() AT TIME ZONE 'UTC', 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"')"), true)
	n, err := m.Rollback(ctx, tdb.Pool, 4)
	must(t, err)
	eq(t, n, 4)
	contains(t, execErr(t, tdb, "SELECT rfc3339(now())"), "rfc3339")
}

func TestKnowledgeScansTableHoldsItsRules(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tdb := testsupport.NewTestDB(t)
	m := embedded(t)
	_, err := m.Migrate(ctx, tdb.Pool)
	must(t, err)
	org, project := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, slug, name) VALUES ($1, 'organization', 'o', 'O')", org)
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, parent_id, slug, name) VALUES ($1, 'project', $2, 'p', 'P')", project, org)
	insert := "INSERT INTO knowledge_scans (id, project_id, kind, trigger, status, started_at, finished_at) VALUES ($1, $2, 'collect', 'schedule', $3, now(), now())"
	execSQL(t, tdb, insert, uuid.Must(uuid.NewV7()), project, "ok")
	contains(t, execErr(t, tdb, insert, uuid.Must(uuid.NewV7()), project, "broken"), "knowledge_scans_status_check")
	execSQL(t, tdb, "DELETE FROM nodes WHERE id = $1", project)
	eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM knowledge_scans"), int64(0), "cascade")
	n, err := m.Rollback(ctx, tdb.Pool, 3)
	must(t, err)
	eq(t, n, 3)
	eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = 'knowledge_scans'"), int64(0))
	eq(t, scalar[string](t, tdb, "SELECT rfc3339('2025-06-01 12:00:00+00'::timestamptz)"), "2025-06-01T12:00:00Z")
}

func TestKnowledgeSourcesWorkingTreeColumn(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tdb := testsupport.NewTestDB(t)
	m := embedded(t)
	_, err := m.Migrate(ctx, tdb.Pool)
	must(t, err)
	_, err = m.Rollback(ctx, tdb.Pool, 2)
	must(t, err)
	org, project := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, slug, name) VALUES ($1, 'organization', 'o', 'O')", org)
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, parent_id, slug, name) VALUES ($1, 'project', $2, 'p', 'P')", project, org)
	execSQL(t, tdb, "INSERT INTO knowledge_sources (project_id, kind, path) VALUES ($1, 'local_git', '/srv/repo')", project)
	_, err = m.Migrate(ctx, tdb.Pool)
	must(t, err)
	eq(t, scalar[bool](t, tdb, "SELECT working_tree FROM knowledge_sources WHERE project_id = $1", project), true)
	n, err := m.Rollback(ctx, tdb.Pool, 2)
	must(t, err)
	eq(t, n, 2)
	eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM information_schema.columns WHERE table_schema = current_schema() "+
		"AND table_name = 'knowledge_sources' AND column_name = 'working_tree'"), int64(0))
}

func TestKnowledgeSourcesIncludeIgnoredColumn(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tdb := testsupport.NewTestDB(t)
	m := embedded(t)
	_, err := m.Migrate(ctx, tdb.Pool)
	must(t, err)
	_, err = m.Rollback(ctx, tdb.Pool, 1)
	must(t, err)
	org, project, dir := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, slug, name) VALUES ($1, 'organization', 'o', 'O')", org)
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, parent_id, slug, name) VALUES ($1, 'project', $2, 'p', 'P')", project, org)
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, parent_id, slug, name) VALUES ($1, 'project', $2, 'd', 'D')", dir, org)
	execSQL(t, tdb, "INSERT INTO knowledge_sources (project_id, kind, path) VALUES ($1, 'local_git', '/srv/repo')", project)
	execSQL(t, tdb, "INSERT INTO knowledge_sources (project_id, kind, path) VALUES ($1, 'local_dir', '/srv/docs')", dir)
	_, err = m.Migrate(ctx, tdb.Pool)
	must(t, err)
	eq(t, scalar[bool](t, tdb, "SELECT include_ignored FROM knowledge_sources WHERE project_id = $1", project), false)
	execSQL(t, tdb, "UPDATE knowledge_sources SET include_ignored = true WHERE project_id = $1", project)
	contains(t, execErr(t, tdb, "UPDATE knowledge_sources SET working_tree = false WHERE project_id = $1", project), "check")
	contains(t, execErr(t, tdb, "UPDATE knowledge_sources SET include_ignored = true WHERE project_id = $1", dir), "check")
	n, err := m.Rollback(ctx, tdb.Pool, 1)
	must(t, err)
	eq(t, n, 1)
	eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM information_schema.columns WHERE table_schema = current_schema() "+
		"AND table_name = 'knowledge_sources' AND column_name IN ('include_ignored', 'working_tree')"), int64(1))
}
