package requests

import (
	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/feature/catalog"
)

type Page struct {
	Limit  *int64 `query:"limit"`
	Offset *int64 `query:"offset"`
}

func (p Page) Query() access.PageQuery { return access.PageQuery{Limit: p.Limit, Offset: p.Offset} }

type Children struct {
	Page
	Parent *uuid.UUID `query:"parent"`
}

type Tree struct {
	Root  *uuid.UUID `query:"root"`
	Depth *int64     `query:"depth"`
}

type Events struct {
	Page
	Type string `query:"type"`
}

type Deployments struct {
	Page
	Service     *string `query:"service"`
	Environment *string `query:"environment"`
	Branch      *string `query:"branch"`
}

type CatalogTable struct {
	Page
	Parent   *uuid.UUID `query:"parent"`
	Q        *string    `query:"q"`
	Kind     *string    `query:"kind"`
	Label    []string   `query:"label"`
	Activity *string    `query:"activity"`
}

func (q CatalogTable) Filter() catalog.TableQuery {
	return catalog.TableQuery{Q: q.Q, Kind: q.Kind, Labels: q.Label, Activity: q.Activity}
}
