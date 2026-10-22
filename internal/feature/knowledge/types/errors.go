package types

import (
	"fmt"

	"svc-registry/internal/platform/apperr"
)

var (
	InvalidSettings   = apperr.Invalid("validation.invalid_knowledge_settings", "include, exclude and branches must be lists of at most 50 valid glob patterns of 1 to 200 characters (include may be null)")
	InvalidSearch     = apperr.Invalid("validation.invalid_search", "q must be 1 to 200 characters, limit 1..=50 and cursor one returned by a previous search")
	InvalidBranch     = apperr.Invalid("validation.invalid_branch", "branch must be a valid branch name")
	InvalidSource     = apperr.Invalid("validation.invalid_knowledge_source", `source must be {"kind":"remote","forge","url","api_url"?,"credentials"?} or {"kind":"local_dir"|"local_git","path"}`)
	InvalidPath       = apperr.Invalid("validation.knowledge_path_not_allowed", "path must be an absolute path inside one of KNOWLEDGE_LOCAL_ROOTS")
	InvalidSearchMode = apperr.Invalid("validation.search_mode_unavailable", "mode must be one of the modes the search engine offers (GET /api/v1/knowledge/search/modes)")
	InvalidScanFilter = apperr.Invalid("validation.invalid_scan_filter", "project must be a UUID, kind collect|index, trigger schedule|manual, status a comma-separated list of ok, unchanged, warning, failed")
)

var (
	ConflictNotSynced     = apperr.Conflicting("conflict.knowledge_not_synced", "the project has no documentation source and is not synchronized with a forge")
	ConflictLocalDisabled = apperr.Conflicting("conflict.local_sources_disabled", "local documentation sources are off: KNOWLEDGE_LOCAL_ROOTS is empty")
)

var (
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

type searchUnavailable struct{}

func (searchUnavailable) Error() string { return "search engine is unavailable" }

func (searchUnavailable) AppError() *apperr.Error {
	return &apperr.Error{Kind: apperr.Unavailable, Code: "search.unavailable", What: "search engine"}
}
