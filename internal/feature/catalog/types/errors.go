package types

import (
	"svc-registry/internal/platform/apperr"
)

var (
	InvalidKind                 = apperr.Invalid("validation.invalid_kind", `kind must be "organization", "folder" or "project"`)
	InvalidSlug                 = apperr.Invalid("validation.invalid_slug", "slug must be 1 to 63 characters a-z, 0-9 and '-', not starting or ending with '-'")
	InvalidNodeName             = apperr.Invalid("validation.invalid_node_name", "name must be 1 to 100 characters without control characters")
	InvalidDescription          = apperr.Invalid("validation.invalid_description", "description must be at most 2000 characters")
	InvalidLabels               = apperr.Invalid("validation.invalid_labels", "labels must be an object of at most 32 string values with keys of 1 to 63 characters a-z, 0-9, '.', '_', '-'")
	InvalidForge                = apperr.Invalid("validation.invalid_forge", `forge must be "github", "gitlab", "forgejo" or "gitea"`)
	InvalidRepoURL              = apperr.Invalid("validation.invalid_repo_url", "repo_url must be an http(s) URL of at most 2048 characters")
	InvalidBranch               = apperr.Invalid("validation.invalid_branch", "default_branch must be 1 to 255 characters without whitespace")
	InvalidRepoFieldsNotAllowed = apperr.Invalid("validation.repo_fields_not_allowed", "repository fields are allowed on projects only")
	InvalidDepth                = apperr.Invalid("validation.invalid_depth", "depth must be an integer 1..=10")
	InvalidRole                 = apperr.Invalid("validation.invalid_role", `role must be "viewer", "editor" or "admin"`)
	InvalidSubjectKind          = apperr.Invalid("validation.invalid_subject_kind", `subject_kind must be "user" or "group"`)
	InvalidUnknownSubject       = apperr.Invalid("validation.unknown_subject", "no user with this email or group with this name")
	InvalidGracePeriod          = apperr.Invalid("validation.invalid_grace_period", "grace_secs must be an integer 0..=604800")
	InvalidBranchState          = apperr.Invalid("validation.invalid_branch_state", `state must be "active", "stale", "gone" or "all"`)
	InvalidBranchPage           = apperr.Invalid("validation.invalid_pagination", "limit must be 1..=200 and offset a non-negative integer")
	InvalidVisibility           = apperr.Invalid("validation.invalid_visibility", `visibility must be "open" or "closed"`)
	VisibilityTopLevel          = apperr.Invalid("validation.visibility_top_level", "visibility applies to top-level organizations only")
	InvalidClusterObservation   = apperr.Invalid("validation.invalid_cluster_observation", "cluster_observation must be true or false")
	InvalidFilter               = apperr.Invalid("validation.invalid_filter", `q must be 1 to 100 characters, kind "organization", "folder" or "project", at most 5 labels "key" or "key=value", `+
		`activity "running", "queued" or "failed", owner "group:<id>", "user:<id>" or "me"`)
	InvalidSecret = apperr.Invalid("validation.invalid_secret", "a secret needs a name of 1 to 100 characters, a description of at most 500 characters and exactly one of "+
		`"value" (1 to 4096 characters without whitespace) or "ref" ("env:<NAME>" or "file:<absolute path>")`)
	InvalidSecretNotAvailable = apperr.Invalid("validation.secret_not_available", "the secret is not defined on this node, its ancestors or globally")
	InvalidCredentialsInline  = apperr.Invalid("validation.credentials_inline_removed", `tokens are no longer accepted inline; create a secret and pass {"secret_id": "<id>"}`)
)

var (
	ConflictSlugTaken               = apperr.Conflicting("conflict.slug_taken", "a sibling with this slug already exists")
	ConflictInvalidParent           = apperr.Conflicting("conflict.invalid_parent", "this kind of node cannot be placed there")
	ConflictCycle                   = apperr.Conflicting("conflict.cycle", "a node cannot be moved under itself or its descendant")
	ConflictNodeNotEmpty            = apperr.Conflicting("conflict.node_not_empty", "the node has children")
	ConflictManagedByForge          = apperr.Conflicting("conflict.managed_by_forge", "the node or field is managed by forge synchronization")
	ConflictTokenNodesFull          = apperr.Conflicting("conflict.token_nodes_full", "the token already names the maximum number of nodes")
	ConflictBranchIsDefault         = apperr.Conflicting("conflict.branch_is_default", "the default branch of a project cannot be deleted")
	ConflictSecretInUse             = apperr.Conflicting("conflict.secret_in_use", "the secret is used by forge connections, documentation sources or clusters")
	ConflictSecretNameTaken         = apperr.Conflicting("conflict.secret_name_taken", "a secret with this name already exists here")
	ConflictSecretsKeyMissing       = apperr.Conflicting("conflict.secrets_key_missing", "SECRETS_KEYS is not set, a secret value cannot be stored")
	ConflictOrganizationHasGroups   = apperr.Conflicting("conflict.organization_has_groups", "an organization with groups cannot be moved under another node")
	ConflictOrganizationGroupsBound = apperr.Conflicting("conflict.organization_groups_bound", "the node or its subtree holds roles or ownership of groups of its organization")
)
