package tests

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"svc-registry/internal/testsupport"
)

var freeTextColumns = []string{
	"clusters.ca_pem",
	"repo_sync_connections.webhook_secret_enc",
	"repo_sync_repositories.description",
	"repo_sync_repositories.readme_md",
	"repo_sync_runs.error_message",
	"knowledge_blobs.content",
	"knowledge_files.path",
	"knowledge_index_state.engine",
	"knowledge_source_repos.description",
	"knowledge_source_repos.readme_md",
	"notification_deliveries.last_error_message",
	"notification_subscriptions.signing_secret_enc",
	"secrets.value_enc",
}

func uncheckedTextColumns(t *testing.T, tdb *testsupport.TestDB) string {
	t.Helper()
	rows, err := tdb.Pool.Query(context.Background(), `
		SELECT c.table_name || '.' || c.column_name
		FROM information_schema.columns c
		WHERE c.table_schema = current_schema()
			AND c.data_type = 'text'
			AND NOT EXISTS (
				SELECT 1
				FROM pg_constraint k
				JOIN pg_attribute a ON a.attrelid = k.conrelid AND a.attnum = ANY(k.conkey)
				WHERE k.contype = 'c'
					AND k.conrelid = (quote_ident(c.table_schema) || '.' || quote_ident(c.table_name))::regclass
					AND a.attname = c.column_name
					AND (cardinality(k.conkey) = 1 OR k.conname = c.table_name || '_' || c.column_name || '_check')
			)
		ORDER BY 1`)
	must(t, err)
	var unchecked []string
	for rows.Next() {
		var name string
		must(t, rows.Scan(&name))
		unchecked = append(unchecked, name)
	}
	must(t, rows.Err())
	return strings.Join(unchecked, ",")
}

func TestEveryTextColumnHasACheckOrIsFreeText(t *testing.T) {
	t.Parallel()
	tdb := migrated(t)
	want := slices.Clone(freeTextColumns)
	slices.Sort(want)
	eq(t, uncheckedTextColumns(t, tdb), strings.Join(want, ","))
	execSQL(t, tdb, "ALTER TABLE groups ADD COLUMN note text")
	contains(t, uncheckedTextColumns(t, tdb), "groups.note")
}

func TestTextChecksRefuseOverlongValues(t *testing.T) {
	t.Parallel()
	tdb := migrated(t)
	org, project := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, slug, name) VALUES ($1, 'organization', 'o', 'O')", org)
	execSQL(t, tdb, "INSERT INTO nodes (id, kind, parent_id, slug, name) VALUES ($1, 'project', $2, 'p', 'P')", project, org)
	snapshot := uuid.Must(uuid.NewV7())
	execSQL(t, tdb, "INSERT INTO knowledge_snapshots (id, project_id, branch, commit_sha, status) VALUES ($1, $2, 'main', 'abc', 'ok')", snapshot, project)
	for _, c := range []struct {
		query string
		args  []any
		check string
	}{
		{"INSERT INTO nodes (id, kind, parent_id, slug, name) VALUES ($1, 'project', $2, $3, 'X')",
			[]any{uuid.Must(uuid.NewV7()), org, strings.Repeat("s", 64)}, "nodes_slug_check"},
		{"INSERT INTO groups (id, name) VALUES ($1, '')", []any{uuid.Must(uuid.NewV7())}, "groups_name_check"},
		{"INSERT INTO knowledge_files (id, snapshot_id, path, git_blob_sha, bytes, skip_reason, kind) VALUES ($1, $2, 'a.md', 'XYZ', 1, 'binary', 'doc')",
			[]any{uuid.Must(uuid.NewV7()), snapshot}, "knowledge_files_git_blob_sha_check"},
		{"INSERT INTO users (id, email, display_name, password_hash) VALUES ($1, $2, 'A', 'h')",
			[]any{uuid.Must(uuid.NewV7()), strings.Repeat("e", 243) + "@example.com"}, "users_email_check"},
	} {
		contains(t, execErr(t, tdb, c.query, c.args...), c.check)
	}
	blob := strings.Repeat("x", 1<<20)
	execSQL(t, tdb, "INSERT INTO knowledge_blobs (sha256, content, bytes) VALUES ($1, $2, $3)",
		[]byte(strings.Repeat("b", 32)), blob, len(blob))
}
