package repositories

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	forgetypes "svc-registry/internal/feature/forge/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type RunRepository struct{ db *pgtx.DB }

func NewRunRepository(db *pgtx.DB) *RunRepository { return &RunRepository{db: db} }

func (s *RunRepository) Start(ctx context.Context, n forgetypes.NewRun) (forgetypes.Run, error) {
	r, err := scanRun(s.db.From(ctx).QueryRow(ctx, `
		INSERT INTO forge_sync_runs AS r (id, connection_id, trigger, status)
		VALUES ($1, $2, $3, 'running')
		RETURNING r.id, r.connection_id, r.trigger, r.status, rfc3339(r.started_at), rfc3339(r.finished_at),
			r.created, r.updated, r.orphaned, r.skipped, r.error_code, r.error_message, r.problems`,
		n.ID, n.ConnectionID, string(n.Trigger)))
	return r, postgres.Translate(err, constraints)
}

func (s *RunRepository) Finish(ctx context.Context, id uuid.UUID, res forgetypes.RunResult, keep int) error {
	problems := res.Problems
	if problems == nil {
		problems = []forgetypes.Problem{}
	}
	if len(problems) > forgetypes.MaxProblems {
		problems = problems[:forgetypes.MaxProblems]
	}
	body, err := json.Marshal(problems)
	if err != nil {
		return apperr.InternalDetail(err.Error())
	}
	err = s.db.InTx(ctx, func(ctx context.Context) error {
		tx := s.db.From(ctx)
		var connID uuid.UUID
		c := res.Counts
		if err := tx.QueryRow(ctx, `
			UPDATE forge_sync_runs
			SET status = $2, finished_at = now(), created = $3, updated = $4, orphaned = $5, skipped = $6,
				error_code = $7, error_message = $8, problems = $9
			WHERE id = $1
			RETURNING connection_id`, id, string(res.Status), c.Created, c.Updated, c.Orphaned, c.Skipped,
			res.ErrorCode, res.ErrorMessage, body).Scan(&connID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			DELETE FROM forge_sync_runs
			WHERE connection_id = $1
				AND id NOT IN (
					SELECT id
					FROM forge_sync_runs
					WHERE connection_id = $1
					ORDER BY started_at DESC, id DESC
					LIMIT $2
				)`, connID, keep)
		return err
	})
	return postgres.Translate(err, constraints)
}

func (s *RunRepository) List(ctx context.Context, connID uuid.UUID, limit int) ([]forgetypes.Run, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT r.id, r.connection_id, r.trigger, r.status, rfc3339(r.started_at), rfc3339(r.finished_at),
			r.created, r.updated, r.orphaned, r.skipped, r.error_code, r.error_message, r.problems
		FROM forge_sync_runs r
		WHERE r.connection_id = $1
		ORDER BY r.started_at DESC, r.id DESC
		LIMIT $2`, connID, limit)
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (forgetypes.Run, error) { return scanRun(r) })
	return out, postgres.Translate(err, constraints)
}
