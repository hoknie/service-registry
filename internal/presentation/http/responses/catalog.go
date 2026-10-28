package responses

import (
	"github.com/google/uuid"

	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	composeservices "svc-registry/internal/feature/compose/services"
)

type Node struct {
	ID                 uuid.UUID            `json:"id"`
	Kind               string               `json:"kind"`
	ParentID           *uuid.UUID           `json:"parent_id"`
	Slug               string               `json:"slug"`
	Name               string               `json:"name"`
	Access             string               `json:"access"`
	Description        *string              `json:"description,omitempty"`
	Labels             *catalogtypes.Labels `json:"labels,omitempty"`
	Hosting            **string             `json:"hosting,omitempty"`
	RepoURL            **string             `json:"repo_url,omitempty"`
	DefaultBranch      **string             `json:"default_branch,omitempty"`
	ClusterObservation *bool                `json:"cluster_observation,omitempty"`
	Visibility         *string              `json:"visibility,omitempty"`
	CreatedAt          *string              `json:"created_at,omitempty"`
	UpdatedAt          *string              `json:"updated_at,omitempty"`
	Depth              *int32               `json:"depth,omitempty"`
	Permissions        *[]string            `json:"permissions,omitempty"`
	Path               *[]Node              `json:"path,omitempty"`
	Key                *Key                 `json:"key,omitempty"`
	Managed            *bool                `json:"managed,omitempty"`
	Repository         **Repository         `json:"repository,omitempty"`
}

func NodeOf(n catalogtypes.Node, readable bool) Node {
	body := Node{ID: n.ID, Kind: string(n.Kind), ParentID: n.ParentID, Slug: n.Slug, Name: n.Name, Access: "navigate"}
	if !readable {
		return body
	}
	body.Access = "read"
	description, labels, created, updated := n.Description, n.Labels, n.CreatedAt, n.UpdatedAt
	if labels == nil {
		labels = catalogtypes.Labels{}
	}
	body.Description, body.Labels, body.CreatedAt, body.UpdatedAt = &description, &labels, &created, &updated
	body.Managed = new(n.Managed)
	if n.ParentID == nil && n.Visibility != "" {
		body.Visibility = new(string(n.Visibility))
	}
	if n.Kind == catalogtypes.KindProject {
		var hosting *string
		if n.Repo.Hosting != nil {
			hosting = new(string(*n.Repo.Hosting))
		}
		url, branch := n.Repo.RepoURL, n.Repo.DefaultBranch
		body.Hosting, body.RepoURL, body.DefaultBranch = &hosting, &url, &branch
		body.ClusterObservation = new(n.ClusterObservation)
	}
	return body
}

func ReadNode(n catalogtypes.Node) Node { return NodeOf(n, true) }

func WalkedNode(w catalogtypes.WalkNode) Node {
	body := NodeOf(w.Node, w.Readable)
	body.Depth = new(w.Depth)
	return body
}

func ScopedNode(s catalogservices.Scoped) Node {
	body := NodeOf(s.Node, s.Role != nil)
	permissions := s.Permissions()
	path := make([]Node, 0, len(s.Path))
	for _, p := range s.Path {
		stub := NodeOf(p.Node, false)
		if p.Readable {
			stub.Access = "read"
		}
		path = append(path, stub)
	}
	body.Permissions, body.Path = &permissions, &path
	return body
}

func ProjectCardOf(card composeservices.ProjectCard) Node {
	body := ScopedNode(card.Scoped)
	if card.HasRepository {
		body.Repository = new(RepositoryOf(card.Repository))
	}
	return body
}

func CreatedNode(n catalogtypes.Node, key *catalogservices.IssuedKey) Node {
	body := ReadNode(n)
	if key != nil {
		body.Key = new(IssuedKeyOf(*key))
	}
	return body
}

type Tree struct {
	Nodes     []Node `json:"nodes"`
	Truncated bool   `json:"truncated"`
}

type Items[T any] struct {
	Items []T `json:"items"`
}

func ItemsOf[D, T any](items []D, conv func(D) T) Items[T] {
	out := make([]T, 0, len(items))
	for _, d := range items {
		out = append(out, conv(d))
	}
	return Items[T]{Items: out}
}

type Binding struct {
	ID          uuid.UUID `json:"id"`
	NodeID      uuid.UUID `json:"node_id"`
	NodeName    string    `json:"node_name"`
	SubjectKind string    `json:"subject_kind"`
	SubjectID   uuid.UUID `json:"subject_id"`
	SubjectName string    `json:"subject_name"`
	Role        string    `json:"role"`
	Inherited   bool      `json:"inherited"`
	CreatedAt   string    `json:"created_at"`
}

func BindingOf(b catalogtypes.Binding, scope uuid.UUID) Binding {
	return Binding{
		ID: b.ID, NodeID: b.NodeID, NodeName: b.NodeName, SubjectKind: string(b.SubjectKind),
		SubjectID: b.SubjectID, SubjectName: b.SubjectName, Role: b.Role.String(),
		Inherited: b.NodeID != scope, CreatedAt: b.CreatedAt,
	}
}

type Key struct {
	ID         uuid.UUID `json:"id"`
	Prefix     string    `json:"prefix"`
	Status     string    `json:"status"`
	CreatedAt  string    `json:"created_at"`
	LastUsedAt *string   `json:"last_used_at"`
	ExpiresAt  *string   `json:"expires_at"`
	RevokedAt  *string   `json:"revoked_at"`
	Secret     *string   `json:"secret,omitempty"`
}

func KeyOf(k catalogtypes.ProjectKey) Key {
	return Key{
		ID: k.ID, Prefix: k.Prefix, Status: string(k.Status), CreatedAt: k.CreatedAt,
		LastUsedAt: k.LastUsedAt, ExpiresAt: k.ExpiresAt, RevokedAt: k.RevokedAt,
	}
}

func IssuedKeyOf(i catalogservices.IssuedKey) Key {
	k := KeyOf(i.Key)
	k.Secret = new(i.Secret)
	return k
}
