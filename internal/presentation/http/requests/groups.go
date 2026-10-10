package requests

import "svc-registry/internal/feature/access"

type GroupName struct {
	Name *string `json:"name" validate:"required"`
}

func (r GroupName) Group() access.GroupName { return access.GroupName{Name: *r.Name} }
