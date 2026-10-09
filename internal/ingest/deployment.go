package ingest

import "github.com/google/uuid"

type Deployment struct {
	ID          uuid.UUID
	ProjectID   uuid.UUID
	EventID     *uuid.UUID
	Service     string
	Environment string
	Version     string
	CommitSHA   *string
	Branch      *string
	Cluster     *string
	Namespace   *string
	URL         *string
	DeployedBy  *string
	Metadata    map[string]string
	OccurredAt  string
	ReceivedAt  string
	Current     bool
	Source      string
}

type Environment struct {
	Service      string
	Environment  string
	Version      string
	CommitSHA    *string
	Branch       *string
	URL          *string
	DeployedBy   *string
	OccurredAt   string
	DeploymentID uuid.UUID
	UpdatedAt    string
	Source       string
}
