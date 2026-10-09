package access

import "errors"

type Invalid int

const (
	InvalidEmail Invalid = iota + 1
	InvalidDisplayName
	InvalidPasswordTooShort
	InvalidPasswordTooLong
	InvalidStatus
	InvalidGroupName
	InvalidPagination
	InvalidTokenName
	InvalidScopes
	InvalidTokenLifetime
	InvalidTokenPrefix
)

func (i Invalid) Code() string {
	switch i {
	case InvalidEmail:
		return "validation.invalid_email"
	case InvalidDisplayName:
		return "validation.invalid_display_name"
	case InvalidPasswordTooShort:
		return "validation.password_too_short"
	case InvalidPasswordTooLong:
		return "validation.password_too_long"
	case InvalidStatus:
		return "validation.invalid_status"
	case InvalidGroupName:
		return "validation.invalid_group_name"
	case InvalidPagination:
		return "validation.invalid_pagination"
	case InvalidTokenName:
		return "validation.invalid_token_name"
	case InvalidScopes:
		return "validation.invalid_scopes"
	case InvalidTokenLifetime:
		return "validation.invalid_token_lifetime"
	case InvalidTokenPrefix:
		return "validation.invalid_token_prefix"
	}
	return "validation.invalid"
}

func (i Invalid) Message() string {
	switch i {
	case InvalidEmail:
		return "email must be one address with a non-empty local part and domain, at most 254 characters"
	case InvalidDisplayName:
		return "display name must be 1 to 100 characters without control characters"
	case InvalidPasswordTooShort:
		return "password must be at least 8 characters"
	case InvalidPasswordTooLong:
		return "password must be at most 256 characters"
	case InvalidStatus:
		return `status must be "active" or "disabled"`
	case InvalidGroupName:
		return "group name must be 1 to 100 characters without control characters"
	case InvalidPagination:
		return "limit must be 1..=200 and offset a non-negative integer"
	case InvalidTokenName:
		return "token name must be 1 to 100 characters without control characters"
	case InvalidScopes:
		return "scopes must be a non-empty list of distinct read, write, admin, mcp; admin only for superadmins"
	case InvalidTokenLifetime:
		return "expires_in_days must be 1..=PAT_MAX_LIFETIME_DAYS; only superadmins may omit it"
	case InvalidTokenPrefix:
		return "prefix must be at most 12 characters"
	}
	return "invalid input"
}

func (i Invalid) Error() string { return i.Message() }

type Conflict int

const (
	ConflictEmailTaken Conflict = iota + 1
	ConflictGroupNameTaken
	ConflictLastSuperadmin
	ConflictMembershipManaged
	ConflictLastLoginMethod
)

func (c Conflict) Code() string {
	switch c {
	case ConflictEmailTaken:
		return "conflict.email_taken"
	case ConflictGroupNameTaken:
		return "conflict.group_name_taken"
	case ConflictLastSuperadmin:
		return "conflict.last_superadmin"
	case ConflictMembershipManaged:
		return "conflict.membership_managed"
	case ConflictLastLoginMethod:
		return "conflict.last_login_method"
	}
	return "conflict.unknown"
}

func (c Conflict) Message() string {
	switch c {
	case ConflictEmailTaken:
		return "a user with this email already exists"
	case ConflictGroupNameTaken:
		return "a group with this name already exists"
	case ConflictLastSuperadmin:
		return "the change would leave no active superadmin"
	case ConflictMembershipManaged:
		return "the membership is managed by a login provider"
	case ConflictLastLoginMethod:
		return "the last way to sign in cannot be removed"
	}
	return "conflict"
}

func (c Conflict) Error() string { return c.Message() }

var (
	ErrNotFound    = errors.New("not found")
	ErrUnavailable = errors.New("database is unavailable")
)

type InternalError struct{ Detail string }

func (e *InternalError) Error() string { return "internal: " + e.Detail }
