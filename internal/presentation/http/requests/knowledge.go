package requests

import (
	"encoding/json"

	knowledgetypes "svc-registry/internal/feature/knowledge/types"
)

type KnowledgeSettings struct {
	Include       Optional[[]string]     `json:"include"`
	Exclude       Optional[[]string]     `json:"exclude"`
	Refs          Optional[[]RefPattern] `json:"refs"`
	Branches      json.RawMessage        `json:"branches"`
	APIs          Optional[bool]         `json:"apis"`
	APIPaths      Optional[[]string]     `json:"api_paths"`
	Refresh       Optional[string]       `json:"refresh"`
	RefreshEvents Optional[[]string]     `json:"refresh_events"`
}

func (r KnowledgeSettings) Settings() (knowledgetypes.NodeSettings, error) {
	if r.Branches != nil {
		return knowledgetypes.NodeSettings{}, knowledgetypes.BranchesReplaced
	}
	out := knowledgetypes.NodeSettings{IncludeSet: r.Include.Present}
	if r.Include.Value != nil {
		out.Include = *r.Include.Value
	}
	for _, f := range []struct {
		in  Optional[[]string]
		out **[]string
	}{{r.Exclude, &out.Exclude}, {r.APIPaths, &out.APIPaths}} {
		if !f.in.Present {
			continue
		}
		if f.in.Value == nil {
			return knowledgetypes.NodeSettings{}, knowledgetypes.InvalidSettings
		}
		*f.out = f.in.Value
	}
	if r.Refs.Present {
		if r.Refs.Value == nil {
			return knowledgetypes.NodeSettings{}, knowledgetypes.InvalidSettings
		}
		refs := make([]knowledgetypes.RefPattern, len(*r.Refs.Value))
		for i, ref := range *r.Refs.Value {
			if ref.Pattern == nil || ref.Kind == nil {
				return knowledgetypes.NodeSettings{}, knowledgetypes.InvalidSettings
			}
			refs[i] = knowledgetypes.RefPattern{Pattern: *ref.Pattern, Kind: knowledgetypes.RefKind(*ref.Kind), Release: ref.Release}
		}
		out.Refs = &refs
	}
	if r.APIs.Present {
		if r.APIs.Value == nil {
			return knowledgetypes.NodeSettings{}, knowledgetypes.InvalidSettings
		}
		out.APIs = r.APIs.Value
	}
	if r.Refresh.Present {
		if r.Refresh.Value == nil {
			return knowledgetypes.NodeSettings{}, knowledgetypes.InvalidSettings
		}
		out.Refresh = new(knowledgetypes.Refresh(*r.Refresh.Value))
	}
	if r.RefreshEvents.Present {
		if r.RefreshEvents.Value == nil {
			return knowledgetypes.NodeSettings{}, knowledgetypes.InvalidSettings
		}
		events := make([]knowledgetypes.CollectEvent, len(*r.RefreshEvents.Value))
		for i, e := range *r.RefreshEvents.Value {
			events[i] = knowledgetypes.CollectEvent(e)
		}
		out.RefreshEvents = &events
	}
	return out, nil
}

type RefPattern struct {
	Pattern *string `json:"pattern"`
	Kind    *string `json:"kind"`
	Release bool    `json:"release"`
}

type SourceCredentials struct {
	SecretID  *string `json:"secret_id"`
	Token     *string `json:"token"`
	Reference *string `json:"reference"`
}

type KnowledgeSource struct {
	Kind           *string                     `json:"kind" validate:"required"`
	Hosting        string                      `json:"hosting"`
	URL            string                      `json:"url"`
	APIURL         string                      `json:"api_url"`
	Path           string                      `json:"path"`
	Credentials    Optional[SourceCredentials] `json:"credentials"`
	WorkingTree    *bool                       `json:"working_tree"`
	IncludeIgnored *bool                       `json:"include_ignored"`
}

func (r KnowledgeSource) Input() knowledgetypes.SourceInput {
	in := knowledgetypes.SourceInput{Kind: *r.Kind, Hosting: r.Hosting, URL: r.URL, APIURL: r.APIURL, Path: r.Path,
		HasCredential: r.Credentials.Present, WorkingTree: r.WorkingTree, IncludeIgnored: r.IncludeIgnored}
	if r.Credentials.Present && r.Credentials.Value == nil {
		in.NoCredentials = true
	}
	if c := r.Credentials.Value; c != nil {
		in.SecretID, in.Inline = c.SecretID, c.Token != nil || c.Reference != nil
	}
	return in
}
