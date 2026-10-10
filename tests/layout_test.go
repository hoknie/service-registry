package tests

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFeaturesExportNoRepositories(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	err := filepath.WalkDir(filepath.Join("..", "internal", "feature"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		rel, _ := filepath.Rel(filepath.Join("..", "internal", "feature"), path)
		dir := filepath.ToSlash(filepath.Dir(rel))
		if strings.Contains(dir, "/") && !strings.HasSuffix(dir, "/service") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gen.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || !ts.Name.IsExported() {
					continue
				}
				if strings.HasSuffix(ts.Name.Name, "Store") {
					t.Errorf("%s: exported %s — storage stays inside its feature", path, ts.Name.Name)
				}
				if ts.Name.Name != "Service" && ts.Name.Name != "Deps" && holdsDatabase(ts.Type) {
					t.Errorf("%s: exported %s holds the database — only the feature's Service may", path, ts.Name.Name)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func holdsDatabase(expr ast.Expr) bool {
	st, ok := expr.(*ast.StructType)
	if !ok {
		return false
	}
	for _, field := range st.Fields.List {
		typ := field.Type
		if star, ok := typ.(*ast.StarExpr); ok {
			typ = star.X
		}
		sel, ok := typ.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok {
			continue
		}
		switch pkg.Name + "." + sel.Sel.Name {
		case "postgres.DB", "postgres.Querier", "pgxpool.Pool", "pgx.Tx":
			return true
		}
	}
	return false
}

func TestOldLayoutPackagesAreGone(t *testing.T) {
	t.Parallel()
	for _, dir := range []string{"service", "postgres", "httpapi", "cli", "webui", "access", "catalog", "ingest", "deploy",
		"forge", "links", "knowledge", "config", "apperr", "auth", "outbound"} {
		if _, err := os.Stat(filepath.Join("..", "internal", dir)); err == nil {
			t.Errorf("internal/%s still exists; see ARCHITECTURE.md for the feature layout", dir)
		}
	}
}
