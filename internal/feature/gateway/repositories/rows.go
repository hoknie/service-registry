package repositories

import (
	"encoding/json"

	"github.com/google/uuid"

	gatewaytypes "svc-registry/internal/feature/gateway/types"
)

func refOf(id *uuid.UUID, slug, name *string) *gatewaytypes.Ref {
	if id == nil || slug == nil || name == nil {
		return nil
	}
	return &gatewaytypes.Ref{ID: *id, Slug: *slug, Name: *name}
}

type refRow struct {
	ID   uuid.UUID `db:"id"`
	Slug string    `db:"slug"`
	Name string    `db:"name"`
}

func (r refRow) ref() gatewaytypes.Ref { return gatewaytypes.Ref{ID: r.ID, Slug: r.Slug, Name: r.Name} }

type folderRow struct {
	ID             uuid.UUID  `db:"id"`
	ParentID       *uuid.UUID `db:"parent_id"`
	OrganizationID *uuid.UUID `db:"organization_id"`
	OrgSlug        *string    `db:"org_slug"`
	OrgName        *string    `db:"org_name"`
	Slug           string     `db:"slug"`
	Name           string     `db:"name"`
	Description    string     `db:"description"`
	Depth          int        `db:"depth"`
	Servers        int64      `db:"servers"`
	Folders        int64      `db:"folders"`
	CreatedAt      string     `db:"created_at"`
	UpdatedAt      string     `db:"updated_at"`
}

func (r folderRow) folder() gatewaytypes.Folder {
	return gatewaytypes.Folder{ID: r.ID, ParentID: r.ParentID, OrganizationID: r.OrganizationID,
		Organization: refOf(r.OrganizationID, r.OrgSlug, r.OrgName), Slug: r.Slug, Name: r.Name, Description: r.Description,
		Depth: r.Depth, Servers: r.Servers, Folders: r.Folders, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		Ancestors: []gatewaytypes.Ref{}}
}

type serverRow struct {
	ID             uuid.UUID  `db:"id"`
	FolderID       *uuid.UUID `db:"folder_id"`
	OrganizationID *uuid.UUID `db:"organization_id"`
	OrgSlug        *string    `db:"org_slug"`
	OrgName        *string    `db:"org_name"`
	Path           string     `db:"path"`
	Slug           string     `db:"slug"`
	Title          string     `db:"title"`
	Description    string     `db:"description"`
	Tags           []string   `db:"tags"`
	Author         string     `db:"author"`
	IconURL        *string    `db:"icon_url"`
	Icon           *string    `db:"icon"`
	HasIcon        bool       `db:"has_icon"`
	LogPayloads    bool       `db:"log_payloads"`
	URL            string     `db:"url"`
	Transport      string     `db:"transport"`
	AuthKind       string     `db:"auth_kind"`
	AuthHeader     *string    `db:"auth_header"`
	SecretID       *uuid.UUID `db:"secret_id"`
	SecretName     *string    `db:"secret_name"`
	SecretNodeID   *uuid.UUID `db:"secret_node_id"`
	SecretNodeSlug *string    `db:"secret_node_slug"`
	SecretNodeName *string    `db:"secret_node_name"`
	Status         string     `db:"status"`
	DisabledTools  []string   `db:"disabled_tools"`
	Everyone       bool       `db:"everyone"`
	Calls          int64      `db:"calls"`
	ToolsCount     int        `db:"tools_count"`
	Snapshot       bool       `db:"snapshot"`
	CheckedAt      *string    `db:"checked_at"`
	Outcome        *string    `db:"outcome"`
	Code           *string    `db:"code"`
	Detail         *string    `db:"detail"`
	DurationMs     *int       `db:"duration_ms"`
	Truncated      bool       `db:"truncated"`
	Info           []byte     `db:"server_info"`
	CreatedAt      string     `db:"created_at"`
	UpdatedAt      string     `db:"updated_at"`
}

func (r serverRow) server() gatewaytypes.Server {
	s := gatewaytypes.Server{ID: r.ID, FolderID: r.FolderID, OrganizationID: r.OrganizationID,
		Organization: refOf(r.OrganizationID, r.OrgSlug, r.OrgName), Path: r.Path, Slug: r.Slug, Title: r.Title,
		Description: r.Description, Tags: r.Tags, Author: r.Author, IconURL: r.IconURL, Icon: r.Icon, HasIcon: r.HasIcon, LogPayloads: r.LogPayloads, URL: r.URL, Transport: r.Transport,
		Auth: gatewaytypes.Auth{Kind: r.AuthKind, SecretID: r.SecretID, SecretName: r.SecretName,
			SecretFrom: refOf(r.SecretNodeID, r.SecretNodeSlug, r.SecretNodeName)},
		Status: gatewaytypes.Status(r.Status), DisabledTools: r.DisabledTools, Everyone: r.Everyone, Calls: r.Calls,
		ToolsCount: r.ToolsCount, Snapshot: r.Snapshot, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Ancestors: []gatewaytypes.Ref{}}
	if r.AuthHeader != nil {
		s.Auth.Header = *r.AuthHeader
	}
	if s.Tags == nil {
		s.Tags = []string{}
	}
	if r.CheckedAt != nil && r.Outcome != nil {
		s.LastCheck = &gatewaytypes.Check{At: *r.CheckedAt, Outcome: *r.Outcome, Code: r.Code, Detail: r.Detail,
			Truncated: r.Truncated}
		if r.DurationMs != nil {
			s.LastCheck.DurationMs = *r.DurationMs
		}
	}
	if len(r.Info) > 0 {
		var info gatewaytypes.ServerInfo
		if json.Unmarshal(r.Info, &info) == nil {
			s.Info = &info
		}
	}
	return s
}

type toolsRow struct {
	Slug     string   `db:"slug"`
	Tools    []byte   `db:"tools"`
	Disabled []string `db:"disabled_tools"`
}

func decodeTools(raw []byte) []gatewaytypes.Tool {
	var tools []gatewaytypes.Tool
	if len(raw) == 0 || json.Unmarshal(raw, &tools) != nil {
		return []gatewaytypes.Tool{}
	}
	return tools
}

type proxyRow struct {
	OwnerKind      string     `db:"owner_kind"`
	OwnerID        uuid.UUID  `db:"owner_id"`
	OwnerName      string     `db:"owner_name"`
	Mode           *string    `db:"mode"`
	URL            *string    `db:"url"`
	NoProxy        *string    `db:"no_proxy"`
	SecretID       *uuid.UUID `db:"secret_id"`
	SecretName     *string    `db:"secret_name"`
	SecretNodeID   *uuid.UUID `db:"secret_node_id"`
	SecretNodeSlug *string    `db:"secret_node_slug"`
	SecretNodeName *string    `db:"secret_node_name"`
}

func (r proxyRow) link() gatewaytypes.ProxyLink {
	l := gatewaytypes.ProxyLink{Owner: gatewaytypes.ProxyOwner{Kind: r.OwnerKind, ID: r.OwnerID, Name: r.OwnerName}}
	if r.Mode != nil {
		l.Proxy = &gatewaytypes.Proxy{Mode: *r.Mode, URL: r.URL, NoProxy: r.NoProxy, SecretID: r.SecretID, SecretName: r.SecretName,
			SecretFrom: refOf(r.SecretNodeID, r.SecretNodeSlug, r.SecretNodeName)}
	}
	return l
}
