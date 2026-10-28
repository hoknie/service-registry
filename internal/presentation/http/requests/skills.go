package requests

import (
	"github.com/google/uuid"

	skillsservices "svc-registry/internal/feature/skills/services"
	skillstypes "svc-registry/internal/feature/skills/types"
)

type SkillFolderCreate struct {
	ParentID       *uuid.UUID `json:"parent_id"`
	OrganizationID *uuid.UUID `json:"organization_id"`
	Slug           *string    `json:"slug" validate:"required"`
	Name           *string    `json:"name" validate:"required"`
	Description    *string    `json:"description"`
}

func (r SkillFolderCreate) Input() skillsservices.FolderInput {
	in := skillsservices.FolderInput{ParentID: r.ParentID, OrganizationID: r.OrganizationID, Slug: *r.Slug, Name: *r.Name}
	if r.Description != nil {
		in.Description = *r.Description
	}
	return in
}

type SkillFolderPatch struct {
	Slug           *string             `json:"slug"`
	Name           *string             `json:"name"`
	Description    *string             `json:"description"`
	Disabled       *bool               `json:"disabled"`
	ParentID       Optional[uuid.UUID] `json:"parent_id"`
	OrganizationID *uuid.UUID          `json:"organization_id"`
}

func (r SkillFolderPatch) Patch() skillsservices.FolderPatch {
	return skillsservices.FolderPatch{Slug: r.Slug, Name: r.Name, Description: r.Description, Disabled: r.Disabled, Move: r.ParentID.Present,
		ParentID: r.ParentID.Value, OrganizationID: r.OrganizationID}
}

type SkillPatch struct {
	Status         *string             `json:"status"`
	Tags           *[]string           `json:"tags"`
	Slug           *string             `json:"slug"`
	About          *string             `json:"about"`
	FolderID       Optional[uuid.UUID] `json:"folder_id"`
	OrganizationID *uuid.UUID          `json:"organization_id"`
}

func (r SkillPatch) Patch() skillsservices.SkillPatch {
	return skillsservices.SkillPatch{Status: r.Status, Tags: r.Tags, Slug: r.Slug, About: r.About, Move: r.FolderID.Present, FolderID: r.FolderID.Value,
		OrganizationID: r.OrganizationID}
}

type SkillsQuery struct {
	FolderID  string   `query:"folder_id"`
	Recursive bool     `query:"recursive"`
	Scope     string   `query:"scope"`
	OrgID     string   `query:"organization_id"`
	Query     string   `query:"q"`
	Tags      []string `query:"tag"`
	Status    string   `query:"status"`
	Active    string   `query:"active"`
	Limit     int      `query:"limit"`
	Cursor    string   `query:"cursor"`
}

func (q SkillsQuery) Input() (skillsservices.ListInput, error) {
	in := skillsservices.ListInput{Recursive: q.Recursive, Scope: q.Scope, Query: q.Query, Tags: q.Tags, Status: q.Status,
		Limit: q.Limit, Cursor: q.Cursor}
	if q.OrgID != "" {
		in.Scope = q.OrgID
	}
	switch q.Active {
	case "":
	case "true", "false":
		in.Active = new(q.Active == "true")
	default:
		return in, skillstypes.InvalidFilter
	}
	if q.FolderID == "root" {
		in.TopLevel = true
	} else if q.FolderID != "" {
		id, err := uuid.Parse(q.FolderID)
		if err != nil {
			return in, skillstypes.InvalidFilter
		}
		in.FolderID = &id
	}
	return in, nil
}

type SkillSource struct {
	Kind        *string `json:"kind" validate:"required"`
	Hosting     string  `json:"hosting"`
	URL         string  `json:"url"`
	APIURL      string  `json:"api_url"`
	Path        string  `json:"path"`
	WorkingTree *bool   `json:"working_tree"`
	Credentials *struct {
		SecretID *string `json:"secret_id"`
	} `json:"credentials"`
	Branch       string   `json:"branch"`
	Root         *string  `json:"root"`
	Roots        []string `json:"roots"`
	Layout       string   `json:"layout"`
	GroupField   *string  `json:"group_field"`
	IntervalSecs *int     `json:"interval_secs"`
	Status       string   `json:"status"`
	Versions     string   `json:"versions"`
	TagPattern   string   `json:"tag_pattern"`
}

func (r SkillSource) Input() skillsservices.SourceInput {
	in := skillsservices.SourceInput{Kind: *r.Kind, Hosting: r.Hosting, URL: r.URL, APIURL: r.APIURL, Path: r.Path, WorkingTree: r.WorkingTree,
		Branch: r.Branch, Root: r.Root, Roots: r.Roots, Layout: r.Layout, GroupField: r.GroupField, IntervalSecs: r.IntervalSecs, Status: r.Status, Versions: r.Versions, TagPattern: r.TagPattern}
	if r.Credentials != nil {
		in.SecretID = r.Credentials.SecretID
	}
	return in
}

type SkillPolicy struct {
	Range      *string   `json:"range"`
	Prerelease *bool     `json:"prerelease"`
	Exclude    *[]string `json:"exclude"`
}

func (r SkillPolicy) Policy() skillstypes.Policy {
	return skillstypes.Policy{Range: r.Range, Prerelease: r.Prerelease, Exclude: r.Exclude}
}

type SkillSpace struct {
	Description *string `json:"description" validate:"required"`
}

type SkillYank struct {
	Yanked *bool `json:"yanked" validate:"required"`
}

type SkillImport struct {
	SkillSource
	ParentID       *uuid.UUID `json:"parent_id"`
	OrganizationID *uuid.UUID `json:"organization_id"`
	Slug           string     `json:"slug"`
	Name           string     `json:"name"`
}

func (r SkillImport) Input() skillsservices.ImportInput {
	return skillsservices.ImportInput{ParentID: r.ParentID, OrganizationID: r.OrganizationID, Slug: r.Slug, Name: r.Name,
		Source: r.SkillSource.Input()}
}

type SkillSet struct {
	Mode    string      `json:"mode"`
	Folders []uuid.UUID `json:"folders"`
	Skills  []uuid.UUID `json:"skills"`
}
