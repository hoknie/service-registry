package tests

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"svc-registry/internal/testsupport"
)

type bindingWorld struct {
	tdb                                  *testsupport.TestDB
	bob                                  uuid.UUID
	acme, backend, frontend, api, globex uuid.UUID
	names                                map[uuid.UUID]string
}

func newBindingWorld(t *testing.T) *bindingWorld {
	t.Helper()
	tdb := testsupport.NewTestDB(t)
	_, err := embedded(t).Migrate(context.Background(), tdb.Pool)
	must(t, err)
	w := &bindingWorld{tdb: tdb, bob: uuid.New(), names: map[uuid.UUID]string{}}
	execSQL(t, tdb, "INSERT INTO users (id, email, display_name, password_hash) VALUES ($1, 'bob@example.com', 'Bob', 'x')", w.bob)
	node := func(kind string, parent *uuid.UUID, slug string) uuid.UUID {
		id := uuid.New()
		execSQL(t, tdb, "INSERT INTO nodes (id, kind, parent_id, slug, name) VALUES ($1, $2, $3, $4, $4)", id, kind, parent, slug)
		w.names[id] = slug
		return id
	}
	w.acme = node("organization", nil, "acme")
	w.globex = node("organization", nil, "globex")
	w.backend = node("folder", &w.acme, "backend")
	w.frontend = node("folder", &w.acme, "frontend")
	w.api = node("project", &w.backend, "api")
	return w
}

func (w *bindingWorld) bind(t *testing.T, node uuid.UUID, role string) {
	t.Helper()
	execSQL(t, w.tdb, "INSERT INTO role_bindings (id, node_id, user_id, role) VALUES ($1, $2, $3, $4)", uuid.New(), node, w.bob, role)
}

func (w *bindingWorld) bindings(t *testing.T, roots []uuid.UUID) string {
	t.Helper()
	return w.bindingsAs(t, roots, false)
}

func (w *bindingWorld) bindingsAs(t *testing.T, roots []uuid.UUID, admin bool) string {
	t.Helper()
	rows, err := w.tdb.Pool.Query(context.Background(), "SELECT node_id, role FROM viewer_bindings($1, $2::uuid[], $3)", w.bob, roots, admin)
	must(t, err)
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id uuid.UUID
		var role string
		must(t, rows.Scan(&id, &role))
		out = append(out, w.names[id]+":"+role)
	}
	must(t, rows.Err())
	slices.Sort(out)
	return strings.Join(out, ",")
}

func TestViewerBindingsNarrowToTheRoots(t *testing.T) {
	t.Parallel()
	w := newBindingWorld(t)
	w.bind(t, w.acme, "editor")
	w.bind(t, w.api, "admin")
	w.bind(t, w.globex, "viewer")
	group := uuid.New()
	execSQL(t, w.tdb, "INSERT INTO groups (id, name) VALUES ($1, 'platform')", group)
	execSQL(t, w.tdb, "INSERT INTO group_members (id, group_id, user_id) VALUES ($1, $2, $3)", uuid.New(), group, w.bob)
	execSQL(t, w.tdb, "INSERT INTO role_bindings (id, node_id, group_id, role) VALUES ($1, $2, $3, 'viewer')", uuid.New(), w.frontend, group)

	tests := []struct {
		name  string
		roots []uuid.UUID
		want  string
	}{
		{"no roots keeps every binding", nil, "acme:editor,api:admin,frontend:viewer,globex:viewer"},
		{"ancestor role lands on the root", []uuid.UUID{w.backend}, "api:admin,backend:editor"},
		{"root itself", []uuid.UUID{w.globex}, "globex:viewer"},
		{"group binding on the root", []uuid.UUID{w.frontend}, "frontend:editor,frontend:viewer"},
		{"sibling branches stay out", []uuid.UUID{w.api}, "api:admin,api:editor"},
		{"missing root", []uuid.UUID{uuid.New()}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eq(t, w.bindings(t, tt.roots), tt.want)
		})
	}
}

func TestViewerBindingsCatalogAdmin(t *testing.T) {
	t.Parallel()
	w := newBindingWorld(t)
	w.bind(t, w.api, "viewer")
	tests := []struct {
		name  string
		roots []uuid.UUID
		admin bool
		want  string
	}{
		{"whole catalog lands on the top level", nil, true, "acme:admin,api:viewer,globex:admin"},
		{"roots get the role", []uuid.UUID{w.backend}, true, "api:viewer,backend:admin"},
		{"without the flag only own bindings", []uuid.UUID{w.backend}, false, "api:viewer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eq(t, w.bindingsAs(t, tt.roots, tt.admin), tt.want)
		})
	}
}

