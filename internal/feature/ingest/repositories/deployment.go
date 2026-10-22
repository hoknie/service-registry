package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	accesstypes "svc-registry/internal/feature/access/types"
	ingesttypes "svc-registry/internal/feature/ingest/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type DeploymentRepository struct{ db *pgtx.DB }

func NewDeploymentRepository(db *pgtx.DB) *DeploymentRepository {
	return &DeploymentRepository{db: db}
}

func (s *DeploymentRepository) List(ctx context.Context, projectID uuid.UUID, f ingesttypes.DeploymentFilter, page accesstypes.PageRequest) (accesstypes.Page[ingesttypes.Deployment], error) {
	total, err := postgres.One(ctx, s.db.From(ctx), constraints, pgx.RowTo[int64], `
		SELECT count(*)
		FROM service_deployments d
		WHERE d.project_id = $1
			AND ($2::text IS NULL OR d.service = $2)
			AND ($3::text IS NULL OR d.environment = $3)
			AND ($4::text IS NULL OR d.branch = $4)`, projectID, f.Service, f.Environment, f.Branch)
	if err != nil {
		return accesstypes.Page[ingesttypes.Deployment]{}, err
	}
	items, err := postgres.List(ctx, s.db.From(ctx), constraints, postgres.Struct(deploymentRow.deployment), `
		SELECT d.id, d.project_id, d.event_id, d.service, d.environment, d.version, d.commit_sha, d.branch,
			d.cluster, d.namespace, d.url, d.deployed_by, d.metadata, d.links, rfc3339(d.occurred_at) AS occurred_at,
			rfc3339(d.created_at) AS received_at, EXISTS (
			SELECT 1
			FROM service_environments s
			WHERE s.deployment_id = d.id
		) AS current, d.source
		FROM service_deployments d
		WHERE d.project_id = $1
			AND ($2::text IS NULL OR d.service = $2)
			AND ($3::text IS NULL OR d.environment = $3)
			AND ($4::text IS NULL OR d.branch = $4)
		ORDER BY d.occurred_at DESC, d.id DESC
		LIMIT $5
		OFFSET $6`, projectID, f.Service, f.Environment, f.Branch, int64(page.Limit), bigint(page.Offset))
	if err != nil {
		return accesstypes.Page[ingesttypes.Deployment]{}, err
	}
	return accesstypes.Page[ingesttypes.Deployment]{Items: items, Total: uint64(max(total, 0)), Limit: page.Limit, Offset: page.Offset}, nil
}

func (s *DeploymentRepository) Environments(ctx context.Context, projectID uuid.UUID) ([]ingesttypes.Environment, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, pgx.RowToStructByPos[ingesttypes.Environment], `
		SELECT s.service, s.environment, d.version, d.commit_sha, d.branch, d.url, d.deployed_by,
			rfc3339(s.occurred_at) AS occurred_at, s.deployment_id, rfc3339(s.updated_at) AS updated_at,
			d.source, d.links
		FROM service_environments s
		JOIN service_deployments d ON d.id = s.deployment_id
		WHERE s.project_id = $1
		ORDER BY s.service, s.environment`, projectID)
}
