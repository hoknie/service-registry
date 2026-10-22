package repositories

import (
	linkstypes "svc-registry/internal/feature/links/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
)

var constraints = postgres.Constraints{
	"link_kinds_key_key":          linkstypes.ConflictKindTaken,
	"link_templates_kind_id_fkey": linkstypes.ConflictKindInUse,
	"link_templates_node_id_fkey": apperr.ErrNotFound,
	"node_vars_node_id_fkey":      apperr.ErrNotFound,
}
