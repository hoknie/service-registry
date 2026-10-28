package tests

import (
	"testing"

	"github.com/google/uuid"
)

func TestSkillFolderFunctionsAndSetTriggers(t *testing.T) {
	t.Parallel()
	a := startApp(t)
	id := func() string { return uuid.NewString() }
	top, mid, leaf := id(), id(), id()
	execSQL(t, a.db, "INSERT INTO skill_folders (id, slug, name) VALUES ($1, 'top', 'Top')", top)
	execSQL(t, a.db, "INSERT INTO skill_folders (id, parent_id, slug, name, disabled) VALUES ($1, $2, 'mid', 'Mid', true)", mid, top)
	execSQL(t, a.db, "INSERT INTO skill_folders (id, parent_id, slug, name, disabled) VALUES ($1, $2, 'leaf', 'Leaf', true)", leaf, mid)
	eq(t, scalar[string](t, a.db, "SELECT skill_folder_disabled($1)::text", leaf), leaf)
	eq(t, scalar[string](t, a.db, "SELECT COALESCE(skill_folder_disabled($1)::text, '')", top), "")
	execSQL(t, a.db, "UPDATE skill_folders SET disabled = false WHERE id = $1", leaf)
	eq(t, scalar[string](t, a.db, "SELECT skill_folder_disabled($1)::text", leaf), mid)
	eq(t, scalar[int64](t, a.db, "SELECT cardinality(skill_folder_ancestors($1))", leaf), int64(3))

	acme := a.nodeID(a.admin(), "organization", "", "acme")
	execSQL(t, a.db, "INSERT INTO skill_organization_sets (organization_id, mode, folder_ids) VALUES ($1, 'selected', ARRAY[$2::uuid, $3::uuid])", acme, mid, top)
	execSQL(t, a.db, "DELETE FROM skill_folders WHERE id = $1", leaf)
	execSQL(t, a.db, "DELETE FROM skill_folders WHERE id = $1", mid)
	eq(t, scalar[string](t, a.db, "SELECT array_to_string(folder_ids, ',') FROM skill_organization_sets"), top)
	contains(t, execErr(t, a.db, "INSERT INTO skill_sources (id, folder_id, kind, path, layout) VALUES ($1, $2, 'local_dir', '/x', 'field')", id(), top),
		"skill_sources_group_field_check")
}
