package requests

import deploytypes "svc-registry/internal/feature/deploy/types"

type CreateEnvironment struct {
	Key      *string            `json:"key" validate:"required"`
	Names    *map[string]string `json:"names" validate:"required"`
	Position *int64             `json:"position"`
}

func (r CreateEnvironment) Environment() deploytypes.CreateEnvironment {
	return deploytypes.CreateEnvironment{Key: *r.Key, Names: *r.Names, Position: r.Position}
}

type UpdateEnvironment struct {
	Names    *map[string]string `json:"names"`
	Position *int64             `json:"position"`
}

func (r UpdateEnvironment) Environment() deploytypes.UpdateEnvironment {
	out := deploytypes.UpdateEnvironment{Position: r.Position}
	if r.Names != nil {
		out.Names = *r.Names
		if out.Names == nil {
			out.Names = map[string]string{}
		}
	}
	return out
}
