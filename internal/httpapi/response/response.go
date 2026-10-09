package response

import (
	"github.com/google/uuid"

	"svc-registry/internal/access"
	"svc-registry/internal/service"
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

func PageOf[D, T any](p access.Page[D], conv func(D) T) Page[T] {
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
	LoginMethod string `json:"login_method"`
}

func UserOf(u access.User) User {
	return User{
		ID: u.ID, Email: u.Email, DisplayName: u.DisplayName, Status: string(u.Status),
		IsSuperadmin: u.IsSuperadmin, IsService: u.IsService, HasPassword: u.HasPassword, CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt,
	}
}

type Group struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	MemberCount int64     `json:"member_count"`
	CreatedAt   string    `json:"created_at"`
	UpdatedAt   string    `json:"updated_at"`
}

func GroupOf(g access.Group) Group {
	return Group{ID: g.ID, Name: g.Name, MemberCount: g.MemberCount, CreatedAt: g.CreatedAt, UpdatedAt: g.UpdatedAt}
}

type Member struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Status      string    `json:"status"`
	Source      string    `json:"source"`
}

type GroupDetails struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	MemberCount int64     `json:"member_count"`
	CreatedAt   string    `json:"created_at"`
	UpdatedAt   string    `json:"updated_at"`
	Members     []Member  `json:"members"`
}

func GroupDetailsOf(d access.GroupDetails) GroupDetails {
	members := make([]Member, 0, len(d.Members))
	for _, m := range d.Members {
		members = append(members, Member{ID: m.User.ID, Email: m.User.Email, DisplayName: m.User.DisplayName,
			Status: string(m.User.Status), Source: string(m.Source)})
	}
	g := d.Group
	return GroupDetails{ID: g.ID, Name: g.Name, MemberCount: g.MemberCount, CreatedAt: g.CreatedAt, UpdatedAt: g.UpdatedAt, Members: members}
}

type Provider struct {
	Key         string `json:"key"`
	DisplayName string `json:"display_name"`
}

type Providers struct {
	Providers     []Provider `json:"providers"`
	PasswordLogin string     `json:"password_login"`
}

func ProvidersOf(providers []service.Provider, mode string) Providers {
	out := make([]Provider, 0, len(providers))
	for _, p := range providers {
		out = append(out, Provider{Key: p.Key, DisplayName: p.DisplayName})
	}
	return Providers{Providers: out, PasswordLogin: mode}
}
