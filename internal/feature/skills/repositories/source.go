package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	skillstypes "svc-registry/internal/feature/skills/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type SourceRepository struct{ db *pgtx.DB }

func NewSourceRepository(db *pgtx.DB) *SourceRepository { return &SourceRepository{db: db} }

type sourceRow struct {
	ID              uuid.UUID         `db:"id"`
	FolderID        uuid.UUID         `db:"folder_id"`
	OrganizationID  *uuid.UUID        `db:"organization_id"`
	Kind            string            `db:"kind"`
	Hosting         *string           `db:"hosting"`
	URL             *string           `db:"url"`
	APIURL          *string           `db:"api_url"`
	Path            *string           `db:"path"`
	WorkingTree     bool              `db:"working_tree"`
	SecretID        *uuid.UUID        `db:"credentials_secret_id"`
	SecretName      *string           `db:"secret_name"`
	Branch          string            `db:"branch"`
	Roots           []string          `db:"roots"`
	Layout          string            `db:"layout"`
	GroupField      *string           `db:"group_field"`
	IntervalSecs    *int              `db:"interval_secs"`
	NewStatus       string            `db:"new_status"`
	Versions        string            `db:"versions"`
	TagPattern      string            `db:"tag_pattern"`
	NextRunAt       *string           `db:"next_run_at"`
	Due             bool              `db:"due"`
	Running         bool              `db:"running"`
	LastCommit      *string           `db:"last_commit"`
	LastTags        map[string]string `db:"last_tags"`
	LastRun         *skillstypes.Run  `db:"last_run"`
	ConfigChangedAt string            `db:"config_changed_at"`
	CreatedAt       string            `db:"created_at"`
	UpdatedAt       string            `db:"updated_at"`
	PausedReason    *string           `db:"paused_reason"`
	Requested       bool              `db:"requested"`
}

func (r sourceRow) source() skillstypes.Source {
	return skillstypes.Source{ID: r.ID, FolderID: r.FolderID, OrganizationID: r.OrganizationID, Kind: r.Kind, Hosting: r.Hosting,
		URL: r.URL, APIURL: r.APIURL, Path: r.Path, WorkingTree: r.WorkingTree, SecretID: r.SecretID, SecretName: r.SecretName,
		Branch: r.Branch, Roots: r.Roots, Layout: r.Layout, GroupField: r.GroupField, IntervalSecs: r.IntervalSecs, NewStatus: skillstypes.Status(r.NewStatus),
		Versions: r.Versions, TagPattern: r.TagPattern, NextRunAt: r.NextRunAt, Due: r.Due, Running: r.Running,
		LastCommit: r.LastCommit, LastTags: r.LastTags, LastRun: r.LastRun, ConfigChangedAt: r.ConfigChangedAt,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, PausedReason: r.PausedReason, Requested: r.Requested}
}

func querySources(ctx context.Context, q pgtx.Querier, folder, id *uuid.UUID, secrets []uuid.UUID) ([]skillstypes.Source, error) {
	return postgres.List(ctx, q, constraints, postgres.Struct(sourceRow.source), `
		SELECT s.id, s.folder_id, f.organization_id, s.kind, s.hosting, s.url, s.api_url, s.path, s.working_tree,
			s.credentials_secret_id, sec.name AS secret_name, s.branch, s.roots, s.layout, s.group_field, s.interval_secs, s.new_status, s.versions,
			s.tag_pattern, rfc3339(s.next_run_at) AS next_run_at, COALESCE(s.next_run_at <= now(), false) AS due,
			COALESCE(s.lease_until > now(), false) AS running, s.last_commit, s.last_tags, s.last_run,
			s.config_changed_at::text AS config_changed_at, rfc3339(s.created_at) AS created_at,
			rfc3339(s.updated_at) AS updated_at, s.requested_at IS NOT NULL AS requested,
			CASE
				WHEN skill_folder_disabled(s.folder_id) IS NOT NULL THEN 'folder_disabled'
				WHEN EXISTS (
					SELECT 1
					FROM skills k
					WHERE k.source_id = s.id
				) AND NOT EXISTS (
					SELECT 1
					FROM skills k
					WHERE k.source_id = s.id
						AND k.status <> 'disabled'
				) THEN 'skills_disabled'
			END AS paused_reason
		FROM skill_sources s
		JOIN skill_folders f ON f.id = s.folder_id
		LEFT JOIN secrets sec ON sec.id = s.credentials_secret_id
		WHERE ($1::uuid IS NULL OR s.folder_id = $1)
			AND ($2::uuid IS NULL OR s.id = $2)
			AND ($3::uuid[] IS NULL OR s.credentials_secret_id = ANY($3))
		ORDER BY s.created_at, s.id`, folder, id, secrets)
}

func (s *SourceRepository) List(ctx context.Context, folder uuid.UUID) ([]skillstypes.Source, error) {
	return querySources(ctx, s.db.From(ctx), &folder, nil, nil)
}

