package forge

import (
	"errors"
	"fmt"
	"time"
)

type Invalid int

const (
	InvalidKind Invalid = iota + 1
	InvalidAPIURL
	InvalidOwnerPath
	InvalidNamePatterns
	InvalidInterval
	InvalidCredentials
	InvalidContainer
	InvalidWebhookMode
)

func (i Invalid) Code() string {
	switch i {
	case InvalidKind:
		return "validation.invalid_forge"
	case InvalidAPIURL:
		return "validation.invalid_api_url"
	case InvalidOwnerPath:
		return "validation.invalid_owner_path"
	case InvalidNamePatterns:
		return "validation.invalid_name_patterns"
	case InvalidInterval:
		return "validation.invalid_sync_interval"
	case InvalidCredentials:
		return "validation.invalid_credentials"
	case InvalidContainer:
		return "validation.invalid_forge_container"
	case InvalidWebhookMode:
		return "validation.invalid_webhook_mode"
	}
	return "validation.invalid"
}

func (i Invalid) Message() string {
	switch i {
	case InvalidKind:
		return `kind must be "github", "gitlab", "forgejo" or "gitea"`
	case InvalidAPIURL:
		return "api_url must be an absolute http(s) address without query or fragment (required for forgejo and gitea)"
	case InvalidOwnerPath:
		return "owner_path must be 1 to 255 characters of letters, digits, '.', '_', '-' (GitLab groups: segments joined by '/')"
	case InvalidNamePatterns:
		return "name patterns: at most 32 glob patterns of 1 to 100 characters"
	case InvalidInterval:
		return "interval_secs must be 60..=86400"
	case InvalidCredentials:
		return `credentials must be {"token": "..."} or {"token_ref": "env:NAME" | "file:/absolute/path"}`
	case InvalidContainer:
		return "a forge connection belongs to an organization or a folder"
	case InvalidWebhookMode:
		return `mode must be "register" or "manual"`
	}
	return "invalid input"
}

func (i Invalid) Error() string { return i.Message() }

type Conflict int

const (
	ConflictOwnerTaken Conflict = iota + 1
	ConflictSecretsKeyMissing
	ConflictPublicURLMissing
)

func (c Conflict) Code() string {
	switch c {
	case ConflictOwnerTaken:
		return "conflict.forge_owner_taken"
	case ConflictSecretsKeyMissing:
		return "conflict.secrets_key_missing"
	case ConflictPublicURLMissing:
		return "conflict.public_url_missing"
	}
	return "conflict.unknown"
}

func (c Conflict) Message() string {
	switch c {
	case ConflictOwnerTaken:
		return "this forge owner is already connected"
	case ConflictSecretsKeyMissing:
		return "SECRETS_KEYS is not configured: secrets cannot be stored"
	case ConflictPublicURLMissing:
		return "PUBLIC_URL is not configured: the webhook address is unknown"
	}
	return "conflict"
}

func (c Conflict) Error() string { return c.Message() }

var (
	ErrUnauthorized           = errors.New("forge rejected the credentials")
	ErrOwnerNotFound          = errors.New("forge owner not found")
	ErrCredentialsUnavailable = errors.New("credentials are unavailable")
	ErrInterrupted            = errors.New("run interrupted")
)

type RateLimited struct{ Reset time.Time }

func (e *RateLimited) Error() string {
	if e.Reset.IsZero() {
		return "forge rate limit exceeded"
	}
	return "forge rate limit exceeded until " + e.Reset.UTC().Format(time.RFC3339)
}

type Upstream struct {
	Status int
	Detail string
}

func (e *Upstream) Error() string {
	return fmt.Sprintf("forge error (status %d): %s", e.Status, e.Detail)
}

func FailureCode(err error) string {
	var limited *RateLimited
	var upstream *Upstream
	switch {
	case errors.Is(err, ErrUnauthorized):
		return "forge.unauthorized"
	case errors.Is(err, ErrOwnerNotFound):
		return "forge.owner_not_found"
	case errors.Is(err, ErrCredentialsUnavailable):
		return "forge.credentials_unavailable"
	case errors.Is(err, ErrInterrupted):
		return "forge.interrupted"
	case errors.Is(err, ErrRepoNotFound):
		return "forge.not_found"
	case errors.As(err, &limited):
		return "forge.rate_limited"
	case errors.As(err, &upstream):
		return "forge.upstream_error"
	}
	return ""
}

var ErrRepoNotFound = errors.New("repository or commit not found on the forge")

var (
	ErrNotFound    = errors.New("not found")
	ErrUnavailable = errors.New("database is unavailable")
)

type InternalError struct{ Detail string }

func (e *InternalError) Error() string { return "internal: " + e.Detail }
