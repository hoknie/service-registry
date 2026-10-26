package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type TagRepository struct{ db *pgtx.DB }

func NewTagRepository(db *pgtx.DB) *TagRepository { return &TagRepository{db: db} }

func (s *TagRepository) Known(ctx context.Context, projectID uuid.UUID) (map[string]time.Time, error) {
	return postgres.Pairs[string, time.Time](ctx, s.db.From(ctx), constraints, `
		SELECT name, first_seen_at
		FROM knowledge_tags
		WHERE project_id = $1`, projectID)
}

func (s *TagRepository) Replace(ctx context.Context, projectID uuid.UUID, tags []knowledgetypes.Tag) ([]string, error) {
	names := make([]string, len(tags))
	commits := make([]string, len(tags))
	seen := make([]time.Time, len(tags))
	for i, t := range tags {
		names[i], commits[i], seen[i] = t.Name, t.Commit, t.FirstSeen
	}
	if _, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO knowledge_tags (project_id, name, commit_sha, first_seen_at)
		SELECT $1, t.name, t.commit_sha, t.first_seen_at
		FROM unnest($2::text[], $3::text[], $4::timestamptz[]) AS t(name, commit_sha, first_seen_at)
		ON CONFLICT (project_id, name)
		DO UPDATE SET commit_sha = EXCLUDED.commit_sha`, projectID, names, commits, seen); err != nil {
		return nil, err
	}
	return postgres.List(ctx, s.db.From(ctx), constraints, pgx.RowTo[string], `
		DELETE FROM knowledge_tags
		WHERE project_id = $1
			AND NOT (name = ANY($2))
		RETURNING name`, projectID, names)
}

func (s *TagRepository) SetRelease(ctx context.Context, projectID uuid.UUID, name string) error {
	if _, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE knowledge_tags
		SET release = false
		WHERE project_id = $1
			AND release
			AND name <> $2`, projectID, name); err != nil {
		return err
	}
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE knowledge_tags
		SET release = true
		WHERE project_id = $1
			AND name = $2
			AND NOT release`, projectID, name)
	return err
}

func (s *TagRepository) Release(ctx context.Context, projectID uuid.UUID) (*string, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowTo[string], `
		SELECT name
		FROM knowledge_tags
		WHERE project_id = $1
			AND release`, projectID)
}

func (s *TagRepository) Has(ctx context.Context, projectID uuid.UUID, name string) (bool, error) {
	found, err := postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowTo[bool], `
		SELECT true
		FROM knowledge_tags
		WHERE project_id = $1
			AND name = $2`, projectID, name)
	return found != nil, err
}

func (s *TagRepository) List(ctx context.Context, projectID uuid.UUID) ([]knowledgetypes.StoredTag, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (knowledgetypes.StoredTag, error) {
		var t knowledgetypes.StoredTag
		err := r.Scan(&t.Name, &t.Commit, &t.FirstSeen, &t.Release)
		return t, err
	}, `
		SELECT name, commit_sha, first_seen_at, release
		FROM knowledge_tags
		WHERE project_id = $1`, projectID)
}

func (s *TagRepository) Releases(ctx context.Context, projects []uuid.UUID) (map[uuid.UUID]string, error) {
	return postgres.Pairs[uuid.UUID, string](ctx, s.db.From(ctx), constraints, `
		SELECT project_id, 'tag:' || name
		FROM knowledge_tags
		WHERE project_id = ANY($1)
			AND release`, projects)
}
