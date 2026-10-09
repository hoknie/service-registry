package request

import "svc-registry/internal/knowledge"

type KnowledgeSettings struct {
	Include  Optional[[]string] `json:"include"`
	Exclude  Optional[[]string] `json:"exclude"`
	Branches Optional[[]string] `json:"branches"`
}

func (r KnowledgeSettings) Settings() (knowledge.NodeSettings, error) {
	out := knowledge.NodeSettings{IncludeSet: r.Include.Present}
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
			return knowledge.NodeSettings{}, knowledge.InvalidSettings
		}
		*f.out = f.in.Value
	}
	return out, nil
}

type SourceCredentials struct {
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

func (r KnowledgeSource) Input() knowledge.SourceInput {
	in := knowledge.SourceInput{Kind: *r.Kind, Forge: r.Forge, URL: r.URL, APIURL: r.APIURL, Path: r.Path,
		HasCredential: r.Credentials.Present, WorkingTree: r.WorkingTree, IncludeIgnored: r.IncludeIgnored}
	if r.Credentials.Present && r.Credentials.Value == nil {
		in.NoCredentials = true
	}
	if c := r.Credentials.Value; c != nil {
		in.Token, in.Reference = c.Token, c.Reference
		if c.Token == nil && c.Reference == nil {
			in.NoCredentials = false
		}
	}
	return in
}
