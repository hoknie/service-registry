package types

import (
	"errors"
	"fmt"
	"time"

	"svc-registry/internal/platform/apperr"
)

var (
	InvalidKind         = apperr.Invalid("validation.invalid_hosting", `kind must be "github", "gitlab", "forgejo" or "gitea"`)
	InvalidAPIURL       = apperr.Invalid("validation.invalid_api_url", "api_url must be an absolute http(s) address without query or fragment (required for forgejo and gitea)")
	InvalidOwnerPath    = apperr.Invalid("validation.invalid_owner_path", "owner_path must be 1 to 255 characters of letters, digits, '.', '_', '-' (GitLab groups: segments joined by '/')")
	InvalidNamePatterns = apperr.Invalid("validation.invalid_name_patterns", "name patterns: at most 32 glob patterns of 1 to 100 characters")
	InvalidInterval     = apperr.Invalid("validation.invalid_sync_interval", "interval_secs must be 60..=86400")
	InvalidCredentials  = apperr.Invalid("validation.invalid_credentials", `credentials must be {"token": "..."} or {"token_ref": "env:NAME" | "file:/absolute/path"}`)
	InvalidContainer    = apperr.Invalid("validation.invalid_repo_sync_container", "a repository sync connection belongs to an organization or a folder")
	InvalidWebhookMode  = apperr.Invalid("validation.invalid_webhook_mode", `mode must be "register" or "manual"`)
)

var (
	ConflictOwnerTaken        = apperr.Conflicting("conflict.repo_sync_owner_taken", "this hosting owner is already connected")
	ConflictSecretsKeyMissing = apperr.Conflicting("conflict.secrets_key_missing", "SECRETS_KEYS is not configured: secrets cannot be stored")
	ConflictPublicURLMissing  = apperr.Conflicting("conflict.public_url_missing", "PUBLIC_URL is not configured: the webhook address is unknown")
	ConflictNotLinked         = apperr.Conflicting("conflict.repo_sync_not_linked", "the project is not linked to a hosting repository")
)

var (
	ErrUnauthorized           error = &failure{code: "hosting.unauthorized", message: "hosting rejected the credentials"}
	ErrOwnerNotFound          error = &failure{code: "hosting.owner_not_found", message: "hosting owner not found"}
	ErrCredentialsUnavailable error = &failure{code: "hosting.credentials_unavailable", message: "credentials are unavailable"}
	ErrInterrupted            error = &failure{code: "repo_sync.interrupted", message: "run interrupted"}
)

type failure struct{ code, message string }

func (f *failure) Error() string { return f.message }

func (f *failure) AppError() *apperr.Error {
	if f.code == "repo_sync.interrupted" {
		return apperr.UnavailableDB()
	}
	return &apperr.Error{Kind: apperr.Unprocessable, Code: f.code, Message: f.message}
}

type RateLimited struct{ Reset time.Time }

func (e *RateLimited) AppError() *apperr.Error {
	out := &apperr.Error{Kind: apperr.UpstreamRateLimited, Code: "hosting.rate_limited", Message: e.Error()}
	if !e.Reset.IsZero() {
		if secs := time.Until(e.Reset).Seconds(); secs > 0 {
			out.RetryAfterSecs = uint64(secs) + 1
		}
	}
	return out
}

func (e *RateLimited) Error() string {
	if e.Reset.IsZero() {
		return "hosting rate limit exceeded"
	}
	return "hosting rate limit exceeded until " + e.Reset.UTC().Format(time.RFC3339)
}

type Upstream struct {
	Status int
	Detail string
}

func (e *Upstream) AppError() *apperr.Error {
	return &apperr.Error{Kind: apperr.BadGateway, Code: "hosting.upstream_error", Message: e.Error()}
}

func (e *Upstream) Error() string {
	return fmt.Sprintf("hosting error (status %d): %s", e.Status, e.Detail)
}

func StopsSync(err error) bool {
	_, limited := errors.AsType[*RateLimited](err)
	return limited || errors.Is(err, ErrInterrupted) || errors.Is(err, ErrUnauthorized)
}

func FailureCode(err error) string {
	switch {
	case errors.Is(err, ErrUnauthorized):
		return "hosting.unauthorized"
	case errors.Is(err, ErrOwnerNotFound):
		return "hosting.owner_not_found"
	case errors.Is(err, ErrCredentialsUnavailable):
		return "hosting.credentials_unavailable"
	case errors.Is(err, ErrInterrupted):
		return "repo_sync.interrupted"
	case errors.Is(err, ErrRepoNotFound):
		return "hosting.not_found"
	}
	if _, ok := errors.AsType[*RateLimited](err); ok {
		return "hosting.rate_limited"
	}
	if _, ok := errors.AsType[*Upstream](err); ok {
		return "hosting.upstream_error"
	}
	return ""
}

var ErrRepoNotFound error = &failure{code: "hosting.not_found", message: "repository or commit not found on the hosting"}

var ErrArchiveUnsupported = errors.New("hosting does not serve repository archives")
