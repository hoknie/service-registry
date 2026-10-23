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
	return postgres.One(ctx, s.db.From(ctx), constraints, scanRun, `
		INSERT INTO forge_sync_runs AS r (id, connection_id, trigger, status)
		VALUES ($1, $2, $3, 'running')
		RETURNING r.id, r.connection_id, r.trigger, r.status, rfc3339(r.started_at), rfc3339(r.finished_at),
			r.created, r.updated, r.orphaned, r.skipped, r.error_code, r.error_message, r.problems`,
		n.ID, n.ConnectionID, string(n.Trigger))
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
		c := res.Counts
		connID, err := postgres.One(ctx, tx, constraints, pgx.RowTo[uuid.UUID], `
			UPDATE forge_sync_runs
			SET status = $2, finished_at = now(), created = $3, updated = $4, orphaned = $5, skipped = $6,
				error_code = $7, error_message = $8, problems = $9
			WHERE id = $1
			RETURNING connection_id`, id, string(res.Status), c.Created, c.Updated, c.Orphaned, c.Skipped,
			res.ErrorCode, res.ErrorMessage, body)
		if err != nil {
			return err
		}
		_, err = postgres.Exec(ctx, tx, constraints, `
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

func (s *RunRepository) PreviousStatus(ctx context.Context, connID, runID uuid.UUID) (*forgetypes.RunStatus, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (forgetypes.RunStatus, error) {
		var status string
		err := r.Scan(&status)
		return forgetypes.RunStatus(status), err
	}, `
		SELECT status
		FROM forge_sync_runs
		WHERE connection_id = $1
			AND id <> $2
			AND finished_at IS NOT NULL
		ORDER BY started_at DESC, id DESC
		LIMIT 1`, connID, runID)
}

func (s *RunRepository) List(ctx context.Context, connID uuid.UUID, limit int) ([]forgetypes.Run, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, scanRun, `
		SELECT r.id, r.connection_id, r.trigger, r.status, rfc3339(r.started_at), rfc3339(r.finished_at),
			r.created, r.updated, r.orphaned, r.skipped, r.error_code, r.error_message, r.problems
		FROM forge_sync_runs r
		WHERE r.connection_id = $1
		ORDER BY r.started_at DESC, r.id DESC
		LIMIT $2`, connID, limit)
}
