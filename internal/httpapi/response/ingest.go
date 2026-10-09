package response

import (
	"encoding/json"

	"github.com/google/uuid"

	"svc-registry/internal/deploy"
	"svc-registry/internal/ingest"
	"svc-registry/internal/service"
)

type Accepted struct {
	ID             uuid.UUID `json:"id"`
	Type           string    `json:"type"`
	Version        int32     `json:"version"`
	OccurredAt     string    `json:"occurred_at"`
	ReceivedAt     string    `json:"received_at"`
	IdempotencyKey string    `json:"idempotency_key"`
	Replayed       bool      `json:"replayed"`
	Result         Result    `json:"result"`
}

type Result struct {
	Deployment *DeploymentResult `json:"deployment,omitempty"`
}

type DeploymentResult struct {
	ID            uuid.UUID `json:"id"`
	Service       string    `json:"service"`
	Environment   string    `json:"environment"`
	Version       string    `json:"version"`
	BecameCurrent bool      `json:"became_current"`
}

func AcceptedOf(a ingest.Accepted) Accepted {
	e := a.Event
	out := Accepted{
		ID: e.ID, Type: e.Type, Version: e.Version, OccurredAt: e.OccurredAt, ReceivedAt: e.ReceivedAt,
		IdempotencyKey: e.IdempotencyKey, Replayed: a.Replayed,
	}
	if d := a.Deployment; d != nil {
		out.Result.Deployment = &DeploymentResult{
			ID: d.ID, Service: d.Service, Environment: d.Environment, Version: d.Version, BecameCurrent: d.BecameCurrent,
		}
	}
	return out
}

type Event struct {
	ID             uuid.UUID       `json:"id"`
	Type           string          `json:"type"`
	Version        int32           `json:"version"`
	OccurredAt     string          `json:"occurred_at"`
	ReceivedAt     string          `json:"received_at"`
	IdempotencyKey string          `json:"idempotency_key"`
	KeyPrefix      *string         `json:"key_prefix"`
	Payload        json.RawMessage `json:"payload"`
}

func EventOf(e ingest.Event) Event {
	return Event{
		ID: e.ID, Type: e.Type, Version: e.Version, OccurredAt: e.OccurredAt, ReceivedAt: e.ReceivedAt,
		IdempotencyKey: e.IdempotencyKey, KeyPrefix: e.KeyPrefix, Payload: e.Payload,
	}
}

type Deployment struct {
	ID          uuid.UUID         `json:"id"`
	Service     string            `json:"service"`
	Environment string            `json:"environment"`
	Version     string            `json:"version"`
	CommitSHA   *string           `json:"commit_sha"`
	Branch      *string           `json:"branch"`
	Cluster     *string           `json:"cluster"`
	Namespace   *string           `json:"namespace"`
	URL         *string           `json:"url"`
	DeployedBy  *string           `json:"deployed_by"`
	Metadata    map[string]string `json:"metadata"`
	OccurredAt  string            `json:"occurred_at"`
	ReceivedAt  string            `json:"received_at"`
	EventID     *uuid.UUID        `json:"event_id"`
	Source      string            `json:"source"`
	Current     bool              `json:"current"`
}

func DeploymentOf(d ingest.Deployment) Deployment {
	metadata := d.Metadata
	if metadata == nil {
		metadata = map[string]string{}
	}
	return Deployment{
		ID: d.ID, Service: d.Service, Environment: d.Environment, Version: d.Version,
		CommitSHA: d.CommitSHA, Branch: d.Branch, Cluster: d.Cluster, Namespace: d.Namespace,
		URL: d.URL, DeployedBy: d.DeployedBy, Metadata: metadata, OccurredAt: d.OccurredAt,
		ReceivedAt: d.ReceivedAt, EventID: d.EventID, Current: d.Current, Source: d.Source,
	}
}

type Observed struct {
	Cluster    string           `json:"cluster"`
	Namespace  string           `json:"namespace"`
	Kind       string           `json:"kind"`
	Workload   string           `json:"workload"`
	Branch     *string          `json:"branch"`
	Version    *string          `json:"version"`
	Images     []deploy.Image   `json:"images"`
	Replicas   *deploy.Replicas `json:"replicas,omitempty"`
	Restarts   *int32           `json:"restarts,omitempty"`
	Rollout    *deploy.Rollout  `json:"rollout,omitempty"`
	Schedule   *string          `json:"schedule,omitempty"`
	Suspended  *bool            `json:"suspended,omitempty"`
	LastRun    **deploy.LastRun `json:"last_run,omitempty"`
	ObservedAt string           `json:"observed_at"`
	Gone       bool             `json:"gone"`
}

func ObservedOf(o deploy.Observed) Observed {
	images := o.Images
	if images == nil {
		images = []deploy.Image{}
	}
	out := Observed{Cluster: o.Cluster, Namespace: o.Namespace, Kind: string(o.Kind), Workload: o.Name, Branch: o.Branch,
		Version: o.Version, Images: images, Replicas: o.State.Replicas, Restarts: o.State.Restarts, Rollout: o.State.Rollout,
		Schedule: o.State.Schedule, Suspended: o.State.Suspended, ObservedAt: o.ObservedAt, Gone: o.Gone}
	if o.Kind == deploy.KindCronJob {
		last := o.State.LastRun
		out.LastRun = &last
	}
	return out
}

type Environment struct {
	Service      string     `json:"service"`
	Environment  string     `json:"environment"`
	Version      *string    `json:"version"`
	CommitSHA    *string    `json:"commit_sha"`
	Branch       *string    `json:"branch"`
	URL          *string    `json:"url"`
	DeployedBy   *string    `json:"deployed_by"`
	OccurredAt   *string    `json:"occurred_at"`
	Source       *string    `json:"source"`
	DeploymentID *uuid.UUID `json:"deployment_id"`
	UpdatedAt    *string    `json:"updated_at"`
	Observed     []Observed `json:"observed"`
	Drift        bool       `json:"drift"`
}

func EnvironmentOf(e service.EnvironmentState) Environment {
	out := Environment{Service: e.Service, Environment: e.Environment, Drift: e.Drift, Observed: make([]Observed, 0, len(e.Observed))}
	for _, o := range e.Observed {
		out.Observed = append(out.Observed, ObservedOf(o))
	}
	if c := e.Current; c != nil {
		version, occurred, source, id, updated := c.Version, c.OccurredAt, c.Source, c.DeploymentID, c.UpdatedAt
		out.Version, out.CommitSHA, out.Branch, out.URL, out.DeployedBy = &version, c.CommitSHA, c.Branch, c.URL, c.DeployedBy
		out.OccurredAt, out.Source, out.DeploymentID, out.UpdatedAt = &occurred, &source, &id, &updated
	}
	return out
}
