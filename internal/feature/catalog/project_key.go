package catalog

import "github.com/google/uuid"

type KeyStatus string

const (
	KeyActive  KeyStatus = "active"
	KeyExpired KeyStatus = "expired"
	KeyRevoked KeyStatus = "revoked"
)

func ParseKeyStatus(s string) (KeyStatus, bool) {
	switch KeyStatus(s) {
	case KeyActive, KeyExpired, KeyRevoked:
		return KeyStatus(s), true
	}
	return "", false
}

type ProjectKey struct {
	ID         uuid.UUID
	ProjectID  uuid.UUID
	Prefix     string
	CreatedAt  string
	LastUsedAt *string
	ExpiresAt  *string
	RevokedAt  *string
	Status     KeyStatus
}
