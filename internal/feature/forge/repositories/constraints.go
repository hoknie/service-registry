package repositories

import (
	forgetypes "svc-registry/internal/feature/forge/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
)

var constraints = postgres.Constraints{
	"forge_connections_owner_key":        forgetypes.ConflictOwnerTaken,
	"forge_connections_node_id_fkey":     apperr.ErrNotFound,
	"forge_repositories_project_id_fkey": apperr.ErrNotFound,
	"forge_groups_node_id_fkey":          apperr.ErrNotFound,
}
