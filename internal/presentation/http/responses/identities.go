package responses

import (
	"github.com/google/uuid"

	accessservice "svc-registry/internal/feature/access/service"
)

type Identity struct {
	ID          uuid.UUID `json:"id"`
	Provider    string    `json:"provider"`
	DisplayName string    `json:"display_name"`
	Email       *string   `json:"email"`
	CreatedAt   string    `json:"created_at"`
	LastLoginAt string    `json:"last_login_at"`
}

type Identities struct {
	Items []Identity `json:"items"`
}

func IdentitiesOf(items []accessservice.IdentityView) Identities {
	out := make([]Identity, 0, len(items))
	for _, i := range items {
		out = append(out, Identity{ID: i.ID, Provider: i.Provider, DisplayName: i.DisplayName, Email: i.Email,
			CreatedAt: i.CreatedAt, LastLoginAt: i.LastLoginAt})
	}
	return Identities{Items: out}
}
