package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// TestRun_StopsEveryStoppableFixture: every fixture the control server
// holds whose type has a stop method — a serve or server of its own, and a
// store it removes — is stopped with the harness by a deferred
// ctrl.<field>.stop() in run(), so no isolated fixture outlives the run.
func TestRun_StopsEveryStoppableFixture(t *testing.T) {
	files := parseHarnessSources(t)
	stoppable := map[string]bool{}
	for _, f := range files {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Name.Name != "stop" || len(fn.Recv.List) != 1 {
				continue
			}
			if star, ok := fn.Recv.List[0].Type.(*ast.StarExpr); ok {
				if id, ok := star.X.(*ast.Ident); ok {
					stoppable[id.Name] = true
				}
			}
		}
	}

	var want []string
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			spec, ok := n.(*ast.TypeSpec)
			if !ok || spec.Name.Name != "controlServer" {
				return true
			}
			for _, field := range spec.Type.(*ast.StructType).Fields.List {
				star, ok := field.Type.(*ast.StarExpr)
				if !ok {
					continue
				}
				if id, ok := star.X.(*ast.Ident); ok && stoppable[id.Name] {
					for _, name := range field.Names {
						want = append(want, name.Name)
					}
				}
			}
			return false
		})
	}
	if len(want) == 0 {
		t.Fatal("no stoppable control-server fixture was found: the witness would be vacuous")
	}

	got := deferredFixtureStops(files)
	sort.Strings(want)
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("run() defers stop() on %v, want every stoppable control fixture %v", got, want)
	}
}

// parseHarnessSources parses every non-test Go file of this package.
func parseHarnessSources(t *testing.T) []*ast.File {
	t.Helper()
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	return files
}

// deferredFixtureStops returns the field of every `defer ctrl.<field>.stop()`
// statement directly in run()'s body.
func deferredFixtureStops(files []*ast.File) []string {
	var out []string
	for _, f := range files {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Name.Name != "run" || fn.Body == nil {
				continue
			}
			for _, stmt := range fn.Body.List {
				d, ok := stmt.(*ast.DeferStmt)
				if !ok {
					continue
				}
				stop, ok := d.Call.Fun.(*ast.SelectorExpr)
				if !ok || stop.Sel.Name != "stop" {
					continue
				}
				field, ok := stop.X.(*ast.SelectorExpr)
				if !ok {
					continue
				}
				if recv, ok := field.X.(*ast.Ident); ok && recv.Name == "ctrl" {
					out = append(out, field.Sel.Name)
				}
			}
		}
	}
	return out
}

// TestDeferredFixtureStops: the scanner reads only run()'s own deferred
// ctrl.<field>.stop() calls — not a stop that is called rather than
// deferred, not another receiver's, not another function's.
func TestDeferredFixtureStops(t *testing.T) {
	const src = `package main
func run() {
	defer ctrl.a.stop()
	ctrl.b.stop()
	defer other.c.stop()
	defer ctrl.d.close()
	defer ctrl.e.stop()
}
func elsewhere() { defer ctrl.f.stop() }
`
	f, err := parser.ParseFile(token.NewFileSet(), "src.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := deferredFixtureStops([]*ast.File{f}); !reflect.DeepEqual(got, []string{"a", "e"}) {
		t.Errorf("deferredFixtureStops = %v, want [a e]", got)
	}
}
