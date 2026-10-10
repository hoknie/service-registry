package tests

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
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

var httpParts = []string{"handlers", "requests", "responses", "middleware", "mcp"}

func TestLayerPackagesStayWithTheirOwners(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..")
	fset := token.NewFileSet()
	for _, top := range []string{"cmd", "internal", "tests"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			dir := filepath.ToSlash(filepath.Dir(rel))
			f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, imp := range f.Imports {
				target := strings.TrimPrefix(strings.Trim(imp.Path.Value, `"`), "svc-registry/")
				if feature, ok := strings.CutSuffix(target, "/repository"); ok && strings.HasPrefix(feature, "internal/feature/") {
					if dir != feature+"/service" && dir != feature+"/repository" {
						t.Errorf("%s imports %s — a repository is used only by the service of its feature", rel, target)
					}
				}
				for _, part := range httpParts {
					if target == "internal/presentation/http/"+part && !strings.HasPrefix(dir, "internal/presentation/http") {
						t.Errorf("%s imports %s — it is for the HTTP layer only", rel, target)
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

var heavyPackages = []string{"k8s.io/api/", "github.com/openai/openai-go"}

func TestBinaryLeavesOutHeavyPackages(t *testing.T) {
	t.Parallel()
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go is not on PATH")
	}
	out, err := exec.Command(goTool, "list", "-deps", "-json=ImportPath,Imports", "svc-registry/cmd/svc-registry").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	importers := map[string]string{}
	found := map[string][]string{}
	dec := json.NewDecoder(strings.NewReader(string(out)))
	for dec.More() {
		var p struct {
			ImportPath string
			Imports    []string
		}
		if err := dec.Decode(&p); err != nil {
			t.Fatal(err)
		}
		for _, imp := range p.Imports {
			if _, ok := importers[imp]; !ok {
				importers[imp] = p.ImportPath
			}
		}
		for _, heavy := range heavyPackages {
			if strings.HasPrefix(p.ImportPath, heavy) {
				found[heavy] = append(found[heavy], p.ImportPath)
			}
		}
	}
	for _, heavy := range heavyPackages {
		pkgs := found[heavy]
		if len(pkgs) == 0 {
			continue
		}
		chain := []string{pkgs[len(pkgs)-1]}
		for at := chain[0]; importers[at] != "" && len(chain) < 20; at = importers[at] {
			chain = append(chain, importers[at])
		}
		t.Errorf("the binary links %d packages of %s (ADR-0064), e.g. %s", len(pkgs), heavy, strings.Join(chain, " <- "))
	}
}
