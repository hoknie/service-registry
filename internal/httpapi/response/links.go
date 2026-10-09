package response

import (
	"github.com/google/uuid"

	"svc-registry/internal/links"
)

type LinkKind struct {
	Key       string            `json:"key"`
	Names     map[string]string `json:"names"`
	Icon      string            `json:"icon"`
	Position  int32             `json:"position"`
	CreatedAt string            `json:"created_at"`
	UpdatedAt string            `json:"updated_at"`
}

func LinkKindOf(k links.Kind) LinkKind {
	return LinkKind{Key: k.Key, Names: k.Names, Icon: string(k.Icon), Position: k.Position, CreatedAt: k.CreatedAt, UpdatedAt: k.UpdatedAt}
}

type LinkTemplate struct {
	LinkKey   string    `json:"link_key"`
	KindKey   string    `json:"kind_key"`
	Template  *string   `json:"template"`
	Disabled  bool      `json:"disabled"`
	Position  int32     `json:"position"`
	NodeID    uuid.UUID `json:"node_id"`
	Inherited bool      `json:"inherited"`
	CreatedAt string    `json:"created_at"`
	UpdatedAt string    `json:"updated_at"`
}

func LinkTemplateOf(t links.Template) LinkTemplate {
	return LinkTemplate{LinkKey: t.LinkKey, KindKey: t.KindKey, Template: t.Template, Disabled: t.Disabled,
		Position: t.Position, NodeID: t.NodeID, Inherited: t.Inherited, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt}
}

type NodeVar struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	NodeID    uuid.UUID `json:"node_id"`
	Inherited bool      `json:"inherited"`
}

func NodeVarOf(v links.Var) NodeVar {
	return NodeVar{Key: v.Key, Value: v.Value, NodeID: v.NodeID, Inherited: v.Inherited}
}

type LinkCheck struct {
	Status     string `json:"status"`
	HTTPStatus *int   `json:"http_status"`
	DurationMS int    `json:"duration_ms"`
	CheckedAt  string `json:"checked_at"`
}

type Link struct {
	LinkKey     string     `json:"link_key"`
	KindKey     string     `json:"kind_key"`
	NodeID      uuid.UUID  `json:"node_id"`
	Inherited   bool       `json:"inherited"`
	Service     *string    `json:"service"`
	Environment *string    `json:"environment"`
	URL         *string    `json:"url"`
	Missing     []string   `json:"missing"`
	Check       *LinkCheck `json:"check"`
}

func LinkOf(l links.Link) Link {
	out := Link{LinkKey: l.LinkKey, KindKey: l.KindKey, NodeID: l.NodeID, Inherited: l.Inherited, Service: l.Service,
		Environment: l.Environment, URL: l.URL, Missing: l.Missing}
	if out.Missing == nil {
		out.Missing = []string{}
	}
	if l.Check != nil {
		out.Check = &LinkCheck{Status: string(l.Check.Status), HTTPStatus: l.Check.HTTPStatus, DurationMS: l.Check.DurationMS,
			CheckedAt: l.Check.CheckedAt}
	}
	return out
}

type LinkCheckEntry struct {
	URL        string `json:"url"`
	Status     string `json:"status"`
	HTTPStatus *int   `json:"http_status"`
	DurationMS int    `json:"duration_ms"`
	CheckedAt  string `json:"checked_at"`
}

func LinkCheckEntryOf(c links.Check) LinkCheckEntry {
	return LinkCheckEntry{URL: c.URL, Status: string(c.Status), HTTPStatus: c.HTTPStatus, DurationMS: c.DurationMS, CheckedAt: c.CheckedAt}
}
