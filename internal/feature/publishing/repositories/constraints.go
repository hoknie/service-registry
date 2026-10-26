package repositories

import (
	publishingtypes "svc-registry/internal/feature/publishing/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
)

var constraints = postgres.Constraints{
	"docs_publications_node_id_fkey":   apperr.ErrNotFound,
	"docs_publications_name_idx":       publishingtypes.ConflictNameTaken,
	"docs_publications_host_idx":       publishingtypes.ConflictHostTaken,
	"docs_publications_name_check":     publishingtypes.InvalidPublication,
	"docs_publications_host_check":     publishingtypes.InvalidPublication,
	"docs_publications_versions_check": publishingtypes.InvalidPublication,
}

func Err(err error) error { return postgres.Translate(err, constraints) }
