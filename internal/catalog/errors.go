package catalog

import "errors"

type Invalid int

const (
	InvalidKind Invalid = iota + 1
	InvalidSlug
	InvalidNodeName
	InvalidDescription
	InvalidLabels
	InvalidForge
	InvalidRepoURL
	InvalidBranch
	InvalidRepoFieldsNotAllowed
	InvalidDepth
	InvalidRole
	InvalidSubjectKind
	InvalidUnknownSubject
	InvalidGracePeriod
	InvalidBranchState
	InvalidBranchPage
	InvalidClusterObservation
)

func (i Invalid) Code() string {
	switch i {
	case InvalidKind:
		return "validation.invalid_kind"
	case InvalidSlug:
		return "validation.invalid_slug"
	case InvalidNodeName:
		return "validation.invalid_node_name"
	case InvalidDescription:
		return "validation.invalid_description"
	case InvalidLabels:
		return "validation.invalid_labels"
	case InvalidForge:
		return "validation.invalid_forge"
	case InvalidRepoURL:
		return "validation.invalid_repo_url"
	case InvalidBranchState:
		return "validation.invalid_branch_state"
	case InvalidBranchPage:
		return "validation.invalid_pagination"
	case InvalidBranch:
		return "validation.invalid_branch"
	case InvalidRepoFieldsNotAllowed:
		return "validation.repo_fields_not_allowed"
	case InvalidDepth:
		return "validation.invalid_depth"
	case InvalidRole:
		return "validation.invalid_role"
	case InvalidSubjectKind:
		return "validation.invalid_subject_kind"
	case InvalidUnknownSubject:
		return "validation.unknown_subject"
	case InvalidGracePeriod:
		return "validation.invalid_grace_period"
	case InvalidClusterObservation:
		return "validation.invalid_cluster_observation"
	}
	return "validation.invalid"
}

func (i Invalid) Message() string {
	switch i {
	case InvalidKind:
		return `kind must be "organization", "folder" or "project"`
	case InvalidBranchState:
		return `state must be "active", "stale", "gone" or "all"`
	case InvalidBranchPage:
		return "limit must be 1..=200 and offset a non-negative integer"
	case InvalidSlug:
		return "slug must be 1 to 63 characters a-z, 0-9 and '-', not starting or ending with '-'"
	case InvalidNodeName:
		return "name must be 1 to 100 characters without control characters"
	case InvalidDescription:
		return "description must be at most 2000 characters"
	case InvalidLabels:
		return "labels must be an object of at most 32 string values with keys of 1 to 63 characters a-z, 0-9, '.', '_', '-'"
	case InvalidForge:
		return `forge must be "github", "gitlab", "forgejo" or "gitea"`
	case InvalidRepoURL:
		return "repo_url must be an http(s) URL of at most 2048 characters"
	case InvalidBranch:
		return "default_branch must be 1 to 255 characters without whitespace"
	case InvalidRepoFieldsNotAllowed:
		return "repository fields are allowed on projects only"
	case InvalidDepth:
		return "depth must be an integer 1..=10"
	case InvalidRole:
		return `role must be "viewer", "editor" or "admin"`
	case InvalidSubjectKind:
		return `subject_kind must be "user" or "group"`
	case InvalidUnknownSubject:
		return "no user with this email or group with this name"
	case InvalidGracePeriod:
		return "grace_secs must be an integer 0..=604800"
	case InvalidClusterObservation:
		return "cluster_observation must be true or false"
	}
	return "invalid input"
}

func (i Invalid) Error() string { return i.Message() }

type Conflict int

const (
	ConflictSlugTaken Conflict = iota + 1
	ConflictInvalidParent
	ConflictCycle
	ConflictNodeNotEmpty
	ConflictManagedByForge
	ConflictBranchIsDefault
)

func (c Conflict) Code() string {
	switch c {
	case ConflictSlugTaken:
		return "conflict.slug_taken"
	case ConflictInvalidParent:
		return "conflict.invalid_parent"
	case ConflictCycle:
		return "conflict.cycle"
	case ConflictNodeNotEmpty:
		return "conflict.node_not_empty"
	case ConflictManagedByForge:
		return "conflict.managed_by_forge"
	case ConflictBranchIsDefault:
		return "conflict.branch_is_default"
	}
	return "conflict.unknown"
}

func (c Conflict) Message() string {
	switch c {
	case ConflictSlugTaken:
		return "a sibling with this slug already exists"
	case ConflictInvalidParent:
		return "this kind of node cannot be placed there"
	case ConflictCycle:
		return "a node cannot be moved under itself or its descendant"
	case ConflictNodeNotEmpty:
		return "the node has children"
	case ConflictManagedByForge:
		return "the node or field is managed by forge synchronization"
	case ConflictBranchIsDefault:
		return "the default branch of a project cannot be deleted"
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
