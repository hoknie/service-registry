package links

import (
	"errors"
	"fmt"

	"svc-registry/internal/platform/apperr"
)

type Invalid int

const (
	InvalidKindKey Invalid = iota + 1
	InvalidKindNames
	InvalidIcon
	InvalidPosition
	InvalidTemplate
	UnknownVariable
	InvalidLinkURL
	UnknownKind
	TooManyTemplates
	InvalidProject
	InvalidVars
	InvalidTemplateIcon
	InvalidLinkTitle
	InvalidIconFormat
)

func (i Invalid) Code() string {
	switch i {
	case InvalidKindKey:
		return "validation.invalid_link_kind_key"
	case InvalidKindNames:
		return "validation.invalid_link_kind_names"
	case InvalidIcon:
		return "validation.invalid_link_icon"
	case InvalidPosition:
		return "validation.invalid_position"
	case InvalidTemplate:
		return "validation.invalid_template"
	case UnknownVariable:
		return "validation.unknown_variable"
	case InvalidLinkURL:
		return "validation.invalid_link_url"
	case UnknownKind:
		return "validation.unknown_link_kind"
	case TooManyTemplates:
		return "validation.too_many_link_templates"
	case InvalidProject:
		return "validation.invalid_project"
	case InvalidVars:
		return "validation.invalid_vars"
	case InvalidTemplateIcon:
		return "validation.invalid_link_icon"
	case InvalidLinkTitle:
		return "validation.invalid_link_title"
	case InvalidIconFormat:
		return "validation.invalid_icon_format"
	}
	return "validation.invalid"
}

func (i Invalid) Message() string {
	switch i {
	case InvalidKindKey:
		return "key must be 1 to 63 characters a-z, 0-9 and '-', not starting or ending with '-'"
	case InvalidKindNames:
		return `names must have "en", "es", "ru" and "zh", each 1 to 100 characters without control characters`
	case InvalidIcon:
		return `icon must be one of "link", "logs", "dashboard", "errors", "alerts", "traces", "runbook", "docs"`
	case InvalidPosition:
		return "position must be an integer 0..=10000"
	case InvalidTemplate:
		return "template must be 1 to 2048 characters of text and {name|filter} substitutions"
	case UnknownVariable:
		return "the template uses an unknown variable"
	case InvalidLinkURL:
		return "the expanded template must be an absolute http(s) URL with a host of at most 2048 characters"
	case UnknownKind:
		return "no link kind with this key"
	case TooManyTemplates:
		return "a node has at most 50 link templates"
	case InvalidProject:
		return "project_id must name a project in this subtree"
	case InvalidVars:
		return "vars must be at most 32 pairs with label-like keys and values of at most 1024 characters without control characters"
	case InvalidTemplateIcon:
		return `icon must be null, {"url": "https://…"} of at most 2048 characters or {"file": "<uploaded id>"}`
	case InvalidLinkTitle:
		return "title must be null or 1 to 100 characters without control characters"
	case InvalidIconFormat:
		return "the icon must be a PNG, WebP, ICO or SVG image"
	}
	return "invalid input"
}

func (i Invalid) Error() string { return i.Message() }

type TemplateError struct {
	Invalid Invalid
	Pos     int
	Detail  string
}

func (e *TemplateError) Code() string { return e.Invalid.Code() }

func (e *TemplateError) Message() string {
	return fmt.Sprintf("template: %s at character %d", e.Detail, e.Pos)
}

func (e *TemplateError) Error() string { return e.Message() }

type Conflict int

const (
	ConflictKindTaken Conflict = iota + 1
	ConflictKindInUse
	ConflictUploadsDisabled
)

func (c Conflict) Code() string {
	switch c {
	case ConflictKindTaken:
		return "conflict.link_kind_taken"
	case ConflictKindInUse:
		return "conflict.link_kind_in_use"
	case ConflictUploadsDisabled:
		return "conflict.uploads_disabled"
	}
	return "conflict.unknown"
}

func (c Conflict) Message() string {
	switch c {
	case ConflictKindTaken:
		return "a link kind with this key already exists"
	case ConflictKindInUse:
		return "the link kind is used by a link template"
	case ConflictUploadsDisabled:
		return "file uploads are disabled (UPLOADS_DIR=off)"
	}
	return "conflict"
}

func (c Conflict) Error() string { return c.Message() }

var (
	ErrIconTooLarge = errors.New("the icon is larger than 64 KiB")
	ErrNotFound     = apperr.Sentinel(apperr.NotFound, "not found")
	ErrUnavailable  = apperr.Sentinel(apperr.Unavailable, "database is unavailable")
)

type InternalError struct{ Detail string }

func (e *InternalError) Error() string { return "internal: " + e.Detail }

func (i Invalid) AppError() *apperr.Error { return apperr.Invalid(i.Code(), i.Message()) }

func (c Conflict) AppError() *apperr.Error { return apperr.Conflicting(c.Code(), c.Message()) }

func (e *InternalError) AppError() *apperr.Error { return apperr.InternalDetail(e.Detail) }

func (e *TemplateError) AppError() *apperr.Error { return apperr.Invalid(e.Code(), e.Message()) }
