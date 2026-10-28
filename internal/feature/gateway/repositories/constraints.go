package repositories

import (
	gatewaytypes "svc-registry/internal/feature/gateway/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
)

var constraints = postgres.Constraints{
	"mcp_folders_parent_slug_idx":                   gatewaytypes.ConflictFolderSlug,
	"mcp_folders_organization_slug_idx":             gatewaytypes.ConflictFolderSlug,
	"mcp_folders_global_slug_idx":                   gatewaytypes.ConflictFolderSlug,
	"mcp_folders_parent_id_fkey":                    apperr.ErrNotFound,
	"mcp_folders_organization_id_fkey":              gatewaytypes.InvalidScope,
	"mcp_folders_slug_check":                        gatewaytypes.InvalidFolder,
	"mcp_folders_name_check":                        gatewaytypes.InvalidFolder,
	"mcp_folders_description_check":                 gatewaytypes.InvalidFolder,
	"mcp_servers_slug_key":                          gatewaytypes.ConflictSlugTaken,
	"mcp_servers_folder_id_fkey":                    apperr.ErrNotFound,
	"mcp_servers_organization_id_fkey":              gatewaytypes.InvalidScope,
	"mcp_servers_credentials_secret_id_fkey":        apperr.ErrNotFound,
	"mcp_servers_slug_check":                        gatewaytypes.InvalidServer(),
	"mcp_servers_title_check":                       gatewaytypes.InvalidServer(),
	"mcp_servers_url_check":                         gatewaytypes.InvalidServer(),
	"mcp_servers_auth_check":                        gatewaytypes.InvalidServer(),
	"mcp_servers_tags_check":                        gatewaytypes.InvalidServer(),
	"mcp_servers_disabled_tools_check":              gatewaytypes.InvalidTools,
	"mcp_server_access_group_id_fkey":               gatewaytypes.InvalidAccess,
	"mcp_server_access_user_id_fkey":                gatewaytypes.InvalidAccess,
	"mcp_server_access_server_id_fkey":              apperr.ErrNotFound,
	"mcp_organizations_organization_id_fkey":        apperr.ErrNotFound,
	"mcp_organizations_description_check":           gatewaytypes.InvalidSpace,
	"mcp_organization_servers_server_id_fkey":       gatewaytypes.InvalidSet,
	"mcp_organization_servers_organization_id_fkey": apperr.ErrNotFound,
	"mcp_proxy_settings_credentials_secret_id_fkey": apperr.ErrNotFound,
	"mcp_proxy_settings_url_check":                  gatewaytypes.InvalidProxy(),
	"mcp_proxy_settings_fields_check":               gatewaytypes.InvalidProxy(),
	"mcp_proxy_settings_server_id_fkey":             apperr.ErrNotFound,
	"mcp_proxy_settings_folder_id_fkey":             apperr.ErrNotFound,
	"mcp_proxy_settings_organization_id_fkey":       apperr.ErrNotFound,
}

func Err(err error) error { return postgres.Translate(err, constraints) }
