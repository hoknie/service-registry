package repositories

import (
	ownershiptypes "svc-registry/internal/feature/ownership/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
)

var constraints = postgres.Constraints{
	"node_owners_node_id_fkey":       apperr.ErrNotFound,
	"node_owners_group_id_fkey":      apperr.ErrNotFound,
	"node_owners_user_id_fkey":       apperr.ErrNotFound,
	"node_owners_node_group_key":     ownershiptypes.InvalidOwners,
	"node_owners_node_user_key":      ownershiptypes.InvalidOwners,
	"node_owners_node_position_key":  ownershiptypes.InvalidOwners,
	"node_owners_position_check":     ownershiptypes.InvalidOwners,
	"node_owner_clears_node_id_fkey": apperr.ErrNotFound,
}
