package types

import (
	"github.com/google/uuid"

	catalogtypes "svc-registry/internal/feature/catalog/types"
)

type Exclusion struct {
	ExternalID string
	FullPath   string
	ProjectID  *uuid.UUID
	CreatedAt  string
}

type Adoption struct {
	ExternalID string
	ProjectID  uuid.UUID
}

type Unlinked struct {
	ConnectionID uuid.UUID
	ExternalID   string
	FullPath     string
	ProjectID    uuid.UUID
}

type RemoteSource struct {
	Kind     Kind
	WebURL   string
	APIURL   string
	SecretID *uuid.UUID
}

type ProjectLink struct {
	ConnectionID uuid.UUID
	NodeID       uuid.UUID
	Owner        string
	Kind         Kind
	WebURL       string
	APIURL       string
	Credentials  catalogtypes.Credentials
	SyncedAt     string
}

func WithoutExcluded(repos []RemoteRepo, excluded map[string]bool) []RemoteRepo {
	if len(excluded) == 0 {
		return repos
	}
	out := repos[:0:0]
	for _, r := range repos {
		if !excluded[r.ExternalID] {
			out = append(out, r)
		}
	}
	return out
}
