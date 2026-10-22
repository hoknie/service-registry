package tests

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

func queryPlumbing(f *ast.File) []string {
	var found []string
	ast.Inspect(f, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		name := sel.Sel.Name
		if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "pgx" && (strings.HasPrefix(name, "Collect") && name != "CollectableRow" || name == "ErrNoRows") {
			found = append(found, "pgx."+name)
		}
		if name == "QueryRow" || name == "RowsAffected" {
			found = append(found, "."+name)
		}
		return true
	})
	return found
}

func TestRepositoriesQueryThroughPostgresFunctions(t *testing.T) {
	t.Parallel()
	self, err := parser.ParseFile(token.NewFileSet(), "self.go", `package repositories
func f() { _ = q.QueryRow(ctx, "SELECT 1"); _, _ = pgx.CollectRows(rows, to); _ = pgx.ErrNoRows; _ = tag.RowsAffected(); var _ pgx.CollectableRow }`, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := queryPlumbing(self); len(got) != 4 {
		t.Fatalf("self-test found %v, want 4 problems", got)
	}
	files, err := filepath.Glob(filepath.Join("..", "internal", "feature", "*", "repositories", "*.go"))
	if err != nil || len(files) == 0 {
		t.Fatalf("repositories: %v %v", files, err)
	}
	fset := token.NewFileSet()
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range queryPlumbing(f) {
			t.Errorf("%s: %s — query through postgres.List/One/Find/Exec/ExecFound (ADR-0075)", path, p)
		}
	}
}
