package requests

import catalogtypes "svc-registry/internal/feature/catalog/types"

type Branches struct {
	Page
	Q     *string `query:"q"`
	State *string `query:"state"`
}

func (q Branches) Query() catalogtypes.BranchQuery {
	return catalogtypes.BranchQuery{Prefix: q.Q, State: q.State, Limit: q.Limit, Offset: q.Offset}
}
