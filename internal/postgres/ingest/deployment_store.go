package ingest

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"svc-registry/internal/access"
	domain "svc-registry/internal/ingest"
	"svc-registry/internal/postgres"
)

type DeploymentStore struct{ pool *pgxpool.Pool }

func NewDeploymentStore(pool *pgxpool.Pool) *DeploymentStore { return &DeploymentStore{pool: pool} }

var (
	deploymentWhere = " FROM service_deployments d WHERE d.project_id = $1 " +
		"AND ($2::text IS NULL OR d.service = $2) AND ($3::text IS NULL OR d.environment = $3) " +
		"AND ($4::text IS NULL OR d.branch = $4)"
	deploymentCount = "SELECT count(*)" + deploymentWhere
	deploymentList  = "SELECT " + deploymentColumns + deploymentWhere +
		" ORDER BY d.occurred_at DESC, d.id DESC LIMIT $5 OFFSET $6"
	environmentList = "SELECT s.service, s.environment, d.version, d.commit_sha, d.branch, d.url, d.deployed_by, " +
		postgres.RFC3339("s.occurred_at") + " AS occurred_at, s.deployment_id, " +
		postgres.RFC3339("s.updated_at") + " AS updated_at, d.source FROM service_environments s " +
		"JOIN service_deployments d ON d.id = s.deployment_id " +
		"WHERE s.project_id = $1 ORDER BY s.service, s.environment"
)

func (s *DeploymentStore) List(ctx context.Context, projectID uuid.UUID, f domain.DeploymentFilter, page access.PageRequest) (access.Page[domain.Deployment], error) {
	var total int64
	if err := s.pool.QueryRow(ctx, deploymentCount, projectID, f.Service, f.Environment, f.Branch).Scan(&total); err != nil {
		return access.Page[domain.Deployment]{}, dbErr(err)
	}
	rows, _ := s.pool.Query(ctx, deploymentList, projectID, f.Service, f.Environment, f.Branch, int64(page.Limit), bigint(page.Offset))
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
	rows, err := s.pool.Query(ctx, environmentList, projectID)
	if err != nil {
		return nil, dbErr(err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByPos[domain.Environment])
	if err != nil {
		return nil, dbErr(err)
	}
	return out, nil
}
