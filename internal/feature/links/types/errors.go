package types

import (
	"errors"
	"fmt"

	"svc-registry/internal/platform/apperr"
)

var (
	InvalidKindKey      = apperr.Invalid("validation.invalid_link_kind_key", "key must be 1 to 63 characters a-z, 0-9 and '-', not starting or ending with '-'")
	InvalidKindNames    = apperr.Invalid("validation.invalid_link_kind_names", `names may have only "en", "es", "ru" and "zh", at least one non-empty, each at most 100 characters without control characters`)
	InvalidIcon         = apperr.Invalid("validation.invalid_link_icon", "icon must be an icon name of 1 to 64 characters a-z, 0-9 and single '-'; icon_url must be null or an https:// URL of at most 2048 characters")
	InvalidPosition     = apperr.Invalid("validation.invalid_position", "position must be an integer 0..=10000")
	InvalidTemplate     = apperr.Invalid("validation.invalid_template", "template must be 1 to 2048 characters of text and {name|filter} substitutions")
	UnknownVariable     = apperr.Invalid("validation.unknown_variable", "the template uses an unknown variable")
	InvalidLinkURL      = apperr.Invalid("validation.invalid_link_url", "the expanded template must be an absolute http(s) URL with a host of at most 2048 characters")
	UnknownKind         = apperr.Invalid("validation.unknown_link_kind", "no link kind with this key")
	TooManyTemplates    = apperr.Invalid("validation.too_many_link_templates", "a node has at most 50 link templates")
	InvalidProject      = apperr.Invalid("validation.invalid_project", "project_id must name a project in this subtree")
	InvalidVars         = apperr.Invalid("validation.invalid_vars", "vars must be at most 32 pairs with label-like keys and values of at most 1024 characters without control characters")
	InvalidTemplateIcon = apperr.Invalid("validation.invalid_link_icon", `icon must be null, {"url": "https://…"} of at most 2048 characters or {"file": "<uploaded id>"}`)
	InvalidLinkTitle    = apperr.Invalid("validation.invalid_link_title", "title must be null or 1 to 100 characters without control characters")
	InvalidIconFormat   = apperr.Invalid("validation.invalid_icon_format", "the icon must be a PNG, WebP, ICO or SVG image")
	InvalidKindOrder    = apperr.Invalid("validation.invalid_link_kind_order", "keys must list link kind keys, each once, at most 1000")
)

var (
	ConflictKindTaken       = apperr.Conflicting("conflict.link_kind_taken", "a link kind with this key already exists")
	ConflictKindInUse       = apperr.Conflicting("conflict.link_kind_in_use", "the link kind is used by a link template")
	ConflictUploadsDisabled = apperr.Conflicting("conflict.uploads_disabled", "file uploads are disabled (UPLOADS_DIR=off)")
	ConflictKindOrderStale  = apperr.Conflicting("conflict.link_kind_order_stale", "the keys do not match the current link kinds; reload the list")
)

type TemplateError struct {
	Invalid *apperr.Error
	Pos     int
	Detail  string
}

func (e *TemplateError) Code() string { return e.Invalid.Code }

func (e *TemplateError) Message() string {
	return fmt.Sprintf("template: %s at character %d", e.Detail, e.Pos)
}

func (e *TemplateError) Error() string { return e.Message() }

var (
	ErrIconTooLarge = errors.New("the icon is larger than 64 KiB")
)

func (e *TemplateError) AppError() *apperr.Error { return apperr.Invalid(e.Code(), e.Message()) }
