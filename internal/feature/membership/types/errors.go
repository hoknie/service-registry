package types

import "svc-registry/internal/platform/apperr"

var (
	NotOrganization      = apperr.Invalid("validation.not_an_organization", "members and invitations belong to organizations at the top level of the catalog")
	InvalidMember        = apperr.Invalid("validation.invalid_member", `member: {"user_id" or "email", "role": "viewer"|"editor"|"admin"}`)
	InvalidInvitation    = apperr.Invalid("validation.invalid_invitation", `invitation: {"email", "role": "viewer"|"editor"|"admin", "locale": "en"|"es"|"ru"|"zh"}`)
	InvalidPagination    = apperr.Invalid("validation.invalid_pagination", "limit is 1 to 200 and cursor comes from a previous page")
	LastAdmin            = apperr.Conflicting("conflict.last_organization_admin", "the organization keeps at least one admin")
	AlreadyMember        = apperr.Conflicting("conflict.already_member", "the person is already a member of the organization")
	InvitationPending    = apperr.Conflicting("conflict.invitation_pending", "an invitation for this address is already waiting; send it again instead")
	TooManyInvitations   = apperr.Conflicting("conflict.too_many_invitations", "an organization keeps at most 100 waiting invitations")
	InvitationClosed     = apperr.Conflicting("conflict.invitation_closed", "the invitation was accepted or revoked")
	NotPending           = apperr.Conflicting("invitation.not_pending", "the invitation expired, was accepted or was revoked")
	SignInRequired       = apperr.Conflicting("invitation.sign_in_required", "an account with this address exists; sign in to accept the invitation")
	EmailMismatch        = apperr.Refusing("invitation.email_mismatch", "the invitation is for another address")
	PasswordLoginOff     = apperr.Refusing("auth.password_login_disabled", "sign-in with a password is turned off")
	ErrUnknownMembership = apperr.InternalDetail("unknown membership operation")
)
