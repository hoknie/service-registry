package repositories

import (
	"context"

	"github.com/jackc/pgx/v5"

	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type BlobRepository struct{ db *pgtx.DB }

func NewBlobRepository(db *pgtx.DB) *BlobRepository { return &BlobRepository{db: db} }

func (s *BlobRepository) Touch(ctx context.Context, sums [][]byte, sizes []int64) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO skill_blobs (sha256, size)
		SELECT b.sha256, b.size
		FROM unnest($1::bytea[], $2::bigint[]) AS b(sha256, size)
		ON CONFLICT (sha256)
		DO UPDATE SET last_referenced_at = now()`, sums, sizes)
	return err
}

func (s *BlobRepository) Unreferenced(ctx context.Context, graceSecs int, limit int) ([][]byte, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, pgx.RowTo[[]byte], `
		SELECT b.sha256
		FROM skill_blobs b
		WHERE b.last_referenced_at < now() - make_interval(secs => $1)
			AND NOT EXISTS (SELECT 1 FROM skill_files f WHERE f.sha256 = b.sha256)
			AND NOT EXISTS (SELECT 1 FROM skills k WHERE k.image_sha256 = b.sha256)
		ORDER BY b.last_referenced_at
		LIMIT $2`, float64(graceSecs), limit)
}

func (s *BlobRepository) Forget(ctx context.Context, sum []byte, graceSecs int) (bool, error) {
	n, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM skill_blobs b
		WHERE b.sha256 = $1
			AND b.last_referenced_at < now() - make_interval(secs => $2)
			AND NOT EXISTS (SELECT 1 FROM skill_files f WHERE f.sha256 = b.sha256)
			AND NOT EXISTS (SELECT 1 FROM skills k WHERE k.image_sha256 = b.sha256)`, sum, float64(graceSecs))
	return n > 0, err
}

func (s *BlobRepository) All(ctx context.Context, after []byte, limit int) ([][]byte, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, pgx.RowTo[[]byte], `
		SELECT sha256
		FROM skill_blobs
		WHERE $1::bytea IS NULL OR sha256 > $1
		ORDER BY sha256
		LIMIT $2`, after, limit)
}

func (s *BlobRepository) Size(ctx context.Context, sum []byte) (*int64, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowTo[int64], `
		SELECT size
		FROM skill_blobs
		WHERE sha256 = $1`, sum)
}
