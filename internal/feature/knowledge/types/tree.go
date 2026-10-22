package types

import (
	"sort"
	"strings"
)

type TreeDir struct {
	Name  string
	Path  string
	Files int
	Git   GitStatus
}

type TreeLevel struct {
	Dirs  []TreeDir
	Files []FileInfo
}

func NormalizeDir(raw string) (string, bool) {
	dir := strings.Trim(strings.TrimSpace(raw), "/")
	if dir == "" {
		return "", true
	}
	for _, s := range strings.Split(dir, "/") {
		if s == "" || s == "." || s == ".." {
			return "", false
		}
	}
	return dir, true
}

func Level(files []FileInfo, dir string) (TreeLevel, bool) {
	prefix := ""
	if dir != "" {
		prefix = dir + "/"
	}
	type agg struct {
		files                int
		untracked, nonIgnore bool
	}
	dirs := map[string]*agg{}
	out := TreeLevel{Dirs: []TreeDir{}, Files: []FileInfo{}}
	found := dir == ""
	for _, f := range files {
		if !strings.HasPrefix(f.Path, prefix) {
			continue
		}
		found = true
		rest := f.Path[len(prefix):]
		name, _, nested := strings.Cut(rest, "/")
		if !nested {
			out.Files = append(out.Files, f)
			continue
		}
		a := dirs[name]
		if a == nil {
			a = &agg{}
			dirs[name] = a
		}
		a.files++
		switch f.Meta.Git {
		case GitUntracked:
			a.untracked, a.nonIgnore = true, true
		case GitIgnored:
		default:
			a.nonIgnore = true
		}
	}
	if !found {
		return TreeLevel{}, false
	}
	for name, a := range dirs {
		d := TreeDir{Name: name, Path: prefix + name, Files: a.files}
		switch {
		case a.untracked:
			d.Git = GitUntracked
		case !a.nonIgnore:
			d.Git = GitIgnored
		}
		out.Dirs = append(out.Dirs, d)
	}
	sort.Slice(out.Dirs, func(i, j int) bool { return out.Dirs[i].Name < out.Dirs[j].Name })
	sort.Slice(out.Files, func(i, j int) bool { return out.Files[i].Path < out.Files[j].Path })
	return out, true
}
