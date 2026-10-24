package tests

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

var repositoryForbiddenCalls = map[string]map[string]bool{
	"uuid":    {"New": true, "NewV7": true, "NewRandom": true, "Must": true},
	"time":    {"Now": true, "Since": true, "Until": true},
	"strings": {"ToLower": true, "ToUpper": true, "TrimSpace": true, "Trim": true, "TrimPrefix": true, "TrimSuffix": true, "EqualFold": true},
}

var repositoryForbiddenMethods = map[string]bool{"InTx": true, "Begin": true, "Commit": true, "Rollback": true}

var repositoryForbiddenImports = map[string]bool{"crypto/rand": true, "math/rand": true, "math/rand/v2": true, "crypto/sha256": true}

var rowDecodingError = map[string]bool{"UnknownValue": true, "InternalDetail": true}

func domainDecisionName(name string) bool {
	for _, prefix := range []string{"Conflict", "Invalid", "Forbidden", "Err", "Refus", "Reject"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func repositoryLogic(f *ast.File, decodesRows bool) []string {
	var found []string
	typesPkgs := map[string]bool{}
	for _, imp := range f.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		if repositoryForbiddenImports[path] {
			found = append(found, "import "+path)
		}
		if strings.HasPrefix(path, "svc-registry/internal/feature/") && strings.HasSuffix(path, "/types") && imp.Name != nil {
			typesPkgs[imp.Name.Name] = true
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && repositoryForbiddenMethods[sel.Sel.Name] {
				found = append(found, "."+sel.Sel.Name)
			}
			return true
		}
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		name := sel.Sel.Name
		pkg, isPkg := sel.X.(*ast.Ident)
		switch {
		case isPkg && repositoryForbiddenCalls[pkg.Name][name]:
			found = append(found, pkg.Name+"."+name)
		case isPkg && pkg.Name == "apperr" && name != "ErrNotFound" && !(decodesRows && rowDecodingError[name]):
			found = append(found, "apperr."+name)
		case isPkg && typesPkgs[pkg.Name] && domainDecisionName(name):
			found = append(found, pkg.Name+"."+name)
		}
		return true
	})
	return found
}

func TestRepositoriesHoldOnlyDataAccess(t *testing.T) {
	t.Parallel()
	self, err := parser.ParseFile(token.NewFileSet(), "self.go", `package repositories
import (
	"crypto/rand"
	catalogtypes "svc-registry/internal/feature/catalog/types"
)
func f() {
	_ = s.db.InTx(ctx, fn); tx, _ := s.db.Begin(ctx); _ = tx.Commit(ctx)
	_ = uuid.Must(uuid.NewV7()); _ = time.Now(); _ = strings.ToLower(x)
	_ = apperr.InternalDetail("x"); _ = apperr.ErrNotFound; _ = catalogtypes.ConflictSlugTaken; _ = catalogtypes.Node{}
	_ = rand.Reader; _ = snapshot.Commit
}`, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := repositoryLogic(self, false); len(got) != 10 {
		t.Fatalf("self-test found %v, want 10 problems", got)
	}
	files, err := filepath.Glob(filepath.Join("..", "internal", "feature", "*", "repositories", "*.go"))
	if err != nil || len(files) == 0 {
		t.Fatalf("repositories: %v %v", files, err)
	}
	fset := token.NewFileSet()
	for _, path := range files {
		base := filepath.Base(path)
		if strings.HasSuffix(path, "_test.go") || base == "constraints.go" {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range repositoryLogic(f, base == "rows.go") {
			t.Errorf("%s: %s — a repository only reads and writes data: transactions, ids, time, normalization and decisions belong to services and use cases (ADR-0109)", path, p)
		}
	}
}
