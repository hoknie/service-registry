package knowledge

import "errors"

type Invalid int

const (
	InvalidSettings Invalid = iota + 1
	InvalidSearch
	InvalidBranch
	InvalidSource
	InvalidPath
	InvalidSearchMode
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
	ErrNotFound    = errors.New("not found")
	ErrUnavailable = errors.New("database is unavailable")

	ErrSearchUnavailable = errors.New("search engine is unavailable")
)

type InternalError struct{ Detail string }

func (e *InternalError) Error() string { return "internal: " + e.Detail }
