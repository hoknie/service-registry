package knowledge

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
)

func WithinRoot(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func LocalPathAllowed(roots []string, p string) error {
	if len(roots) == 0 {
		return ConflictLocalDisabled
	}
	resolved := p
	if r, err := filepath.EvalSymlinks(p); err == nil {
		resolved = r
	}
	for _, root := range roots {
		if WithinRoot(root, resolved) {
			return nil
		}
	}
	return InvalidPath
}

func ResolveLocalPath(roots []string, p string) (string, error) {
	resolved, err := filepath.EvalSymlinks(p)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", FailSourceNotFound
	case err != nil:
		return "", FailSourceUnreadable
	}
	for _, root := range roots {
		if WithinRoot(root, resolved) {
			return resolved, nil
		}
	}
	return "", FailNotAllowed
}
