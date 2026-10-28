package responses

import (
	"github.com/google/uuid"

	skillsservices "svc-registry/internal/feature/skills/services"
	skillstypes "svc-registry/internal/feature/skills/types"
)

type SkillRef struct {
	ID   uuid.UUID `json:"id"`
	Slug string    `json:"slug"`
	Name string    `json:"name"`
}

type SkillUser struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type SkillVersion struct {
	Version    string     `json:"version"`
	CreatedAt  string     `json:"created_at"`
	Source     string     `json:"source"`
	Commit     *string    `json:"commit"`
	Tag        *string    `json:"tag"`
	CreatedBy  *SkillUser `json:"created_by"`
	Files      int        `json:"files"`
	Size       int64      `json:"size"`
	Digest     string     `json:"digest"`
	Prerelease bool       `json:"prerelease"`
	Available  bool       `json:"available"`
	Yanked     bool       `json:"yanked"`
}

type SkillSourceRef struct {
	ID   uuid.UUID `json:"id"`
	Kind string    `json:"kind"`
	URL  string    `json:"url"`
}

type Skill struct {
	ID                uuid.UUID       `json:"id"`
	FolderID          *uuid.UUID      `json:"folder_id"`
	OrganizationID    *uuid.UUID      `json:"organization_id"`
	Organization      *SkillRef       `json:"organization"`
	Path              string          `json:"path"`
	Slug              string          `json:"slug"`
	Name              string          `json:"name"`
	Title             string          `json:"title"`
	Description       string          `json:"description"`
	About             string          `json:"about"`
	License           string          `json:"license"`
	Compatibility     string          `json:"compatibility"`
	AllowedTools      []string        `json:"allowed_tools"`
	Tags              []string        `json:"tags"`
	Author            string          `json:"author"`
	Status            string          `json:"status"`
	ImageURL          *string         `json:"image_url"`
	Version           *SkillVersion   `json:"version"`
	VersionsAvailable int             `json:"versions_available"`
	Downloads         int64           `json:"downloads"`
	Source            *SkillSourceRef `json:"source"`
	Managed           bool            `json:"managed"`
	CanWrite          bool            `json:"can_write"`
	CreatedBy         *SkillUser      `json:"created_by"`
	CreatedAt         string          `json:"created_at"`
	UpdatedAt         string          `json:"updated_at"`
}

type SkillPage struct {
	Items      []Skill `json:"items"`
	NextCursor *string `json:"next_cursor"`
}

func skillRef(r *skillstypes.Ref) *SkillRef {
	if r == nil {
		return nil
	}
	return &SkillRef{ID: r.ID, Slug: r.Slug, Name: r.Name}
}

func skillUser(u *skillstypes.UserRef) *SkillUser {
	if u == nil {
		return nil
	}
	return &SkillUser{ID: u.ID, Name: u.Name}
}

func SkillVersionOf(v skillstypes.Version) SkillVersion {
	return SkillVersion{Version: v.Version, CreatedAt: v.CreatedAt, Source: string(v.Origin), Commit: v.Commit, Tag: v.Tag,
		CreatedBy: skillUser(v.CreatedBy), Files: v.Files, Size: v.Size, Digest: v.Digest, Prerelease: v.Prerelease,
		Available: v.Available, Yanked: v.Yanked}
}

func SkillVersionsOf(vs []skillstypes.Version) []SkillVersion {
	out := make([]SkillVersion, len(vs))
	for i, v := range vs {
		out[i] = SkillVersionOf(v)
	}
	return out
}

