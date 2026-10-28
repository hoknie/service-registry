package types

import (
	"errors"
	"fmt"

	"svc-registry/internal/platform/apperr"
)

var (
	InvalidSettings   = apperr.Invalid("validation.invalid_knowledge_settings", "include, exclude, api_paths and refs patterns must be lists of at most 50 valid glob patterns of 1 to 200 characters (include may be null)")
	InvalidSearch     = apperr.Invalid("validation.invalid_search", "q must be 1 to 200 characters, limit 1..=50 and cursor one returned by a previous search")
	BranchesReplaced  = apperr.Invalid("validation.knowledge_branches_replaced", `branches is replaced by refs: [{"pattern","kind":"branch"|"tag","release"?}]`)
	InvalidRef        = apperr.Invalid("validation.invalid_ref", "give at most one of branch, tag and release; tag must be 1 to 255 characters and release must be current")
	InvalidBranch     = apperr.Invalid("validation.invalid_branch", "branch must be a valid branch name")
	InvalidSource     = apperr.Invalid("validation.invalid_knowledge_source", `source must be {"kind":"remote","hosting","url","api_url"?,"credentials"?} or {"kind":"local_dir"|"local_git","path"}`)
	InvalidPath       = apperr.Invalid("validation.knowledge_path_not_allowed", "path must be an absolute path inside one of KNOWLEDGE_LOCAL_ROOTS")
	InvalidSearchMode = apperr.Invalid("validation.search_mode_unavailable", "mode must be one of the modes the search engine offers (GET /api/v1/knowledge/search/modes)")
	InvalidScanFilter = apperr.Invalid("validation.invalid_scan_filter", "project must be a UUID, kind collect|index, trigger schedule|manual, status a comma-separated list of ok, unchanged, warning, failed")
)

var (
	ConflictNotSynced     = apperr.Conflicting("conflict.knowledge_not_synced", "the project has no documentation source and is not linked to repository sync")
	ConflictLocalDisabled = apperr.Conflicting("conflict.local_sources_disabled", "local documentation sources are off: KNOWLEDGE_LOCAL_ROOTS is empty")
)

var (
	ErrSearchUnavailable error = searchUnavailable{}

	ErrEmbeddingsUnavailable = fmt.Errorf("%w: embeddings API", ErrSearchUnavailable)
	ErrEmbeddingsDimensions  = fmt.Errorf("%w: embedding dimensions", ErrEmbeddingsUnavailable)
	ErrEngineUnavailable     = fmt.Errorf("%w: search engine", ErrSearchUnavailable)
	ErrEngineDimensions      = fmt.Errorf("%w: collection dimensions", ErrEngineUnavailable)

	ErrSnapshotUnsupported = errors.New("source does not serve snapshots")
	ErrSnapshotTooLarge    = errors.New("snapshot archive is too large")
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

func IndexFailureCode(err error) string {
	switch {
	case errors.Is(err, ErrEmbeddingsDimensions):
		return "search.embeddings_dimensions"
	case errors.Is(err, ErrEngineDimensions):
		return "search.engine_dimensions"
	case errors.Is(err, ErrEmbeddingsUnavailable):
		return "search.embeddings_unavailable"
	case errors.Is(err, ErrEngineUnavailable), errors.Is(err, ErrSearchUnavailable):
		return "search.engine_unavailable"
	}
	return "internal"
}
