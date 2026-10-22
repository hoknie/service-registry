package requests

import accesstypes "svc-registry/internal/feature/access/types"

type GroupName struct {
	Name *string `json:"name" validate:"required"`
}

func (r GroupName) Group() accesstypes.GroupName { return accesstypes.GroupName{Name: *r.Name} }
