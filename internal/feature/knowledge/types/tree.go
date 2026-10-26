package types

import (
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
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

const maxTitleRunes = 200

var titleMarkup = regexp.MustCompile("!?\\[([^\\]]*)\\]\\([^)]*\\)|[*_`]")

func IsMarkdown(path string) bool {
	p := strings.ToLower(path)
	return strings.HasSuffix(p, ".md") || strings.HasSuffix(p, ".markdown")
}

func MarkdownTitle(content string) string {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	fence := ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if fence != "" {
			if strings.HasPrefix(trimmed, fence) {
				fence = ""
			}
			continue
		}
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			fence = trimmed[:3]
			continue
		}
		if rest, ok := strings.CutPrefix(trimmed, "# "); ok {
			return cleanTitle(strings.TrimRight(strings.TrimSpace(rest), "# "))
		}
		if trimmed != "" && i+1 < len(lines) && isSetextRule(lines[i+1]) && !strings.HasPrefix(trimmed, "#") {
			return cleanTitle(trimmed)
		}
	}
	return ""
}

func isSetextRule(line string) bool {
	t := strings.TrimSpace(line)
	return t != "" && strings.Trim(t, "=") == ""
}

func cleanTitle(s string) string {
	s = strings.TrimSpace(titleMarkup.ReplaceAllString(s, "$1"))
	if utf8.RuneCountInString(s) > maxTitleRunes {
		s = string([]rune(s)[:maxTitleRunes])
	}
	return s
}

func (l TreeLevel) MarkdownPaths() []string {
	var out []string
	for _, f := range l.Files {
		if f.Skip == nil && IsMarkdown(f.Path) {
			out = append(out, f.Path)
		}
	}
	return out
}

func (l *TreeLevel) SetTitles(contents map[string]string) {
	for i := range l.Files {
		if c, ok := contents[l.Files[i].Path]; ok {
			l.Files[i].Title = MarkdownTitle(c)
		}
	}
}
