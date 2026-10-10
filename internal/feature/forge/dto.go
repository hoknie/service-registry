package forge

import (
	"time"

	"github.com/google/uuid"

	"svc-registry/internal/feature/catalog"
)

type CreateConnection struct {
	Kind            string
	APIURL          *string
	OwnerPath       string
	MirrorSubgroups *bool
	IncludeArchived *bool
	IncludeForks    *bool
	NameInclude     *[]string
	NameExclude     *[]string
	BranchInclude   *[]string
	IntervalSecs    *int64
	Credentials     *catalog.CredentialsInput
}

type UpdateConnection struct {
	APIURL          *string
	OwnerPath       *string
	MirrorSubgroups *bool
	IncludeArchived *bool
	IncludeForks    *bool
	NameInclude     *[]string
	NameExclude     *[]string
	BranchInclude   *[]string
	IntervalSecs    *int64
	Credentials     *catalog.CredentialsInput
}

type Settings struct {
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
}

type NewConnection struct {
	ID       uuid.UUID
	NodeID   uuid.UUID
	Settings Settings
	SecretID uuid.UUID
}

type Webhook struct {
	Mode      *WebhookMode
	SecretEnc *string
	HookID    *string
}

type NewRun struct {
	ID           uuid.UUID
	ConnectionID uuid.UUID
	Trigger      Trigger
}

type RepoRecord struct {
	ProjectID    uuid.UUID
	ConnectionID uuid.UUID
	Remote       RemoteRepo
	Details      *Details
}

type SecretRow struct {
	ID               uuid.UUID
	CredentialsEnc   *string
	WebhookSecretEnc *string
}

type Lease struct {
	Secs int
	Next *time.Time
}
