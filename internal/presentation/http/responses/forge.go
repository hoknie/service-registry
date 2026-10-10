package responses

import (
	"github.com/google/uuid"

	"svc-registry/internal/feature/forge"
	forgeservice "svc-registry/internal/feature/forge/service"
)

type Repository struct {
	ConnectionID  uuid.UUID          `json:"connection_id"`
	FullPath      string             `json:"full_path"`
	WebURL        string             `json:"web_url"`
	Description   string             `json:"description"`
	Topics        []string           `json:"topics"`
	Languages     map[string]float64 `json:"languages"`
	DefaultBranch *string            `json:"default_branch"`
	Archived      bool               `json:"archived"`
	Visibility    string             `json:"visibility"`
	Stars         int                `json:"stars"`
	License       *string            `json:"license"`
	PushedAt      *string            `json:"pushed_at"`
	SyncedAt      string             `json:"synced_at"`
	OrphanedAt    *string            `json:"orphaned_at"`
	HasReadme     bool               `json:"has_readme"`
}

func RepositoryOf(r *forge.Repository) *Repository {
	if r == nil {
		return nil
	}
	return &Repository{
		ConnectionID: r.ConnectionID, FullPath: r.FullPath, WebURL: r.WebURL, Description: r.Description,
		Topics: r.Topics, Languages: r.Languages, DefaultBranch: r.DefaultBranch, Archived: r.Archived,
		Visibility: string(r.Visibility), Stars: r.Stars, License: r.License, PushedAt: r.PushedAt,
		SyncedAt: r.SyncedAt, OrphanedAt: r.OrphanedAt, HasReadme: r.HasReadme,
	}
}

type Credentials struct {
	Kind        string  `json:"kind"`
	Fingerprint *string `json:"fingerprint,omitempty"`
	Ref         *string `json:"ref,omitempty"`
}

type Webhook struct {
	Mode string  `json:"mode"`
	URL  *string `json:"url"`
}

type Run struct {
	ID           uuid.UUID       `json:"id"`
	Trigger      string          `json:"trigger"`
	Status       string          `json:"status"`
	StartedAt    string          `json:"started_at"`
	FinishedAt   *string         `json:"finished_at"`
	Created      int             `json:"created"`
	Updated      int             `json:"updated"`
	Orphaned     int             `json:"orphaned"`
	Skipped      int             `json:"skipped"`
	ErrorCode    *string         `json:"error_code"`
	ErrorMessage *string         `json:"error_message"`
	Problems     []forge.Problem `json:"problems"`
}

func RunOf(r forge.Run) Run {
	problems := r.Problems
	if problems == nil {
		problems = []forge.Problem{}
	}
	return Run{ID: r.ID, Trigger: string(r.Trigger), Status: string(r.Status), StartedAt: r.StartedAt,
		FinishedAt: r.FinishedAt, Created: r.Counts.Created, Updated: r.Counts.Updated, Orphaned: r.Counts.Orphaned,
		Skipped: r.Counts.Skipped, ErrorCode: r.ErrorCode, ErrorMessage: r.ErrorMessage, Problems: problems}
}

type Connection struct {
	ID              uuid.UUID   `json:"id"`
	NodeID          uuid.UUID   `json:"node_id"`
	Kind            string      `json:"kind"`
	APIURL          string      `json:"api_url"`
	OwnerPath       string      `json:"owner_path"`
	MirrorSubgroups bool        `json:"mirror_subgroups"`
	IncludeArchived bool        `json:"include_archived"`
	IncludeForks    bool        `json:"include_forks"`
	NameInclude     []string    `json:"name_include"`
	NameExclude     []string    `json:"name_exclude"`
	BranchInclude   []string    `json:"branch_include"`
	IntervalSecs    int         `json:"interval_secs"`
	Credentials     Credentials `json:"credentials"`
	Webhook         *Webhook    `json:"webhook"`
	LastRun         *Run        `json:"last_run"`
	NextRunAt       string      `json:"next_run_at"`
	CreatedAt       string      `json:"created_at"`
	UpdatedAt       string      `json:"updated_at"`
}

func ConnectionOf(v forgeservice.ConnectionView) Connection {
	c := v.Connection
	out := Connection{
		ID: c.ID, NodeID: c.NodeID, Kind: string(c.Kind), APIURL: c.APIURL, OwnerPath: c.OwnerPath,
		MirrorSubgroups: c.MirrorSubgroups, IncludeArchived: c.IncludeArchived, IncludeForks: c.IncludeForks,
		NameInclude: nonNil(c.NameInclude), NameExclude: nonNil(c.NameExclude), BranchInclude: nonNil(c.BranchInclude), IntervalSecs: c.IntervalSecs,
		Credentials: Credentials{Kind: string(c.Credentials.Kind)},
		NextRunAt:   c.NextRunAt, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
	if c.Credentials.Kind == forge.CredentialsRef {
		ref := c.Credentials.Ref
		out.Credentials.Ref = &ref
	} else {
		fp := c.Credentials.Fingerprint
		out.Credentials.Fingerprint = &fp
	}
	if c.WebhookMode != nil {
		out.Webhook = &Webhook{Mode: string(*c.WebhookMode), URL: v.WebhookURL}
	}
	if c.LastRun != nil {
		r := RunOf(*c.LastRun)
		out.LastRun = &r
	}
	return out
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

type OK struct {
	OK bool `json:"ok"`
}

type PreviewItem struct {
	FullPath string  `json:"full_path"`
	Action   string  `json:"action"`
	Code     *string `json:"code,omitempty"`
}

func PreviewItemOf(i forgeservice.PreviewItem) PreviewItem {
	return PreviewItem{FullPath: i.FullPath, Action: i.Action, Code: i.Code}
}

type WebhookSetup struct {
	Mode   string  `json:"mode"`
	URL    string  `json:"url"`
	Secret *string `json:"secret,omitempty"`
}

func WebhookSetupOf(w forgeservice.WebhookSetup) WebhookSetup {
	return WebhookSetup{Mode: string(w.Mode), URL: w.URL, Secret: w.Secret}
}

type Readme struct {
	Markdown      string  `json:"markdown"`
	Truncated     bool    `json:"truncated"`
	WebURL        string  `json:"web_url"`
	DefaultBranch *string `json:"default_branch"`
	Kind          string  `json:"kind"`
}

func ReadmeOf(r forge.Readme) Readme {
	return Readme{Markdown: r.Markdown, Truncated: r.Truncated, WebURL: r.WebURL, DefaultBranch: r.DefaultBranch, Kind: string(r.Kind)}
}
