package repositories

import (
	"maps"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
)

var constraints = postgres.Constraints{
	"nodes_parent_slug_key":                            catalogtypes.ConflictSlugTaken,
	"nodes_parent_id_fkey":                             apperr.ErrNotFound,
	"role_bindings_node_id_fkey":                       apperr.ErrNotFound,
	"project_keys_project_id_fkey":                     apperr.ErrNotFound,
	"secrets_node_id_fkey":                             apperr.ErrNotFound,
	"secrets_name_key":                                 catalogtypes.ConflictSecretNameTaken,
	"repo_sync_connections_credentials_secret_id_fkey": catalogtypes.ConflictSecretInUse,
	"knowledge_sources_credentials_secret_id_fkey":     catalogtypes.ConflictSecretInUse,
	"clusters_credentials_secret_id_fkey":              catalogtypes.ConflictSecretInUse,
	"role_bindings_group_organization_check":           catalogtypes.InvalidUnknownSubject,
	"nodes_organization_groups_check":                  catalogtypes.ConflictOrganizationHasGroups,
	"nodes_group_organization_check":                   catalogtypes.ConflictOrganizationGroupsBound,
}

var nodeDeleteConstraints = func() postgres.Constraints {
	c := maps.Clone(constraints)
	c["nodes_parent_id_fkey"] = catalogtypes.ConflictNodeNotEmpty
	return c
}()
