package tests

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	maxFilesPerFolder   = 12
	maxFileLines        = 300
	maxFunctionLines    = 80
	maxExportedMethods  = 12
	sizeLimitsDecision  = "ADR-0073"
	featureRootForLimit = "internal/feature/"
)

func TestFeatureFoldersHoldAtMostTwelveFiles(t *testing.T) {
	t.Parallel()
	err := filepath.WalkDir(filepath.Join("..", "internal", "feature"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		n := 0
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".go") && !strings.HasSuffix(e.Name(), "_test.go") {
				n++
			}
		}
		if n > maxFilesPerFolder {
			t.Errorf("%s holds %d files, at most %d (%s): split it by subdomain", path, n, maxFilesPerFolder, sizeLimitsDecision)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestProductionCodeStaysWithinSizeLimits(t *testing.T) {
	t.Parallel()
	for _, root := range []string{"internal", "pkg"} {
		err := filepath.WalkDir(filepath.Join("..", root), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel := filepath.ToSlash(strings.TrimPrefix(path, ".."+string(filepath.Separator)))
			if d.IsDir() && rel == "internal/testsupport" {
				return filepath.SkipDir
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			problems, err := sizeProblems(rel, src)
			if err != nil {
				return err
			}
			for _, p := range problems {
				t.Error(p)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestSizeLimitsCatchViolations(t *testing.T) {
	t.Parallel()
	long := "package services\n\n" + strings.Repeat("var _ = 0\n", maxFileLines)
	body := "package services\n\nfunc f() {\n" + strings.Repeat("\t_ = 0\n", maxFunctionLines+1) + "}\n"
	sql := "package repositories\n\nfunc q() {\n\t_ = `\n" + strings.Repeat("\tSELECT 1\n", maxFunctionLines+10) + "`\n}\n"
	var methods strings.Builder
	methods.WriteString("package services\n\ntype Nodes struct{}\n")
	for i := 0; i <= maxExportedMethods; i++ {
		fmt.Fprintf(&methods, "func (s *Nodes) M%d() {}\n", i)
	}
	for name, tc := range map[string]struct {
		path, src, want string
	}{
		"long file":       {"internal/feature/catalog/services/x.go", long, "lines, at most 300"},
		"long function":   {"internal/feature/catalog/services/x.go", body, "f has 81 lines"},
		"long sql passes": {"internal/feature/catalog/repositories/x.go", sql, ""},
		"many methods":    {"internal/feature/catalog/services/x.go", methods.String(), "Nodes has 13 exported methods"},
	} {
		problems, err := sizeProblems(tc.path, []byte(tc.src))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		got := strings.Join(problems, "\n")
		if tc.want == "" && got != "" {
			t.Errorf("%s: unexpected problems:\n%s", name, got)
		}
		if tc.want != "" && !strings.Contains(got, tc.want) {
			t.Errorf("%s: problems %q, want %q", name, got, tc.want)
		}
	}
}

func sizeProblems(path string, src []byte) ([]string, error) {
	var problems []string
	if n := strings.Count(string(src), "\n"); n > maxFileLines {
		problems = append(problems, fmt.Sprintf("%s has %d lines, at most %d (%s)", path, n, maxFileLines, sizeLimitsDecision))
	}
	inFeature := strings.HasPrefix(path, featureRootForLimit)
	inServices := inFeature && strings.Contains(path, "/services/")
	inRepositories := inFeature && strings.Contains(path, "/repositories/")
	if !inServices && !inRepositories {
		return problems, nil
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	methods := map[string]int{}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		if n := codeLines(fset, fn.Body); n > maxFunctionLines {
			problems = append(problems, fmt.Sprintf("%s: %s has %d lines, at most %d (%s)", path, fn.Name.Name, n, maxFunctionLines, sizeLimitsDecision))
		}
		if inServices && fn.Recv != nil && fn.Name.IsExported() {
			methods[receiverName(fn)]++
		}
	}
	for recv, n := range methods {
		if n > maxExportedMethods {
			problems = append(problems, fmt.Sprintf("%s: %s has %d exported methods, at most %d (%s)", path, recv, n, maxExportedMethods, sizeLimitsDecision))
		}
	}
	return problems, nil
}

func codeLines(fset *token.FileSet, body *ast.BlockStmt) int {
	n := fset.Position(body.Rbrace).Line - fset.Position(body.Lbrace).Line - 1
	ast.Inspect(body, func(node ast.Node) bool {
		if lit, ok := node.(*ast.BasicLit); ok && lit.Kind == token.STRING && strings.HasPrefix(lit.Value, "`") {
			n -= fset.Position(lit.End()).Line - fset.Position(lit.Pos()).Line
		}
		return true
	})
	return n
}

func receiverName(fn *ast.FuncDecl) string {
	typ := fn.Recv.List[0].Type
	if star, ok := typ.(*ast.StarExpr); ok {
		typ = star.X
	}
	if id, ok := typ.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}
