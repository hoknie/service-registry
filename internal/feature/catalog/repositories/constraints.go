package repositories

import (
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
)

var constraints = postgres.Constraints{
	"nodes_parent_slug_key":                        catalogtypes.ConflictSlugTaken,
	"nodes_parent_id_fkey":                         apperr.ErrNotFound,
	"role_bindings_node_id_fkey":                   apperr.ErrNotFound,
	"project_keys_project_id_fkey":                 apperr.ErrNotFound,
	"secrets_node_id_fkey":                         apperr.ErrNotFound,
	"secrets_name_key":                             catalogtypes.ConflictSecretNameTaken,
	"forge_connections_credentials_secret_id_fkey": catalogtypes.ConflictSecretInUse,
	"knowledge_sources_credentials_secret_id_fkey": catalogtypes.ConflictSecretInUse,
	"clusters_credentials_secret_id_fkey":          catalogtypes.ConflictSecretInUse,
}
