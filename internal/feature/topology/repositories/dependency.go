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

func keyOf(t topologytypes.NewTarget) string {
	switch {
	case t.ProjectID != nil:
		return t.ProjectID.String()
	case t.External != nil:
		return "external:" + *t.External
	}
	return "ref:" + t.Ref
}

func (s *DependencyRepository) ReplaceOrigin(ctx context.Context, project uuid.UUID, source topologytypes.Source, origin string, targets []topologytypes.NewTarget) error {
	q := s.db.From(ctx)
	keys := make([]string, 0, len(targets))
	for _, t := range targets {
		keys = append(keys, keyOf(t))
	}
	if _, err := postgres.Exec(ctx, q, constraints, `
		DELETE FROM service_dependencies
		WHERE project_id = $1
			AND source = $2
			AND origin = $3
			AND NOT coalesce(target_project_id::text, 'external:' || target_external, 'ref:' || target_ref) = ANY($4::text[])`,
		project, string(source), origin, keys); err != nil {
		return err
	}
	for _, t := range targets {
		if _, err := postgres.Exec(ctx, q, constraints, `
			INSERT INTO service_dependencies (id, project_id, source, origin, target_project_id, target_external, target_ref)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (project_id, source, origin,
				(coalesce(target_project_id::text, 'external:' || target_external, 'ref:' || target_ref)))
			DO UPDATE SET seen_at = now(), target_ref = EXCLUDED.target_ref`,
			uuid.Must(uuid.NewV7()), project, string(source), origin, t.ProjectID, t.External, t.Ref); err != nil {
			return err
		}
	}
	return nil
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

func (s *DependencyRepository) InsertManual(ctx context.Context, project uuid.UUID, t topologytypes.NewTarget, note *string) (uuid.UUID, error) {
	id := uuid.Must(uuid.NewV7())
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO service_dependencies (id, project_id, source, origin, target_project_id, target_external, target_ref, note)
		VALUES ($1, $2, 'manual', '', $3, $4, $5, $6)`, id, project, t.ProjectID, t.External, t.Ref, note)
	return id, err
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
