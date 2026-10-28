package requests

import (
	"strings"

	"github.com/google/uuid"

	gatewayservices "svc-registry/internal/feature/gateway/services"
	gatewaytypes "svc-registry/internal/feature/gateway/types"
)

type MCPFolderCreate struct {
	ParentID       *uuid.UUID `json:"parent_id"`
	OrganizationID *uuid.UUID `json:"organization_id"`
	Slug           *string    `json:"slug" validate:"required"`
	Name           *string    `json:"name" validate:"required"`
	Description    *string    `json:"description"`
}

func (r MCPFolderCreate) Input() gatewayservices.FolderInput {
	in := gatewayservices.FolderInput{ParentID: r.ParentID, OrganizationID: r.OrganizationID, Slug: *r.Slug, Name: *r.Name}
	if r.Description != nil {
		in.Description = *r.Description
	}
	return in
}

type MCPFolderPatch struct {
	Slug           *string             `json:"slug"`
	Name           *string             `json:"name"`
	Description    *string             `json:"description"`
	ParentID       Optional[uuid.UUID] `json:"parent_id"`
	OrganizationID *uuid.UUID          `json:"organization_id"`
}

func (r MCPFolderPatch) Patch() gatewayservices.FolderPatch {
	return gatewayservices.FolderPatch{Slug: r.Slug, Name: r.Name, Description: r.Description, Move: r.ParentID.Present,
		ParentID: r.ParentID.Value, OrganizationID: r.OrganizationID}
}

type MCPCredentials struct {
	SecretID *uuid.UUID `json:"secret_id"`
}

type MCPAuth struct {
	Kind        *string         `json:"kind" validate:"required"`
	Header      *string         `json:"header"`
	Credentials *MCPCredentials `json:"credentials"`
}

func (a MCPAuth) auth() gatewaytypes.Auth {
	out := gatewaytypes.Auth{Kind: *a.Kind}
	if a.Header != nil {
		out.Header = strings.TrimSpace(*a.Header)
	}
	if a.Credentials != nil {
		out.SecretID = a.Credentials.SecretID
	}
	return out
}

type MCPServerCreate struct {
	FolderID       *uuid.UUID `json:"folder_id"`
	OrganizationID *uuid.UUID `json:"organization_id"`
	Slug           *string    `json:"slug" validate:"required"`
	Title          *string    `json:"title" validate:"required"`
	Description    *string    `json:"description"`
	Tags           []string   `json:"tags"`
	IconURL        *string    `json:"icon_url"`
	Icon           *string    `json:"icon"`
	URL            *string    `json:"url" validate:"required"`
	Transport      *string    `json:"transport"`
	Auth           *MCPAuth   `json:"auth"`
	Status         *string    `json:"status"`
	LogPayloads    *bool      `json:"log_payloads"`
}

