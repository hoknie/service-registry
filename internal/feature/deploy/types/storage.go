package types

import (
	"github.com/google/uuid"
)

type LegacyCluster struct {
	ID   uuid.UUID
	Name string
	Enc  *string
	Ref  *string
}

type ProjectRef struct {
	ID      uuid.UUID
	Observe bool
}

type StoredCredentials struct {
	SecretID *uuid.UUID
}

type NewAgentToken struct {
	ID        uuid.UUID
	ClusterID uuid.UUID
	Prefix    string
	Hash      [32]byte
}

type AgentToken struct {
	ID        uuid.UUID
	Prefix    string
	CreatedAt string
	ExpiresAt *string
}
