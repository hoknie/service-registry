package tests

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

var presentationPackages = []string{
	"internal/presentation/http/handlers",
	"internal/presentation/http/mcp",
	"internal/presentation/jobs",
}

var inputExtraction = map[string]bool{
	"access.Authenticate":      true,
	"access.AuthenticateToken": true,
	"catalog.ResolveNode":      true,
}

var serviceAuthorization = map[string]bool{
	"Authorize":          true,
	"AuthorizeContainer": true,
	"AuthorizeProject":   true,
	"Scope":              true,
}

const featureServicePrefix = "svc-registry/internal/feature/"

type serviceCall struct {
	feature string
	method  string
}

type presentationFunc struct {
	name    string
	pos     string
	calls   []serviceCall
	helpers []string
}

func TestPresentationCallsOneFeaturePerFunction(t *testing.T) {
	t.Parallel()
	for _, dir := range presentationPackages {
		sources := map[string]string{}
		entries, err := os.ReadDir(filepath.Join("..", dir))
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			b, err := os.ReadFile(filepath.Join("..", dir, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			sources[filepath.ToSlash(filepath.Join(dir, e.Name()))] = string(b)
		}
		problems, err := presentationProblems(sources)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range problems {
			t.Error(p)
		}
	}
}

func TestPresentationCallsCheckCatchesViolations(t *testing.T) {
	t.Parallel()
	const header = `package handlers

import (
	accessservices "svc-registry/internal/feature/access/services"
	catalogservices "svc-registry/internal/feature/catalog/services"
	forgeservices "svc-registry/internal/feature/forge/services"
)

type Handlers struct {
	Access  *accessservices.AccessServices
	Catalog *catalogservices.CatalogServices
	Forge   *forgeservices.ForgeServices
}
`
	for name, tc := range map[string]struct {
		body string
		want string
	}{
		"two features": {
			body: "func (a *Handlers) Get() { a.Catalog.GetNode(); a.Forge.Managed() }",
			want: "Handlers.Get calls catalog, forge",
		},
		"through a helper": {
			body: "func (a *Handlers) Get() { a.Catalog.GetNode(); a.managed() }\nfunc (a *Handlers) managed() { a.Forge.Managed() }",
			want: "Handlers.Get calls catalog, forge",
		},
		"authorization": {
			body: "func (a *Handlers) Get() { a.Catalog.Authorize() }",
			want: "Handlers.Get calls Catalog.Authorize",
		},
		"one feature with input extraction": {
			body: "func (a *Handlers) Get() { a.Access.Authenticate(); a.Catalog.ResolveNode(); a.Forge.Managed() }",
		},
		"two features through area services": {
			body: "type Areas struct {\n\tCatalog *catalogservices.CatalogServices\n\tForge   *forgeservices.ForgeServices\n}\n" +
				"func (a *Areas) Get() { a.Catalog.Nodes.Create(); a.Forge.Projects.Managed() }",
			want: "Areas.Get calls catalog, forge",
		},
		"several methods of one feature": {
			body: "func (a *Handlers) Get() { a.Forge.Managed(); a.Forge.ProjectReadme() }",
		},
	} {
		problems, err := presentationProblems(map[string]string{"handlers/x.go": header + tc.body})
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

func presentationProblems(sources map[string]string) ([]string, error) {
	fset := token.NewFileSet()
	var files []*ast.File
	fields := map[string]string{}
	paths := make([]string, 0, len(sources))
	for path := range sources {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		f, err := parser.ParseFile(fset, path, sources[path], parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		files = append(files, f)
		collectServiceFields(f, fields)
	}
	funcs := map[string]*presentationFunc{}
	var order []string
	for _, f := range files {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			key, recv := funcKey(fn)
			pf := &presentationFunc{name: key, pos: fset.Position(fn.Pos()).String()}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch fun := call.Fun.(type) {
				case *ast.Ident:
					pf.helpers = append(pf.helpers, fun.Name)
				case *ast.SelectorExpr:
					for x := fun.X; ; {
						inner, ok := x.(*ast.SelectorExpr)
						if !ok {
							break
						}
						if feature, ok := fields[inner.Sel.Name]; ok {
							pf.calls = append(pf.calls, serviceCall{feature: feature, method: fun.Sel.Name})
							if serviceAuthorization[fun.Sel.Name] {
								pf.calls = append(pf.calls, serviceCall{feature: "!" + inner.Sel.Name, method: fun.Sel.Name})
							}
							return true
						}
						x = inner.X
					}
					if x, ok := fun.X.(*ast.Ident); ok && recv != "" && x.Name == recv {
						pf.helpers = append(pf.helpers, receiverType(fn)+"."+fun.Sel.Name)
					}
				}
				return true
			})
			if prev, ok := funcs[pf.name]; ok {
				prev.calls = append(prev.calls, pf.calls...)
				prev.helpers = append(prev.helpers, pf.helpers...)
				continue
			}
			funcs[pf.name] = pf
			order = append(order, pf.name)
		}
	}
	var problems []string
	for _, name := range order {
		pf := funcs[name]
		calls := reachableCalls(funcs, name, map[string]bool{})
		features := map[string]bool{}
		for _, c := range calls {
			if strings.HasPrefix(c.feature, "!") {
				problems = append(problems, pf.pos+": "+name+" calls "+c.feature[1:]+"."+c.method+" — the feature's service checks access")
				continue
			}
			if !inputExtraction[c.feature+"."+c.method] {
				features[c.feature] = true
			}
		}
		if len(features) > 1 {
			list := make([]string, 0, len(features))
			for f := range features {
				list = append(list, f)
			}
			sort.Strings(list)
			problems = append(problems, pf.pos+": "+name+" calls "+strings.Join(list, ", ")+" — one feature per presentation function (ADR-0072)")
		}
	}
	return problems, nil
}

func funcKey(fn *ast.FuncDecl) (string, string) {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name, ""
	}
	recv := ""
	if names := fn.Recv.List[0].Names; len(names) > 0 {
		recv = names[0].Name
	}
	return receiverType(fn) + "." + fn.Name.Name, recv
}

