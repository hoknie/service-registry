package requests

import linkstypes "svc-registry/internal/feature/links/types"

type CreateLinkKind struct {
	Key      *string             `json:"key" validate:"required"`
	Names    *map[string]*string `json:"names" validate:"required"`
	Icon     *string             `json:"icon"`
	IconURL  *string             `json:"icon_url"`
	Position *int64              `json:"position"`
}

func (r CreateLinkKind) Kind() linkstypes.CreateKind {
	return linkstypes.CreateKind{Key: *r.Key, Names: *r.Names, Icon: r.Icon, IconURL: r.IconURL, Position: r.Position}
}

type UpdateLinkKind struct {
	Names    *map[string]*string `json:"names"`
	Icon     *string             `json:"icon"`
	IconURL  Optional[string]    `json:"icon_url"`
	Position *int64              `json:"position"`
}

func (r UpdateLinkKind) Kind() linkstypes.UpdateKind {
	out := linkstypes.UpdateKind{Icon: r.Icon, Position: r.Position, IconURLSet: r.IconURL.Present, IconURL: r.IconURL.Value}
	if r.Names != nil {
		out.Names = *r.Names
		if out.Names == nil {
			out.Names = map[string]*string{}
		}
	}
	return out
}

type ReorderLinkKinds struct {
	Keys *[]string `json:"keys" validate:"required"`
}
