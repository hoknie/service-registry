package links

import (
	"errors"
	"net/url"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"svc-registry/pkg/linktemplate"
)

const MaxTemplateLen = linktemplate.MaxLen

const (
	VarProjectID    = "project.id"
	VarProjectSlug  = "project.slug"
	VarProjectName  = "project.name"
	VarPath         = "path"
	VarRepoFullPath = "repo.full_path"
	VarBranch       = "branch"
	VarService      = "service"
	VarEnvironment  = "environment"
	VarVersion      = "version"
	VarCommit       = "commit"
	VarCluster      = "cluster"
	VarNamespace    = "namespace"
	VarURL          = "url"
)

var fixedVars = []string{VarProjectID, VarProjectSlug, VarProjectName, VarPath, VarRepoFullPath, VarBranch,
	VarService, VarEnvironment, VarVersion, VarCommit, VarCluster, VarNamespace, VarURL}

var DeploymentVars = []string{VarService, VarEnvironment, VarVersion, VarCommit, VarCluster, VarNamespace, VarURL}

type Pattern = linktemplate.Pattern

type Values = linktemplate.Values

func usesDeployment(p *Pattern) bool {
	for _, n := range p.Names() {
		if slices.Contains(DeploymentVars, n) {
			return true
		}
	}
	return false
}

func Parse(template string) (*Pattern, error) {
	p, err := linktemplate.Parse(template, Known)
	var e *linktemplate.Error
	if errors.As(err, &e) {
		te := &TemplateError{Invalid: InvalidTemplate, Pos: e.Pos, Detail: e.Detail}
		if e.UnknownVariable {
			te.Invalid = UnknownVariable
		}
		return nil, te
	}
	return p, err
}

func Known(name string) bool {
	if slices.Contains(fixedVars, name) {
		return true
	}
	if rest, ok := strings.CutPrefix(name, "labels."); ok {
		return rest != ""
	}
	if rest, ok := strings.CutPrefix(name, "vars."); ok {
		return rest != ""
	}
	if rest, ok := strings.CutPrefix(name, "node."); ok {
		_, ok := NodeDepth(rest)
		return ok
	}
	return false
}

func NodeDepth(s string) (int, bool) {
	if s == "" || len(s) > 4 || s[0] == '0' {
		return 0, false
	}
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
		n = n*10 + int(s[i]-'0')
	}
	return n, true
}

func Escape(s string) string { return linktemplate.Escape(s) }

func CheckURL(s string) error {
	if utf8.RuneCountInString(s) > MaxTemplateLen || strings.IndexFunc(s, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
		return InvalidLinkURL
	}
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
		return InvalidLinkURL
	}
	return nil
}

func sampleValues(name string) (string, bool) {
	switch name {
	case VarProjectID:
		return "01890a5d-ac96-774b-bcce-b302099a8057", true
	case VarProjectSlug, VarService, VarNamespace:
		return "api", true
	case VarProjectName:
		return "Billing API", true
	case VarPath:
		return "acme/backend/api", true
	case VarRepoFullPath:
		return "acme/api", true
	case VarBranch:
		return "main", true
	case VarEnvironment:
		return "production", true
	case VarVersion:
		return "1.0.0", true
	case VarCommit:
		return "0123456789abcdef0123456789abcdef01234567", true
	case VarCluster:
		return "prod-1", true
	case VarURL:
		return "https://api.example.com", true
	}
	if strings.HasPrefix(name, "node.") {
		return "acme", true
	}
	return "value", true
}

func CheckTemplate(template string) error {
	p, err := Parse(template)
	if err != nil {
		return err
	}
	s, _ := p.Expand(sampleValues)
	return CheckURL(s)
}
