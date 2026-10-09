package request

import "svc-registry/internal/catalog"

type Branches struct {
	Page
	Q     *string `query:"q"`
	State *string `query:"state"`
}

func (q Branches) Query() catalog.BranchQuery {
	return catalog.BranchQuery{Prefix: q.Q, State: q.State, Limit: q.Limit, Offset: q.Offset}
}
