package repositories

import (
	"github.com/google/uuid"

	skillstypes "svc-registry/internal/feature/skills/types"
)

func refOf(id *uuid.UUID, slug, name *string) *skillstypes.Ref {
	if id == nil || slug == nil || name == nil {
		return nil
	}
	return &skillstypes.Ref{ID: *id, Slug: *slug, Name: *name}
}

type folderRow struct {
	ID             uuid.UUID  `db:"id"`
	ParentID       *uuid.UUID `db:"parent_id"`
	OrganizationID *uuid.UUID `db:"organization_id"`
	OrgSlug        *string    `db:"org_slug"`
	OrgName        *string    `db:"org_name"`
	Slug           string     `db:"slug"`
	Name           string     `db:"name"`
	Description    string     `db:"description"`
	Depth          int        `db:"depth"`
	Skills         int64      `db:"skills"`
	Folders        int64      `db:"folders"`
	CreatedAt      string     `db:"created_at"`
	UpdatedAt      string     `db:"updated_at"`
	Disabled       bool       `db:"disabled"`
	SourceID       *uuid.UUID `db:"source_id"`
	DisabledByID   *uuid.UUID `db:"disabled_by_id"`
	DisabledByName *string    `db:"disabled_by_name"`
	DisabledBySlug *string    `db:"disabled_by_slug"`
}

func (r folderRow) folder() skillstypes.Folder {
	f := skillstypes.Folder{ID: r.ID, ParentID: r.ParentID, OrganizationID: r.OrganizationID,
		Organization: refOf(r.OrganizationID, r.OrgSlug, r.OrgName), Slug: r.Slug, Name: r.Name, Description: r.Description,
		Depth: r.Depth, Skills: r.Skills, Folders: r.Folders, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Ancestors: []skillstypes.Ref{},
		Disabled: r.Disabled, SourceID: r.SourceID}
	if r.DisabledByID != nil {
		f.DisabledBy = &skillstypes.Ref{ID: *r.DisabledByID, Slug: deref(r.DisabledBySlug), Name: deref(r.DisabledByName)}
	}
	return f
}

type skillRow struct {
	ID             uuid.UUID  `db:"id"`
	FolderID       *uuid.UUID `db:"folder_id"`
	OrganizationID *uuid.UUID `db:"organization_id"`
	OrgSlug        *string    `db:"org_slug"`
	OrgName        *string    `db:"org_name"`
	Path           string     `db:"path"`
	Slug           string     `db:"slug"`
	Name           string     `db:"name"`
	Title          string     `db:"title"`
	Description    string     `db:"description"`
	About          string     `db:"about"`
	License        string     `db:"license"`
	Compatibility  string     `db:"compatibility"`
	AllowedTools   []string   `db:"allowed_tools"`
	Tags           []string   `db:"tags"`
	Author         string     `db:"author"`
	ImagePath      *string    `db:"image_path"`
	ImageSHA256    []byte     `db:"image_sha256"`
	ImageType      *string    `db:"image_type"`
	Status         string     `db:"status"`
	Downloads      int64      `db:"downloads"`
	SourceID       *uuid.UUID `db:"source_id"`
	SourceKind     *string    `db:"source_kind"`
	SourceURL      *string    `db:"source_url"`
	SourcePath     *string    `db:"source_path"`
	CreatedByID    *uuid.UUID `db:"created_by_id"`
	CreatedByName  *string    `db:"created_by_name"`
	CreatedAt      string     `db:"created_at"`
	UpdatedAt      string     `db:"updated_at"`
	CursorAt       string     `db:"cursor_at"`
	DisabledByID   *uuid.UUID `db:"disabled_by_id"`
	DisabledByName *string    `db:"disabled_by_name"`
	DisabledBySlug *string    `db:"disabled_by_slug"`
}

