package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type ActivityRepository struct{ db *pgtx.DB }

func NewActivityRepository(db *pgtx.DB) *ActivityRepository { return &ActivityRepository{db: db} }

func (s *ActivityRepository) Containers(ctx context.Context, userID uuid.UUID, all bool, projects, containers []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	out := map[uuid.UUID][]uuid.UUID{}
	if len(projects) == 0 || len(containers) == 0 {
		return out, nil
	}
	found, err := postgres.List(ctx, s.db.From(ctx), constraints, pgx.RowToStructByName[containedRow], `
		WITH RECURSIVE bound AS (
			SELECT node_id
			FROM role_bindings
			WHERE user_id = $1
			UNION ALL
			SELECT b.node_id
			FROM role_bindings b
			JOIN group_members gm ON gm.group_id = b.group_id
			WHERE gm.user_id = $1
		),
		up AS (
			SELECT n.id AS project_id, n.id AS node_id, n.parent_id, 0 AS depth
			FROM nodes n
			WHERE n.id = ANY($3)
				AND n.kind = 'project'
			UNION ALL
			SELECT u.project_id, p.id, p.parent_id, u.depth + 1
			FROM up u
			JOIN nodes p ON p.id = u.parent_id
			WHERE u.depth < 64
		),
		readable AS (
			SELECT DISTINCT project_id
			FROM up
			WHERE $2 OR node_id IN (SELECT node_id FROM bound)
		)
		SELECT DISTINCT u.node_id AS container_id, u.project_id
		FROM up u
		JOIN readable r ON r.project_id = u.project_id
		WHERE u.depth > 0
			AND u.node_id = ANY($4)`, userID, all, projects, containers)
	if err != nil {
		return nil, err
	}
	for _, r := range found {
		out[r.ContainerID] = append(out[r.ContainerID], r.ProjectID)
	}
	return out, nil
}
