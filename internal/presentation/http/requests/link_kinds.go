package requests

import "svc-registry/internal/feature/links"

type CreateLinkKind struct {
	Key      *string            `json:"key" validate:"required"`
	Names    *map[string]string `json:"names" validate:"required"`
	Icon     *string            `json:"icon"`
	Position *int64             `json:"position"`
}

func (r CreateLinkKind) Kind() links.CreateKind {
	return links.CreateKind{Key: *r.Key, Names: *r.Names, Icon: r.Icon, Position: r.Position}
}

type UpdateLinkKind struct {
	Names    *map[string]string `json:"names"`
	Icon     *string            `json:"icon"`
	Position *int64             `json:"position"`
}

func (r UpdateLinkKind) Kind() links.UpdateKind {
	out := links.UpdateKind{Icon: r.Icon, Position: r.Position}
	if r.Names != nil {
		out.Names = *r.Names
		if out.Names == nil {
			out.Names = map[string]string{}
		}
	}
	return out
}
