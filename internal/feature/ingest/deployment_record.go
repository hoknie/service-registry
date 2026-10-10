package ingest

import (
	"github.com/google/uuid"
)

type DeploymentRecord struct {
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
