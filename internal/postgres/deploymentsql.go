package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Deployment struct {
	ID            uuid.UUID
	EnvironmentID uuid.UUID
	ProjectID     uuid.UUID
	EventID       *uuid.UUID
	Source        string
	Service       string
	Environment   string
	Version       string
	CommitSHA     *string
	Branch        *string
	Cluster       *string
	Namespace     *string
	URL           *string
	DeployedBy    *string
	Metadata      map[string]string
	OccurredAt    any
}

func InsertDeployment(ctx context.Context, tx pgx.Tx, d Deployment) (bool, error) {
	metadata := d.Metadata
	if metadata == nil {
		metadata = map[string]string{}
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO service_deployments (id, project_id, event_id, service, environment, version,
			commit_sha, branch, cluster, namespace, url, deployed_by, metadata, occurred_at, source)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::jsonb, $14::timestamptz, $15)`,
		d.ID, d.ProjectID, d.EventID, d.Service, d.Environment, d.Version,
		d.CommitSHA, d.Branch, d.Cluster, d.Namespace, d.URL, d.DeployedBy, metadata, d.OccurredAt, d.Source); err != nil {
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
