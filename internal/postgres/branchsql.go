package postgres

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func mergeSources(source string) string {
	return "ARRAY(SELECT s FROM unnest(b.sources || ARRAY['" + source + "']::text[]) s GROUP BY s " +
		"ORDER BY array_position(ARRAY['forge', 'ingest', 'cluster', 'manual']::text[], s))"
}

var (
	branchUpsertIngest = "INSERT INTO branches AS b (id, project_id, name, head_sha, sources, last_activity_at) " +
		"VALUES ($1, $2, $3, $4, ARRAY['ingest'], $5::timestamptz) ON CONFLICT (project_id, name) DO UPDATE SET " +
		"sources = " + mergeSources("ingest") + ", " +
		"head_sha = CASE WHEN $4::text IS NOT NULL AND ($5::timestamptz > b.last_activity_at OR b.head_sha IS NULL) THEN $4 ELSE b.head_sha END, " +
		"last_activity_at = GREATEST(b.last_activity_at, $5::timestamptz), gone_at = NULL, updated_at = now()"
	branchUpsertForge = "INSERT INTO branches AS b (id, project_id, name, head_sha, protected, sources, last_activity_at) " +
		"VALUES ($1, $2, $3, $4, $5, ARRAY['forge'], COALESCE($6, now())) ON CONFLICT (project_id, name) DO UPDATE SET " +
		"sources = " + mergeSources("forge") + ", head_sha = COALESCE($4, b.head_sha), protected = COALESCE($5, b.protected), " +
		"last_activity_at = CASE WHEN $6::timestamptz IS NOT NULL THEN GREATEST(b.last_activity_at, $6) " +
		"WHEN $4::text IS DISTINCT FROM b.head_sha THEN now() ELSE b.last_activity_at END, " +
		"gone_at = NULL, updated_at = now()"
)

var branchUpsertCluster = "INSERT INTO branches AS b (id, project_id, name, head_sha, sources, last_activity_at) " +
	"VALUES ($1, $2, $3, $4, ARRAY['cluster'], COALESCE($5::timestamptz, now())) ON CONFLICT (project_id, name) DO UPDATE SET " +
	"sources = " + mergeSources("cluster") + ", " +
	"head_sha = CASE WHEN $5::timestamptz IS NOT NULL AND $4::text IS NOT NULL THEN $4 ELSE b.head_sha END, " +
	"last_activity_at = CASE WHEN $5::timestamptz IS NOT NULL THEN GREATEST(b.last_activity_at, $5) ELSE b.last_activity_at END, " +
	"gone_at = NULL, updated_at = now()"

var hexSHA = regexp.MustCompile(`^[0-9a-f]{7,64}$`)

func sha(s *string) *string {
	if s == nil {
		return nil
	}
	v := strings.ToLower(*s)
	if !hexSHA.MatchString(v) {
		return nil
	}
	return &v
}

func UpsertIngestBranch(ctx context.Context, db Execer, projectID uuid.UUID, name string, commit *string, at string) error {
	_, err := db.Exec(ctx, branchUpsertIngest, uuid.Must(uuid.NewV7()), projectID, name, sha(commit), at)
	return err
}

func UpsertForgeBranch(ctx context.Context, db Execer, projectID uuid.UUID, name, head string, protected *bool, committed *time.Time) error {
	_, err := db.Exec(ctx, branchUpsertForge, uuid.Must(uuid.NewV7()), projectID, name, sha(&head), protected, committed)
	return err
}

func UpsertClusterBranch(ctx context.Context, db Execer, projectID uuid.UUID, name string, commit *string, activity *time.Time) error {
	_, err := db.Exec(ctx, branchUpsertCluster, uuid.Must(uuid.NewV7()), projectID, name, sha(commit), activity)
	return err
}
