package responses

import (
	"github.com/google/uuid"

	skillsservices "svc-registry/internal/feature/skills/services"
	skillstypes "svc-registry/internal/feature/skills/types"
)

type SkillSourceCredentials struct {
	SecretID uuid.UUID `json:"secret_id"`
	Name     *string   `json:"name"`
}

type SkillSource struct {
	ID           uuid.UUID               `json:"id"`
	FolderID     uuid.UUID               `json:"folder_id"`
	Kind         string                  `json:"kind"`
	Hosting      *string                 `json:"hosting"`
	URL          *string                 `json:"url"`
	APIURL       *string                 `json:"api_url"`
	Path         *string                 `json:"path"`
	WorkingTree  bool                    `json:"working_tree"`
	Credentials  *SkillSourceCredentials `json:"credentials"`
	Branch       string                  `json:"branch"`
	Root         string                  `json:"root"`
	Roots        []string                `json:"roots"`
	Layout       string                  `json:"layout"`
	GroupField   *string                 `json:"group_field"`
	IntervalSecs *int                    `json:"interval_secs"`
	Status       string                  `json:"status"`
	Versions     string                  `json:"versions"`
	TagPattern   string                  `json:"tag_pattern"`
	State        string                  `json:"state"`
	PausedReason *string                 `json:"paused_reason"`
	Effective    int                     `json:"effective_interval_secs"`
	NextRunAt    *string                 `json:"next_run_at"`
	LastRun      *skillstypes.Run        `json:"last_run"`
	CreatedAt    string                  `json:"created_at"`
	UpdatedAt    string                  `json:"updated_at"`
}

func SkillSourceOf(s skillstypes.Source) SkillSource {
	out := SkillSource{ID: s.ID, FolderID: s.FolderID, Kind: s.Kind, Hosting: s.Hosting, URL: s.URL, APIURL: s.APIURL, Path: s.Path,
		WorkingTree: s.WorkingTree, Branch: s.Branch, Roots: s.Roots, Layout: s.Layout, GroupField: s.GroupField, IntervalSecs: s.IntervalSecs, Status: string(s.NewStatus),
		Versions: s.Versions, TagPattern: s.TagPattern, State: s.State(), PausedReason: s.PausedReason, Effective: s.EffectiveSecs, NextRunAt: s.NextRunAt, LastRun: s.LastRun,
		CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt}
	if len(s.Roots) > 0 {
		out.Root = s.Roots[0]
	}
	if s.SecretID != nil {
		out.Credentials = &SkillSourceCredentials{SecretID: *s.SecretID, Name: s.SecretName}
	}
	if out.LastRun != nil && out.LastRun.Problems == nil {
		out.LastRun.Problems = []skillstypes.Problem{}
	}
	return out
}

type SkillSpace struct {
	Organization SkillRef `json:"organization"`
	Description  string   `json:"description"`
	CanWrite     bool     `json:"can_write"`
}

func SkillSpaceOf(s skillsservices.Space) SkillSpace {
	return SkillSpace{Organization: SkillRef{ID: s.Organization.ID, Slug: s.Organization.Slug, Name: s.Organization.Name},
		Description: s.Description, CanWrite: s.CanWrite}
}

func SkillSourcesOf(ss []skillstypes.Source) map[string][]SkillSource {
	out := make([]SkillSource, len(ss))
	for i, s := range ss {
		out[i] = SkillSourceOf(s)
	}
	return map[string][]SkillSource{"items": out}
}

type SkillImported struct {
	Folder SkillFolder `json:"folder"`
	Source SkillSource `json:"source"`
}

type SkillSetItem struct {
	ID     uuid.UUID `json:"id"`
	Path   string    `json:"path"`
	Name   string    `json:"name"`
	Skills *int64    `json:"skills,omitempty"`
	Status string    `json:"status,omitempty"`
}

type SkillSet struct {
	Mode      string         `json:"mode"`
	Folders   []SkillSetItem `json:"folders"`
	Skills    []SkillSetItem `json:"skills"`
	CanManage bool           `json:"can_manage"`
}

func SkillSetOf(s skillsservices.SkillSet) SkillSet {
	out := SkillSet{Mode: s.Mode, Folders: []SkillSetItem{}, Skills: []SkillSetItem{}, CanManage: s.CanManage}
	for _, f := range s.Folders {
		out.Folders = append(out.Folders, SkillSetItem{ID: f.ID, Path: f.Path, Name: f.Name, Skills: new(f.Skills)})
	}
	for _, k := range s.Skills {
		out.Skills = append(out.Skills, SkillSetItem{ID: k.ID, Path: k.Path, Name: k.Name, Status: string(k.Status)})
	}
	return out
}

type SkillDefaults struct {
	SyncIntervalSecs int64 `json:"sync_interval_secs"`
}
