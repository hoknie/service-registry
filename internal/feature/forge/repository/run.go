package repository

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"svc-registry/internal/feature/forge"
	"svc-registry/internal/platform/postgres"
)

type Runs struct{ db *postgres.DB }

func NewRuns(db *postgres.DB) *Runs { return &Runs{db: db} }

func (s *Runs) Start(ctx context.Context, n forge.NewRun) (forge.Run, error) {
	r, err := scanRun(s.db.From(ctx).QueryRow(ctx, `
		INSERT INTO forge_sync_runs AS r (id, connection_id, trigger, status)
		VALUES ($1, $2, $3, 'running')
		RETURNING r.id, r.connection_id, r.trigger, r.status, rfc3339(r.started_at), rfc3339(r.finished_at),
			r.created, r.updated, r.orphaned, r.skipped, r.error_code, r.error_message, r.problems`,
		n.ID, n.ConnectionID, string(n.Trigger)))
	return r, dbErr(err)
}

func (s *Runs) Finish(ctx context.Context, id uuid.UUID, res forge.RunResult, keep int) error {
	problems := res.Problems
	if problems == nil {
		problems = []forge.Problem{}
	}
	if len(problems) > forge.MaxProblems {
		problems = problems[:forge.MaxProblems]
	}
	body, err := json.Marshal(problems)
	if err != nil {
		return &forge.InternalError{Detail: err.Error()}
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
	return dbErr(err)
}

func (s *Runs) List(ctx context.Context, connID uuid.UUID, limit int) ([]forge.Run, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT r.id, r.connection_id, r.trigger, r.status, rfc3339(r.started_at), rfc3339(r.finished_at),
			r.created, r.updated, r.orphaned, r.skipped, r.error_code, r.error_message, r.problems
		FROM forge_sync_runs r
		WHERE r.connection_id = $1
		ORDER BY r.started_at DESC, r.id DESC
		LIMIT $2`, connID, limit)
	if err != nil {
		return nil, dbErr(err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (forge.Run, error) { return scanRun(r) })
	return out, dbErr(err)
}
