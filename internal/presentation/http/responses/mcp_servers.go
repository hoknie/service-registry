package responses

import (
	"encoding/json"

	"github.com/google/uuid"

	gatewayservices "svc-registry/internal/feature/gateway/services"
	gatewaytypes "svc-registry/internal/feature/gateway/types"
)

type MCPRef struct {
	ID   uuid.UUID `json:"id"`
	Slug string    `json:"slug"`
	Name string    `json:"name"`
}

func mcpRefOf(r *gatewaytypes.Ref) *MCPRef {
	if r == nil {
		return nil
	}
	return &MCPRef{ID: r.ID, Slug: r.Slug, Name: r.Name}
}

func mcpRefs(rs []gatewaytypes.Ref) []MCPRef {
	out := make([]MCPRef, len(rs))
	for i, r := range rs {
		out[i] = MCPRef{ID: r.ID, Slug: r.Slug, Name: r.Name}
	}
	return out
}

type MCPFolder struct {
	ID             uuid.UUID  `json:"id"`
	ParentID       *uuid.UUID `json:"parent_id"`
	OrganizationID *uuid.UUID `json:"organization_id"`
	Organization   *MCPRef    `json:"organization"`
	Slug           string     `json:"slug"`
	Name           string     `json:"name"`
	Description    string     `json:"description"`
	Path           string     `json:"path"`
	Ancestors      []MCPRef   `json:"ancestors"`
	Servers        int64      `json:"servers"`
	Folders        int64      `json:"folders"`
	CanWrite       bool       `json:"can_write"`
	CreatedAt      string     `json:"created_at"`
	UpdatedAt      string     `json:"updated_at"`
}

func MCPFolderOf(f gatewaytypes.Folder) MCPFolder {
	return MCPFolder{ID: f.ID, ParentID: f.ParentID, OrganizationID: f.OrganizationID, Organization: mcpRefOf(f.Organization),
		Slug: f.Slug, Name: f.Name, Description: f.Description, Path: f.Path, Ancestors: mcpRefs(f.Ancestors), Servers: f.Servers,
		Folders: f.Folders, CanWrite: f.CanWrite, CreatedAt: f.CreatedAt, UpdatedAt: f.UpdatedAt}
}

func MCPFoldersOf(fs []gatewaytypes.Folder) map[string][]MCPFolder {
	out := make([]MCPFolder, len(fs))
	for i, f := range fs {
		out[i] = MCPFolderOf(f)
	}
	return map[string][]MCPFolder{"items": out}
}

type MCPAuth struct {
	Kind        string       `json:"kind"`
	Header      *string      `json:"header,omitempty"`
	Credentials *Credentials `json:"credentials,omitempty"`
}

type MCPCheck struct {
	At         string  `json:"at"`
	Outcome    string  `json:"outcome"`
	Code       *string `json:"code"`
	Detail     *string `json:"detail"`
	DurationMs int     `json:"duration_ms"`
	Truncated  bool    `json:"truncated"`
}

type MCPServerInfo struct {
	Name            string `json:"name"`
	Title           string `json:"title"`
	Version         string `json:"version"`
	ProtocolVersion string `json:"protocol_version"`
	Instructions    string `json:"instructions"`
}

type MCPServer struct {
	ID             uuid.UUID      `json:"id"`
	FolderID       *uuid.UUID     `json:"folder_id"`
	OrganizationID *uuid.UUID     `json:"organization_id"`
	Organization   *MCPRef        `json:"organization"`
	Path           string         `json:"path"`
	Ancestors      []MCPRef       `json:"ancestors"`
	Slug           string         `json:"slug"`
	Title          string         `json:"title"`
	Description    string         `json:"description"`
	Tags           []string       `json:"tags"`
	Author         string         `json:"author"`
	IconURL        *string        `json:"icon_url"`
	Icon           *string        `json:"icon"`
	HasIcon        bool           `json:"has_icon"`
	LogPayloads    bool           `json:"log_payloads"`
	Status         string         `json:"status"`
	State          string         `json:"state"`
	ToolsCount     int            `json:"tools_count"`
	Calls          int64          `json:"calls"`
	Auth           MCPAuth        `json:"auth"`
	URL            *string        `json:"url,omitempty"`
	Transport      *string        `json:"transport,omitempty"`
	ServerInfo     *MCPServerInfo `json:"server_info,omitempty"`
	LastCheck      *MCPCheck      `json:"last_check,omitempty"`
	CanWrite       bool           `json:"can_write"`
	CanManage      bool           `json:"can_manage"`
	CreatedAt      string         `json:"created_at"`
	UpdatedAt      string         `json:"updated_at"`
}