func strings0(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

func SkillOf(s skillstypes.Skill) Skill {
	out := Skill{ID: s.ID, FolderID: s.FolderID, OrganizationID: s.OrganizationID, Organization: skillRef(s.Organization), Path: s.Path,
		Slug: s.Slug, Name: s.Name, Title: s.Title, Description: s.Description, About: s.About, License: s.License, Compatibility: s.Compatibility,
		AllowedTools: strings0(s.AllowedTools), Tags: strings0(s.Tags), Author: s.Author, Status: string(s.Status),
		VersionsAvailable: s.VersionsAvailable, Downloads: s.Downloads, Managed: s.Managed(), CanWrite: s.CanWrite,
		CreatedBy: skillUser(s.CreatedBy), CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt}
	if s.HasImage {
		out.ImageURL = new("/api/v1/skills/" + s.ID.String() + "/image")
	}
	if s.Version != nil {
		out.Version = new(SkillVersionOf(*s.Version))
	}
	if s.Source != nil {
		out.Source = &SkillSourceRef{ID: s.Source.ID, Kind: s.Source.Kind, URL: s.Source.URL}
	}
	return out
}

func SkillPageOf(p skillsservices.Page) SkillPage {
	out := SkillPage{Items: make([]Skill, len(p.Items)), NextCursor: p.NextCursor}
	for i, s := range p.Items {
		out.Items[i] = SkillOf(s)
	}
	return out
}

type SkillFolder struct {
	ID             uuid.UUID  `json:"id"`
	ParentID       *uuid.UUID `json:"parent_id"`
	OrganizationID *uuid.UUID `json:"organization_id"`
	Organization   *SkillRef  `json:"organization"`
	Slug           string     `json:"slug"`
	Name           string     `json:"name"`
	Description    string     `json:"description"`
	Ancestors      []SkillRef `json:"ancestors"`
	Skills         int64      `json:"skills"`
	Folders        int64      `json:"folders"`
	CanWrite       bool       `json:"can_write"`
}

func SkillFolderOf(f skillstypes.Folder) SkillFolder {
	out := SkillFolder{ID: f.ID, ParentID: f.ParentID, OrganizationID: f.OrganizationID, Organization: skillRef(f.Organization),
		Slug: f.Slug, Name: f.Name, Description: f.Description, Ancestors: []SkillRef{}, Skills: f.Skills, Folders: f.Folders, CanWrite: f.CanWrite}
	for _, a := range f.Ancestors {
		out.Ancestors = append(out.Ancestors, SkillRef{ID: a.ID, Slug: a.Slug, Name: a.Name})
	}
	return out
}

func SkillFoldersOf(fs []skillstypes.Folder) map[string][]SkillFolder {
	out := make([]SkillFolder, len(fs))
	for i, f := range fs {
		out[i] = SkillFolderOf(f)
	}
	return map[string][]SkillFolder{"items": out}
}

type SkillFile struct {
	Path  string  `json:"path"`
	Size  int64   `json:"size"`
	Kind  string  `json:"kind"`
	Title *string `json:"title"`
}

type SkillFiles struct {
	Version string      `json:"version"`
	Items   []SkillFile `json:"items"`
}

func SkillFilesOf(sel skillsservices.Selected) SkillFiles {
	out := SkillFiles{Version: sel.Version.Version, Items: make([]SkillFile, len(sel.Files))}
	for i, f := range sel.Files {
		out.Items[i] = SkillFile{Path: f.Path, Size: f.Size, Kind: string(f.Kind), Title: f.Title}
	}
	return out
}

type SkillFileContent struct {
	Path    string  `json:"path"`
	Kind    string  `json:"kind"`
	Size    int64   `json:"size"`
	Content *string `json:"content"`
	Version string  `json:"version"`
}

func SkillFileContentOf(f skillstypes.FileContent) SkillFileContent {
	return SkillFileContent{Path: f.Path, Kind: string(f.Kind), Size: f.Size, Content: f.Content, Version: f.Version}
}

type SkillTag struct {
	Tag   string `json:"tag"`
	Count int64  `json:"count"`
}

func SkillTagsOf(ts []skillstypes.TagCount) map[string][]SkillTag {
	out := make([]SkillTag, len(ts))
	for i, t := range ts {
		out[i] = SkillTag{Tag: t.Tag, Count: t.Count}
	}
	return map[string][]SkillTag{"items": out}
}

type SkillPolicy struct {
	Range      *string   `json:"range"`
	Prerelease *bool     `json:"prerelease"`
	Exclude    *[]string `json:"exclude"`
}

type SkillEffectivePolicy struct {
	Range      string   `json:"range"`
	Prerelease bool     `json:"prerelease"`
	Exclude    []string `json:"exclude"`
}

type SkillPolicyView struct {
	Own       SkillPolicy          `json:"own"`
	Effective SkillEffectivePolicy `json:"effective"`
	From      map[string]string    `json:"from"`
	CanWrite  bool                 `json:"can_write"`
}

func SkillPolicyOf(v skillsservices.PolicyView) SkillPolicyView {
	return SkillPolicyView{Own: SkillPolicy{Range: v.Own.Range, Prerelease: v.Own.Prerelease, Exclude: v.Own.Exclude},
		Effective: SkillEffectivePolicy{Range: v.Effective.Range, Prerelease: v.Effective.Prerelease, Exclude: strings0(v.Effective.Exclude)},
		From:      v.Effective.From, CanWrite: v.CanWrite}
}
