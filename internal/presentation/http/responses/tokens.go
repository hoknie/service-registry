package responses

import (
	"github.com/google/uuid"

	accessservices "svc-registry/internal/feature/access/services"
	accesstypes "svc-registry/internal/feature/access/types"
)

type TokenNode struct {
	ID   uuid.UUID `json:"id"`
	Kind *string   `json:"kind"`
	Name *string   `json:"name"`
	Path *string   `json:"path"`
}

type Token struct {
	ID           uuid.UUID     `json:"id"`
	Name         string        `json:"name"`
	Prefix       string        `json:"prefix"`
	Scopes       []string      `json:"scopes"`
	Nodes        []TokenNode   `json:"nodes"`
	CatalogAdmin bool          `json:"catalog_admin"`
	Status       string        `json:"status"`
	CreatedAt    string        `json:"created_at"`
	ExpiresAt    *string       `json:"expires_at"`
	LastUsedAt   *string       `json:"last_used_at"`
	RevokedAt    *string       `json:"revoked_at"`
	UserID       *uuid.UUID    `json:"user_id,omitempty"`
	UserEmail    *string       `json:"user_email,omitempty"`
	Secret       *string       `json:"secret,omitempty"`
	MCPServers   []TokenServer `json:"mcp_servers"`
}

type TokenServer struct {
	ID             uuid.UUID  `json:"id"`
	Slug           string     `json:"slug"`
	Title          string     `json:"title"`
	OrganizationID *uuid.UUID `json:"organization_id,omitempty"`
}

func tokenServers(set accesstypes.ServerSet, refs []accesstypes.ServerRef) []TokenServer {
	if !set.Limited {
		return nil
	}
	out := make([]TokenServer, len(refs))
	for i, r := range refs {
		out[i] = TokenServer{ID: r.ID, Slug: r.Slug, Title: r.Title}
	}
	return out
}

func TokenOf(t accesstypes.Token) Token {
	var nodes []TokenNode
	if t.NodeIDs != nil {
		nodes = make([]TokenNode, len(t.Nodes))
		for i, n := range t.Nodes {
			nodes[i] = TokenNode{ID: n.ID, Kind: n.Kind, Name: n.Name, Path: n.Path}
		}
	}
	return Token{
		ID: t.ID, Name: t.Name, Prefix: t.Prefix, Scopes: t.Scopes.Strings(), Nodes: nodes, CatalogAdmin: t.CatalogAdmin, Status: string(t.Status),
		CreatedAt: t.CreatedAt, ExpiresAt: t.ExpiresAt, LastUsedAt: t.LastUsedAt, RevokedAt: t.RevokedAt,
		MCPServers: tokenServers(t.MCPServerSet, t.MCPServers),
	}
}

func OwnedTokenOf(t accesstypes.Token) Token {
	out := TokenOf(t)
	id, email := t.UserID, t.UserEmail
	out.UserID, out.UserEmail = &id, &email
	return out
}

func IssuedTokenOf(i accessservices.IssuedToken) Token {
	out := TokenOf(i.Token)
	out.Secret = new(i.Secret)
	return out
}
