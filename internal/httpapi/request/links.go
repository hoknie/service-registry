package request

import (
	"github.com/google/uuid"

	"svc-registry/internal/links"
)

type PutLinkTemplate struct {
	KindKey  *string `json:"kind_key" validate:"required"`
	Template *string `json:"template"`
	Disabled *bool   `json:"disabled"`
	Position *int64  `json:"position"`
}

func (r PutLinkTemplate) Put() links.PutTemplate {
	return links.PutTemplate{KindKey: *r.KindKey, Template: r.Template, Disabled: r.Disabled, Position: r.Position}
}

type PreviewLinkTemplate struct {
	Template    *string    `json:"template" validate:"required"`
	ProjectID   *uuid.UUID `json:"project_id"`
	Branch      *string    `json:"branch"`
	Environment *string    `json:"environment"`
}

func (r PreviewLinkTemplate) Preview() links.Preview {
	return links.Preview{Template: *r.Template, ProjectID: r.ProjectID, Branch: r.Branch, Environment: r.Environment}
}

type PutNodeVars struct {
	Vars *map[string]string `json:"vars" validate:"required"`
}

type Links struct {
	Branch      string `query:"branch"`
	Environment string `query:"environment"`
}

func (q Links) Query() links.LinkQuery {
	return links.LinkQuery{Branch: q.Branch, Environment: q.Environment}
}
