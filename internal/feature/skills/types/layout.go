package types

import (
	"net/url"
	"path"
	"regexp"
	"slices"
	"strings"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"unicode/utf8"
)

const (
	LayoutFlat  = "flat"
	LayoutTree  = "tree"
	LayoutField = "field"
	MaxRoots    = 20
)

var (
	groupFieldPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,100}$`)
	linkMarkers       = []string{"/-/tree/", "/-/blob/", "/src/branch/", "/tree/", "/blob/"}
	cyrillic          = map[rune]string{'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "e", 'ж': "zh", 'з': "z",
		'и': "i", 'й': "y", 'к': "k", 'л': "l", 'м': "m", 'н': "n", 'о': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u",
		'ф': "f", 'х': "kh", 'ц': "ts", 'ч': "ch", 'ш': "sh", 'щ': "shch", 'ъ': "", 'ы': "y", 'ь': "", 'э': "e", 'ю': "yu",
		'я': "ya", 'і': "i", 'ї': "yi", 'є': "ye", 'ґ': "g"}
)

type RepoLink struct {
	URL    string
	Branch string
	Root   string
}

func ParseRepoLink(raw string) (RepoLink, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return RepoLink{URL: raw}, false
	}
	for _, marker := range linkMarkers {
		repo, rest, ok := strings.Cut(u.Path, marker)
		if !ok || strings.Trim(repo, "/") == "" {
			continue
		}
		branch, root, _ := strings.Cut(strings.Trim(rest, "/"), "/")
		if branch == "" {
			continue
		}
		u.Path, u.RawPath, u.RawQuery, u.Fragment = strings.TrimSuffix(repo, "/"), "", "", ""
		return RepoLink{URL: u.String(), Branch: branch, Root: root}, true
	}
	return RepoLink{URL: raw}, false
}

func SourceRoots(roots []string, root *string) ([]string, error) {
	if root != nil {
		if roots != nil {
			return nil, InvalidSource
		}
		roots = []string{*root}
	}
	return CleanRoots(roots)
}

func CleanRoots(raw []string) ([]string, error) {
	if len(raw) == 0 {
		return []string{""}, nil
	}
	if len(raw) > MaxRoots {
		return nil, InvalidSource
	}
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		clean, err := CleanRoot(r)
		if err != nil || len(clean) > 512 || slices.Contains(out, clean) {
			return nil, InvalidSource
		}
		out = append(out, clean)
	}
	return out, nil
}

func CleanLayout(layout string, field *string) (string, *string, error) {
	if layout == "" {
		layout = LayoutFlat
	}
	switch layout {
	case LayoutFlat, LayoutTree:
		if field != nil && strings.TrimSpace(*field) != "" {
			return "", nil, InvalidSource
		}
		return layout, nil, nil
	case LayoutField:
		if field == nil || !groupFieldPattern.MatchString(strings.TrimSpace(*field)) {
			return "", nil, InvalidSource
		}
		return layout, new(strings.TrimSpace(*field)), nil
	}
	return "", nil, InvalidSource
}

func FieldValue(m Manifest, field string) string {
	var v any = m.Fields
	for part := range strings.SplitSeq(field, ".") {
		obj, ok := v.(map[string]any)
		if !ok {
			return ""
		}
		v = obj[part]
	}
	if list, ok := v.([]any); ok {
		if len(list) == 0 {
			return ""
		}
		v = list[0]
	}
	s := scalar(v)
	if utf8.RuneCountInString(s) > 200 {
		s = string([]rune(s)[:200])
	}
	return s
}

func Place(layout, field, root, dir string, m Manifest) string {
	switch layout {
	case LayoutTree:
		parent := path.Dir(RelativePath(dir, root))
		if parent == "." || dir == root {
			return ""
		}
		return parent
	case LayoutField:
		return FieldValue(m, field)
	}
	return ""
}

func SlugOf(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if s, ok := cyrillic[r]; ok {
			b.WriteString(s)
			dash = false
			continue
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	s := strings.Trim(b.String(), "-")
	if len(s) > 64 {
		s = strings.TrimRight(s[:64], "-")
	}
	if s == "" {
		return "group"
	}
	return s
}

func FindSkillDirsIn(paths, roots []string) []string {
	var out []string
	for _, r := range roots {
		for _, d := range FindSkillDirs(paths, r) {
			if !slices.Contains(out, d) {
				out = append(out, d)
			}
		}
	}
	slices.Sort(out)
	return out
}

func RootOf(dir string, roots []string) string {
	best := ""
	for _, r := range roots {
		if Under(dir, r) && len(r) >= len(best) {
			best = r
		}
	}
	return best
}

func MissingRoots(paths, roots []string) []string {
	var out []string
	for _, r := range roots {
		if r != "" && !slices.ContainsFunc(paths, func(p string) bool { return Under(p, r) }) {
			out = append(out, r)
		}
	}
	return out
}

func KnowledgeSource(src Source) knowledgetypes.Source {
	out := knowledgetypes.Source{Kind: knowledgetypes.SourceKind(src.Kind), WorkingTree: src.WorkingTree}
	out.Forge, out.URL, out.APIURL, out.Path = value(src.Forge), value(src.URL), value(src.APIURL), value(src.Path)
	return out
}

func KnowledgeSettings(src Source) knowledgetypes.Settings {
	include := []string{}
	for _, r := range src.Roots {
		if r == "" {
			include = []string{"**"}
			break
		}
		include = append(include, r+"/**")
	}
	refs := []knowledgetypes.RefPattern{}
	if src.Branch != "" {
		refs = append(refs, knowledgetypes.RefPattern{Pattern: src.Branch, Kind: knowledgetypes.RefBranch})
	}
	if src.Versions == VersionsTags {
		refs = append(refs, knowledgetypes.RefPattern{Pattern: "**", Kind: knowledgetypes.RefTag})
	}
	return knowledgetypes.Settings{Include: include, Exclude: []string{}, Refs: refs}
}

func AuthorOf(src Source) string {
	if src.URL != nil {
		full := knowledgetypes.Source{URL: *src.URL}.FullPath()
		if full != "" {
			return full
		}
	}
	if src.Path != nil {
		return path.Base(*src.Path)
	}
	return ""
}

func value(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

type Link struct {
	URL    string
	Branch string
	Roots  []string
}

func LinkFields(remote bool, url, branch string, roots []string, root *string) Link {
	out := Link{URL: url, Branch: branch, Roots: roots}
	link, ok := ParseRepoLink(url)
	if !remote || !ok {
		return out
	}
	out.URL = link.URL
	if strings.TrimSpace(branch) == "" {
		out.Branch = link.Branch
	}
	if roots == nil && root == nil && link.Root != "" {
		out.Roots = []string{link.Root}
	}
	return out
}

func RepoName(url, dir string) string {
	raw := url
	if raw == "" {
		raw = dir
	}
	if link, ok := ParseRepoLink(raw); ok {
		raw = link.URL
	}
	name := strings.TrimSuffix(path.Base(strings.TrimRight(raw, "/")), ".git")
	if name == "." || name == "/" || name == "" {
		return "skills"
	}
	return name
}
