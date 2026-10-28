package types

import (
	"bytes"
	"regexp"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"

	"svc-registry/internal/platform/apperr"
)

const ManifestFile = "SKILL.md"

var (
	namePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	tagPattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9.+#-]{0,39}$`)
)

type Manifest struct {
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	License       string         `json:"license,omitempty"`
	Compatibility string         `json:"compatibility,omitempty"`
	AllowedTools  []string       `json:"allowed_tools,omitempty"`
	Title         string         `json:"title,omitempty"`
	Tags          []string       `json:"tags,omitempty"`
	Author        string         `json:"author,omitempty"`
	Image         string         `json:"image,omitempty"`
	Version       string         `json:"version,omitempty"`
	Fields        map[string]any `json:"-"`
}

func (m Manifest) DisplayTitle() string {
	if m.Title != "" {
		return m.Title
	}
	return m.Name
}

func Frontmatter(content []byte) (string, string, bool) {
	text := strings.ReplaceAll(strings.TrimPrefix(string(content), "\ufeff"), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) < 2 || strings.TrimRight(lines[0], " \t") != "---" {
		return "", text, false
	}
	for i := 1; i < len(lines); i++ {
		if end := strings.TrimRight(lines[i], " \t"); end == "---" || end == "..." {
			return strings.Join(lines[1:i], "\n"), strings.Join(lines[i+1:], "\n"), true
		}
	}
	return "", text, false
}

func ParseManifest(content []byte) (Manifest, error) {
	if !utf8.Valid(content) {
		return Manifest{}, ManifestInvalid()
	}
	front, _, ok := Frontmatter(content)
	if !ok {
		return Manifest{}, ManifestInvalid(apperr.Field{Path: "frontmatter", Code: "missing"})
	}
	var raw map[string]any
	dec := yaml.NewDecoder(bytes.NewReader([]byte(front)))
	if err := dec.Decode(&raw); err != nil && front != "" {
		return Manifest{}, ManifestInvalid(apperr.Field{Path: "frontmatter", Code: "invalid"})
	}
	var fields []apperr.Field
	bad := func(path, code string) { fields = append(fields, apperr.Field{Path: path, Code: code}) }
	m := Manifest{
		Name:          text(raw, "name", 64, true, bad),
		Description:   text(raw, "description", 1024, true, bad),
		License:       text(raw, "license", 200, false, bad),
		Compatibility: text(raw, "compatibility", 500, false, bad),
		AllowedTools:  list(raw["allowed-tools"], "allowed-tools", 100, bad),
	}
	if m.Name != "" && !namePattern.MatchString(m.Name) {
		bad("name", "invalid")
	}
	meta, isMap := raw["metadata"].(map[string]any)
	if raw["metadata"] != nil && !isMap {
		bad("metadata", "invalid")
	}
	m.Title = text(meta, "title", 200, false, func(p, c string) { bad("metadata."+p, c) })
	m.Author = text(meta, "author", 200, false, func(p, c string) { bad("metadata."+p, c) })
	m.Image = text(meta, "image", 512, false, func(p, c string) { bad("metadata."+p, c) })
	if m.Image != "" {
		if clean, err := CleanPath(m.Image); err == nil {
			m.Image = clean
		} else {
			bad("metadata.image", "invalid")
		}
	}
	m.Tags = manifestTags(meta["tags"], func(c string) { bad("metadata.tags", c) })
	if v := scalar(meta["version"]); v != "" {
		canonical, err := ParseVersion(v)
		if err != nil {
			bad("metadata.version", "invalid")
		}
		m.Version = canonical
	} else if meta["version"] != nil {
		bad("metadata.version", "invalid")
	}
	if len(fields) > 0 {
		return Manifest{}, ManifestInvalid(fields...)
	}
	m.Fields = raw
	return m, nil
}

func scalar(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case int, int64, float64, bool:
		return strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(yamlScalar(x), "\n"), "\n"))
	}
	return ""
}

func yamlScalar(v any) string {
	out, err := yaml.Marshal(v)
	if err != nil {
		return ""
	}
	return string(out)
}

func text(raw map[string]any, key string, limit int, required bool, bad func(path, code string)) string {
	v, ok := raw[key]
	if !ok || v == nil {
		if required {
			bad(key, "required")
		}
		return ""
	}
	s := scalar(v)
	switch {
	case s == "" && required:
		bad(key, "required")
	case s == "" && v != nil:
		if _, isString := v.(string); !isString {
			bad(key, "invalid")
		}
	case utf8.RuneCountInString(s) > limit:
		bad(key, "too_long")
	}
	return s
}

func list(v any, path string, limit int, bad func(path, code string)) []string {
	var out []string
	switch x := v.(type) {
	case nil:
		return []string{}
	case string:
		out = strings.Fields(strings.ReplaceAll(x, ",", " "))
	case []any:
		for _, item := range x {
			s := scalar(item)
			if s == "" {
				bad(path, "invalid")
				return []string{}
			}
			out = append(out, s)
		}
	default:
		bad(path, "invalid")
		return []string{}
	}
	if len(out) > limit {
		bad(path, "too_long")
	}
	return out
}

func manifestTags(v any, bad func(code string)) []string {
	var raw []string
	switch x := v.(type) {
	case nil:
		return []string{}
	case string:
		raw = strings.Split(x, ",")
	case []any:
		for _, item := range x {
			raw = append(raw, scalar(item))
		}
	default:
		bad("invalid")
		return []string{}
	}
	tags, err := NormalizeTags(raw)
	if err != nil {
		bad("invalid")
		return []string{}
	}
	return tags
}

func NormalizeTags(raw []string) ([]string, error) {
	out := []string{}
	seen := map[string]bool{}
	for _, t := range raw {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" || seen[t] {
			continue
		}
		if !tagPattern.MatchString(t) {
			return nil, InvalidSkill
		}
		seen[t] = true
		out = append(out, t)
	}
	if len(out) > 20 {
		return nil, InvalidSkill
	}
	return out, nil
}

func ValidName(name string) bool { return len(name) <= 64 && namePattern.MatchString(name) }

func ValidTag(tag string) bool { return tagPattern.MatchString(tag) }
