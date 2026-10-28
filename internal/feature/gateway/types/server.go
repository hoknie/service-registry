package types

import (
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"svc-registry/internal/platform/apperr"
)

type Status string

const (
	StatusDraft     Status = "draft"
	StatusPublished Status = "published"
	StatusDisabled  Status = "disabled"
)

func ParseStatus(s string) (Status, bool) {
	switch Status(s) {
	case StatusDraft, StatusPublished, StatusDisabled:
		return Status(s), true
	}
	return "", false
}

type State string

const (
	StateUnknown  State = "unknown"
	StateOK       State = "ok"
	StateFailing  State = "failing"
	StateDisabled State = "disabled"
)

func StateOf(status Status, last *Check, snapshot bool) State {
	switch {
	case status == StatusDisabled:
		return StateDisabled
	case last == nil || (last.Outcome != OutcomeOK && !snapshot):
		return StateUnknown
	case last.Outcome == OutcomeOK:
		return StateOK
	}
	return StateFailing
}

const (
	TransportStreamable = "streamable"
	TransportSSE        = "sse"
	AuthNone            = "none"
	AuthBearer          = "bearer"
	AuthHeader          = "header"
)

var (
	serverSlug  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)
	tagPattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9.+#-]{0,31}$`)
	iconName    = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	headerToken = regexp.MustCompile("^[!#$%&'*+.^_`|~0-9A-Za-z-]{1,100}$")
	reserved    = []string{"host", "content-length", "content-type", "accept", "mcp-session-id", "mcp-protocol-version",
		"connection", "transfer-encoding", "upgrade", "te", "trailer", "proxy-authorization"}
)

type Auth struct {
	Kind       string
	Header     string
	SecretID   *uuid.UUID
	SecretName *string
	SecretFrom *Ref
}

type Server struct {
	ID             uuid.UUID
	FolderID       *uuid.UUID
	OrganizationID *uuid.UUID
	Organization   *Ref
	Slug           string
	Title          string
	Description    string
	Tags           []string
	Author         string
	IconURL        *string
	Icon           *string
	HasIcon        bool
	URL            string
	Transport      string
	Auth           Auth
	Status         Status
	DisabledTools  []string
	LogPayloads    bool
	Everyone       bool
	Calls          int64
	Path           string
	Ancestors      []Ref
	ToolsCount     int
	Snapshot       bool
	Info           *ServerInfo
	LastCheck      *Check
	CanWrite       bool
	CanManage      bool
	CreatedAt      string
	UpdatedAt      string
}

func (s Server) State() State { return StateOf(s.Status, s.LastCheck, s.Snapshot) }

func (s Server) Scope() Scope { return Scope{OrganizationID: s.OrganizationID} }

type ServerInfo struct {
	Name            string `json:"name"`
	Title           string `json:"title,omitempty"`
	Version         string `json:"version"`
	ProtocolVersion string `json:"protocol_version"`
	Instructions    string `json:"instructions,omitempty"`
}

type ServerFields struct {
	Slug        string
	Title       string
	Description string
	Tags        []string
	Author      string
	IconURL     *string
	Icon        *string
	URL         string
	Transport   string
	Auth        Auth
	Status      Status
	LogPayloads bool
}

func CleanServer(in ServerFields) (ServerFields, error) {
	var bad []apperr.Field
	add := func(path, code string) { bad = append(bad, apperr.Field{Path: path, Code: code}) }
	in.Title, in.Description, in.Author = strings.TrimSpace(in.Title), strings.TrimSpace(in.Description), strings.TrimSpace(in.Author)
	if !serverSlug.MatchString(in.Slug) {
		add("slug", "invalid")
	}
	if in.Title == "" || utf8.RuneCountInString(in.Title) > 100 {
		add("title", "invalid")
	}
	if utf8.RuneCountInString(in.Description) > 4000 {
		add("description", "too_long")
	}
	if utf8.RuneCountInString(in.Author) > 100 {
		add("author", "too_long")
	}
	tags, ok := NewTags(in.Tags)
	if !ok {
		add("tags", "invalid")
	}
	in.Tags = tags
	if in.IconURL != nil && !validURL(*in.IconURL, true) {
		add("icon_url", "invalid")
	}
	if in.Icon != nil && (!iconName.MatchString(*in.Icon) || len(*in.Icon) > 64) {
		add("icon", "invalid")
	}
	in.URL = strings.TrimSpace(in.URL)
	if !validURL(in.URL, false) {
		add("url", "invalid")
	}
	if in.Transport == "" {
		in.Transport = TransportStreamable
	}
	if in.Transport != TransportStreamable && in.Transport != TransportSSE {
		add("transport", "invalid")
	}
	if in.Status == "" {
		in.Status = StatusDraft
	}
	if _, ok := ParseStatus(string(in.Status)); !ok {
		add("status", "invalid")
	}
	bad = append(bad, checkAuth(&in.Auth)...)
	if len(bad) > 0 {
		return ServerFields{}, InvalidServer(bad...)
	}
	return in, nil
}

func checkAuth(a *Auth) []apperr.Field {
	switch a.Kind {
	case "", AuthNone:
		a.Kind, a.Header, a.SecretID = AuthNone, "", nil
		return nil
	case AuthBearer:
		a.Header = ""
	case AuthHeader:
		if !headerToken.MatchString(a.Header) || slices.Contains(reserved, strings.ToLower(a.Header)) {
			return []apperr.Field{{Path: "auth.header", Code: "invalid"}}
		}
	default:
		return []apperr.Field{{Path: "auth.kind", Code: "invalid"}}
	}
	if a.SecretID == nil {
		return []apperr.Field{{Path: "auth.credentials.secret_id", Code: "required"}}
	}
	return nil
}

func validURL(raw string, httpsOnly bool) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.Hostname() == "" || u.User != nil || len(raw) > 2000 || u.Fragment != "" {
		return false
	}
	return u.Scheme == "https" || (!httpsOnly && u.Scheme == "http")
}

func NewTags(raw []string) ([]string, bool) {
	out := make([]string, 0, len(raw))
	for _, t := range raw {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" || slices.Contains(out, t) {
			continue
		}
		if !tagPattern.MatchString(t) {
			return nil, false
		}
		out = append(out, t)
	}
	return out, len(out) <= 20
}

func ValidTag(t string) bool { return tagPattern.MatchString(t) }
