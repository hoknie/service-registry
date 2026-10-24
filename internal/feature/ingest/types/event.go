package types

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const FutureTolerance = 5 * time.Minute

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

type Replay struct {
	EventID uuid.UUID
	Same    bool
}

func CheckOccurredAt(occurredAt string, now time.Time) error {
	at, err := time.Parse(time.RFC3339, occurredAt)
	if err != nil || !at.After(now.Add(FutureTolerance)) {
		return nil
	}
	return Invalid(FieldError{Path: "occurred_at", Code: FieldInFuture})
}
