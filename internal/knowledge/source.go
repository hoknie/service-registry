package knowledge

import (
	"context"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"svc-registry/internal/forge"
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

type Credentials struct {
	Mode        string
	Fingerprint *string
}

type Source struct {
	Kind           SourceKind
	Forge          string
	URL            string
	APIURL         string
	Path           string
	Credentials    Credentials
	Heads          map[string]string
	DefaultBranch  string
	WorkingTree    bool
	IncludeIgnored bool
	UpdatedAt      string
}

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
	Token          *string
	Reference      *string
	HasCredential  bool
	NoCredentials  bool
	WorkingTree    *bool
	IncludeIgnored *bool
}

type ValidSource struct {
	Source
	Token     *string
	Reference *string
	Keep      bool
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
		case in.Token != nil && in.Reference != nil:
			return ValidSource{}, InvalidSource
		case in.Token != nil:
			t := *in.Token
			if t == "" || len(t) > 4096 || strings.IndexFunc(t, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
				return ValidSource{}, InvalidSource
			}
			out.Token = &t
		case in.Reference != nil:
			ref := strings.TrimSpace(*in.Reference)
			if !forge.ValidRef(ref) {
				return ValidSource{}, InvalidSource
			}
			out.Reference = &ref
		case in.HasCredential && !in.NoCredentials:
			return ValidSource{}, InvalidSource
		default:
			out.Keep = !in.HasCredential
		}
	case SourceLocalDir, SourceLocalGit:
		p := strings.TrimSpace(in.Path)
		if p == "" || len(p) > 4096 || !utf8.ValidString(p) || strings.ContainsRune(p, 0) || !filepath.IsAbs(p) ||
			in.Forge != "" || in.URL != "" || in.APIURL != "" || in.Token != nil || in.Reference != nil {
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

type Reader interface {
	Heads(ctx context.Context) (heads map[string]string, defaultBranch string, err error)
	Tree(ctx context.Context, branch, head string) ([]Entry, bool, error)
	Read(ctx context.Context, e Entry, limit int64) ([]byte, error)
}

type ReaderFactory interface {
	Open(ctx context.Context, s Source, settings Settings, token string) (Reader, error)
}

func CommitOf(head string) string {
	commit, _, _ := strings.Cut(head, "+")
	return commit
}
