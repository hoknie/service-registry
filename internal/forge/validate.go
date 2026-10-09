package forge

import (
	"net/url"
	"path"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	MaxPatterns = 32
	MinInterval = 60
	MaxInterval = 86400
)

var (
	ownerSegment = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	envRef       = regexp.MustCompile(`^env:[A-Za-z_][A-Za-z0-9_]{0,127}$`)
)

func ValidateCreate(in CreateConnection, defaultInterval int) (Settings, error) {
	kind, ok := ParseKind(strings.TrimSpace(in.Kind))
	if !ok {
		return Settings{}, InvalidKind
	}
	s := Settings{Kind: kind, MirrorSubgroups: kind == KindGitlab, IntervalSecs: defaultInterval,
		NameInclude: []string{}, NameExclude: []string{}, BranchInclude: []string{}}
	apply(&s, UpdateConnection{
		APIURL: in.APIURL, OwnerPath: &in.OwnerPath, MirrorSubgroups: in.MirrorSubgroups,
		IncludeArchived: in.IncludeArchived, IncludeForks: in.IncludeForks,
		NameInclude: in.NameInclude, NameExclude: in.NameExclude, BranchInclude: in.BranchInclude,
		IntervalSecs: in.IntervalSecs,
	})
	if in.APIURL == nil {
		s.APIURL = kind.DefaultAPIURL()
	}
	return s, check(&s, in.IntervalSecs)
}

func ValidateUpdate(current Connection, in UpdateConnection) (Settings, error) {
	s := Settings{
		Kind: current.Kind, APIURL: current.APIURL, OwnerPath: current.OwnerPath,
		MirrorSubgroups: current.MirrorSubgroups, IncludeArchived: current.IncludeArchived,
		IncludeForks: current.IncludeForks, NameInclude: current.NameInclude,
		NameExclude: current.NameExclude, BranchInclude: current.BranchInclude, IntervalSecs: current.IntervalSecs,
	}
	apply(&s, in)
	return s, check(&s, in.IntervalSecs)
}

func apply(s *Settings, in UpdateConnection) {
	if in.APIURL != nil {
		s.APIURL = *in.APIURL
	}
	if in.OwnerPath != nil {
		s.OwnerPath = *in.OwnerPath
	}
	if in.MirrorSubgroups != nil {
		s.MirrorSubgroups = *in.MirrorSubgroups
	}
	if in.IncludeArchived != nil {
		s.IncludeArchived = *in.IncludeArchived
	}
	if in.IncludeForks != nil {
		s.IncludeForks = *in.IncludeForks
	}
	if in.NameInclude != nil {
		s.NameInclude = *in.NameInclude
	}
	if in.NameExclude != nil {
		s.NameExclude = *in.NameExclude
	}
	if in.BranchInclude != nil {
		s.BranchInclude = *in.BranchInclude
	}
}

func check(s *Settings, interval *int64) error {
	if interval != nil {
		if *interval < MinInterval || *interval > MaxInterval {
			return InvalidInterval
		}
		s.IntervalSecs = int(*interval)
	}
	api, err := validateAPIURL(s.APIURL)
	if err != nil {
		return err
	}
	s.APIURL = api
	owner, err := validateOwner(s.Kind, s.OwnerPath)
	if err != nil {
		return err
	}
	s.OwnerPath = owner
	if s.NameInclude, err = validatePatterns(s.NameInclude); err != nil {
		return err
	}
	if s.NameExclude, err = validatePatterns(s.NameExclude); err != nil {
		return err
	}
	if s.BranchInclude, err = validatePatterns(s.BranchInclude); err != nil {
		return err
	}
	if s.Kind != KindGitlab {
		s.MirrorSubgroups = false
	}
	return nil
}

func validateAPIURL(raw string) (string, error) {
	v := strings.TrimRight(strings.TrimSpace(raw), "/")
	u, err := url.Parse(v)
	if v == "" || len(v) > 2048 || err != nil || (u.Scheme != "http" && u.Scheme != "https") ||
		u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", InvalidAPIURL
	}
	return v, nil
}

func validateOwner(kind Kind, raw string) (string, error) {
	v := strings.Trim(strings.TrimSpace(raw), "/")
	if v == "" || len(v) > 255 {
		return "", InvalidOwnerPath
	}
	segments := strings.Split(v, "/")
	if len(segments) > 1 && kind != KindGitlab {
		return "", InvalidOwnerPath
	}
	for _, seg := range segments {
		if !ownerSegment.MatchString(seg) {
			return "", InvalidOwnerPath
		}
	}
	return v, nil
}

func validatePatterns(raw []string) ([]string, error) {
	if len(raw) > MaxPatterns {
		return nil, InvalidNamePatterns
	}
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if n := utf8.RuneCountInString(p); n < 1 || n > 100 {
			return nil, InvalidNamePatterns
		}
		if _, err := path.Match(strings.ToLower(p), ""); err != nil {
			return nil, InvalidNamePatterns
		}
		out = append(out, p)
	}
	return out, nil
}

func ValidateCredentials(in *CredentialsInput) (ValidCredentials, error) {
	if in == nil || (in.Token == nil) == (in.TokenRef == nil) {
		return ValidCredentials{}, InvalidCredentials
	}
	if in.Token != nil {
		t := *in.Token
		if t == "" || len(t) > 4096 || strings.IndexFunc(t, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
			return ValidCredentials{}, InvalidCredentials
		}
		return ValidCredentials{Token: &t}, nil
	}
	ref := strings.TrimSpace(*in.TokenRef)
	if !ValidRef(ref) {
		return ValidCredentials{}, InvalidCredentials
	}
	return ValidCredentials{Ref: &ref}, nil
}

func ValidRef(ref string) bool {
	if envRef.MatchString(ref) {
		return true
	}
	p, ok := strings.CutPrefix(ref, "file:")
	return ok && strings.HasPrefix(p, "/") && len(p) <= 4096 && !strings.ContainsRune(p, 0) &&
		strings.IndexFunc(p, unicode.IsControl) < 0
}

func ValidateWebhookMode(raw string) (WebhookMode, error) {
	if m, ok := ParseWebhookMode(raw); ok {
		return m, nil
	}
	return "", InvalidWebhookMode
}
