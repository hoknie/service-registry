package types

import (
	"errors"
	"io/fs"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	catalogtypes "svc-registry/internal/feature/catalog/types"
)

type SourceKind string

const (
	SourceRemote   SourceKind = "remote"
	SourceLocalDir SourceKind = "local_dir"
	SourceLocalGit SourceKind = "local_git"
	SourceForge    SourceKind = "forge"
)

const LocalBranch = "local"

const (
	SourceTable             = "knowledge_sources"
	SourceColumnCredentials = "credentials_enc"
)

type Source struct {
	Kind           SourceKind
	Forge          string
	URL            string
	APIURL         string
	Path           string
	Credentials    catalogtypes.Credentials
	Heads          map[string]string
	DefaultBranch  string
	WorkingTree    bool
	IncludeIgnored bool
	UpdatedAt      string
	Origin         string
}

const (
	OriginOwn   = "own"
	OriginForge = "forge"
)

func (s Source) IsLocal() bool { return s.Kind == SourceLocalDir || s.Kind == SourceLocalGit }

func (s Source) FullPath() string {
	u, err := url.Parse(s.URL)
	if err != nil {
		return ""
	}
	return strings.TrimSuffix(strings.Trim(u.Path, "/"), ".git")
}

type SourceInput struct {
	Kind           string
	Forge          string
	URL            string
	APIURL         string
	Path           string
	SecretID       *string
	Inline         bool
	HasCredential  bool
	NoCredentials  bool
	WorkingTree    *bool
	IncludeIgnored *bool
}

type ValidSource struct {
	Source
	SecretID *uuid.UUID
	Keep     bool
}

var forges = map[string]bool{"github": true, "gitlab": true, "gitea": true, "forgejo": true}

func ValidateSource(in SourceInput) (ValidSource, error) {
	out := ValidSource{Source: Source{Kind: SourceKind(in.Kind)}}
	if in.WorkingTree != nil && *in.WorkingTree && out.Kind != SourceLocalGit {
		return ValidSource{}, InvalidSource
	}
	out.WorkingTree = out.Kind == SourceLocalGit && (in.WorkingTree == nil || *in.WorkingTree)
	out.IncludeIgnored = in.IncludeIgnored != nil && *in.IncludeIgnored
	if out.IncludeIgnored && !out.WorkingTree {
		return ValidSource{}, InvalidSource
	}
	switch out.Kind {
	case SourceRemote:
		if !forges[in.Forge] || strings.TrimSpace(in.Path) != "" {
			return ValidSource{}, InvalidSource
		}
		web, ok := cleanURL(in.URL, true)
		if !ok {
			return ValidSource{}, InvalidSource
		}
		api := strings.TrimSuffix(strings.TrimSpace(in.APIURL), "/")
		if api == "" {
			api = defaultAPI(in.Forge, web)
		} else if _, ok := cleanURL(api, false); !ok {
			return ValidSource{}, InvalidSource
		}
		out.Forge, out.URL, out.APIURL = in.Forge, web, api
		switch {
		case in.Inline:
			return ValidSource{}, catalogtypes.InvalidCredentialsInline
		case in.SecretID != nil:
			id, err := uuid.Parse(strings.TrimSpace(*in.SecretID))
			if err != nil {
				return ValidSource{}, InvalidSource
			}
			out.SecretID = &id
		case in.HasCredential && !in.NoCredentials:
			return ValidSource{}, InvalidSource
		default:
			out.Keep = !in.HasCredential
		}
	case SourceLocalDir, SourceLocalGit:
		p := strings.TrimSpace(in.Path)
		if p == "" || len(p) > 4096 || !utf8.ValidString(p) || strings.ContainsRune(p, 0) || !filepath.IsAbs(p) ||
			in.Forge != "" || in.URL != "" || in.APIURL != "" || in.SecretID != nil || in.Inline {
			return ValidSource{}, InvalidSource
		}
		if filepath.Clean(p) != p {
			return ValidSource{}, InvalidPath
		}
		out.Path = p
	default:
		return ValidSource{}, InvalidSource
	}
	return out, nil
}

func cleanURL(raw string, repo bool) (string, bool) {
	raw = strings.TrimSuffix(strings.TrimSpace(raw), "/")
	u, err := url.Parse(raw)
	if err != nil || len(raw) > 2048 || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", false
	}
	host := u.Hostname()
	local := host == "localhost" || host == "127.0.0.1" || host == "::1"
	if u.Scheme != "https" && !(u.Scheme == "http" && local) {
		return "", false
	}
	if repo {
		p := strings.TrimSuffix(strings.Trim(u.Path, "/"), ".git")
		if strings.Count(p, "/") < 1 || strings.Contains(p, "//") || path.Clean("/"+p) != "/"+p {
			return "", false
		}
		u.Path = "/" + p
	}
	return u.String(), true
}

func defaultAPI(kind, web string) string {
	u, _ := url.Parse(web)
	base := u.Scheme + "://" + u.Host
	switch {
	case kind == "github" && u.Hostname() == "github.com":
		return "https://api.github.com"
	case kind == "github":
		return base + "/api/v3"
	case kind == "gitlab":
		return base + "/api/v4"
	}
	return base + "/api/v1"
}

type Failure string

const (
	FailSourceNotFound   Failure = "source.not_found"
	FailNotARepository   Failure = "source.not_a_repository"
	FailSourceUnreadable Failure = "source.unreadable"
	FailNotAllowed       Failure = "source.not_allowed"
)

func (f Failure) Error() string { return string(f) }

func CommitOf(head string) string {
	commit, _, _ := strings.Cut(head, "+")
	return commit
}

func WithinRoot(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func LocalPathAllowed(roots []string, p string) error {
	if len(roots) == 0 {
		return ConflictLocalDisabled
	}
	resolved := p
	if r, err := filepath.EvalSymlinks(p); err == nil {
		resolved = r
	}
	for _, root := range roots {
		if WithinRoot(root, resolved) {
			return nil
		}
	}
	return InvalidPath
}

func ResolveLocalPath(roots []string, p string) (string, error) {
	resolved, err := filepath.EvalSymlinks(p)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", FailSourceNotFound
	case err != nil:
		return "", FailSourceUnreadable
	}
	for _, root := range roots {
		if WithinRoot(root, resolved) {
			return resolved, nil
		}
	}
	return "", FailNotAllowed
}
