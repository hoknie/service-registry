package types

import (
	"svc-registry/internal/platform/apperr"
)

var (
	InvalidEmail            = apperr.Invalid("validation.invalid_email", "email must be one address with a non-empty local part and domain, at most 254 characters")
	InvalidDisplayName      = apperr.Invalid("validation.invalid_display_name", "display name must be 1 to 100 characters without control characters")
	InvalidPasswordTooShort = apperr.Invalid("validation.password_too_short", "password must be at least 8 characters")
	InvalidPasswordTooLong  = apperr.Invalid("validation.password_too_long", "password must be at most 256 characters")
	InvalidStatus           = apperr.Invalid("validation.invalid_status", `status must be "active" or "disabled"`)
	InvalidGroupName        = apperr.Invalid("validation.invalid_group_name", "group name must be 1 to 100 characters without control characters")
	InvalidPagination       = apperr.Invalid("validation.invalid_pagination", "limit must be 1..=200 and offset a non-negative integer")
	InvalidTokenName        = apperr.Invalid("validation.invalid_token_name", "token name must be 1 to 100 characters without control characters")
	InvalidScopes           = apperr.Invalid("validation.invalid_scopes", "scopes must be a non-empty list of distinct read, write, admin, mcp; admin only for superadmins")
	InvalidTokenLifetime    = apperr.Invalid("validation.invalid_token_lifetime", "expires_in_days must be 1..=PAT_MAX_LIFETIME_DAYS; only superadmins may omit it")
	InvalidTokenPrefix      = apperr.Invalid("validation.invalid_token_prefix", "prefix must be at most 12 characters")
)

var (
	ConflictEmailTaken        = apperr.Conflicting("conflict.email_taken", "a user with this email already exists")
	ConflictGroupNameTaken    = apperr.Conflicting("conflict.group_name_taken", "a group with this name already exists")
	ConflictLastSuperadmin    = apperr.Conflicting("conflict.last_superadmin", "the change would leave no active superadmin")
	ConflictMembershipManaged = apperr.Conflicting("conflict.membership_managed", "the membership is managed by a login provider")
	ConflictLastLoginMethod   = apperr.Conflicting("conflict.last_login_method", "the last way to sign in cannot be removed")
	ConflictSelfService       = apperr.Conflicting("conflict.self_service", "you cannot make your own account a service account")
	ConflictUserDisabled      = apperr.Conflicting("conflict.user_disabled", "the user is disabled")
)