func MCPServerOf(s gatewaytypes.Server) MCPServer {
	out := MCPServer{ID: s.ID, FolderID: s.FolderID, OrganizationID: s.OrganizationID, Organization: mcpRefOf(s.Organization),
		Path: s.Path, Ancestors: mcpRefs(s.Ancestors), Slug: s.Slug, Title: s.Title, Description: s.Description, Tags: s.Tags,
		Author: s.Author, IconURL: s.IconURL, Icon: s.Icon, HasIcon: s.HasIcon, LogPayloads: s.LogPayloads, Status: string(s.Status), State: string(s.State()), ToolsCount: s.ToolsCount,
		Calls: s.Calls, Auth: MCPAuth{Kind: s.Auth.Kind}, CanWrite: s.CanWrite, CanManage: s.CanManage, CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt}
	if !s.CanWrite {
		return out
	}
	out.URL, out.Transport = &s.URL, &s.Transport
	if s.Auth.Kind == gatewaytypes.AuthHeader {
		out.Auth.Header = &s.Auth.Header
	}
	if s.Auth.SecretID != nil {
		ref := &SecretRef{ID: *s.Auth.SecretID, Name: text(s.Auth.SecretName)}
		if s.Auth.SecretFrom != nil {
			ref.From = &NodeRef{ID: s.Auth.SecretFrom.ID, Name: s.Auth.SecretFrom.Name}
		}
		out.Auth.Credentials = &Credentials{Kind: "secret", Secret: ref}
	}
	if s.Info != nil {
		out.ServerInfo = &MCPServerInfo{Name: s.Info.Name, Title: s.Info.Title, Version: s.Info.Version,
			ProtocolVersion: s.Info.ProtocolVersion, Instructions: s.Info.Instructions}
	}
	if s.LastCheck != nil {
		c := s.LastCheck
		out.LastCheck = &MCPCheck{At: c.At, Outcome: c.Outcome, Code: c.Code, Detail: c.Detail, DurationMs: c.DurationMs, Truncated: c.Truncated}
	}
	return out
}

func text(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

type MCPServerPage struct {
	Items      []MCPServer `json:"items"`
	NextCursor *string     `json:"next_cursor"`
}

func MCPServerPageOf(p gatewayservices.ServerPage) MCPServerPage {
	out := MCPServerPage{Items: make([]MCPServer, len(p.Items)), NextCursor: p.NextCursor}
	for i, s := range p.Items {
		out.Items[i] = MCPServerOf(s)
	}
	return out
}

type MCPTool struct {
	Name         string          `json:"name"`
	GatewayName  string          `json:"gateway_name"`
	Title        string          `json:"title"`
	Description  string          `json:"description"`
	InputSchema  json.RawMessage `json:"input_schema"`
	OutputSchema json.RawMessage `json:"output_schema"`
	Annotations  json.RawMessage `json:"annotations"`
	Enabled      bool            `json:"enabled"`
	Skipped      *string         `json:"skipped"`
}

func raw(m json.RawMessage) json.RawMessage {
	if len(m) == 0 {
		return json.RawMessage("null")
	}
	return m
}

func MCPToolsOf(ts []gatewaytypes.ToolView) map[string][]MCPTool {
	out := make([]MCPTool, len(ts))
	for i, t := range ts {
		out[i] = MCPTool{Name: t.Name, GatewayName: t.GatewayName, Title: t.Title, Description: t.Description,
			InputSchema: raw(t.InputSchema), OutputSchema: raw(t.OutputSchema), Annotations: raw(t.Annotations), Enabled: t.Enabled}
		if t.Skipped != "" {
			out[i].Skipped = &t.Skipped
		}
	}
	return map[string][]MCPTool{"items": out}
}

type MCPSpace struct {
	Organization MCPRef `json:"organization"`
	Description  string `json:"description"`
	CanWrite     bool   `json:"can_write"`
	CanManage    bool   `json:"can_manage"`
}

func MCPSpaceOf(s gatewayservices.Space) MCPSpace {
	return MCPSpace{Organization: MCPRef{ID: s.Organization.ID, Slug: s.Organization.Slug, Name: s.Organization.Name},
		Description: s.Description, CanWrite: s.CanWrite, CanManage: s.CanManage}
}

type MCPProxy struct {
	Mode        string       `json:"mode"`
	URL         *string      `json:"url"`
	NoProxy     *string      `json:"no_proxy"`
	Credentials *Credentials `json:"credentials"`
}

func mcpProxyOf(p *gatewaytypes.Proxy) *MCPProxy {
	if p == nil {
		return nil
	}
	out := &MCPProxy{Mode: p.Mode, URL: p.URL, NoProxy: p.NoProxy}
	if p.SecretID != nil {
		ref := &SecretRef{ID: *p.SecretID, Name: text(p.SecretName)}
		if p.SecretFrom != nil {
			ref.From = &NodeRef{ID: p.SecretFrom.ID, Name: p.SecretFrom.Name}
		}
		out.Credentials = &Credentials{Kind: "secret", Secret: ref}
	}
	return out
}

type MCPProxyOwner struct {
	Kind string    `json:"kind"`
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type MCPProxySetting struct {
	Own       *MCPProxy      `json:"own"`
	Effective *MCPProxy      `json:"effective"`
	From      *MCPProxyOwner `json:"from"`
}

func MCPProxySettingOf(s gatewaytypes.ProxySetting) MCPProxySetting {
	out := MCPProxySetting{Own: mcpProxyOf(s.Own), Effective: mcpProxyOf(&s.Effective)}
	if s.From != nil {
		out.From = &MCPProxyOwner{Kind: s.From.Kind, ID: s.From.ID, Name: s.From.Name}
	}
	return out
}
