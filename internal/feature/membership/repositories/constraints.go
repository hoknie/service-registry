package repositories

import (
	membershiptypes "svc-registry/internal/feature/membership/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
)

var constraints = postgres.Constraints{
	"organization_members_pkey":                     membershiptypes.AlreadyMember,
	"organization_members_organization_id_fkey":     apperr.ErrNotFound,
	"organization_members_user_id_fkey":             apperr.ErrNotFound,
	"organization_members_top_level_check":          membershiptypes.NotOrganization,
	"organization_members_source_check":             membershiptypes.InvalidMember,
	"organization_invitations_pending_idx":          membershiptypes.InvitationPending,
	"organization_invitations_organization_id_fkey": apperr.ErrNotFound,
	"organization_invitations_email_check":          membershiptypes.InvalidInvitation,
	"organization_invitations_role_check":           membershiptypes.InvalidInvitation,
	"organization_invitations_locale_check":         membershiptypes.InvalidInvitation,
}

func Err(err error) error { return postgres.Translate(err, constraints) }
