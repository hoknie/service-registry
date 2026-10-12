package tests

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var commentDirectives = []string{"//go:", "//line ", "// Code generated "}

var nosecDirective = regexp.MustCompile(`^// #nosec G\d+(,G\d+)* -- \S`)

func isDirective(text string) bool {
	for _, p := range commentDirectives {
		if strings.HasPrefix(text, p) {
			return true
		}
	}
	return nosecDirective.MatchString(text)
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

func TestNosecDirectiveNeedsRuleAndReason(t *testing.T) {
	t.Parallel()
	for text, want := range map[string]bool{
		"// #nosec G304 -- the path comes from the operator's configuration": true,
		"// #nosec G401,G505 -- git object ids are SHA-1":                    true,
		"// #nosec G304":                     false,
		"// #nosec -- no rule":               false,
		"// #nosec G304 --":                  false,
		"// nosec G304 -- not the directive": false,
		"//go:embed migrations":              true,
		"// a plain comment":                 false,
	} {
		if got := isDirective(text); got != want {
			t.Errorf("isDirective(%q) = %v, want %v", text, got, want)
		}
	}
}
