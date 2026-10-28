package types

import (
	"path"
	"regexp"
	"slices"
	"strings"
)

const (
	VersionsBranch    = "branch"
	VersionsTags      = "tags"
	DefaultTagPattern = "v{version}"
	MaxTagsPerRun     = 50
)

func FindSkillDirs(paths []string, root string) []string {
	root = strings.Trim(root, "/")
	var dirs []string
	for _, p := range paths {
		if path.Base(p) != ManifestFile {
			continue
		}
		dir := path.Dir(p)
		if dir == "." {
			dir = ""
		}
		if root != "" && dir != root && !strings.HasPrefix(dir, root+"/") {
			continue
		}
		dirs = append(dirs, dir)
	}
	slices.SortFunc(dirs, func(a, b string) int { return len(a) - len(b) })
	var out []string
	for _, d := range dirs {
		if !slices.ContainsFunc(out, func(o string) bool { return Under(d, o) }) {
			out = append(out, d)
		}
	}
	slices.Sort(out)
	return out
}

func Under(p, dir string) bool {
	return dir == "" || p == dir || strings.HasPrefix(p, dir+"/")
}

func FilesUnder(paths []string, dir string) []string {
	var out []string
	for _, p := range paths {
		if Under(p, dir) {
			out = append(out, p)
		}
	}
	return out
}

func RelativePath(p, dir string) string {
	if dir == "" {
		return p
	}
	return strings.TrimPrefix(p, dir+"/")
}

type TagPattern struct {
	re      *regexp.Regexp
	hasName bool
}

func ParseTagPattern(raw string) (TagPattern, error) {
	if raw == "" || len(raw) > 200 || strings.Count(raw, "{version}") != 1 || strings.Count(raw, "{name}") > 1 {
		return TagPattern{}, InvalidTagPattern
	}
	expr := regexp.QuoteMeta(raw)
	expr = strings.Replace(expr, regexp.QuoteMeta("{version}"), `(?P<version>[0-9A-Za-z.+-]+)`, 1)
	hasName := strings.Contains(raw, "{name}")
	expr = strings.Replace(expr, regexp.QuoteMeta("{name}"), `(?P<name>[a-z0-9]+(?:-[a-z0-9]+)*)`, 1)
	if strings.ContainsAny(strings.NewReplacer("{version}", "", "{name}", "").Replace(raw), "{}") {
		return TagPattern{}, InvalidTagPattern
	}
	return TagPattern{re: regexp.MustCompile("^" + expr + "$"), hasName: hasName}, nil
}

func (t TagPattern) Match(tag string) (name, version string, ok bool) {
	m := t.re.FindStringSubmatch(tag)
	if m == nil {
		return "", "", false
	}
	v, err := ParseVersion(m[t.re.SubexpIndex("version")])
	if err != nil {
		return "", "", false
	}
	if t.hasName {
		name = m[t.re.SubexpIndex("name")]
	}
	return name, v, true
}

func (t TagPattern) ForAll() bool { return !t.hasName }

type Tagged struct {
	Name    string
	Version string
	Commit  string
	Skill   string
}

func OrderTags(tags map[string]string, pattern TagPattern) []Tagged {
	out := make([]Tagged, 0, len(tags))
	for name, commit := range tags {
		skill, version, ok := pattern.Match(name)
		if ok {
			out = append(out, Tagged{Name: name, Version: version, Commit: commit, Skill: skill})
		}
	}
	slices.SortFunc(out, func(a, b Tagged) int {
		if c := Compare(b.Version, a.Version); c != 0 {
			return c
		}
		return strings.Compare(a.Name, b.Name)
	})
	return out
}

func NewTags(all, done map[string]string, pattern TagPattern) map[string]string {
	out := map[string]string{}
	for name, commit := range all {
		if _, _, ok := pattern.Match(name); ok && done[name] != commit {
			out[name] = commit
		}
	}
	return out
}