func (s *SourceRepository) Get(ctx context.Context, id uuid.UUID) (*skillstypes.Source, error) {
	found, err := querySources(ctx, s.db.From(ctx), nil, &id, nil)
	if err != nil || len(found) == 0 {
		return nil, err
	}
	return &found[0], nil
}

func (s *SourceRepository) UsingSecrets(ctx context.Context, secrets []uuid.UUID) (map[uuid.UUID]int64, error) {
	return postgres.Pairs[uuid.UUID, int64](ctx, s.db.From(ctx), constraints, `
		SELECT credentials_secret_id, count(*)
		FROM skill_sources
		WHERE credentials_secret_id = ANY($1)
		GROUP BY credentials_secret_id`, secrets)
}

func (s *SourceRepository) Save(ctx context.Context, src skillstypes.Source) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO skill_sources (id, folder_id, kind, hosting, url, api_url, path, working_tree, credentials_secret_id, branch,
			roots, interval_secs, new_status, versions, tag_pattern, layout, group_field, next_run_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, now())
		ON CONFLICT (id)
		DO UPDATE SET kind = $3, hosting = $4, url = $5, api_url = $6, path = $7, working_tree = $8,
			credentials_secret_id = $9, branch = $10, roots = $11, interval_secs = $12, new_status = $13, versions = $14,
			tag_pattern = $15, layout = $16, group_field = $17, next_run_at = now(), config_changed_at = now(), last_commit = NULL, last_tags = '{}',
			updated_at = now()`,
		src.ID, src.FolderID, src.Kind, src.Hosting, src.URL, src.APIURL, src.Path, src.WorkingTree, src.SecretID, src.Branch,
		src.Roots, src.IntervalSecs, string(src.NewStatus), src.Versions, src.TagPattern, src.Layout, src.GroupField)
	return err
}

func (s *SourceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		DELETE FROM skill_sources
		WHERE id = $1`, id)
}

func (s *SourceRepository) Queue(ctx context.Context, id uuid.UUID) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		UPDATE skill_sources
		SET next_run_at = LEAST(COALESCE(next_run_at, now()), now()), requested_at = now()
		WHERE id = $1`, id)
}

func (s *SourceRepository) Wake(ctx context.Context, id uuid.UUID) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE skill_sources
		SET next_run_at = now()
		WHERE id = $1
			AND next_run_at > now()`, id)
	return err
}

func (s *SourceRepository) WakeFolder(ctx context.Context, folder uuid.UUID) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		WITH RECURSIVE down AS (
			SELECT id
			FROM skill_folders
			WHERE id = $1
			UNION ALL
			SELECT f.id
			FROM skill_folders f
			JOIN down ON f.parent_id = down.id
		)
		UPDATE skill_sources
		SET next_run_at = now()
		WHERE folder_id IN (SELECT id FROM down)
			AND next_run_at > now()`, folder)
	return err
}

func (s *SourceRepository) WakeOrganization(ctx context.Context, organization uuid.UUID) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE skill_sources
		SET next_run_at = now()
		WHERE folder_id IN (
				SELECT id
				FROM skill_folders
				WHERE organization_id = $1
			)
			AND next_run_at > now()`, organization)
	return err
}

func (s *SourceRepository) Postpone(ctx context.Context, id uuid.UUID, intervalSecs int) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE skill_sources
		SET lease_until = NULL, requested_at = NULL, next_run_at = now() + make_interval(secs => $2)
		WHERE id = $1`, id, float64(intervalSecs))
	return err
}

func (s *SourceRepository) Claim(ctx context.Context, limit, leaseSecs int) ([]uuid.UUID, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, pgx.RowTo[uuid.UUID], `
		UPDATE skill_sources
		SET lease_until = now() + make_interval(secs => $2)
		WHERE id IN (
			SELECT id
			FROM skill_sources
			WHERE next_run_at <= now()
				AND (lease_until IS NULL OR lease_until < now())
			ORDER BY next_run_at
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		)
		RETURNING id`, limit, float64(leaseSecs))
}

func (s *SourceRepository) Extend(ctx context.Context, id uuid.UUID, leaseSecs int) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE skill_sources
		SET lease_until = now() + make_interval(secs => $2)
		WHERE id = $1`, id, float64(leaseSecs))
	return err
}

func (s *SourceRepository) Finish(ctx context.Context, id uuid.UUID, run skillstypes.Run, commit *string, tags map[string]string,
	intervalSecs int, configAt string) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE skill_sources
		SET last_run = $2, last_commit = COALESCE($3, last_commit), last_tags = COALESCE($4, last_tags), lease_until = NULL,
			requested_at = NULL,
			next_run_at = CASE WHEN config_changed_at::text = $6 THEN now() + make_interval(secs => $5) ELSE now() END
		WHERE id = $1`, id, run, commit, tags, float64(intervalSecs), configAt)
	return err
}
