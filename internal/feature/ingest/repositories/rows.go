package repositories

import (
	"encoding/json"

	"github.com/google/uuid"

	ingesttypes "svc-registry/internal/feature/ingest/types"
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

type replayRow struct {
	ID   uuid.UUID `db:"id"`
	Same bool      `db:"same"`
}

func (r eventRow) event() ingesttypes.Event { return ingesttypes.Event(r) }

type deploymentRow struct {
	ID          uuid.UUID                    `db:"id"`
	ProjectID   uuid.UUID                    `db:"project_id"`
	EventID     *uuid.UUID                   `db:"event_id"`
	Service     string                       `db:"service"`
	Environment string                       `db:"environment"`
	Version     string                       `db:"version"`
	CommitSHA   *string                      `db:"commit_sha"`
	Branch      *string                      `db:"branch"`
	Cluster     *string                      `db:"cluster"`
	Namespace   *string                      `db:"namespace"`
	URL         *string                      `db:"url"`
	DeployedBy  *string                      `db:"deployed_by"`
	Metadata    map[string]string            `db:"metadata"`
	Links       []ingesttypes.DeploymentLink `db:"links"`
	OccurredAt  string                       `db:"occurred_at"`
	ReceivedAt  string                       `db:"received_at"`
	Current     bool                         `db:"current"`
	Source      string                       `db:"source"`
}

func (r deploymentRow) deployment() ingesttypes.Deployment { return ingesttypes.Deployment(r) }

func bigint(n uint64) int64 {
	if n > 1<<63-1 {
		return 1<<63 - 1
	}
	return int64(n)
}
