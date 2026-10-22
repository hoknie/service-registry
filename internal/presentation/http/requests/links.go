package requests

import (
	"bytes"
	"encoding/json"

	"github.com/google/uuid"

	linkstypes "svc-registry/internal/feature/links/types"
)

type PutLinkTemplate struct {
	KindKey  *string         `json:"kind_key" validate:"required"`
	Template *string         `json:"template"`
	Disabled *bool           `json:"disabled"`
	Position *int64          `json:"position"`
	Title    *string         `json:"title"`
	Icon     json.RawMessage `json:"icon"`
}

func (r PutLinkTemplate) Put() linkstypes.PutTemplate {
	return linkstypes.PutTemplate{KindKey: *r.KindKey, Template: r.Template, Disabled: r.Disabled, Position: r.Position,
		Title: r.Title, Icon: iconInput(r.Icon)}
}

func iconInput(raw json.RawMessage) *linkstypes.IconInput {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var in struct {
		URL  *string `json:"url"`
		File *string `json:"file"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		return &linkstypes.IconInput{Err: linkstypes.InvalidTemplateIcon}
	}
	return &linkstypes.IconInput{URL: in.URL, File: in.File}
}

type PreviewLinkTemplate struct {
	Template    *string    `json:"template" validate:"required"`
	ProjectID   *uuid.UUID `json:"project_id"`
	Branch      *string    `json:"branch"`
	Environment *string    `json:"environment"`
}

func (r PreviewLinkTemplate) Preview() linkstypes.Preview {
	return linkstypes.Preview{Template: *r.Template, ProjectID: r.ProjectID, Branch: r.Branch, Environment: r.Environment}
}

type PutNodeVars struct {
	Vars *map[string]string `json:"vars" validate:"required"`
}

type Links struct {
	Branch      string `query:"branch"`
	Environment string `query:"environment"`
}

func (q Links) Query() linkstypes.LinkQuery {
	return linkstypes.LinkQuery{Branch: q.Branch, Environment: q.Environment}
}
