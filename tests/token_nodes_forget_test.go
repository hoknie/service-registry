package tests

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"svc-registry/internal/testsupport"
)

func TestDeletedNodesLeaveTheTokenNodes(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	tr := newTree(app)
	bob := bobSession(app)
	app.grant(tr.admin, tr.backend, "user", "bob@example.com", "editor")
	secret := app.secretOn(bob, []string{tr.api, tr.secret}, "read")
	web := app.nodeID(tr.admin, "project", tr.backend, "web")

	eq(t, app.call("DELETE", nodePath(tr.api), tr.admin).status, 204)
	eq(t, strings.Join(tokenNodeIDs(t, app, bob), ","), tr.secret)
	eq(t, app.bearer("GET", nodePath(tr.secret), secret, nil).status, 200)
	eq(t, app.bearer("GET", nodePath(web), secret, nil).status, 404)

	eq(t, app.call("DELETE", nodePath(tr.secret), tr.admin).status, 204)
	eq(t, strings.Join(tokenNodeIDs(t, app, bob), ","), tr.secret, "the last node stays")
	eq(t, scalar[bool](t, app.db, "SELECT node_ids IS NOT NULL AND cardinality(node_ids) = 1 FROM personal_access_tokens"), true)
	contains(t, app.bearer("GET", "/api/v1/catalog/nodes", secret, nil).text(), `"items":[]`)
	eq(t, app.bearer("GET", nodePath(web), secret, nil).status, 404, "the token does not widen")
	eq(t, app.bearer("GET", nodePath(tr.backend), secret, nil).status, 404)
}

func TestDeletedNodesLeaveTheGrantNodes(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	tr := newTree(app)
	bob := app.user("bob@example.com", false)
	client, grant := newID(), newID()
	execSQL(t, app.db, "INSERT INTO oauth_clients (id, name, redirect_uris) VALUES ($1, 'Agent', '{http://127.0.0.1/cb}')", client)
	execSQL(t, app.db, "INSERT INTO oauth_grants (id, user_id, client_id, scopes, node_ids, refresh_hash, refresh_expires_at) "+
		"VALUES ($1, $2, $3, '{mcp}', $4, $5, now() + interval '1 day')", grant, bob.ID, client, []string{tr.api, tr.secret}, []byte(strings.Repeat("g", 32)))
	nodes := func() string {
		return scalar[string](t, app.db, "SELECT array_to_string(node_ids, ',') FROM oauth_grants WHERE id = $1", grant)
	}

	eq(t, app.call("DELETE", nodePath(tr.secret), tr.admin).status, 204)
	eq(t, nodes(), tr.api)
	eq(t, app.call("DELETE", nodePath(tr.api), tr.admin).status, 204)
	eq(t, nodes(), tr.api, "the grant never becomes unrestricted")
}

func TestCatalogRootTokenCreatesAfterItsLastOrganizationIsGone(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	tr := newTree(app)
	solo := app.nodeID(tr.admin, "organization", "", "solo")
	secret := app.secretOn(tr.admin, []string{solo}, "write", "catalog.root")

	eq(t, app.bearer("DELETE", nodePath(solo), secret, nil).status, 204)
	eq(t, strings.Join(tokenNodeIDs(t, app, tr.admin), ","), solo)
	r := app.bearer("POST", "/api/v1/catalog/nodes", secret, obj{"kind": "organization", "parent_id": nil, "slug": "next", "name": "Next"})
	eq(t, r.status, 201, r.text())
	eq(t, strings.Join(tokenNodeIDs(t, app, tr.admin), ","), idOf(r.json(t)))
}

func TestTokenNodesForgetTriggerRollsBack(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tdb := testsupport.NewTestDB(t)
	m := embedded(t)
	_, err := m.Migrate(ctx, tdb.Pool)
	must(t, err)
	trigger := "SELECT count(*) FROM pg_trigger WHERE tgrelid = 'nodes'::regclass AND tgname = 'nodes_forget_in_token_scopes'"
	eq(t, scalar[int64](t, tdb, trigger), int64(1))
	n, err := rollbackThrough(t, m, tdb, squashMigration)
	must(t, err)
	eq(t, n, migrationsFrom(t, m, squashMigration))
	eq(t, scalar[int64](t, tdb, trigger), int64(0))
	eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM pg_proc WHERE pronamespace = current_schema()::regnamespace AND proname = 'live_node_ids'"), int64(0))
}

func TestTokenNodeIDsIndexesServeTheTriggerAndRollBack(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tdb := testsupport.NewTestDB(t)
	m := embedded(t)
	_, err := m.Migrate(ctx, tdb.Pool)
	must(t, err)
	gone := []uuid.UUID{uuid.Must(uuid.NewV7())}
	for _, table := range []string{"personal_access_tokens", "oauth_grants", "oauth_authorizations"} {
		tx, err := tdb.Pool.Begin(ctx)
		must(t, err)
		_, err = tx.Exec(ctx, "SET LOCAL enable_seqscan = off")
		must(t, err)
		rows, err := tx.Query(ctx, "EXPLAIN SELECT 1 FROM "+table+" WHERE node_ids && $1", gone)
		must(t, err)
		var plan []string
		for rows.Next() {
			var line string
			must(t, rows.Scan(&line))
			plan = append(plan, line)
		}
		must(t, rows.Err())
		must(t, tx.Rollback(ctx))
		contains(t, strings.Join(plan, "\n"), table+"_node_ids_idx")
	}

	n, err := rollbackThrough(t, m, tdb, squashMigration)
	must(t, err)
	eq(t, n, migrationsFrom(t, m, squashMigration))
	eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM pg_indexes WHERE schemaname = current_schema() AND indexname LIKE '%_node_ids_idx'"), int64(0))
}
