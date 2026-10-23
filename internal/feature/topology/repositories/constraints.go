package repositories

import (
	topologytypes "svc-registry/internal/feature/topology/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
)

var constraints = postgres.Constraints{
	"service_dependencies_project_id_fkey":        apperr.ErrNotFound,
	"service_dependencies_target_project_id_fkey": topologytypes.InvalidDependency,
	"service_dependencies_target_key":             topologytypes.ConflictDependencyExists,
	"service_dependencies_self_check":             topologytypes.InvalidDependency,
	"service_dependencies_target_external_check":  topologytypes.InvalidDependency,
	"service_dependencies_target_ref_check":       topologytypes.InvalidDependency,
	"service_dependencies_note_check":             topologytypes.InvalidDependency,
}
