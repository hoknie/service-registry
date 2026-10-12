package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Snapshots) ProjectsWithDocs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	out := map[uuid.UUID]bool{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT DISTINCT project_id
		FROM knowledge_snapshots
		WHERE project_id = ANY($1)
			AND status <> 'failed'`, ids)
	if err != nil {
		return nil, dbErr(err)
	}
	found, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return nil, dbErr(err)
	}
	for _, id := range found {
		out[id] = true
	}
	return out, nil
}
