package ingest

import (
	"encoding/json"
	"strings"

	"github.com/google/uuid"
)

type EventType string

const (
	TypeServiceDeployed EventType = "service.deployed"
)

func ParseEventType(s string) (EventType, bool) {
	if EventType(s) == TypeServiceDeployed {
		return TypeServiceDeployed, true
	}
	return "", false
}

func (t EventType) Supports(version int64) bool {
	return t == TypeServiceDeployed && version == 1
}

type Envelope struct {
	Type            EventType
	Version         int32
	OccurredAt      string
	IdempotencyKey  string
	ServiceDeployed *ServiceDeployed
	Payload         json.RawMessage
}

type ServiceDeployed struct {
	Service     string
	Version     string
	Environment string
	CommitSHA   *string
	Branch      *string
	Cluster     *string
	Namespace   *string
	URL         *string
	DeployedBy  *string
	Metadata    map[string]string
}

type NewEvent struct {
	ID            uuid.UUID
	ProjectID     uuid.UUID
	KeyID         uuid.UUID
	DeploymentID  uuid.UUID
	EnvironmentID uuid.UUID
	Envelope      Envelope
}

type DeploymentResult struct {
	ID            uuid.UUID
	Service       string
	Environment   string
	Version       string
	BecameCurrent bool
}

type EventFilter struct {
	Type *string
}

type DeploymentFilter struct {
	Service     *string
	Environment *string
	Branch      *string
}

func NewDeploymentFilter(service, environment, branch *string) DeploymentFilter {
	norm := func(v *string) *string {
		if v == nil {
			return nil
		}
		s := strings.ToLower(trim(*v))
		if s == "" {
			return nil
		}
		return &s
	}
	f := DeploymentFilter{Service: norm(service), Environment: norm(environment)}
	if branch != nil && trim(*branch) != "" {
		f.Branch = branch
	}
	return f
}
