package repository

import (
	"encoding/json"
	"errors"

	"github.com/google/uuid"

	"svc-registry/internal/feature/ingest"
	"svc-registry/internal/platform/apperr"
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

func (r eventRow) event() ingest.Event { return ingest.Event(r) }

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

func (r deploymentRow) deployment() ingest.Deployment { return ingest.Deployment(r) }

func dbErr(err error) error {
	if err == nil {
		return nil
	}
	var rejection *ingest.Rejection
	var conflict ingest.Conflict
	var internalErr *ingest.InternalError
	if errors.As(err, &rejection) || errors.As(err, &conflict) || errors.As(err, &internalErr) {
		return err
	}
	if apperr.IsUnavailable(err) {
		return ingest.ErrUnavailable
	}
	return &ingest.InternalError{Detail: err.Error()}
}

func bigint(n uint64) int64 {
	if n > 1<<63-1 {
		return 1<<63 - 1
	}
	return int64(n)
}