func TestViewerBindingsOpenOrganizations(t *testing.T) {
	t.Parallel()
	w := newBindingWorld(t)
	execSQL(t, w.tdb, "UPDATE nodes SET visibility = 'open' WHERE id = $1", w.acme)
	eq(t, w.bindings(t, nil), "acme:viewer")
	eq(t, w.bindings(t, []uuid.UUID{w.api}), "api:viewer")
	eq(t, w.bindings(t, []uuid.UUID{w.globex}), "")
	w.bind(t, w.backend, "editor")
	eq(t, w.bindings(t, []uuid.UUID{w.api}), "api:editor,api:viewer")
	_, err := w.tdb.Pool.Exec(context.Background(), "UPDATE nodes SET visibility = 'open' WHERE id = $1", w.backend)
	eq(t, err != nil, true)
}

func TestViewerBindingsSpecScenario(t *testing.T) {
	t.Parallel()
	w := newBindingWorld(t)
	w.bind(t, w.acme, "editor")
	eq(t, w.bindings(t, []uuid.UUID{w.backend}), "backend:editor")
	msg := execErr(t, w.tdb, "INSERT INTO personal_access_tokens (id, user_id, name, prefix, token_hash, scopes, node_ids) "+
		"VALUES ($1, $2, 't', 'svcp_0000000', decode(repeat('00', 32), 'hex'), '{read}', '{}')", uuid.New(), w.bob)
	contains(t, msg, "personal_access_tokens_node_ids_check")
}

func TestTokenNodesRollbackRefusesActiveScopedTokens(t *testing.T) {
	t.Parallel()
	w := newBindingWorld(t)
	m := embedded(t)
	id := uuid.New()
	execSQL(t, w.tdb, "INSERT INTO personal_access_tokens (id, user_id, name, prefix, token_hash, scopes, node_ids) "+
		"VALUES ($1, $2, 't', 'svcp_0000000', decode(repeat('00', 32), 'hex'), '{read}', ARRAY[$3::uuid])", id, w.bob, w.api)
	_, err := rollbackThrough(t, m, w.tdb, squashMigration)
	if err == nil {
		t.Fatal("rollback with an active scoped token succeeded")
	}
	contains(t, err.Error(), "revoke them")
	columns := "SELECT count(*) FROM information_schema.columns WHERE table_schema = current_schema() " +
		"AND table_name = 'personal_access_tokens' AND column_name = 'node_ids'"
	functions := "SELECT count(*) FROM pg_proc WHERE proname = 'viewer_bindings' AND pronamespace = current_schema()::regnamespace"
	eq(t, scalar[int64](t, w.tdb, columns), int64(1))

	w2 := newBindingWorld(t)
	execSQL(t, w2.tdb, "INSERT INTO personal_access_tokens (id, user_id, name, prefix, token_hash, scopes, node_ids, revoked_at) "+
		"VALUES ($1, $2, 't', 'svcp_0000000', decode(repeat('00', 32), 'hex'), '{read}', ARRAY[$3::uuid], now())", id, w2.bob, w2.api)
	n, err := rollbackThrough(t, m, w2.tdb, squashMigration)
	must(t, err)
	eq(t, n, migrationsFrom(t, m, squashMigration))
	eq(t, scalar[int64](t, w2.tdb, columns), int64(0))
	eq(t, scalar[int64](t, w2.tdb, functions), int64(0))
	eq(t, scalar[int64](t, w2.tdb, "SELECT count(*) FROM personal_access_tokens"), int64(1))
}

func TestRepositoriesReadBindingsOnlyThroughFunction(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("../internal/feature/*/repositories/*.go")
	must(t, err)
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") || strings.HasSuffix(f, filepath.Join("catalog", "repositories", "binding.go")) {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), f, nil, 0)
		must(t, err)
		ast.Inspect(parsed, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			for _, part := range stringParts(lit) {
				fields := strings.Join(strings.Fields(part), " ")
				if strings.Contains(fields, "FROM role_bindings") || strings.Contains(fields, "JOIN role_bindings") {
					t.Errorf("%s reads role_bindings directly; use viewer_bindings", f)
				}
			}
			return true
		})
	}
}

func TestViewerBindingsGiveOwnersEditor(t *testing.T) {
	t.Parallel()
	w := newBindingWorld(t)
	w.bind(t, w.api, "admin")
	execSQL(t, w.tdb, "INSERT INTO node_owners (id, node_id, user_id, position) VALUES ($1, $2, $3, 0)", uuid.New(), w.acme, w.bob)
	group := uuid.New()
	execSQL(t, w.tdb, "INSERT INTO groups (id, name) VALUES ($1, 'platform')", group)
	execSQL(t, w.tdb, "INSERT INTO group_members (id, group_id, user_id) VALUES ($1, $2, $3)", uuid.New(), group, w.bob)
	execSQL(t, w.tdb, "INSERT INTO node_owners (id, node_id, group_id, position) VALUES ($1, $2, $3, 0)", uuid.New(), w.globex, group)
	tests := []struct {
		name  string
		roots []uuid.UUID
		want  string
	}{
		{"own and group sets", nil, "acme:editor,api:admin,globex:editor"},
		{"owner role lands on the root", []uuid.UUID{w.backend}, "api:admin,backend:editor"},
		{"other roots stay out", []uuid.UUID{w.globex}, "globex:editor"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eq(t, w.bindings(t, tt.roots), tt.want)
		})
	}
}
