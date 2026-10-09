package tests

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

var commentDirectives = []string{"//go:", "//line ", "// Code generated "}

func isDirective(text string) bool {
	for _, p := range commentDirectives {
		if strings.HasPrefix(text, p) {
			return true
		}
	}
	return false
}

func TestNoCommentsInGoCode(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	for _, root := range []string{"cmd", "internal", "pkg", "tests", "migrations"} {
		err := filepath.WalkDir(filepath.Join("..", root), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
				return err
			}
			f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if err != nil {
				return err
			}
			for _, g := range f.Comments {
				for _, c := range g.List {
					if !isDirective(c.Text) {
						t.Errorf("%s: comment %q", fset.Position(c.Pos()), firstLine(c.Text))
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 60 {
		s = s[:60] + "…"
	}
	return s
}
