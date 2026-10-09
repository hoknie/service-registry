package knowledge

import (
	"path"
	"strings"
	"unicode/utf8"

	"github.com/bmatcuk/doublestar/v4"
)

type Settings struct {
	Include  []string
	Exclude  []string
	Branches []string
}

const (
	maxPatterns      = 50
	maxPatternLength = 200
)

func DefaultSettings() Settings { return Settings{Exclude: []string{}, Branches: []string{}} }

func ValidateSettings(s Settings) (Settings, error) {
	if s.Exclude == nil {
		s.Exclude = []string{}
	}
	if s.Branches == nil {
		s.Branches = []string{}
	}
	for _, list := range [][]string{s.Include, s.Exclude, s.Branches} {
		if !validPatterns(list) {
			return Settings{}, InvalidSettings
		}
	}
	return s, nil
}

func validPatterns(list []string) bool {
	if len(list) > maxPatterns {
		return false
	}
	for _, p := range list {
		if p == "" || utf8.RuneCountInString(p) > maxPatternLength || !utf8.ValidString(p) ||
			strings.ContainsAny(p, "\x00\n\r") || !doublestar.ValidatePattern(p) {
			return false
		}
	}
	return true
}

func IsRootReadme(p string) bool {
	if strings.Contains(p, "/") {
		return false
	}
	name := strings.ToLower(p)
	return name == "readme" || (strings.HasPrefix(name, "readme.") && len(name) > len("readme."))
}

func (s Settings) Collects(p string) bool {
	p = strings.TrimPrefix(path.Clean("/"+p), "/")
	included := false
	if s.Include == nil {
		included = IsRootReadme(p)
	} else {
		for _, pattern := range s.Include {
			if ok, _ := doublestar.Match(pattern, p); ok {
				included = true
				break
			}
		}
	}
	if !included {
		return false
	}
	for _, pattern := range s.Exclude {
		if ok, _ := doublestar.Match(pattern, p); ok {
			return false
		}
	}
	return true
}

func (s Settings) CollectsBranch(name, defaultBranch string) bool {
	if name != "" && name == defaultBranch {
		return true
	}
	for _, pattern := range s.Branches {
		if ok, _ := doublestar.Match(pattern, name); ok {
			return true
		}
	}
	return false
}
