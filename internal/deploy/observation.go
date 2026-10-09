package deploy

import (
	"time"

	"github.com/google/uuid"
)

type Reason string

const (
	ReasonNoAnnotation       Reason = "no_annotation"
	ReasonProjectNotFound    Reason = "project_not_found"
	ReasonInvalidService     Reason = "invalid_service"
	ReasonInvalidEnvironment Reason = "invalid_environment"
)

type Rollout string

const (
	RolloutComplete    Rollout = "complete"
	RolloutProgressing Rollout = "progressing"
	RolloutStalled     Rollout = "stalled"
)

type Image struct {
	Container string  `json:"container"`
	Image     string  `json:"image"`
	Digest    *string `json:"digest"`
}

type Replicas struct {
	Desired int32 `json:"desired"`
	Ready   int32 `json:"ready"`
	Updated int32 `json:"updated"`
}

type LastRun struct {
	StartedAt *string `json:"started_at"`
	Status    string  `json:"status"`
}

type State struct {
	Replicas         *Replicas `json:"replicas,omitempty"`
	Restarts         *int32    `json:"restarts,omitempty"`
	Rollout          *Rollout  `json:"rollout,omitempty"`
	ProgressingSince *string   `json:"progressing_since,omitempty"`
	Schedule         *string   `json:"schedule,omitempty"`
	Suspended        *bool     `json:"suspended,omitempty"`
	LastRun          *LastRun  `json:"last_run,omitempty"`
}

type Observation struct {
	UID            string
	Kind           Kind
	Namespace      string
	Name           string
	ProjectID      *uuid.UUID
	Reason         *Reason
	Annotation     *string
	Service        *string
	Environment    *string
	Branch         *string
	Version        *string
	Images         []Image
	State          State
	PendingVersion *string
	PendingSince   *time.Time
	BranchActivity *time.Time
}

type Previous struct {
	Version        *string
	PendingVersion *string
	PendingSince   *time.Time
	State          State
}

type Observed struct {
	Cluster     string
	Namespace   string
	Kind        Kind
	Name        string
	Service     string
	Environment string
	Branch      *string
	Version     *string
	Images      []Image
	State       State
	ObservedAt  string
	Gone        bool
}

type Unmatched struct {
	Namespace  string
	Kind       Kind
	Name       string
	Reason     Reason
	Annotation *string
	ObservedAt string
}

type ClusterDeployment struct {
	ID          uuid.UUID
	ProjectID   uuid.UUID
	Service     string
	Environment string
	Version     string
	Branch      *string
	CommitSHA   *string
	Cluster     string
	Namespace   string
	OccurredAt  time.Time
}
