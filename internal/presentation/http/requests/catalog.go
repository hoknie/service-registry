package requests

import (
	"encoding/json"

	"github.com/google/uuid"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
)

type Repo struct {
	Forge         *string `json:"forge"`
	RepoURL       *string `json:"repo_url"`
	DefaultBranch *string `json:"default_branch"`
}

func (r Repo) input() catalogtypes.RepoInput {
	return catalogtypes.RepoInput{Forge: r.Forge, RepoURL: r.RepoURL, DefaultBranch: r.DefaultBranch}
}

func labels(raw json.RawMessage) *catalogtypes.LabelsInput {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var l catalogtypes.Labels
	if err := json.Unmarshal(raw, &l); err != nil || l == nil {
		return &catalogtypes.LabelsInput{Err: catalogtypes.InvalidLabels}
	}
	return &catalogtypes.LabelsInput{Labels: l}
}

func flag(raw json.RawMessage, invalid error) *catalogtypes.FlagInput {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var v bool
	if err := json.Unmarshal(raw, &v); err != nil {
		return &catalogtypes.FlagInput{Err: invalid}
	}
	return &catalogtypes.FlagInput{Value: v}
}

type CreateNode struct {
	Kind        *string         `json:"kind" validate:"required"`
	ParentID    *uuid.UUID      `json:"parent_id"`
	Slug        *string         `json:"slug" validate:"required"`
	Name        *string         `json:"name" validate:"required"`
	Description *string         `json:"description"`
	Labels      json.RawMessage `json:"labels"`
	Repo
	ClusterObservation json.RawMessage  `json:"cluster_observation"`
	Source             *KnowledgeSource `json:"source"`
}

func (r CreateNode) SourceInput() *knowledgetypes.SourceInput {
	if r.Source == nil {
		return nil
	}
	in := r.Source.Input()
	return &in
}

func (r CreateNode) Node() catalogtypes.CreateNode {
	return catalogtypes.CreateNode{
		Kind: *r.Kind, ParentID: r.ParentID, Slug: *r.Slug, Name: *r.Name,
		Description: r.Description, Labels: labels(r.Labels), Repo: r.input(),
		ClusterObservation: flag(r.ClusterObservation, catalogtypes.InvalidClusterObservation),
	}
}

type UpdateNode struct {
	Slug        *string         `json:"slug"`
	Name        *string         `json:"name"`
	Description *string         `json:"description"`
	Labels      json.RawMessage `json:"labels"`
	Repo
	ClusterObservation json.RawMessage `json:"cluster_observation"`
}

func (r UpdateNode) Changes() catalogtypes.UpdateNode {
	return catalogtypes.UpdateNode{Slug: r.Slug, Name: r.Name, Description: r.Description, Labels: labels(r.Labels), Repo: r.input(),
		ClusterObservation: flag(r.ClusterObservation, catalogtypes.InvalidClusterObservation)}
}

type MoveNode struct {
	ParentID Optional[string] `json:"parent_id"`
}

type GrantRole struct {
	SubjectKind *string `json:"subject_kind" validate:"required"`
	Subject     *string `json:"subject" validate:"required"`
	Role        *string `json:"role" validate:"required"`
}

func (r GrantRole) Grant() catalogtypes.GrantRole {
	return catalogtypes.GrantRole{SubjectKind: *r.SubjectKind, Subject: *r.Subject, Role: *r.Role}
}

type RotateKey struct {
	GraceSecs *int64 `json:"grace_secs"`
}
