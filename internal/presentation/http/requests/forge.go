package requests

import "svc-registry/internal/feature/forge"

type Credentials struct {
	Token    *string `json:"token"`
	TokenRef *string `json:"token_ref"`
}

func (c *Credentials) input() *forge.CredentialsInput {
	if c == nil {
		return nil
	}
	return &forge.CredentialsInput{Token: c.Token, TokenRef: c.TokenRef}
}

type CreateConnection struct {
	Kind            *string      `json:"kind" validate:"required"`
	APIURL          *string      `json:"api_url"`
	OwnerPath       *string      `json:"owner_path" validate:"required"`
	MirrorSubgroups *bool        `json:"mirror_subgroups"`
	IncludeArchived *bool        `json:"include_archived"`
	IncludeForks    *bool        `json:"include_forks"`
	NameInclude     *[]string    `json:"name_include"`
	NameExclude     *[]string    `json:"name_exclude"`
	BranchInclude   *[]string    `json:"branch_include"`
	IntervalSecs    *int64       `json:"interval_secs"`
	Credentials     *Credentials `json:"credentials" validate:"required"`
}

func (r CreateConnection) Connection() forge.CreateConnection {
	return forge.CreateConnection{
		Kind: *r.Kind, APIURL: r.APIURL, OwnerPath: *r.OwnerPath, MirrorSubgroups: r.MirrorSubgroups,
		IncludeArchived: r.IncludeArchived, IncludeForks: r.IncludeForks, NameInclude: r.NameInclude,
		NameExclude: r.NameExclude, BranchInclude: r.BranchInclude, IntervalSecs: r.IntervalSecs, Credentials: r.Credentials.input(),
	}
}

type UpdateConnection struct {
	APIURL          *string      `json:"api_url"`
	OwnerPath       *string      `json:"owner_path"`
	MirrorSubgroups *bool        `json:"mirror_subgroups"`
	IncludeArchived *bool        `json:"include_archived"`
	IncludeForks    *bool        `json:"include_forks"`
	NameInclude     *[]string    `json:"name_include"`
	NameExclude     *[]string    `json:"name_exclude"`
	BranchInclude   *[]string    `json:"branch_include"`
	IntervalSecs    *int64       `json:"interval_secs"`
	Credentials     *Credentials `json:"credentials"`
}

func (r UpdateConnection) Changes() forge.UpdateConnection {
	return forge.UpdateConnection{
		APIURL: r.APIURL, OwnerPath: r.OwnerPath, MirrorSubgroups: r.MirrorSubgroups,
		IncludeArchived: r.IncludeArchived, IncludeForks: r.IncludeForks, NameInclude: r.NameInclude,
		NameExclude: r.NameExclude, BranchInclude: r.BranchInclude, IntervalSecs: r.IntervalSecs, Credentials: r.Credentials.input(),
	}
}

type Webhook struct {
	Mode *string `json:"mode" validate:"required"`
}
