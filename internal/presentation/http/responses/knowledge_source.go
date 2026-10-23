package responses

import (
	catalogtypes "svc-registry/internal/feature/catalog/types"
	knowledgeservices "svc-registry/internal/feature/knowledge/services"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/pkg/ptr"
)

type SourceCredentials struct {
	Mode   string     `json:"mode"`
	Secret *SecretRef `json:"secret"`
}

func sourceCredentialsOf(c catalogtypes.Credentials) SourceCredentials {
	out := SourceCredentials{Mode: string(c.Kind), Secret: SecretRefOf(c.Secret)}
	if c.Kind == catalogtypes.CredentialsSecret && out.Secret == nil && c.SecretID != nil {
		out.Secret = &SecretRef{ID: *c.SecretID}
	}
	return out
}

type KnowledgeSource struct {
	Kind           string            `json:"kind"`
	Origin         string            `json:"origin"`
	Forge          *string           `json:"forge"`
	URL            *string           `json:"url"`
	APIURL         *string           `json:"api_url"`
	Path           *string           `json:"path"`
	Credentials    SourceCredentials `json:"credentials"`
	WorkingTree    bool              `json:"working_tree"`
	IncludeIgnored bool              `json:"include_ignored"`
	UpdatedAt      string            `json:"updated_at"`
}

func KnowledgeSourceOf(s *knowledgetypes.Source) *KnowledgeSource {
	if s == nil {
		return nil
	}
	origin := s.Origin
	if origin == "" {
		origin = knowledgetypes.OriginOwn
	}
	return &KnowledgeSource{Kind: string(s.Kind), Origin: origin, Forge: ptr.NonEmpty(s.Forge), URL: ptr.NonEmpty(s.URL), APIURL: ptr.NonEmpty(s.APIURL),
		Path: ptr.NonEmpty(s.Path), Credentials: sourceCredentialsOf(s.Credentials),
		WorkingTree: s.WorkingTree, IncludeIgnored: s.IncludeIgnored, UpdatedAt: s.UpdatedAt}
}

type SourceCheck struct {
	OK            bool     `json:"ok"`
	Branches      int      `json:"branches"`
	BranchNames   []string `json:"branch_names"`
	DefaultBranch *string  `json:"default_branch"`
	APIURL        *string  `json:"api_url"`
}

type SourceCheckFailed struct {
	OK        bool   `json:"ok"`
	ErrorCode string `json:"error_code"`
}

func SourceCheckOf(c knowledgeservices.SourceCheck) any {
	if !c.OK {
		return SourceCheckFailed{ErrorCode: c.ErrorCode}
	}
	names := c.BranchNames
	if names == nil {
		names = []string{}
	}
	out := SourceCheck{OK: true, Branches: c.Branches, BranchNames: names}
	if c.DefaultBranch != "" && c.DefaultBranch != knowledgetypes.LocalBranch {
		out.DefaultBranch = &c.DefaultBranch
	}
	if c.APIURL != "" {
		out.APIURL = &c.APIURL
	}
	return out
}
