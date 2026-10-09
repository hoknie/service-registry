package forge

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "svc-registry/internal/forge"
)

type RunStore struct{ pool *pgxpool.Pool }

func NewRunStore(pool *pgxpool.Pool) *RunStore { return &RunStore{pool: pool} }

var (
	runInsert = "INSERT INTO forge_sync_runs AS r (id, connection_id, trigger, status) VALUES ($1, $2, $3, 'running') " +
		"RETURNING " + runColumns
	runFinish = "UPDATE forge_sync_runs SET status = $2, finished_at = now(), created = $3, updated = $4, orphaned = $5, " +
		"skipped = $6, error_code = $7, error_message = $8, problems = $9 WHERE id = $1 RETURNING connection_id"
	runPrune = "DELETE FROM forge_sync_runs WHERE connection_id = $1 AND id NOT IN (SELECT id FROM forge_sync_runs " +
		"WHERE connection_id = $1 ORDER BY started_at DESC, id DESC LIMIT $2)"
	runList = "SELECT " + runColumns + " FROM forge_sync_runs r WHERE r.connection_id = $1 " +
		"ORDER BY r.started_at DESC, r.id DESC LIMIT $2"
)

func (s *RunStore) Start(ctx context.Context, n domain.NewRun) (domain.Run, error) {
	r, err := scanRun(s.pool.QueryRow(ctx, runInsert, n.ID, n.ConnectionID, string(n.Trigger)))
	return r, dbErr(err)
}

func (s *RunStore) Finish(ctx context.Context, id uuid.UUID, res domain.RunResult, keep int) error {
	problems := res.Problems
	if problems == nil {
		problems = []domain.Problem{}
	}
	if len(problems) > domain.MaxProblems {
		problems = problems[:domain.MaxProblems]
	}
	body, err := json.Marshal(problems)
	if err != nil {
		return &domain.InternalError{Detail: err.Error()}
	}
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var connID uuid.UUID
		c := res.Counts
		if err := tx.QueryRow(ctx, runFinish, id, string(res.Status), c.Created, c.Updated, c.Orphaned, c.Skipped,
			res.ErrorCode, res.ErrorMessage, body).Scan(&connID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, runPrune, connID, keep)
		return err
	})
	return dbErr(err)
}

func (s *RunStore) List(ctx context.Context, connID uuid.UUID, limit int) ([]domain.Run, error) {
	rows, err := s.pool.Query(ctx, runList, connID, limit)
	if err != nil {
		return nil, dbErr(err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.Run, error) { return scanRun(r) })
	return out, dbErr(err)
}
