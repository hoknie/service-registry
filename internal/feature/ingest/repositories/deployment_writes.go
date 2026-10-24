package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	ingesttypes "svc-registry/internal/feature/ingest/types"
	"svc-registry/internal/platform/postgres"
)

func (s *DeploymentRepository) Insert(ctx context.Context, d ingesttypes.DeploymentRecord) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO service_deployments (id, project_id, event_id, service, environment, version,
			commit_sha, branch, cluster, namespace, url, deployed_by, metadata, occurred_at, source, links)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::jsonb, $14::timestamptz, $15, $16::jsonb)`,
		d.ID, d.ProjectID, d.EventID, d.Service, d.Environment, d.Version,
		d.CommitSHA, d.Branch, d.Cluster, d.Namespace, d.URL, d.DeployedBy, d.Metadata, d.OccurredAt, d.Source, d.Links)
	return err
}

func (s *DeploymentRepository) AdvanceEnvironment(ctx context.Context, d ingesttypes.DeploymentRecord) (bool, error) {
	found, err := postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowTo[uuid.UUID], `
		INSERT INTO service_environments AS s (id, project_id, service, environment, deployment_id,
			occurred_at)
		VALUES ($1, $2, $3, $4, $5, $6::timestamptz)
		ON CONFLICT (project_id, service, environment)
		DO UPDATE SET deployment_id = EXCLUDED.deployment_id, occurred_at = EXCLUDED.occurred_at,
			updated_at = now()
		WHERE (s.occurred_at, s.deployment_id) < (EXCLUDED.occurred_at, EXCLUDED.deployment_id)
		RETURNING s.id`, d.EnvironmentID, d.ProjectID, d.Service, d.Environment, d.ID, d.OccurredAt)
	return found != nil, err
}

func (s *DeploymentRepository) MarkCurrent(ctx context.Context, id uuid.UUID) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE service_deployments
		SET became_current = true
		WHERE id = $1`, id)
	return err
}

func (s *DeploymentRepository) ByEvent(ctx context.Context, eventID uuid.UUID) (*ingesttypes.DeploymentResult, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowToStructByPos[ingesttypes.DeploymentResult], `
		SELECT id, service, environment, version, became_current
		FROM service_deployments
		WHERE event_id = $1`, eventID)
}
