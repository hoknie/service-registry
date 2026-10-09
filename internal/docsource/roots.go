package docsource

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"

	"svc-registry/internal/knowledge"
)

func within(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func Allowed(roots []string, p string) error {
	if len(roots) == 0 {
		return knowledge.ConflictLocalDisabled
	}
	resolved := p
	if r, err := filepath.EvalSymlinks(p); err == nil {
		resolved = r
	}
	for _, root := range roots {
		if within(root, resolved) {
			return nil
		}
	}
	return knowledge.InvalidPath
}

func Resolve(roots []string, p string) (string, error) {
	resolved, err := filepath.EvalSymlinks(p)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", knowledge.FailSourceNotFound
	case err != nil:
		return "", knowledge.FailSourceUnreadable
	}
	for _, root := range roots {
		if within(root, resolved) {
			return resolved, nil
		}
	}
	return "", knowledge.FailNotAllowed
}
