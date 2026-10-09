package ingest

import (
	"encoding/json"
	"errors"

	"github.com/google/uuid"

	"svc-registry/internal/apperr"
	domain "svc-registry/internal/ingest"
)

type eventRow struct {
	ID             uuid.UUID       `db:"id"`
	ProjectID      uuid.UUID       `db:"project_id"`
	Type           string          `db:"type"`
	Version        int32           `db:"version"`
	IdempotencyKey string          `db:"idempotency_key"`
	OccurredAt     string          `db:"occurred_at"`
	ReceivedAt     string          `db:"received_at"`
	KeyPrefix      *string         `db:"key_prefix"`
	Payload        json.RawMessage `db:"payload"`
}

func (r eventRow) event() domain.Event {
	return domain.Event{
		ID: r.ID, ProjectID: r.ProjectID, Type: r.Type, Version: r.Version, IdempotencyKey: r.IdempotencyKey,
		OccurredAt: r.OccurredAt, ReceivedAt: r.ReceivedAt, KeyPrefix: r.KeyPrefix, Payload: r.Payload,
	}
}

type deploymentRow struct {
	ID          uuid.UUID         `db:"id"`
	ProjectID   uuid.UUID         `db:"project_id"`
	EventID     *uuid.UUID        `db:"event_id"`
	Service     string            `db:"service"`
	Environment string            `db:"environment"`
	Version     string            `db:"version"`
	CommitSHA   *string           `db:"commit_sha"`
	Branch      *string           `db:"branch"`
	Cluster     *string           `db:"cluster"`
	Namespace   *string           `db:"namespace"`
	URL         *string           `db:"url"`
	DeployedBy  *string           `db:"deployed_by"`
	Metadata    map[string]string `db:"metadata"`
	OccurredAt  string            `db:"occurred_at"`
	ReceivedAt  string            `db:"received_at"`
	Current     bool              `db:"current"`
	Source      string            `db:"source"`
}

func (r deploymentRow) deployment() domain.Deployment {
	return domain.Deployment{
		ID: r.ID, ProjectID: r.ProjectID, EventID: r.EventID, Service: r.Service, Environment: r.Environment,
		Version: r.Version, CommitSHA: r.CommitSHA, Branch: r.Branch, Cluster: r.Cluster, Namespace: r.Namespace,
		URL: r.URL, DeployedBy: r.DeployedBy, Metadata: r.Metadata, OccurredAt: r.OccurredAt,
		ReceivedAt: r.ReceivedAt, Current: r.Current, Source: r.Source,
	}
}

func dbErr(err error) error {
	if err == nil {
		return nil
	}
	var rejection *domain.Rejection
	var conflict domain.Conflict
	var internalErr *domain.InternalError
	if errors.As(err, &rejection) || errors.As(err, &conflict) || errors.As(err, &internalErr) {
		return err
	}
	if apperr.IsUnavailable(err) {
		return domain.ErrUnavailable
	}
	return &domain.InternalError{Detail: err.Error()}
}

func bigint(n uint64) int64 {
	if n > 1<<63-1 {
		return 1<<63 - 1
	}
	return int64(n)
}