func (r skillRow) skill() skillstypes.Skill {
	s := skillstypes.Skill{ID: r.ID, FolderID: r.FolderID, OrganizationID: r.OrganizationID,
		Organization: refOf(r.OrganizationID, r.OrgSlug, r.OrgName), Path: r.Path, Slug: r.Slug, Name: r.Name, Title: r.Title,
		Description: r.Description, About: r.About, License: r.License, Compatibility: r.Compatibility, AllowedTools: r.AllowedTools, Tags: r.Tags,
		Author: r.Author, ImagePath: r.ImagePath, ImageSHA256: r.ImageSHA256, ImageType: r.ImageType,
		HasUploadedImage: r.ImageSHA256 != nil, Status: skillstypes.Status(r.Status), Downloads: r.Downloads,
		SourcePath: r.SourcePath, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
	if r.SourceID != nil {
		s.Source = &skillstypes.SourceRef{ID: *r.SourceID, Kind: deref(r.SourceKind), URL: deref(r.SourceURL)}
	}
	if r.DisabledByID != nil {
		s.DisabledBy = &skillstypes.Ref{ID: *r.DisabledByID, Slug: deref(r.DisabledBySlug), Name: deref(r.DisabledByName)}
	}
	if r.CreatedByID != nil {
		s.CreatedBy = &skillstypes.UserRef{ID: *r.CreatedByID, Name: deref(r.CreatedByName)}
	}
	return s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

type listedRow struct {
	skillRow
}

func (r listedRow) listed() Listed {
	return Listed{Skill: r.skill(), Cursor: skillstypes.Cursor{UpdatedAt: r.CursorAt, Name: r.Name, ID: r.ID}}
}

type Listed struct {
	Skill  skillstypes.Skill
	Cursor skillstypes.Cursor
}

type versionRow struct {
	ID            uuid.UUID            `db:"id"`
	SkillID       uuid.UUID            `db:"skill_id"`
	Version       string               `db:"version"`
	CreatedAt     string               `db:"created_at"`
	Origin        string               `db:"origin"`
	CommitSHA     *string              `db:"commit_sha"`
	Tag           *string              `db:"tag"`
	CreatedByID   *uuid.UUID           `db:"created_by_id"`
	CreatedByName *string              `db:"created_by_name"`
	Files         int                  `db:"files"`
	Size          int64                `db:"size"`
	Digest        string               `db:"digest"`
	Yanked        bool                 `db:"yanked"`
	Manifest      skillstypes.Manifest `db:"manifest"`
}

func (r versionRow) version() skillstypes.Version {
	v := skillstypes.Version{ID: r.ID, SkillID: r.SkillID, Version: r.Version, CreatedAt: r.CreatedAt,
		Origin: skillstypes.Origin(r.Origin), Commit: r.CommitSHA, Tag: r.Tag, Files: r.Files, Size: r.Size, Digest: r.Digest,
		Yanked: r.Yanked, Manifest: r.Manifest}
	if r.CreatedByID != nil {
		v.CreatedBy = &skillstypes.UserRef{ID: *r.CreatedByID, Name: deref(r.CreatedByName)}
	}
	return v
}

type fileRow struct {
	Path        string  `db:"path"`
	Size        int64   `db:"size"`
	SHA256      []byte  `db:"sha256"`
	Kind        string  `db:"kind"`
	ContentType string  `db:"content_type"`
	Title       *string `db:"title"`
}

func (r fileRow) file() skillstypes.File {
	return skillstypes.File{Path: r.Path, Size: r.Size, SHA256: r.SHA256, Kind: skillstypes.FileKind(r.Kind),
		ContentType: r.ContentType, Title: r.Title}
}

type policyRow struct {
	Range      *string   `db:"version_range"`
	Prerelease *bool     `db:"prerelease"`
	Excluded   *[]string `db:"excluded"`
}

func (r policyRow) policy() skillstypes.Policy {
	return skillstypes.Policy{Range: r.Range, Prerelease: r.Prerelease, Exclude: r.Excluded}
}
