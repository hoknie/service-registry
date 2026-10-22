package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	accesstypes "svc-registry/internal/feature/access/types"
	ingesttypes "svc-registry/internal/feature/ingest/types"
	"svc-registry/internal/platform/postgres"
)

type DeploymentRepository struct{ db *postgres.DB }

func NewDeploymentRepository(db *postgres.DB) *DeploymentRepository {
	return &DeploymentRepository{db: db}
}

func (s *DeploymentRepository) List(ctx context.Context, projectID uuid.UUID, f ingesttypes.DeploymentFilter, page accesstypes.PageRequest) (accesstypes.Page[ingesttypes.Deployment], error) {
	var total int64
	if err := s.db.From(ctx).QueryRow(ctx, `
		SELECT count(*)
		FROM service_deployments d
		WHERE d.project_id = $1
			AND ($2::text IS NULL OR d.service = $2)
			AND ($3::text IS NULL OR d.environment = $3)
			AND ($4::text IS NULL OR d.branch = $4)`, projectID, f.Service, f.Environment, f.Branch).Scan(&total); err != nil {
		return accesstypes.Page[ingesttypes.Deployment]{}, postgres.Translate(err, constraints)
	}
	rows, _ := s.db.From(ctx).Query(ctx, `
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
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (ingesttypes.Deployment, error) {
		row, err := pgx.RowToStructByName[deploymentRow](r)
		return row.deployment(), err
	})
	if err != nil {
		return accesstypes.Page[ingesttypes.Deployment]{}, postgres.Translate(err, constraints)
	}
	return accesstypes.Page[ingesttypes.Deployment]{Items: items, Total: uint64(max(total, 0)), Limit: page.Limit, Offset: page.Offset}, nil
}

func (s *DeploymentRepository) Environments(ctx context.Context, projectID uuid.UUID) ([]ingesttypes.Environment, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT s.service, s.environment, d.version, d.commit_sha, d.branch, d.url, d.deployed_by,
			rfc3339(s.occurred_at) AS occurred_at, s.deployment_id, rfc3339(s.updated_at) AS updated_at,
			d.source, d.links
		FROM service_environments s
		JOIN service_deployments d ON d.id = s.deployment_id
		WHERE s.project_id = $1
		ORDER BY s.service, s.environment`, projectID)
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByPos[ingesttypes.Environment])
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	return out, nil
}
