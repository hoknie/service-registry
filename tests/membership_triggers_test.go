package tests

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"svc-registry/internal/testsupport"
)

func TestOrganizationMembersFollowRolesOwnersAndGroups(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tdb := testsupport.NewTestDB(t)
	m := embedded(t)
	_, err := m.Migrate(ctx, tdb.Pool)
	must(t, err)
	_, err = rollbackThrough(t, m, tdb, "create_organization_members")
	must(t, err)

	id := func() uuid.UUID { return uuid.Must(uuid.NewV7()) }
	acme, globex, backend, api := id(), id(), id(), id()
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, slug, name) VALUES ($1, 'organization', 'acme', 'Acme')", acme)
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, slug, name) VALUES ($1, 'organization', 'globex', 'Globex')", globex)
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, parent_id, slug, name) VALUES ($1, 'folder', $2, 'backend', 'Backend')", backend, acme)
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, parent_id, slug, name) VALUES ($1, 'project', $2, 'api', 'API')", api, backend)
	user := func(email string) uuid.UUID {
		u := id()
		execSQL(t, tdb, "INSERT INTO users (id, email, display_name, password_hash) VALUES ($1, $2, $2, 'x')", u, email)
		return u
	}
	bob, ann := user("bob@example.com"), user("ann@example.com")
	execSQL(t, tdb, "INSERT INTO role_bindings (id, node_id, user_id, role) VALUES ($1, $2, $3, 'viewer')", id(), backend, bob)

	n, err := m.Migrate(ctx, tdb.Pool)
	must(t, err)
	eq(t, n, migrationsFrom(t, m, "create_organization_members"))
	source := func(org, u uuid.UUID) string {
		return scalar[string](t, tdb, "SELECT COALESCE((SELECT source FROM organization_members WHERE organization_id = $1 AND user_id = $2), '')", org, u)
	}
	eq(t, source(acme, bob), "role", "backfilled from a role inside the organization")

	execSQL(t, tdb, "INSERT INTO node_owners (id, node_id, user_id, position) VALUES ($1, $2, $3, 0)", id(), api, ann)
	eq(t, source(acme, ann), "owner")
	execSQL(t, tdb, "INSERT INTO role_bindings (id, node_id, user_id, role) VALUES ($1, $2, $3, 'admin')", id(), acme, ann)
	eq(t, source(acme, ann), "owner", "the first way stays")

	carol := user("carol@example.com")
	group := id()
	execSQL(t, tdb, "INSERT INTO groups (id, name, organization_id) VALUES ($1, 'support', $2)", group, globex)
	execSQL(t, tdb, "INSERT INTO group_members (id, group_id, user_id) VALUES ($1, $2, $3)", id(), group, carol)
	eq(t, source(globex, carol), "group")

	contains(t, execErr(t, tdb, "INSERT INTO organization_members (organization_id, user_id, source) VALUES ($1, $2, 'manual')", backend, carol),
		"belong to top-level nodes only")

	execSQL(t, tdb, "UPDATE nodes SET parent_id = $1 WHERE id = $2", globex, backend)
	eq(t, source(globex, bob), "role", "a moved subtree brings its people")
	eq(t, source(globex, ann), "owner")

	eq(t, scalar[int64](t, tdb, "SELECT organization_admins($1)", acme), int64(1))
	eq(t, scalar[bool](t, tdb, "SELECT remove_organization_member($1, $2)", globex, ann), true)
	eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM node_owners WHERE user_id = $1", ann), int64(0))
	eq(t, source(acme, ann), "owner", "other organizations keep the member")
	eq(t, scalar[bool](t, tdb, "SELECT remove_organization_member($1, $2)", globex, carol), true)
	eq(t, scalar[int64](t, tdb, "SELECT count(*) FROM group_members WHERE user_id = $1", carol), int64(0))
	eq(t, scalar[bool](t, tdb, "SELECT remove_organization_member($1, $2)", globex, carol), false)
}
