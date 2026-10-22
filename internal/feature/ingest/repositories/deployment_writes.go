package repositories

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	ingesttypes "svc-registry/internal/feature/ingest/types"
	"svc-registry/pkg/pgtx"
)

func InsertDeployment(ctx context.Context, tx pgtx.Querier, d ingesttypes.DeploymentRecord) (bool, error) {
	metadata := d.Metadata
	if metadata == nil {
		metadata = map[string]string{}
	}
	links := d.Links
	if links == nil {
		links = []ingesttypes.DeploymentLink{}
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO service_deployments (id, project_id, event_id, service, environment, version,
			commit_sha, branch, cluster, namespace, url, deployed_by, metadata, occurred_at, source, links)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::jsonb, $14::timestamptz, $15, $16::jsonb)`,
		d.ID, d.ProjectID, d.EventID, d.Service, d.Environment, d.Version,
		d.CommitSHA, d.Branch, d.Cluster, d.Namespace, d.URL, d.DeployedBy, metadata, d.OccurredAt, d.Source, links); err != nil {
		return false, err
	}
	envRow := d.EnvironmentID
	if envRow == uuid.Nil {
		envRow = uuid.Must(uuid.NewV7())
	}
	var envID uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO service_environments AS s (id, project_id, service, environment, deployment_id,
			occurred_at)
		VALUES ($1, $2, $3, $4, $5, $6::timestamptz)
		ON CONFLICT (project_id, service, environment)
		DO UPDATE SET deployment_id = EXCLUDED.deployment_id, occurred_at = EXCLUDED.occurred_at,
			updated_at = now()
		WHERE (s.occurred_at, s.deployment_id) < (EXCLUDED.occurred_at, EXCLUDED.deployment_id)
		RETURNING s.id`, envRow, d.ProjectID, d.Service, d.Environment, d.ID, d.OccurredAt).Scan(&envID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_, err = tx.Exec(ctx, `
		UPDATE service_deployments
		SET became_current = true
		WHERE id = $1`, d.ID)
	return err == nil, err
}
