package forge

import (
	"path"
	"strings"
)

func BranchFilter(s Settings, defaultBranch string) func(name string) bool {
	return func(name string) bool {
		if name == defaultBranch || len(s.BranchInclude) == 0 {
			return true
		}
		for _, p := range s.BranchInclude {
			if ok, _ := path.Match(p, name); ok {
				return true
			}
		}
		return false
	}
}

func Keep(s Settings, r RemoteRepo) bool {
	if (r.Archived && !s.IncludeArchived) || (r.Fork && !s.IncludeForks) {
		return false
	}
	name := strings.ToLower(r.Name)
	matches := func(patterns []string) bool {
		for _, p := range patterns {
			if ok, _ := path.Match(strings.ToLower(p), name); ok {
				return true
			}
		}
		return false
	}
	if len(s.NameInclude) > 0 && !matches(s.NameInclude) {
		return false
	}
	return !matches(s.NameExclude)
}
