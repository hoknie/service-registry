package catalog

import "github.com/google/uuid"

type LabelsInput struct {
	Labels Labels
	Err    error
}

type FlagInput struct {
	Value bool
	Err   error
}

type CreateNode struct {
	Kind               string
	ParentID           *uuid.UUID
	Slug               string
	Name               string
	Description        *string
	Labels             *LabelsInput
	Repo               RepoInput
	ClusterObservation *FlagInput
}

type UpdateNode struct {
	Slug               *string
	Name               *string
	Description        *string
	Labels             *LabelsInput
	Repo               RepoInput
	ClusterObservation *FlagInput
}

type RepoInput struct {
	Forge         *string
	RepoURL       *string
	DefaultBranch *string
}

func (r RepoInput) IsEmpty() bool {
	return r.Forge == nil && r.RepoURL == nil && r.DefaultBranch == nil
}

type NewNode struct {
	ID                 uuid.UUID
	Kind               NodeKind
	ParentID           *uuid.UUID
	Slug               string
	Name               string
	Description        string
	Labels             Labels
	Repo               Repo
	ClusterObservation *bool
}

type Change[T any] struct {
	Set   bool
	Value *T
}

type NodeChanges struct {
	Slug               *string
	Name               *string
	Description        *string
	Labels             Labels
	Forge              Change[Forge]
	RepoURL            Change[string]
	DefaultBranch      Change[string]
	ClusterObservation *bool
}

type Walk struct {
	UserID       uuid.UUID
	Root         *uuid.UUID
	RootReadable bool
	Depth        uint32
	Limit        uint32
	Offset       uint64
}

type GrantRole struct {
	SubjectKind string
	Subject     string
	Role        string
}

type NewBinding struct {
	ID          uuid.UUID
	NodeID      uuid.UUID
	SubjectKind SubjectKind
	Subject     string
	Role        Role
}

type NewKey struct {
	ID        uuid.UUID
	ProjectID uuid.UUID
	Prefix    string
	Hash      [32]byte
}
