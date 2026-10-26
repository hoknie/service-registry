package responses

import (
	"github.com/google/uuid"

	accessservices "svc-registry/internal/feature/access/services"
	accesstypes "svc-registry/internal/feature/access/types"
)

type Status struct {
	Status string `json:"status"`
}

type Page[T any] struct {
	Items  []T    `json:"items"`
	Total  uint64 `json:"total"`
	Limit  uint32 `json:"limit"`
	Offset uint64 `json:"offset"`
}

func PageOf[D, T any](p accesstypes.Page[D], conv func(D) T) Page[T] {
	items := make([]T, 0, len(p.Items))
	for _, d := range p.Items {
		items = append(items, conv(d))
	}
	return Page[T]{Items: items, Total: p.Total, Limit: p.Limit, Offset: p.Offset}
}

type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	DisplayName  string    `json:"display_name"`
	Status       string    `json:"status"`
	IsSuperadmin bool      `json:"is_superadmin"`
	IsService    bool      `json:"is_service"`
	HasPassword  bool      `json:"has_password"`
	CreatedAt    string    `json:"created_at"`
	UpdatedAt    string    `json:"updated_at"`
}

type Me struct {
	User
	LoginMethod string   `json:"login_method"`
	Rights      Rights   `json:"rights"`
	Features    Features `json:"features"`
}

type Features struct {
	Metrics bool `json:"metrics"`
}

func UserOf(u accesstypes.User) User {
	return User{
		ID: u.ID, Email: u.Email, DisplayName: u.DisplayName, Status: string(u.Status),
		IsSuperadmin: u.IsSuperadmin, IsService: u.IsService, HasPassword: u.HasPassword, CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt,
	}
}

type Provider struct {
	Key         string `json:"key"`
	DisplayName string `json:"display_name"`
}

type Providers struct {
	Providers     []Provider `json:"providers"`
	PasswordLogin string     `json:"password_login"`
}

func ProvidersOf(providers []accessservices.Provider, mode string) Providers {
	out := make([]Provider, 0, len(providers))
	for _, p := range providers {
		out = append(out, Provider{Key: p.Key, DisplayName: p.DisplayName})
	}
	return Providers{Providers: out, PasswordLogin: mode}
}
