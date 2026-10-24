package types

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
	Links         []DeploymentLink
	OccurredAt    any
}

func (d DeploymentRecord) Stored() DeploymentRecord {
	if d.Metadata == nil {
		d.Metadata = map[string]string{}
	}
	if d.Links == nil {
		d.Links = []DeploymentLink{}
	}
	return d
}

func (d DeploymentRecord) Result(current bool) DeploymentResult {
	return DeploymentResult{ID: d.ID, Service: d.Service, Environment: d.Environment, Version: d.Version, BecameCurrent: current}
}

func (e NewEvent) Deployment(p *ServiceDeployed) DeploymentRecord {
	return DeploymentRecord{
		ID: e.DeploymentID, EnvironmentID: e.EnvironmentID, ProjectID: e.ProjectID, EventID: new(e.ID), Source: "event",
		Service: p.Service, Environment: p.Environment, Version: p.Version, CommitSHA: p.CommitSHA, Branch: p.Branch,
		Cluster: p.Cluster, Namespace: p.Namespace, URL: p.URL, DeployedBy: p.DeployedBy, Metadata: p.Metadata, Links: p.Links,
		OccurredAt: e.Envelope.OccurredAt,
	}
}
