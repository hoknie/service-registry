package tests

import "testing"

func TestSnapshotLimitsAreExplainedToSuperadminsOnly(t *testing.T) {
	t.Parallel()
	d := startDocs(t, map[string]string{
		"a.md": "a", "b.md": "b", "api/openapi.yaml": "openapi: 3.0.0\n", "proto/Billing.PROTO": "syntax = \"proto3\";\n",
	}, "KNOWLEDGE_MAX_FILES", "2")
	execSQL(t, d.app.db, "INSERT INTO knowledge_patterns (node_id, include_set, include) VALUES ($1, true, $2)", d.id, []string{"*.md"})
	d.app.collect()
	eq(t, d.files("main"), "a.md:,api/openapi.yaml:,b.md:limit,proto/Billing.PROTO:limit")

	admin := d.app.get(knowledgePath(d.id), d.admin).json(t)
	snap := at(admin, "branches", 0, "snapshot")
	eq(t, at(snap, "skipped_limit"), any(float64(2)))
	eq(t, at(snap, "skipped_too_large"), any(float64(0)))
	eq(t, len(list(t, snap, "limits")), 1)
	eq(t, at(snap, "limits", 0, "name"), any("KNOWLEDGE_MAX_FILES"))
	eq(t, at(snap, "limits", 0, "value"), any(float64(2)))
	files := d.app.get(knowledgePath(d.id)+"/files", d.admin).json(t)
	eq(t, at(files, "snapshot", "limits", 0, "name"), any("KNOWLEDGE_MAX_FILES"))

	bob := d.viewer()
	viewer := d.app.get(knowledgePath(d.id), bob).json(t)
	eq(t, at(viewer, "branches", 0, "snapshot", "skipped_limit"), any(float64(2)))
	eq(t, at(viewer, "branches", 0, "snapshot", "limits"), nil)
	eq(t, at(d.app.get(knowledgePath(d.id)+"/files", bob).json(t), "snapshot", "limits"), nil)

	scans := d.app.get(knowledgePath(d.id)+"/scans", d.admin).json(t)
	branch := at(scans, "items", 0, "branches", 0)
	eq(t, at(branch, "skipped", "limit"), any(float64(2)))
	eq(t, at(branch, "api_skipped"), any(float64(1)))
}

func TestCompleteSnapshotReachesNoLimits(t *testing.T) {
	t.Parallel()
	d := startDocs(t, map[string]string{"README.md": "# Pets"})
	d.app.collect()
	snap := at(d.app.get(knowledgePath(d.id), d.admin).json(t), "branches", 0, "snapshot")
	eq(t, at(snap, "skipped_limit"), any(float64(0)))
	eq(t, len(list(t, snap, "limits")), 0)
}
