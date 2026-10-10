package deploy

import (
	"github.com/google/uuid"

	"svc-registry/internal/feature/forge"
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
	Name         *string
	Environment  *string
	InCluster    *bool
	APIURL       *string
	CAPEM        *string
	Credentials  *forge.CredentialsInput
	Namespaces   *[]string
	Rules        *[]Rule
	IntervalSecs *int64
	Enabled      *bool
}

type NewCluster struct {
	ID          uuid.UUID
	Settings    Settings
	Credentials forge.StoredCredentials
}

type ClusterUpdate struct {
	Settings    Settings
	Credentials *forge.StoredCredentials
}
