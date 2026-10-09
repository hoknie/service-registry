package tests

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var sqlText = regexp.MustCompile(`^\s*(SELECT|INSERT|UPDATE|DELETE|WITH)\b|\bFROM\s+\w|\bWHERE\s+\w`)

func stringParts(e ast.Expr) []string {
	switch x := e.(type) {
	case *ast.BasicLit:
		if x.Kind != token.STRING {
			return nil
		}
		s, err := strconv.Unquote(x.Value)
		if err != nil {
			return nil
		}
		return []string{s}
	case *ast.BinaryExpr:
		if x.Op != token.ADD {
			return nil
		}
		return append(stringParts(x.X), stringParts(x.Y)...)
	case *ast.ParenExpr:
		return stringParts(x.X)
	}
	return nil
}

func TestNoSQLInPackageLevelConstantsOrVariables(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	for _, root := range []string{"internal", "pkg"} {
		err := filepath.WalkDir(filepath.Join("..", root), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			for _, decl := range f.Decls {
				g, ok := decl.(*ast.GenDecl)
				if !ok || (g.Tok != token.CONST && g.Tok != token.VAR) {
					continue
				}
				for _, spec := range g.Specs {
					v := spec.(*ast.ValueSpec)
					for i, value := range v.Values {
						for _, part := range stringParts(value) {
							if sqlText.MatchString(part) {
								t.Errorf("%s: package-level %s %s holds SQL; write the query at the call site (ADR-0054)",
									fset.Position(v.Pos()), g.Tok, v.Names[i].Name)
								break
							}
						}
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
