package tests

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"svc-registry/internal/testsupport"
)

func TestSchemaHasNoForgeNames(t *testing.T) {
	t.Parallel()
	tdb := migrated(t)
	eq(t, scalar[int64](t, tdb, `
		SELECT (SELECT count(*) FROM pg_class WHERE relnamespace = current_schema()::regnamespace AND relname ~ 'forge(?!jo|t)')
			+ (SELECT count(*) FROM pg_constraint WHERE connamespace = current_schema()::regnamespace AND conname ~ 'forge(?!jo|t)')
			+ (SELECT count(*) FROM pg_attribute a JOIN pg_class c ON c.oid = a.attrelid
				WHERE c.relnamespace = current_schema()::regnamespace AND NOT a.attisdropped AND a.attname ~ 'forge(?!jo|t)')
			+ (SELECT count(*) FROM pg_constraint WHERE connamespace = current_schema()::regnamespace
				AND pg_get_constraintdef(oid) ~ 'forge(?!jo|t)')
			+ (SELECT count(*) FROM pg_proc WHERE pronamespace = 'public'::regnamespace AND prosrc ~ 'forge(?!jo|t)')`), int64(0))
}

func TestRepoSyncMigrationsRewriteStoredValues(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tdb := testsupport.NewTestDB(t)
	m := embedded(t)
	_, err := m.Migrate(ctx, tdb.Pool)
	must(t, err)
	_, err = rollbackThrough(t, m, tdb, "alter_forge_connections_rename")
	must(t, err)
	org, project, conn, run := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	sub, failed, recovered := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, slug, name) VALUES ($1, 'organization', 'o', 'O')", org)
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, parent_id, slug, name, forge) VALUES ($1, 'project', $2, 'p', 'P', 'gitea')", project, org)
	secret := uuid.Must(uuid.NewV7())
	execSQL(t, tdb, "INSERT INTO secrets (id, name, value_ref) VALUES ($1, 'gh', 'env:GH')", secret)
	execSQL(t, tdb, "INSERT INTO forge_connections (id, node_id, kind, api_url, owner_path, interval_secs, credentials_secret_id) "+
		"VALUES ($1, $2, 'github', 'https://api.github.com', 'acme', 900, $3)", conn, org, secret)
	execSQL(t, tdb, "INSERT INTO forge_sync_runs (id, connection_id, trigger, status, error_code, problems) "+
		`VALUES ($1, $2, 'manual', 'failed', 'forge.unauthorized', '[{"full_path":"acme/a","code":"forge.value_too_long"}]')`, run, conn)
	execSQL(t, tdb, "INSERT INTO branches (id, project_id, name, sources) VALUES ($1, $2, 'main', '{forge,ingest}')", uuid.Must(uuid.NewV7()), project)
	execSQL(t, tdb, "INSERT INTO knowledge_patterns (node_id, refresh_events) VALUES ($1, '{service.deployed,forge.webhook,forge.manual}')", org)
	execSQL(t, tdb, "INSERT INTO notification_subscriptions (id, node_id, key, channel, secret_id, event_types) "+
		"VALUES ($1, $2, 'k', 'webhook', $3, '{forge.sync_failed,forge.sync_recovered}')", sub, org, secret)
	payload := `{"type":"forge.sync_failed","data":{"forge":"github","error_code":"forge.interrupted"}}`
	execSQL(t, tdb, "INSERT INTO notification_events (id, type, node_id, payload, dedupe_key, occurred_at) "+
		"VALUES ($1, 'forge.sync_failed', $2, $3, $5, now()), ($4, 'forge.sync_recovered', $2, '{}', 'r', now())",
		failed, org, payload, recovered, "forge:"+failed.String())
	execSQL(t, tdb, "INSERT INTO notification_episodes (key, id, type, node_id, signature, event_id) VALUES ('e', $3, 'forge.sync_failed', $1, 's', $2)", org, failed, uuid.Must(uuid.NewV7()))
	_, err = m.Migrate(ctx, tdb.Pool)
	must(t, err)
	eq(t, scalar[string](t, tdb, "SELECT hosting FROM nodes WHERE id = $1", project), "gitea")
	eq(t, scalar[string](t, tdb, "SELECT error_code || ' ' || (problems->0->>'code') FROM repo_sync_runs WHERE id = $1", run),
		"hosting.unauthorized hosting.value_too_long")
	eq(t, scalar[string](t, tdb, "SELECT array_to_string(sources, ',') FROM branches WHERE project_id = $1", project), "repo_sync,ingest")
	eq(t, scalar[string](t, tdb, "SELECT array_to_string(refresh_events, ',') FROM knowledge_patterns WHERE node_id = $1", org),
		"service.deployed,repo_sync.webhook,repo_sync.manual")
	eq(t, scalar[string](t, tdb, "SELECT array_to_string(event_types, ',') FROM notification_subscriptions WHERE id = $1", sub),
		"repo_sync.failed,repo_sync.recovered")
	eq(t, scalar[string](t, tdb, "SELECT type || ' ' || left(dedupe_key, 10) || ' ' || (payload->>'type') || ' ' || (payload->'data'->>'hosting') "+
		"|| ' ' || (payload->'data'->>'error_code') FROM notification_events WHERE id = $1", failed),
		"repo_sync.failed repo_sync: repo_sync.failed github repo_sync.interrupted")
	eq(t, scalar[string](t, tdb, "SELECT type FROM notification_events WHERE id = $1", recovered), "repo_sync.recovered")
	eq(t, scalar[string](t, tdb, "SELECT type FROM notification_episodes WHERE event_id = $1", failed), "repo_sync.failed")
	_, err = rollbackThrough(t, m, tdb, "alter_forge_connections_rename")
	must(t, err)
	eq(t, scalar[string](t, tdb, "SELECT forge FROM nodes WHERE id = $1", project), "gitea")
	eq(t, scalar[string](t, tdb, "SELECT error_code || ' ' || (problems->0->>'code') FROM forge_sync_runs WHERE id = $1", run),
		"forge.unauthorized forge.value_too_long")
	eq(t, scalar[string](t, tdb, "SELECT array_to_string(sources, ',') FROM branches WHERE project_id = $1", project), "forge,ingest")
	eq(t, scalar[string](t, tdb, "SELECT array_to_string(event_types, ',') FROM notification_subscriptions WHERE id = $1", sub),
		"forge.sync_failed,forge.sync_recovered")
	eq(t, scalar[string](t, tdb, "SELECT (payload->'data'->>'forge') || ' ' || (payload->'data'->>'error_code') FROM notification_events WHERE id = $1", failed),
		"github forge.interrupted")
}
