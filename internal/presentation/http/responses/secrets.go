package responses

import (
	"github.com/google/uuid"

	"svc-registry/internal/feature/catalog"
)

type NodeRef struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type SecretRef struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	From *NodeRef  `json:"from"`
}

type Credentials struct {
	Kind        string     `json:"kind"`
	Secret      *SecretRef `json:"secret,omitempty"`
	Storage     *string    `json:"storage,omitempty"`
	Fingerprint *string    `json:"fingerprint,omitempty"`
	Ref         *string    `json:"ref,omitempty"`
}

func nodeRefOf(n *catalog.NodeRef) *NodeRef {
	if n == nil {
		return nil
	}
	return &NodeRef{ID: n.ID, Name: n.Name}
}

func SecretRefOf(r *catalog.SecretRef) *SecretRef {
	if r == nil {
		return nil
	}
	return &SecretRef{ID: r.ID, Name: r.Name, From: nodeRefOf(r.From)}
}

func CredentialsOf(c catalog.Credentials) Credentials {
	out := Credentials{Kind: string(c.Kind), Secret: SecretRefOf(c.Secret)}
	if c.Kind == catalog.CredentialsSecret && out.Secret == nil && c.SecretID != nil {
		out.Secret = &SecretRef{ID: *c.SecretID}
	}
	if c.Kind == catalog.CredentialsLegacy {
		st := string(c.Storage)
		out.Storage, out.Fingerprint, out.Ref = &st, c.Fingerprint, c.Ref
	}
	return out
}

type Secret struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	NodeID      *uuid.UUID `json:"node_id"`
	Storage     string     `json:"storage"`
	Fingerprint *string    `json:"fingerprint"`
	Ref         *string    `json:"ref"`
	UsedBy      int64      `json:"used_by"`
	From        *NodeRef   `json:"from"`
	Own         bool       `json:"own"`
	CreatedAt   string     `json:"created_at"`
	UpdatedAt   string     `json:"updated_at"`
}

func SecretOf(s catalog.Secret, here *uuid.UUID) Secret {
	own := s.NodeID == nil && here == nil || s.NodeID != nil && here != nil && *s.NodeID == *here
	return Secret{ID: s.ID, Name: s.Name, Description: s.Description, NodeID: s.NodeID, Storage: string(s.Storage),
		Fingerprint: s.Fingerprint, Ref: s.Ref, UsedBy: s.UsedBy, From: nodeRefOf(s.From), Own: own,
		CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt}
}

type Secrets struct {
	Items []Secret `json:"items"`
}

func SecretsOf(items []catalog.Secret, here *uuid.UUID) Secrets {
	out := Secrets{Items: make([]Secret, 0, len(items))}
	for _, s := range items {
		out.Items = append(out.Items, SecretOf(s, here))
	}
	return out
}

type LabelKey struct {
	Key   string `json:"key"`
	Count int64  `json:"count"`
}

type LabelValue struct {
	Value string `json:"value"`
	Count int64  `json:"count"`
}

func LabelSuggestionsOf(items []catalog.LabelSuggestion, values bool) any {
	if values {
		out := make([]LabelValue, 0, len(items))
		for _, it := range items {
			out = append(out, LabelValue{Value: it.Value, Count: it.Count})
		}
		return struct {
			Items []LabelValue `json:"items"`
		}{out}
	}
	out := make([]LabelKey, 0, len(items))
	for _, it := range items {
		out = append(out, LabelKey{Key: it.Value, Count: it.Count})
	}
	return struct {
		Items []LabelKey `json:"items"`
	}{out}
}
