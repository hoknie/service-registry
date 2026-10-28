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
	Forge        *string                 `json:"forge"`
	URL          *string                 `json:"url"`
	APIURL       *string                 `json:"api_url"`
	Path         *string                 `json:"path"`
	WorkingTree  bool                    `json:"working_tree"`
	Credentials  *SkillSourceCredentials `json:"credentials"`
	Branch       string                  `json:"branch"`
	Root         string                  `json:"root"`
	IntervalSecs *int                    `json:"interval_secs"`
	Status       string                  `json:"status"`
	Versions     string                  `json:"versions"`
	TagPattern   string                  `json:"tag_pattern"`
	State        string                  `json:"state"`
	NextRunAt    *string                 `json:"next_run_at"`
	LastRun      *skillstypes.Run        `json:"last_run"`
	CreatedAt    string                  `json:"created_at"`
	UpdatedAt    string                  `json:"updated_at"`
}

func SkillSourceOf(s skillstypes.Source) SkillSource {
	out := SkillSource{ID: s.ID, FolderID: s.FolderID, Kind: s.Kind, Forge: s.Forge, URL: s.URL, APIURL: s.APIURL, Path: s.Path,
		WorkingTree: s.WorkingTree, Branch: s.Branch, Root: s.Root, IntervalSecs: s.IntervalSecs, Status: string(s.NewStatus),
		Versions: s.Versions, TagPattern: s.TagPattern, State: s.State(), NextRunAt: s.NextRunAt, LastRun: s.LastRun,
		CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt}
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
