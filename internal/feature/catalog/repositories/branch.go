package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type BranchRepository struct {
	db        *pgtx.DB
	staleDays uint32
}

func NewBranchRepository(db *pgtx.DB, staleDays uint32) *BranchRepository {
	return &BranchRepository{db: db, staleDays: staleDays}
}

func scanBranch(row pgx.CollectableRow) (catalogtypes.Branch, error) {
	var b catalogtypes.Branch
	var sources []string
	if err := row.Scan(&b.ProjectID, &b.Name, &b.HeadSHA, &b.IsDefault, &b.Protected, &sources, &b.Pinned, &b.Stale,
		&b.FirstSeenAt, &b.LastActivityAt, &b.GoneAt); err != nil {
		return catalogtypes.Branch{}, err
	}
	for _, s := range sources {
		b.Sources = append(b.Sources, catalogtypes.BranchSource(s))
	}
	return b, nil
}

func (s *BranchRepository) List(ctx context.Context, projectID uuid.UUID, f catalogtypes.BranchFilter) ([]catalogtypes.Branch, uint64, error) {
	stale := int(s.staleDays)
	items, err := postgres.List(ctx, s.db.From(ctx), constraints, scanBranch, `
		SELECT b.project_id, b.name, b.head_sha, b.is_default, b.protected, b.sources, b.pinned,
			b.last_activity_at < now() - make_interval(days => $2::int) AS stale, rfc3339(b.first_seen_at),
			rfc3339(b.last_activity_at), rfc3339(b.gone_at)
		FROM branches b
		WHERE b.project_id = $1
			AND starts_with(b.name, $3)
			AND (
				$4 = 'all'
				OR ($4 = 'gone' AND b.gone_at IS NOT NULL)
				OR ($4 = 'active' AND b.gone_at IS NULL AND b.last_activity_at >= now() - make_interval(days => $2::int))
				OR ($4 = 'stale' AND b.gone_at IS NULL AND b.last_activity_at < now() - make_interval(days => $2::int))
			)
		ORDER BY b.is_default DESC, b.pinned DESC, b.last_activity_at DESC, b.name
		LIMIT $5
		OFFSET $6`, projectID, stale, f.Prefix, string(f.State), int64(f.Limit), int64(f.Offset))
	if err != nil {
		return nil, 0, err
	}
	total, err := postgres.One(ctx, s.db.From(ctx), constraints, pgx.RowTo[int64], `
		SELECT count(*)
		FROM branches b
		WHERE b.project_id = $1
			AND starts_with(b.name, $3)
			AND (
				$4 = 'all'
				OR ($4 = 'gone' AND b.gone_at IS NOT NULL)
				OR ($4 = 'active' AND b.gone_at IS NULL AND b.last_activity_at >= now() - make_interval(days => $2::int))
				OR ($4 = 'stale' AND b.gone_at IS NULL AND b.last_activity_at < now() - make_interval(days => $2::int))
			)`,
		projectID, stale, f.Prefix, string(f.State))
	if err != nil {
		return nil, 0, err
	}
	return items, uint64(max(total, 0)), nil
}

func (s *BranchRepository) Get(ctx context.Context, projectID uuid.UUID, name string) (*catalogtypes.Branch, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, scanBranch, `
		SELECT b.project_id, b.name, b.head_sha, b.is_default, b.protected, b.sources, b.pinned,
			b.last_activity_at < now() - make_interval(days => $2::int) AS stale, rfc3339(b.first_seen_at),
			rfc3339(b.last_activity_at), rfc3339(b.gone_at)
		FROM branches b
		WHERE b.project_id = $1
			AND b.name = $3`, projectID, int(s.staleDays), name)
}

func (s *BranchRepository) SetDefault(ctx context.Context, projectID uuid.UUID, name *string) error {
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		tx := s.db.From(ctx)
		if _, err := tx.Exec(ctx, `
			UPDATE branches
			SET is_default = false, updated_at = now()
			WHERE project_id = $1
				AND is_default
				AND ($2::text IS NULL OR name <> $2)`, projectID, name); err != nil {
			return err
		}
		if name == nil {
			return nil
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO branches AS b (id, project_id, name, is_default, sources)
			VALUES ($1, $2, $3, true, ARRAY['manual'])
			ON CONFLICT (project_id, name)
			DO UPDATE SET is_default = true, updated_at = now()
			WHERE NOT b.is_default`, uuid.Must(uuid.NewV7()), projectID, *name)
		return err
	})
	return postgres.Translate(err, constraints)
}

func (s *BranchRepository) SyncForge(ctx context.Context, projectID uuid.UUID, branches []catalogtypes.ForgeBranch, complete bool) error {
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		tx := s.db.From(ctx)
		names := make([]string, 0, len(branches))
		for _, b := range branches {
			if err := UpsertForgeBranch(ctx, tx, projectID, b.Name, b.HeadSHA, b.Protected, b.CommittedAt); err != nil {
				return err
			}
			names = append(names, b.Name)
		}
		if !complete {
			return nil
		}
		_, err := tx.Exec(ctx, `
			UPDATE branches
			SET gone_at = now(), updated_at = now()
			WHERE project_id = $1
				AND 'forge' = ANY(sources)
				AND gone_at IS NULL
				AND NOT (name = ANY($2::text[]))`, projectID, names)
		return err
	})
	return postgres.Translate(err, constraints)
}

func (s *BranchRepository) SetPinned(ctx context.Context, projectID uuid.UUID, name string, pinned bool) (*catalogtypes.Branch, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, scanBranch, `
		UPDATE branches AS b
		SET pinned = $4, updated_at = now()
		WHERE b.project_id = $1
			AND b.name = $3
		RETURNING b.project_id, b.name, b.head_sha, b.is_default, b.protected, b.sources, b.pinned,
			b.last_activity_at < now() - make_interval(days => $2::int) AS stale, rfc3339(b.first_seen_at),
			rfc3339(b.last_activity_at), rfc3339(b.gone_at)`, projectID, int(s.staleDays), name, pinned)
}

func (s *BranchRepository) Delete(ctx context.Context, projectID uuid.UUID, name string) error {
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		tx := s.db.From(ctx)
		isDefault, err := postgres.Find(ctx, tx, constraints, pgx.RowTo[bool], `
			SELECT is_default
			FROM branches
			WHERE project_id = $1
				AND name = $2
			FOR UPDATE`, projectID, name)
		if err != nil {
			return err
		}
		if isDefault == nil {
			return apperr.ErrNotFound
		}
		if *isDefault {
			return catalogtypes.ConflictBranchIsDefault
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM branches
			WHERE project_id = $1
				AND name = $2`, projectID, name); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			DELETE FROM knowledge_snapshots
			WHERE project_id = $1
				AND branch = $2`, projectID, name)
		return err
	})
	return postgres.Translate(err, constraints)
}

func (s *BranchRepository) Prune(ctx context.Context, retentionDays, staleDays uint32) (int64, error) {
	return postgres.One(ctx, s.db.From(ctx), constraints, pgx.RowTo[int64], `
		WITH gone AS (
			DELETE FROM branches
			WHERE NOT pinned
				AND NOT is_default
				AND (
					gone_at < now() - make_interval(days => $1::int)
					OR (NOT sources && ARRAY['forge', 'repository'] AND last_activity_at < now() - make_interval(days => $1::int + $2::int))
				)
			RETURNING project_id, name
		),
		snapshots AS (
			DELETE FROM knowledge_snapshots s USING gone g
			WHERE s.project_id = g.project_id
				AND s.branch = g.name
		)
		SELECT count(*)
		FROM gone`, int(retentionDays), int(staleDays))
}
