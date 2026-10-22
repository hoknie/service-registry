package repositories

import (
	deploytypes "svc-registry/internal/feature/deploy/types"
	"svc-registry/internal/platform/postgres"
)

var constraints = postgres.Constraints{
	"environments_key_key": deploytypes.ConflictEnvironmentTaken,
	"clusters_name_key":    deploytypes.ConflictClusterNameTaken,
}
