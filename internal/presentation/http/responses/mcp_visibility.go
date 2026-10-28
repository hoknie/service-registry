package responses

import (
	"github.com/google/uuid"

	gatewayservices "svc-registry/internal/feature/gateway/services"
	gatewaytypes "svc-registry/internal/feature/gateway/types"
)

type MCPGroup struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type MCPUser struct {
	ID    uuid.UUID `json:"id"`
	Email string    `json:"email"`
	Name  string    `json:"name,omitempty"`
}

type MCPAccess struct {
	Everyone  bool       `json:"everyone"`
	Groups    []MCPGroup `json:"groups"`
	Users     []MCPUser  `json:"users"`
	CanManage bool       `json:"can_manage"`
}

func MCPAccessOf(a gatewaytypes.AccessList) MCPAccess {
	out := MCPAccess{Everyone: a.Everyone, Groups: make([]MCPGroup, len(a.Groups)), Users: make([]MCPUser, len(a.Users)), CanManage: a.CanManage}
	for i, g := range a.Groups {
		out.Groups[i] = MCPGroup{ID: g.ID, Name: g.Name}
	}
	for i, u := range a.Users {
		out.Users[i] = MCPUser{ID: u.ID, Email: u.Email, Name: u.Name}
	}
	return out
}

type MCPServerRef struct {
	ID             uuid.UUID  `json:"id"`
	Slug           string     `json:"slug"`
	Title          string     `json:"title"`
	OrganizationID *uuid.UUID `json:"organization_id"`
	Status         string     `json:"status"`
}

type MCPOrganizationSet struct {
	Mode      string         `json:"mode"`
	Servers   []MCPServerRef `json:"servers"`
	CanManage bool           `json:"can_manage"`
}

func MCPOrganizationSetOf(s gatewaytypes.OrgSet) MCPOrganizationSet {
	out := MCPOrganizationSet{Mode: s.Mode, Servers: make([]MCPServerRef, len(s.Servers)), CanManage: s.CanManage}
	for i, r := range s.Servers {
		out.Servers[i] = MCPServerRef{ID: r.ID, Slug: r.Slug, Title: r.Title, OrganizationID: r.OrganizationID, Status: string(r.Status)}
	}
	return out
}

type MCPVisibilityOrganization struct {
	ID        uuid.UUID `json:"id"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	Mode      string    `json:"mode"`
	CanManage bool      `json:"can_manage"`
}

type MCPVisibilityServer struct {
	ID             uuid.UUID   `json:"id"`
	Slug           string      `json:"slug"`
	Title          string      `json:"title"`
	OrganizationID *uuid.UUID  `json:"organization_id"`
	Status         string      `json:"status"`
	State          string      `json:"state"`
	Access         MCPAccess   `json:"access"`
	In             []uuid.UUID `json:"in"`
	CanManage      bool        `json:"can_manage"`
}

type MCPVisibility struct {
	Organizations []MCPVisibilityOrganization `json:"organizations"`
	Servers       []MCPVisibilityServer       `json:"servers"`
}

func MCPVisibilityOf(s gatewayservices.Summary) MCPVisibility {
	out := MCPVisibility{Organizations: make([]MCPVisibilityOrganization, len(s.Organizations)), Servers: make([]MCPVisibilityServer, len(s.Servers))}
	for i, c := range s.Organizations {
		out.Organizations[i] = MCPVisibilityOrganization{ID: c.Organization.ID, Slug: c.Organization.Slug, Name: c.Organization.Name,
			Mode: c.Mode, CanManage: c.CanManage}
	}
	for i, r := range s.Servers {
		out.Servers[i] = MCPVisibilityServer{ID: r.Server.ID, Slug: r.Server.Slug, Title: r.Server.Title,
			OrganizationID: r.Server.OrganizationID, Status: string(r.Server.Status), State: string(r.Server.State()),
			Access: MCPAccessOf(r.Access), In: r.In, CanManage: r.Server.CanManage}
	}
	return out
}

type MCPCallUser struct {
	ID    uuid.UUID `json:"id"`
	Email string    `json:"email"`
	Name  string    `json:"name"`
}

type MCPCallVia struct {
	Kind string    `json:"kind"`
	ID   uuid.UUID `json:"id"`
	Name *string   `json:"name"`
}

type MCPCall struct {
	ID         uuid.UUID    `json:"id"`
	At         string       `json:"at"`
	Tool       string       `json:"tool"`
	User       *MCPCallUser `json:"user"`
	Via        *MCPCallVia  `json:"via"`
	DurationMs int          `json:"duration_ms"`
	Outcome    string       `json:"outcome"`
	Code       *string      `json:"code"`
	Request    *string      `json:"request"`
	Response   *string      `json:"response"`
	Truncated  bool         `json:"truncated"`
}

type MCPCalls struct {
	Items      []MCPCall `json:"items"`
	NextCursor *string   `json:"next_cursor"`
}

func MCPCallsOf(p gatewayservices.CallPage) MCPCalls {
	out := MCPCalls{Items: make([]MCPCall, len(p.Items)), NextCursor: p.NextCursor}
	for i, c := range p.Items {
		item := MCPCall{ID: c.ID, At: c.At, Tool: c.Tool, DurationMs: c.DurationMs, Outcome: c.Outcome, Code: c.Code, Request: c.Request,
			Response: c.Response, Truncated: c.Truncated}
		if c.UserID != nil {
			item.User = &MCPCallUser{ID: *c.UserID, Email: text(c.UserEmail), Name: text(c.UserName)}
		}
		if c.ViaKind != nil && c.ViaID != nil {
			item.Via = &MCPCallVia{Kind: *c.ViaKind, ID: *c.ViaID, Name: c.ViaName}
		}
		out.Items[i] = item
	}
	return out
}
