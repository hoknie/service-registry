package catalog

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	TreeDefaultDepth = 3
	TreeMaxDepth     = 10
	GraceDefaultSecs = 86_400
	GraceMaxSecs     = 604_800
)

func trim(s string) string { return strings.TrimFunc(s, unicode.IsSpace) }

func hasControl(s string) bool { return strings.IndexFunc(s, unicode.IsControl) >= 0 }

func hasSpaceOrControl(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0
}

func isLowerDigit(b byte) bool { return (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') }

func isAlnum(b byte) bool { return isLowerDigit(b) || (b >= 'A' && b <= 'Z') }

func ValidateKind(raw string) (NodeKind, error) {
	if k, ok := ParseKind(raw); ok {
		return k, nil
	}
	return "", InvalidKind
}

func ValidateSlug(raw string) (string, error) {
	s := strings.ToLower(trim(raw))
	if len(s) < 1 || len(s) > 63 || s[0] == '-' || s[len(s)-1] == '-' {
		return "", InvalidSlug
	}
	for i := 0; i < len(s); i++ {
		if !isLowerDigit(s[i]) && s[i] != '-' {
			return "", InvalidSlug
		}
	}
	return s, nil
}

func ValidateName(raw string) (string, error) {
	n := trim(raw)
	if c := utf8.RuneCountInString(n); c < 1 || c > 100 || hasControl(n) {
		return "", InvalidNodeName
	}
	return n, nil
}

func ValidateDescription(raw string) (string, error) {
	bad := strings.IndexFunc(raw, func(r rune) bool {
		return unicode.IsControl(r) && r != '\n' && r != '\t' && r != '\r'
	}) >= 0
	if utf8.RuneCountInString(raw) > 2000 || bad {
		return "", InvalidDescription
	}
	return trim(raw), nil
}

func IsLabelKey(k string) bool { return labelKeyOK(k) }

func labelKeyOK(k string) bool {
	if len(k) < 1 || len(k) > 63 || !isAlnum(k[0]) || !isAlnum(k[len(k)-1]) {
		return false
	}
	for i := 0; i < len(k); i++ {
		if b := k[i]; !isLowerDigit(b) && b != '.' && b != '_' && b != '-' {
			return false
		}
	}
	return true
}

func ValidateLabels(raw LabelsInput) (Labels, error) {
	if raw.Err != nil {
		return nil, raw.Err
	}
	if len(raw.Labels) > 32 {
		return nil, InvalidLabels
	}
	out := make(Labels, len(raw.Labels))
	for k, v := range raw.Labels {
		if !labelKeyOK(k) || utf8.RuneCountInString(v) > 63 || hasControl(v) {
			return nil, InvalidLabels
		}
		out[k] = v
	}
	return out, nil
}

func optional(raw string) *string {
	if t := trim(raw); t != "" {
		return &t
	}
	return nil
}

func ValidateForge(raw string) (*Forge, error) {
	f := optional(raw)
	if f == nil {
		return nil, nil
	}
	if forge, ok := ParseForge(*f); ok {
		return &forge, nil
	}
	return nil, InvalidForge
}

func ValidateRepoURL(raw string) (*string, error) {
	u := optional(raw)
	if u == nil {
		return nil, nil
	}
	rest, ok := strings.CutPrefix(*u, "https://")
	if !ok {
		rest, ok = strings.CutPrefix(*u, "http://")
	}
	host := rest
	if i := strings.IndexAny(rest, "/?#"); i >= 0 {
		host = rest[:i]
	}
	hostOK := ok && host != "" && !strings.HasPrefix(host, ":") && !strings.Contains(host, "@")
	if !hostOK || utf8.RuneCountInString(*u) > 2048 || hasSpaceOrControl(*u) {
		return nil, InvalidRepoURL
	}
	return u, nil
}

func ValidateBranch(raw string) (*string, error) {
	b := optional(raw)
	if b == nil {
		return nil, nil
	}
	if utf8.RuneCountInString(*b) > 255 || strings.HasPrefix(*b, "-") || hasSpaceOrControl(*b) {
		return nil, InvalidBranch
	}
	return b, nil
}

func ValidateRepo(kind NodeKind, raw RepoInput) (Repo, error) {
	if kind != KindProject {
		if raw.IsEmpty() {
			return Repo{}, nil
		}
		return Repo{}, InvalidRepoFieldsNotAllowed
	}
	var repo Repo
	var err error
	if raw.Forge != nil {
		if repo.Forge, err = ValidateForge(*raw.Forge); err != nil {
			return Repo{}, err
		}
	}
	if raw.RepoURL != nil {
		if repo.RepoURL, err = ValidateRepoURL(*raw.RepoURL); err != nil {
			return Repo{}, err
		}
	}
	if raw.DefaultBranch != nil {
		if repo.DefaultBranch, err = ValidateBranch(*raw.DefaultBranch); err != nil {
			return Repo{}, err
		}
	}
	return repo, nil
}

func ValidateRepoChanges(kind NodeKind, raw RepoInput, changes *NodeChanges) error {
	if kind != KindProject && !raw.IsEmpty() {
		return InvalidRepoFieldsNotAllowed
	}
	if raw.Forge != nil {
		f, err := ValidateForge(*raw.Forge)
		if err != nil {
			return err
		}
		changes.Forge = Change[Forge]{Set: true, Value: f}
	}
	if raw.RepoURL != nil {
		u, err := ValidateRepoURL(*raw.RepoURL)
		if err != nil {
			return err
		}
		changes.RepoURL = Change[string]{Set: true, Value: u}
	}
	if raw.DefaultBranch != nil {
		b, err := ValidateBranch(*raw.DefaultBranch)
		if err != nil {
			return err
		}
		changes.DefaultBranch = Change[string]{Set: true, Value: b}
	}
	return nil
}

func ValidateDepth(raw *int64) (uint32, error) {
	if raw == nil {
		return TreeDefaultDepth, nil
	}
	if *raw < 1 || *raw > TreeMaxDepth {
		return 0, InvalidDepth
	}
	return uint32(*raw), nil
}

func ValidateGrace(raw *int64) (uint64, error) {
	if raw == nil {
		return GraceDefaultSecs, nil
	}
	if *raw < 0 || *raw > GraceMaxSecs {
		return 0, InvalidGracePeriod
	}
	return uint64(*raw), nil
}

func ValidateClusterObservation(kind NodeKind, in *FlagInput) (*bool, error) {
	if in == nil {
		return nil, nil
	}
	if kind != KindProject {
		return nil, InvalidRepoFieldsNotAllowed
	}
	if in.Err != nil {
		return nil, in.Err
	}
	v := in.Value
	return &v, nil
}
