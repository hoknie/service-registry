package knowledge

import (
	"fmt"

	"svc-registry/internal/platform/apperr"
)

type Invalid int

const (
	InvalidSettings Invalid = iota + 1
	InvalidSearch
	InvalidBranch
	InvalidSource
	InvalidPath
	InvalidSearchMode
	InvalidScanFilter
)

func (i Invalid) Code() string {
	switch i {
	case InvalidSettings:
		return "validation.invalid_knowledge_settings"
	case InvalidSearch:
		return "validation.invalid_search"
	case InvalidBranch:
		return "validation.invalid_branch"
	case InvalidSource:
		return "validation.invalid_knowledge_source"
	case InvalidPath:
		return "validation.knowledge_path_not_allowed"
	case InvalidSearchMode:
		return "validation.search_mode_unavailable"
	case InvalidScanFilter:
		return "validation.invalid_scan_filter"
	}
	return "validation.invalid"
}

func (i Invalid) Message() string {
	switch i {
	case InvalidSettings:
		return "include, exclude and branches must be lists of at most 50 valid glob patterns of 1 to 200 characters (include may be null)"
	case InvalidSearch:
		return "q must be 1 to 200 characters, limit 1..=50 and cursor one returned by a previous search"
	case InvalidBranch:
		return "branch must be a valid branch name"
	case InvalidSource:
		return `source must be {"kind":"remote","forge","url","api_url"?,"credentials"?} or {"kind":"local_dir"|"local_git","path"}`
	case InvalidPath:
		return "path must be an absolute path inside one of KNOWLEDGE_LOCAL_ROOTS"
	case InvalidSearchMode:
		return "mode must be one of the modes the search engine offers (GET /api/v1/knowledge/search/modes)"
	case InvalidScanFilter:
		return "project must be a UUID, kind collect|index, trigger schedule|manual, status a comma-separated list of ok, unchanged, warning, failed"
	}
	return "invalid input"
}

func (i Invalid) Error() string { return i.Message() }

type Conflict int

const (
	ConflictNotSynced Conflict = iota + 1
	ConflictLocalDisabled
)

func (c Conflict) Code() string {
	switch c {
	case ConflictNotSynced:
		return "conflict.knowledge_not_synced"
	case ConflictLocalDisabled:
		return "conflict.local_sources_disabled"
	}
	return "conflict.unknown"
}

func (c Conflict) Message() string {
	switch c {
	case ConflictNotSynced:
		return "the project has no documentation source and is not synchronized with a forge"
	case ConflictLocalDisabled:
		return "local documentation sources are off: KNOWLEDGE_LOCAL_ROOTS is empty"
	}
	return "conflict"
}

func (c Conflict) Error() string { return c.Message() }

var (
	ErrNotFound    = apperr.Sentinel(apperr.NotFound, "not found")
	ErrUnavailable = apperr.Sentinel(apperr.Unavailable, "database is unavailable")

	ErrSearchUnavailable error = searchUnavailable{}

	ErrEmbeddingsUnavailable = fmt.Errorf("%w: embeddings API", ErrSearchUnavailable)
	ErrEmbeddingsDimensions  = fmt.Errorf("%w: embedding dimensions", ErrEmbeddingsUnavailable)
	ErrEngineUnavailable     = fmt.Errorf("%w: search engine", ErrSearchUnavailable)
	ErrEngineDimensions      = fmt.Errorf("%w: collection dimensions", ErrEngineUnavailable)
)

type IndexFailure struct {
	Code   string
	Detail string
	Err    error
}

func NewIndexFailure(err error) *IndexFailure {
	return &IndexFailure{Code: IndexFailureCode(err), Detail: RedactDetail(err.Error()), Err: err}
}

func (f *IndexFailure) Error() string { return f.Code + ": " + f.Detail }
func (f *IndexFailure) Unwrap() error { return f.Err }

type InternalError struct{ Detail string }

func (e *InternalError) Error() string { return "internal: " + e.Detail }

func (i Invalid) AppError() *apperr.Error { return apperr.Invalid(i.Code(), i.Message()) }

func (c Conflict) AppError() *apperr.Error { return apperr.Conflicting(c.Code(), c.Message()) }

func (e *InternalError) AppError() *apperr.Error { return apperr.InternalDetail(e.Detail) }

type searchUnavailable struct{}

func (searchUnavailable) Error() string { return "search engine is unavailable" }

func (searchUnavailable) AppError() *apperr.Error {
	return &apperr.Error{Kind: apperr.Unavailable, Code: "search.unavailable", What: "search engine"}
}
