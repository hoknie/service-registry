package webui

import (
	"os"
	"path/filepath"
	"strings"
)

type want struct {
	br, gzip bool
	locale   string
}

type found struct {
	path         string
	contentType  string
	etag         string
	size         int64
	encoding     string
	notFoundPage bool
}

type lookupResult struct {
	file     *found
	notFound bool
	page     []byte
	noExport bool
}

func lookup(rootDir string, segments []string, w want) lookupResult {
	root, err := realPath(rootDir)
	if err != nil {
		return lookupResult{noExport: true}
	}
	if info, err := os.Stat(filepath.Join(root, ExportMarker)); err != nil || !info.Mode().IsRegular() {
		return lookupResult{noExport: true}
	}
	if path, ok := find(root, segments); ok {
		if f, ok := respond(root, path, w); ok {
			return lookupResult{file: f}
		}
	}
	return lookupResult{notFound: true, page: notFoundPage(root, w.locale)}
}

func realPath(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

func find(root string, segments []string) (string, bool) {
	if len(segments) == 0 {
		return "", false
	}
	for _, s := range segments {
		if strings.HasPrefix(s, ".") {
			return "", false
		}
	}
	base := filepath.Join(append([]string{root}, segments...)...)
	for _, candidate := range []string{base, base + ".html", filepath.Join(base, "index.html")} {
		if resolved, ok := inside(root, candidate); ok {
			return resolved, true
		}
	}
	return "", false
}

func inside(root, candidate string) (string, bool) {
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", false
	}
	if resolved != root && !strings.HasPrefix(resolved, root+string(filepath.Separator)) {
		return "", false
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	return resolved, true
}

func respond(root, path string, w want) (*found, bool) {
	name := filepath.Base(path)
	f := &found{contentType: ContentType(name), notFoundPage: name == ExportMarker}

	file := path
	for _, v := range []struct {
		enc, suffix string
		accepted    bool
	}{{"br", ".br", w.br}, {"gzip", ".gz", w.gzip}} {
		if !v.accepted {
			continue
		}
		if resolved, ok := inside(root, path+v.suffix); ok {
			f.encoding, file = v.enc, resolved
			break
		}
	}

	info, err := os.Stat(file)
	if err != nil {
		return nil, false
	}
	f.path, f.size, f.etag = file, info.Size(), etag(info.Size(), info.ModTime(), f.encoding)
	return f, true
}

func notFoundPage(root, locale string) []byte {
	for _, p := range []string{filepath.Join(root, locale, ExportMarker), filepath.Join(root, ExportMarker)} {
		if resolved, ok := inside(root, p); ok {
			if body, err := os.ReadFile(resolved); err == nil {
				return body
			}
		}
	}
	return nil
}
