package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"svc-registry/internal/feature/catalog"
	"svc-registry/internal/platform/postgres"
)

type Branches struct {
	db        *postgres.DB
	staleDays uint32
}

func NewBranches(db *postgres.DB, staleDays uint32) *Branches {
	return &Branches{db: db, staleDays: staleDays}
}

func scanBranch(row pgx.Row) (catalog.Branch, error) {
	var b catalog.Branch
	var sources []string
	if err := row.Scan(&b.ProjectID, &b.Name, &b.HeadSHA, &b.IsDefault, &b.Protected, &sources, &b.Pinned, &b.Stale,
		&b.FirstSeenAt, &b.LastActivityAt, &b.GoneAt); err != nil {
		return catalog.Branch{}, err
	}
	for _, s := range sources {
		b.Sources = append(b.Sources, catalog.BranchSource(s))
	}
	return b, nil
}

func (s *Branches) List(ctx context.Context, projectID uuid.UUID, f catalog.BranchFilter) ([]catalog.Branch, uint64, error) {
	stale := int(s.staleDays)
	rows, err := s.db.From(ctx).Query(ctx, `
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
		return nil, 0, dbErr(err)
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (catalog.Branch, error) { return scanBranch(r) })
	if err != nil {
		return nil, 0, dbErr(err)
	}
	var total int64
	if err := s.db.From(ctx).QueryRow(ctx, `
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
		projectID, stale, f.Prefix, string(f.State)).Scan(&total); err != nil {
		return nil, 0, dbErr(err)
	}
	return items, uint64(max(total, 0)), nil
}

func (s *Branches) Get(ctx context.Context, projectID uuid.UUID, name string) (*catalog.Branch, error) {
	b, err := scanBranch(s.db.From(ctx).QueryRow(ctx, `
		SELECT b.project_id, b.name, b.head_sha, b.is_default, b.protected, b.sources, b.pinned,
			b.last_activity_at < now() - make_interval(days => $2::int) AS stale, rfc3339(b.first_seen_at),
			rfc3339(b.last_activity_at), rfc3339(b.gone_at)
		FROM branches b
		WHERE b.project_id = $1
			AND b.name = $3`, projectID, int(s.staleDays), name))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, dbErr(err)
	}
	return &b, nil
}

func (s *Branches) SetDefault(ctx context.Context, projectID uuid.UUID, name *string) error {
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
	return dbErr(err)
}

func (s *Branches) SyncForge(ctx context.Context, projectID uuid.UUID, branches []catalog.ForgeBranch, complete bool) error {
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
	return dbErr(err)
}

func (s *Branches) SetPinned(ctx context.Context, projectID uuid.UUID, name string, pinned bool) (*catalog.Branch, error) {
	b, err := scanBranch(s.db.From(ctx).QueryRow(ctx, `
		UPDATE branches AS b
		SET pinned = $4, updated_at = now()
		WHERE b.project_id = $1
			AND b.name = $3
		RETURNING b.project_id, b.name, b.head_sha, b.is_default, b.protected, b.sources, b.pinned,
			b.last_activity_at < now() - make_interval(days => $2::int) AS stale, rfc3339(b.first_seen_at),
			rfc3339(b.last_activity_at), rfc3339(b.gone_at)`, projectID, int(s.staleDays), name, pinned))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, dbErr(err)
	}
	return &b, nil
}

func (s *Branches) Delete(ctx context.Context, projectID uuid.UUID, name string) error {
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		tx := s.db.From(ctx)
		var isDefault bool
		if err := tx.QueryRow(ctx, `
			SELECT is_default
			FROM branches
			WHERE project_id = $1
				AND name = $2
			FOR UPDATE`, projectID, name).Scan(&isDefault); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return catalog.ErrNotFound
			}
			return err
		}
		if isDefault {
			return catalog.ConflictBranchIsDefault
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM branches
			WHERE project_id = $1
				AND name = $2`, projectID, name); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			DELETE FROM knowledge_snapshots
			WHERE project_id = $1
				AND branch = $2`, projectID, name)
		return err
	})
	return dbErr(err)
}

func (s *Branches) Prune(ctx context.Context, retentionDays, staleDays uint32) (int64, error) {
	var n int64
	if err := s.db.From(ctx).QueryRow(ctx, `
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
		FROM gone`, int(retentionDays), int(staleDays)).Scan(&n); err != nil {
		return 0, dbErr(err)
	}
	return n, nil
}
