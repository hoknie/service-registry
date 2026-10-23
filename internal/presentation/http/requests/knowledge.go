package requests

import knowledgetypes "svc-registry/internal/feature/knowledge/types"

type KnowledgeSettings struct {
	Include  Optional[[]string] `json:"include"`
	Exclude  Optional[[]string] `json:"exclude"`
	Branches Optional[[]string] `json:"branches"`
	APIs     Optional[bool]     `json:"apis"`
	Refresh  Optional[string]   `json:"refresh"`
}

func (r KnowledgeSettings) Settings() (knowledgetypes.NodeSettings, error) {
	out := knowledgetypes.NodeSettings{IncludeSet: r.Include.Present}
	if r.Include.Value != nil {
		out.Include = *r.Include.Value
	}
	for _, f := range []struct {
		in  Optional[[]string]
		out **[]string
	}{{r.Exclude, &out.Exclude}, {r.Branches, &out.Branches}} {
		if !f.in.Present {
			continue
		}
		if f.in.Value == nil {
			return knowledgetypes.NodeSettings{}, knowledgetypes.InvalidSettings
		}
		*f.out = f.in.Value
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
	return out, nil
}

type SourceCredentials struct {
	SecretID  *string `json:"secret_id"`
	Token     *string `json:"token"`
	Reference *string `json:"reference"`
}

type KnowledgeSource struct {
	Kind           *string                     `json:"kind" validate:"required"`
	Forge          string                      `json:"forge"`
	URL            string                      `json:"url"`
	APIURL         string                      `json:"api_url"`
	Path           string                      `json:"path"`
	Credentials    Optional[SourceCredentials] `json:"credentials"`
	WorkingTree    *bool                       `json:"working_tree"`
	IncludeIgnored *bool                       `json:"include_ignored"`
}

func (r KnowledgeSource) Input() knowledgetypes.SourceInput {
	in := knowledgetypes.SourceInput{Kind: *r.Kind, Forge: r.Forge, URL: r.URL, APIURL: r.APIURL, Path: r.Path,
		HasCredential: r.Credentials.Present, WorkingTree: r.WorkingTree, IncludeIgnored: r.IncludeIgnored}
	if r.Credentials.Present && r.Credentials.Value == nil {
		in.NoCredentials = true
	}
	if c := r.Credentials.Value; c != nil {
		in.SecretID, in.Inline = c.SecretID, c.Token != nil || c.Reference != nil
	}
	return in
}
