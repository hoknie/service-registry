package types

import (
	"path"
	"strings"
)

type Loader func(paths []string) (map[string]string, error)

type Files struct {
	paths map[string]bool
	load  Loader
	cache map[string]string
}

func NewFiles(paths []string, load Loader, preloaded map[string]string) *Files {
	f := &Files{paths: map[string]bool{}, load: load, cache: map[string]string{}}
	for _, p := range paths {
		f.paths[p] = true
	}
	for p, c := range preloaded {
		f.cache[p] = c
	}
	return f
}

func (f *Files) Has(p string) bool { return f.paths[p] }

func (f *Files) Read(p string) (string, bool, error) {
	if !f.paths[p] {
		return "", false, nil
	}
	if c, ok := f.cache[p]; ok {
		return c, true, nil
	}
	got, err := f.load([]string{p})
	if err != nil {
		return "", false, err
	}
	c, ok := got[p]
	if ok {
		f.cache[p] = c
	}
	return c, ok, nil
}

func (f *Files) ResolveImport(imp string) (string, bool) {
	imp = strings.TrimPrefix(path.Clean("/"+imp), "/")
	if f.paths[imp] {
		return imp, true
	}
	found := ""
	for p := range f.paths {
		if strings.HasSuffix(p, "/"+imp) {
			if found != "" {
				return "", false
			}
			found = p
		}
	}
	return found, found != ""
}

func ResolveRef(from, target string) (string, bool) {
	if target == "" || strings.Contains(target, "://") || strings.HasPrefix(target, "/") {
		return "", false
	}
	p := path.Clean(path.Join(path.Dir(from), target))
	if p == ".." || strings.HasPrefix(p, "../") {
		return "", false
	}
	return p, true
}
