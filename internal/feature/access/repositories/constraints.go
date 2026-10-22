package repositories

import (
	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
)

var constraints = postgres.Constraints{
	"users_email_lower_key":               accesstypes.ConflictEmailTaken,
	"groups_name_lower_key":               accesstypes.ConflictGroupNameTaken,
	"group_members_group_id_fkey":         apperr.ErrNotFound,
	"group_members_user_id_fkey":          apperr.ErrNotFound,
	"personal_access_tokens_user_id_fkey": apperr.ErrNotFound,
	"user_identities_subject_key":         accesstypes.RefuseIdentityTaken,
}
