package types

import "github.com/google/uuid"

type Labels map[string]string

type NodeKind string

const (
	KindOrganization NodeKind = "organization"
	KindFolder       NodeKind = "folder"
	KindProject      NodeKind = "project"
)

func ParseKind(s string) (NodeKind, bool) {
	switch NodeKind(s) {
	case KindOrganization, KindFolder, KindProject:
		return NodeKind(s), true
	}
	return "", false
}

func (k NodeKind) MayBeUnder(parent *NodeKind) bool {
	if parent == nil {
		return k == KindOrganization
	}
	switch *parent {
	case KindOrganization:
		return true
	case KindFolder:
		return k != KindOrganization
	}
	return false
}

type OrgVisibility string

const (
	OrgOpen   OrgVisibility = "open"
	OrgClosed OrgVisibility = "closed"
)

func ParseOrgVisibility(s *string) (OrgVisibility, error) {
	if s != nil {
		switch v := OrgVisibility(*s); v {
		case OrgOpen, OrgClosed:
			return v, nil
		}
	}
	return "", InvalidVisibility
}

type Forge string

const (
	ForgeGithub  Forge = "github"
	ForgeGitlab  Forge = "gitlab"
	ForgeForgejo Forge = "forgejo"
	ForgeGitea   Forge = "gitea"
)

func ParseForge(s string) (Forge, bool) {
	switch Forge(s) {
	case ForgeGithub, ForgeGitlab, ForgeForgejo, ForgeGitea:
		return Forge(s), true
	}
	return "", false
}

type Repo struct {
	Forge         *Forge
	RepoURL       *string
	DefaultBranch *string
}

type Node struct {
	ID                 uuid.UUID
	Kind               NodeKind
	ParentID           *uuid.UUID
	Slug               string
	Name               string
	Description        string
	Labels             Labels
	Repo               Repo
	ClusterObservation bool
	Visibility         OrgVisibility
	CreatedAt          string
	UpdatedAt          string
	Managed            bool
}

type ChainNode struct {
	Node  Node
	Bound *Role
}

type NodeChain struct {
	Nodes     []ChainNode
	Navigable bool
}

type WalkNode struct {
	Node     Node
	Depth    int32
	Readable bool
}