func text(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (r MCPServerCreate) Input() gatewayservices.ServerInput {
	f := gatewaytypes.ServerFields{Slug: *r.Slug, Title: *r.Title, Description: text(r.Description), Tags: r.Tags,
		IconURL: r.IconURL, Icon: r.Icon, URL: *r.URL, Transport: text(r.Transport), Status: gatewaytypes.Status(text(r.Status)),
		LogPayloads: r.LogPayloads == nil || *r.LogPayloads}
	if r.Auth != nil {
		f.Auth = r.Auth.auth()
	}
	return gatewayservices.ServerInput{FolderID: r.FolderID, OrganizationID: r.OrganizationID, Fields: f}
}

type MCPServerPatch struct {
	Slug           *string             `json:"slug"`
	Title          *string             `json:"title"`
	Description    *string             `json:"description"`
	Tags           *[]string           `json:"tags"`
	IconURL        Optional[string]    `json:"icon_url"`
	Icon           Optional[string]    `json:"icon"`
	URL            *string             `json:"url"`
	Transport      *string             `json:"transport"`
	Auth           *MCPAuth            `json:"auth"`
	Status         *string             `json:"status"`
	LogPayloads    *bool               `json:"log_payloads"`
	FolderID       Optional[uuid.UUID] `json:"folder_id"`
	OrganizationID *uuid.UUID          `json:"organization_id"`
}

func (r MCPServerPatch) Patch() gatewayservices.ServerPatch {
	p := gatewayservices.ServerPatch{Slug: r.Slug, Title: r.Title, Description: r.Description, Tags: r.Tags,
		URL: r.URL, Transport: r.Transport, Status: r.Status, LogPayloads: r.LogPayloads, Move: r.FolderID.Present, FolderID: r.FolderID.Value, OrgID: r.OrganizationID}
	if r.IconURL.Present {
		p.IconURL = &r.IconURL.Value
	}
	if r.Icon.Present {
		p.Icon = &r.Icon.Value
	}
	if r.Auth != nil {
		a := r.Auth.auth()
		p.Auth = &a
	}
	return p
}

type MCPServersQuery struct {
	FolderID string   `query:"folder_id"`
	OrgID    string   `query:"organization_id"`
	Query    string   `query:"q"`
	Tags     []string `query:"tag"`
	State    string   `query:"state"`
	Status   string   `query:"status"`
	Limit    int      `query:"limit"`
	Cursor   string   `query:"cursor"`
}

func (q MCPServersQuery) Input() (gatewayservices.ServerQuery, error) {
	in := gatewayservices.ServerQuery{Query: q.Query, Tags: q.Tags, Limit: q.Limit, Cursor: q.Cursor}
	if q.Limit < 0 || q.Limit > 200 {
		return in, gatewaytypes.InvalidPagination
	}
	for _, t := range q.Tags {
		if !gatewaytypes.ValidTag(t) {
			return in, gatewaytypes.InvalidFilter
		}
	}
	switch q.State {
	case "":
	case "ok", "failing", "unknown", "disabled":
		in.State = &q.State
	default:
		return in, gatewaytypes.InvalidFilter
	}
	if q.Status != "" {
		if _, ok := gatewaytypes.ParseStatus(q.Status); !ok {
			return in, gatewaytypes.InvalidFilter
		}
		in.Status = &q.Status
	}
	switch q.FolderID {
	case "":
	case "root":
		in.TopLevel = true
	default:
		id, err := uuid.Parse(q.FolderID)
		if err != nil {
			return in, gatewaytypes.InvalidFilter
		}
		in.FolderID = &id
	}
	switch q.OrgID {
	case "":
	case "global":
		in.OnlyGlobal = true
	default:
		id, err := uuid.Parse(q.OrgID)
		if err != nil {
			return in, gatewaytypes.InvalidFilter
		}
		in.OrganizationID = &id
	}
	return in, nil
}

type MCPDisabledTools struct {
	Tools *[]string `json:"tools" validate:"required"`
}

type MCPAccess struct {
	Everyone *bool       `json:"everyone" validate:"required"`
	Groups   []uuid.UUID `json:"groups"`
	Users    []uuid.UUID `json:"users"`
}

type MCPOrganizationSet struct {
	Mode    *string     `json:"mode" validate:"required"`
	Servers []uuid.UUID `json:"servers"`
}

type MCPSpace struct {
	Description *string `json:"description" validate:"required"`
}

type MCPProxy struct {
	Mode        *string         `json:"mode" validate:"required"`
	URL         *string         `json:"url"`
	NoProxy     *string         `json:"no_proxy"`
	Credentials *MCPCredentials `json:"credentials"`
}

func (r MCPProxy) Proxy() gatewaytypes.Proxy {
	p := gatewaytypes.Proxy{Mode: *r.Mode, URL: r.URL, NoProxy: r.NoProxy}
	if r.Credentials != nil {
		p.SecretID = r.Credentials.SecretID
	}
	return p
}

type MCPCallsQuery struct {
	Tool    string `query:"tool"`
	Outcome string `query:"outcome"`
	UserID  string `query:"user_id"`
	Limit   int    `query:"limit"`
	Cursor  string `query:"cursor"`
}

func (q MCPCallsQuery) Input() (gatewayservices.CallQuery, error) {
	in := gatewayservices.CallQuery{Limit: q.Limit, Cursor: q.Cursor}
	if q.Limit < 0 || q.Limit > 200 {
		return in, gatewaytypes.InvalidPagination
	}
	if q.Tool != "" {
		in.Tool = &q.Tool
	}
	switch q.Outcome {
	case "":
	case gatewaytypes.CallOK, gatewaytypes.CallToolError, gatewaytypes.CallFailed:
		in.Outcome = &q.Outcome
	default:
		return in, gatewaytypes.InvalidFilter
	}
	if q.UserID != "" {
		id, err := uuid.Parse(q.UserID)
		if err != nil {
			return in, gatewaytypes.InvalidFilter
		}
		in.UserID = &id
	}
	return in, nil
}
