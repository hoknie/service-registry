package response

import (
	"github.com/google/uuid"

	"svc-registry/internal/catalog"
	"svc-registry/internal/service"
)

type Node struct {
	ID                 uuid.UUID       `json:"id"`
	Kind               string          `json:"kind"`
	ParentID           *uuid.UUID      `json:"parent_id"`
	Slug               string          `json:"slug"`
	Name               string          `json:"name"`
	Access             string          `json:"access"`
	Description        *string         `json:"description,omitempty"`
	Labels             *catalog.Labels `json:"labels,omitempty"`
	Forge              **string        `json:"forge,omitempty"`
	RepoURL            **string        `json:"repo_url,omitempty"`
	DefaultBranch      **string        `json:"default_branch,omitempty"`
	ClusterObservation *bool           `json:"cluster_observation,omitempty"`
	CreatedAt          *string         `json:"created_at,omitempty"`
	UpdatedAt          *string         `json:"updated_at,omitempty"`
	Depth              *int32          `json:"depth,omitempty"`
	Permissions        *[]string       `json:"permissions,omitempty"`
	Path               *[]Node         `json:"path,omitempty"`
	Key                *Key            `json:"key,omitempty"`
	Managed            *bool           `json:"managed,omitempty"`
	Repository         **Repository    `json:"repository,omitempty"`
}

func (n Node) WithManaged(managed map[uuid.UUID]bool) Node {
	if n.Access == "read" {
		m := managed[n.ID]
		n.Managed = &m
	}
	return n
}

func NodeOf(n catalog.Node, readable bool) Node {
	body := Node{ID: n.ID, Kind: string(n.Kind), ParentID: n.ParentID, Slug: n.Slug, Name: n.Name, Access: "navigate"}
	if !readable {
		return body
	}
	body.Access = "read"
	description, labels, created, updated := n.Description, n.Labels, n.CreatedAt, n.UpdatedAt
	if labels == nil {
		labels = catalog.Labels{}
	}
	body.Description, body.Labels, body.CreatedAt, body.UpdatedAt = &description, &labels, &created, &updated
	if n.Kind == catalog.KindProject {
		var forge *string
		if n.Repo.Forge != nil {
			f := string(*n.Repo.Forge)
			forge = &f
		}
		url, branch := n.Repo.RepoURL, n.Repo.DefaultBranch
		body.Forge, body.RepoURL, body.DefaultBranch = &forge, &url, &branch
		observe := n.ClusterObservation
		body.ClusterObservation = &observe
	}
	return body
}

func ReadNode(n catalog.Node) Node { return NodeOf(n, true) }

func WalkedNode(w catalog.WalkNode) Node {
	body := NodeOf(w.Node, w.Readable)
	depth := w.Depth
	body.Depth = &depth
	return body
}

func ScopedNode(s service.Scoped) Node {
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

func CreatedNode(n catalog.Node, key *service.IssuedKey) Node {
	body := ReadNode(n)
	if key != nil {
		k := IssuedKeyOf(*key)
		body.Key = &k
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

func BindingOf(b catalog.Binding, scope uuid.UUID) Binding {
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

func KeyOf(k catalog.ProjectKey) Key {
	return Key{
		ID: k.ID, Prefix: k.Prefix, Status: string(k.Status), CreatedAt: k.CreatedAt,
		LastUsedAt: k.LastUsedAt, ExpiresAt: k.ExpiresAt, RevokedAt: k.RevokedAt,
	}
}

func IssuedKeyOf(i service.IssuedKey) Key {
	k := KeyOf(i.Key)
	secret := i.Secret
	k.Secret = &secret
	return k
}
