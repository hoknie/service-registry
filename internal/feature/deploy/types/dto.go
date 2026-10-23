package types

import (
	"github.com/google/uuid"

	catalogtypes "svc-registry/internal/feature/catalog/types"
)

type CreateEnvironment struct {
	Key      string
	Names    map[string]string
	Position *int64
}

type UpdateEnvironment struct {
	Names    map[string]string
	Position *int64
}

type NewEnvironment struct {
	ID       uuid.UUID
	Key      string
	Names    map[string]string
	Position int32
}

type EnvironmentChanges struct {
	Names    map[string]string
	Position *int32
}

type ClusterInput struct {
	Mode         *string
	Name         *string
	Environment  *string
	InCluster    *bool
	APIURL       *string
	CAPEM        *string
	Credentials  *catalogtypes.CredentialsInput
	Namespaces   *[]string
	Rules        *[]Rule
	IntervalSecs *int64
	Enabled      *bool
}

type NewCluster struct {
	ID       uuid.UUID
	Settings Settings
	SecretID *uuid.UUID
}

type ClusterUpdate struct {
	Settings Settings
	SecretID *uuid.UUID
}
