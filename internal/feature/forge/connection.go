package forge

import (
	"github.com/google/uuid"

	"svc-registry/internal/feature/catalog"
)

type Kind string

const (
	KindGithub  Kind = "github"
	KindGitlab  Kind = "gitlab"
	KindForgejo Kind = "forgejo"
	KindGitea   Kind = "gitea"
)

func ParseKind(s string) (Kind, bool) {
	switch Kind(s) {
	case KindGithub, KindGitlab, KindForgejo, KindGitea:
		return Kind(s), true
	}
	return "", false
}

func (k Kind) Forge() catalog.Forge { return catalog.Forge(k) }

func (k Kind) DefaultAPIURL() string {
	switch k {
	case KindGithub:
		return "https://api.github.com"
	case KindGitlab:
		return "https://gitlab.com"
	}
	return ""
}

type WebhookMode string

const (
	WebhookRegister WebhookMode = "register"
	WebhookManual   WebhookMode = "manual"
)

func ParseWebhookMode(s string) (WebhookMode, bool) {
	switch WebhookMode(s) {
	case WebhookRegister, WebhookManual:
		return WebhookMode(s), true
	}
	return "", false
}

type Connection struct {
	ID              uuid.UUID
	NodeID          uuid.UUID
	Kind            Kind
	APIURL          string
	OwnerPath       string
	MirrorSubgroups bool
	IncludeArchived bool
	IncludeForks    bool
	NameInclude     []string
	NameExclude     []string
	BranchInclude   []string
	IntervalSecs    int
	Credentials     catalog.Credentials
	WebhookMode     *WebhookMode
	NextRunAt       string
	LastRun         *Run
	CreatedAt       string
	UpdatedAt       string
}

type Secrets struct {
	CredentialsSecretID *uuid.UUID
	CredentialsEnc      *string
	CredentialsRef      *string
	WebhookSecretEnc    *string
	WebhookID           *string
}

const (
	Table               = "forge_connections"
	ColumnCredentials   = "credentials_enc"
	ColumnWebhookSecret = "webhook_secret_enc"
)
