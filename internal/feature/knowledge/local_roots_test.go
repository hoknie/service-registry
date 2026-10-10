package knowledge

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestLocalPathAllowed(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := LocalPathAllowed(nil, root); !errors.Is(err, ConflictLocalDisabled) {
		t.Fatal(err)
	}
	if err := LocalPathAllowed([]string{root}, filepath.Join(root, "later/api")); err != nil {
		t.Fatal(err)
	}
	if err := LocalPathAllowed([]string{root}, root+"-other/x"); !errors.Is(err, InvalidPath) {
		t.Fatal(err)
	}
}
