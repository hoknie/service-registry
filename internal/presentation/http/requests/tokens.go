package requests

import accesstypes "svc-registry/internal/feature/access/types"

type CreateToken struct {
	Name          *string   `json:"name" validate:"required"`
	Scopes        *[]string `json:"scopes" validate:"required"`
	ExpiresInDays *int64    `json:"expires_in_days"`
	Nodes         *[]string `json:"nodes"`
}

func (r CreateToken) Token() accesstypes.CreateToken {
	return accesstypes.CreateToken{Name: *r.Name, Scopes: *r.Scopes, ExpiresInDays: r.ExpiresInDays, Nodes: r.Nodes}
}

type Tokens struct {
	Page
	Prefix *string `query:"prefix"`
}

func (q Tokens) Query() accesstypes.TokenQuery {
	return accesstypes.TokenQuery{Prefix: q.Prefix, Page: q.Page.Query()}
}
