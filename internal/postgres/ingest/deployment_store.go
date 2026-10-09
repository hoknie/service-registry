package ingest

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"svc-registry/internal/access"
	domain "svc-registry/internal/ingest"
)

type DeploymentStore struct{ pool *pgxpool.Pool }

func NewDeploymentStore(pool *pgxpool.Pool) *DeploymentStore { return &DeploymentStore{pool: pool} }

func (s *DeploymentStore) List(ctx context.Context, projectID uuid.UUID, f domain.DeploymentFilter, page access.PageRequest) (access.Page[domain.Deployment], error) {
	var total int64
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM service_deployments d
		WHERE d.project_id = $1
			AND ($2::text IS NULL OR d.service = $2)
			AND ($3::text IS NULL OR d.environment = $3)
			AND ($4::text IS NULL OR d.branch = $4)`, projectID, f.Service, f.Environment, f.Branch).Scan(&total); err != nil {
		return access.Page[domain.Deployment]{}, dbErr(err)
	}
	rows, _ := s.pool.Query(ctx, `
		SELECT d.id, d.project_id, d.event_id, d.service, d.environment, d.version, d.commit_sha, d.branch,
			d.cluster, d.namespace, d.url, d.deployed_by, d.metadata, rfc3339(d.occurred_at) AS occurred_at,
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
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.Deployment, error) {
		row, err := pgx.RowToStructByName[deploymentRow](r)
		return row.deployment(), err
	})
	if err != nil {
		return access.Page[domain.Deployment]{}, dbErr(err)
	}
	return access.Page[domain.Deployment]{Items: items, Total: uint64(max(total, 0)), Limit: page.Limit, Offset: page.Offset}, nil
}

func (s *DeploymentStore) Environments(ctx context.Context, projectID uuid.UUID) ([]domain.Environment, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT s.service, s.environment, d.version, d.commit_sha, d.branch, d.url, d.deployed_by,
			rfc3339(s.occurred_at) AS occurred_at, s.deployment_id, rfc3339(s.updated_at) AS updated_at,
			d.source
		FROM service_environments s
		JOIN service_deployments d ON d.id = s.deployment_id
		WHERE s.project_id = $1
		ORDER BY s.service, s.environment`, projectID)
	if err != nil {
		return nil, dbErr(err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByPos[domain.Environment])
	if err != nil {
		return nil, dbErr(err)
	}
	return out, nil
}
