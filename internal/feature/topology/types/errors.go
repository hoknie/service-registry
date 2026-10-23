package types

import "svc-registry/internal/platform/apperr"

var (
	InvalidDependency        = apperr.Invalid("validation.invalid_dependency", "the dependency target is not a readable project or a valid external system")
	InvalidDependencyDepth   = apperr.Invalid("validation.invalid_dependency_depth", "depth must be 1, 2 or 3")
	ConflictDependencyExists = apperr.Conflicting("conflict.dependency_exists", "a manual dependency on this target already exists")
)
