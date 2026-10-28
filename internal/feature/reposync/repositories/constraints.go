package repositories

import (
	reposynctypes "svc-registry/internal/feature/reposync/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
)

var constraints = postgres.Constraints{
	"repo_sync_connections_owner_key":        reposynctypes.ConflictOwnerTaken,
	"repo_sync_connections_node_id_fkey":     apperr.ErrNotFound,
	"repo_sync_repositories_project_id_fkey": apperr.ErrNotFound,
	"repo_sync_groups_node_id_fkey":          apperr.ErrNotFound,
}
