package types

import (
	"strings"

	"svc-registry/internal/platform/apperr"
)

var (
	InvalidSkill = apperr.Invalid("validation.invalid_skill",
		`skill fields: status is draft or published, tags are up to 20 of a-z, 0-9, ., +, # and - (at most 40 each), slug is 1 to 64 of a-z, 0-9 and -`)
	InvalidSpace  = apperr.Invalid("validation.invalid_skill_space", "the description of a skills space is at most 2000 characters")
	InvalidFolder = apperr.Invalid("validation.invalid_skill_folder",
		"folder fields: slug is 1 to 64 of a-z, 0-9 and -, name is 1 to 200 characters, description at most 2000")
	InvalidScope       = apperr.Invalid("validation.invalid_skill_scope", "organization_id must be an organization at the top level of the catalog")
	InvalidImage       = apperr.Invalid("validation.invalid_skill_image", "the image must be PNG, JPEG, WebP or SVG")
	ImageTooLarge      = apperr.TooLargeError("validation.skill_image_too_large", "the image is larger than 512 KiB")
	FolderTooDeep      = apperr.Invalid("validation.skill_folder_too_deep", "skill folders nest at most 8 levels")
	ManifestMissing    = apperr.Invalid("validation.skill_manifest_missing", "SKILL.md is missing at the root of the skill")
	ArchiveInvalid     = apperr.Invalid("validation.skill_archive_invalid", "the archive or a file path is invalid: relative UTF-8 paths without . or .., unique, not encrypted")
	UploadTooLarge     = apperr.TooLargeError("validation.skill_too_large", "the upload is larger than SKILLS_MAX_UPLOAD_BYTES")
	ContentTooLarge    = apperr.Invalid("validation.skill_too_large", "the skill is over SKILLS_MAX_TOTAL_BYTES, SKILLS_MAX_FILE_BYTES or SKILLS_MAX_FILES")
	InvalidVersion     = apperr.Invalid("validation.invalid_skill_version", "a version is a Semantic Versioning 2.0.0 version such as 1.4.0 or 2.0.0-rc.1")
	InvalidRange       = apperr.Invalid("validation.invalid_version_range", "range is an npm-style version range such as ^1.2, ~1.4.0, >=1.0.0 <3.0.0 or 1.x")
	InvalidPolicy      = apperr.Invalid("validation.invalid_version_policy", `version policy: {"range": string|null, "prerelease": bool|null, "exclude": [versions]|null}, exclude at most 50`)
	InvalidTagPattern  = apperr.Invalid("validation.invalid_tag_pattern", "tag_pattern has exactly one {version}, at most one {name} and is at most 200 characters")
	InvalidSource      = apperr.Invalid("validation.invalid_skill_source", "source fields: branch at most 255, root a relative path, interval_secs 300 to 86400, status draft or published, versions branch or tags (not for local_dir)")
	InvalidFilter      = apperr.Invalid("validation.invalid_filter", "filters: status draft or published, scope global or an organization id, tag of a-z, 0-9, ., +, # and -")
	InvalidPagination  = apperr.Invalid("validation.invalid_pagination", "limit is 1 to 100 and cursor comes from a previous page")
	ConflictSlugTaken  = apperr.Conflicting("conflict.skill_slug_taken", "the slug is used by another skill or folder in the same place")
	ConflictNotEmpty   = apperr.Conflicting("conflict.skill_folder_not_empty", "the folder has folders, skills or sources")
	ConflictCycle      = apperr.Conflicting("conflict.skill_folder_cycle", "a folder cannot move into itself or its descendant")
	ConflictManaged    = apperr.Conflicting("conflict.skill_managed_by_source", "the skill comes from a git source; change it in the repository")
	ConflictName       = apperr.Conflicting("conflict.skill_name_changed", "the name in SKILL.md differs from the skill's name")
	ConflictVersion    = apperr.Conflicting("conflict.skill_version_exists", "the version already exists with other content")
	ConflictStorageOff = apperr.Conflicting("conflict.skills_storage_disabled", "SKILLS_STORAGE=off: skill files cannot be stored")
	Ambiguous          = apperr.Conflicting("skill_ambiguous", "several skills have this name; pass the id or the path of one")
	StorageUnavailable = &apperr.Error{Kind: apperr.Unavailable, Code: "skills.storage_unavailable", What: "skills storage"}
)

func ManifestInvalid(fields ...apperr.Field) *apperr.Error {
	return &apperr.Error{Kind: apperr.Validation, Code: "validation.skill_manifest_invalid",
		Message: "SKILL.md needs YAML frontmatter with name (a-z, 0-9 and -, at most 64) and description (1 to 1024)", Fields: fields}
}

const (
	ProblemManifestMissing = "skills.manifest_missing"
	ProblemManifestInvalid = "skills.manifest_invalid"
	ProblemTooLarge        = "skills.too_large"
	ProblemPathInvalid     = "skills.path_invalid"
	ProblemNameConflict    = "skills.name_conflict"
	ProblemNameChanged     = "skills.name_changed"
	ProblemVersionExists   = "skills.version_exists"
	ProblemVersionMismatch = "skills.version_mismatch"
	ProblemTagsDeferred    = "skills.tags_deferred"
	ProblemTreeTruncated   = "skills.tree_truncated"
	ProblemVersionInvalid  = "skills.version_invalid"

	FailUnreachable    = "skills.source_unreachable"
	FailUnauthorized   = "skills.source_unauthorized"
	FailBranchNotFound = "skills.branch_not_found"
	FailStorage        = "skills.storage_unavailable"
)

func ProblemOf(err error) string {
	e := apperr.From(err)
	switch e.Code {
	case ManifestMissing.Code:
		return ProblemManifestMissing
	case "validation.skill_manifest_invalid":
		return ProblemManifestInvalid
	case ContentTooLarge.Code:
		return ProblemTooLarge
	case ArchiveInvalid.Code:
		return ProblemPathInvalid
	case ConflictSlugTaken.Code:
		return ProblemNameConflict
	case ConflictName.Code:
		return ProblemNameChanged
	case ConflictVersion.Code:
		return ProblemVersionExists
	case InvalidVersion.Code:
		return ProblemVersionInvalid
	case StorageUnavailable.Code:
		return FailStorage
	}
	return ""
}

func FailureCode(err error, branchMissing bool) string {
	code := apperr.From(err).Code
	switch {
	case strings.Contains(code, "unauthorized"), strings.Contains(code, "credentials"):
		return FailUnauthorized
	case branchMissing:
		return FailBranchNotFound
	case code == StorageUnavailable.Code:
		return FailStorage
	}
	return FailUnreachable
}
