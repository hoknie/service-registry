package request

import "svc-registry/internal/access"

type CreateToken struct {
	Name          *string   `json:"name" validate:"required"`
	Scopes        *[]string `json:"scopes" validate:"required"`
	ExpiresInDays *int64    `json:"expires_in_days"`
}

func (r CreateToken) Token() access.CreateToken {
	return access.CreateToken{Name: *r.Name, Scopes: *r.Scopes, ExpiresInDays: r.ExpiresInDays}
}

type Tokens struct {
	Page
	Prefix *string `query:"prefix"`
}

func (q Tokens) Query() access.TokenQuery {
	return access.TokenQuery{Prefix: q.Prefix, Page: q.Page.Query()}
}
