package ingest

import (
	"encoding/json"

	"github.com/google/uuid"
)

type Event struct {
	ID             uuid.UUID
	ProjectID      uuid.UUID
	Type           string
	Version        int32
	IdempotencyKey string
	OccurredAt     string
	ReceivedAt     string
	KeyPrefix      *string
	Payload        json.RawMessage
}

type Accepted struct {
	Event      Event
	Replayed   bool
	Deployment *DeploymentResult
}
