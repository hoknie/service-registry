package repositories

import (
	skillstypes "svc-registry/internal/feature/skills/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
)

var constraints = postgres.Constraints{
	"skill_folders_parent_slug_idx":              skillstypes.ConflictSlugTaken,
	"skill_folders_organization_slug_idx":        skillstypes.ConflictSlugTaken,
	"skill_folders_global_slug_idx":              skillstypes.ConflictSlugTaken,
	"skill_folders_parent_id_fkey":               apperr.ErrNotFound,
	"skill_folders_organization_id_fkey":         skillstypes.InvalidScope,
	"skill_folders_slug_check":                   skillstypes.InvalidFolder,
	"skill_folders_name_check":                   skillstypes.InvalidFolder,
	"skill_folders_description_check":            skillstypes.InvalidFolder,
	"skills_folder_slug_idx":                     skillstypes.ConflictSlugTaken,
	"skills_organization_slug_idx":               skillstypes.ConflictSlugTaken,
	"skills_global_slug_idx":                     skillstypes.ConflictSlugTaken,
	"skills_folder_id_fkey":                      apperr.ErrNotFound,
	"skills_organization_id_fkey":                skillstypes.InvalidScope,
	"skills_slug_check":                          skillstypes.InvalidSkill,
	"skills_tags_check":                          skillstypes.InvalidSkill,
	"skills_status_check":                        skillstypes.InvalidSkill,
	"skills_about_check":                         skillstypes.InvalidSkill,
	"skill_versions_skill_version_idx":           skillstypes.ConflictVersion,
	"skill_versions_version_check":               skillstypes.InvalidVersion,
	"skill_sources_folder_id_fkey":               apperr.ErrNotFound,
	"skill_sources_credentials_secret_id_fkey":   apperr.ErrNotFound,
	"skill_sources_interval_secs_check":          skillstypes.InvalidSource,
	"skill_sources_branch_check":                 skillstypes.InvalidSource,
	"skill_sources_root_check":                   skillstypes.InvalidSource,
	"skill_sources_tag_pattern_check":            skillstypes.InvalidTagPattern,
	"skill_version_policies_skill_id_fkey":       apperr.ErrNotFound,
	"skill_version_policies_version_range_check": skillstypes.InvalidRange,
	"skill_version_policies_excluded_check":      skillstypes.InvalidPolicy,
	"skill_spaces_description_check":             skillstypes.InvalidSpace,
	"skill_spaces_organization_id_fkey":          apperr.ErrNotFound,
}

func Err(err error) error { return postgres.Translate(err, constraints) }