func receiverType(fn *ast.FuncDecl) string {
	typ := fn.Recv.List[0].Type
	if star, ok := typ.(*ast.StarExpr); ok {
		typ = star.X
	}
	if id, ok := typ.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

func collectServiceFields(f *ast.File, fields map[string]string) {
	aliases := map[string]string{}
	for _, imp := range f.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		if !strings.HasPrefix(path, featureServicePrefix) || !strings.HasSuffix(path, "/services") {
			continue
		}
		feature, kind, _ := strings.Cut(strings.TrimPrefix(path, featureServicePrefix), "/")
		alias := kind
		if imp.Name != nil {
			alias = imp.Name.Name
		}
		aliases[alias] = feature
	}
	ast.Inspect(f, func(n ast.Node) bool {
		st, ok := n.(*ast.StructType)
		if !ok {
			return true
		}
		for _, field := range st.Fields.List {
			star, ok := field.Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			sel, ok := star.X.(*ast.SelectorExpr)
			if !ok || !sel.Sel.IsExported() {
				continue
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok {
				continue
			}
			feature, ok := aliases[pkg.Name]
			if !ok {
				continue
			}
			for _, name := range field.Names {
				fields[name.Name] = feature
			}
		}
		return true
	})
}

func reachableCalls(funcs map[string]*presentationFunc, name string, seen map[string]bool) []serviceCall {
	pf, ok := funcs[name]
	if !ok || seen[name] {
		return nil
	}
	seen[name] = true
	calls := append([]serviceCall(nil), pf.calls...)
	for _, h := range pf.helpers {
		calls = append(calls, reachableCalls(funcs, h, seen)...)
	}
	return calls
}
