package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	topologytypes "svc-registry/internal/feature/topology/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type DependencyRepository struct{ db *pgtx.DB }

func NewDependencyRepository(db *pgtx.DB) *DependencyRepository { return &DependencyRepository{db: db} }

var edgeOf = postgres.Struct(edgeRow.edge)

func (s *DependencyRepository) DeleteOriginExcept(ctx context.Context, project uuid.UUID, source topologytypes.Source, origin string, keep []string) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM service_dependencies
		WHERE project_id = $1
			AND source = $2
			AND origin = $3
			AND NOT coalesce(target_project_id::text, 'external:' || target_external, 'ref:' || target_ref) = ANY($4::text[])`,
		project, string(source), origin, keep)
	return err
}

func (s *DependencyRepository) Upsert(ctx context.Context, e topologytypes.NewEdge) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO service_dependencies (id, project_id, source, origin, target_project_id, target_external, target_ref)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (project_id, source, origin,
			(coalesce(target_project_id::text, 'external:' || target_external, 'ref:' || target_ref)))
		DO UPDATE SET seen_at = now(), target_ref = EXCLUDED.target_ref`,
		e.ID, e.ProjectID, string(e.Source), e.Origin, e.Target.ProjectID, e.Target.External, e.Target.Ref)
	return err
}

func (s *DependencyRepository) DropOtherOrigins(ctx context.Context, project uuid.UUID, source topologytypes.Source, origin string) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM service_dependencies
		WHERE project_id = $1
			AND source = $2
			AND origin <> $3`, project, string(source), origin)
	return err
}

func (s *DependencyRepository) KeepClusterOrigins(ctx context.Context, cluster uuid.UUID, projects []uuid.UUID, origins []string) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM service_dependencies d
		WHERE d.source = 'cluster'
			AND starts_with(d.origin, $1::text || '/')
			AND NOT EXISTS (
				SELECT 1
				FROM unnest($2::uuid[], $3::text[]) AS k(project_id, origin)
				WHERE k.project_id = d.project_id AND k.origin = d.origin
			)`, cluster.String(), projects, origins)
	return err
}

func (s *DependencyRepository) InsertManual(ctx context.Context, e topologytypes.NewEdge) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO service_dependencies (id, project_id, source, origin, target_project_id, target_external, target_ref, note)
		VALUES ($1, $2, 'manual', '', $3, $4, $5, $6)`, e.ID, e.ProjectID, e.Target.ProjectID, e.Target.External, e.Target.Ref, e.Note)
	return err
}

func (s *DependencyRepository) DeleteManual(ctx context.Context, project, id uuid.UUID) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		DELETE FROM service_dependencies
		WHERE id = $1
			AND project_id = $2
			AND source = 'manual'`, id, project)
}

func (s *DependencyRepository) Touching(ctx context.Context, projects []uuid.UUID, outgoingOnly bool) ([]topologytypes.Edge, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, edgeOf, `
		SELECT id, project_id, source, origin, target_project_id, target_external, target_ref, note, rfc3339(seen_at) AS seen_at
		FROM service_dependencies
		WHERE project_id = ANY($1)
			OR (NOT $2 AND target_project_id = ANY($1))
		ORDER BY id`, projects, outgoingOnly)
}

func (s *DependencyRepository) SubtreeProjects(ctx context.Context, root uuid.UUID) ([]uuid.UUID, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, pgx.RowTo[uuid.UUID], `
		WITH RECURSIVE down AS (
			SELECT id, kind
			FROM nodes
			WHERE id = $1
			UNION ALL
			SELECT c.id, c.kind
			FROM nodes c
			JOIN down d ON c.parent_id = d.id
		)
		SELECT d.id
		FROM down d
		WHERE d.kind = 'project'
			AND EXISTS (SELECT 1 FROM service_dependencies s WHERE s.project_id = d.id)`, root)
}
