package specalign

import (
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestInProcessContextRoots is the static half of ledger SI-359 (7) and
// (14) (spec/gitx-recorder-seam ac-2; parent spec/ritual-write-scope-v3
// dc-5): the ritual witness's in-process drivers log a workbench or MCP
// run through a gitx.Observer attached to the context the server hands
// its handlers, so a handler that roots a context of its own
// (context.Background or context.TODO) drops every git call beneath it
// from the log. The test fails on any such root in code those servers
// reach, except the allowed ones, each named with its reason.
//
// The reach is the import closure of internal/workbench and
// internal/mcpserve (the packages behind workbench.NewHandler and
// mcpserve.NewServer, which the drivers serve), over every non-test file
// whatever its build constraints, read from the module's own source with
// go/parser. Nothing runs `go list`, so go.mod is never touched. A package
// in the closure holds code the servers may reach, so the closure is a
// superset of the code their routes and tools reach through static calls
// only: the fail-closed direction for those. Code a package outside the
// closure hands the servers at run time is not in it, such as a handler or
// tool an init function in a package that imports the servers registers
// with them; every current registrant is inside the closure (R5c2 review
// R5C2R-6). A root is any reference to context.Background or
// context.TODO, called or not, through whatever name the file imports
// "context" by (a dot import included), placed by its enclosing function
// declaration, or by the package-level variable whose initializer holds
// it.
//
// Disclosed, not proven here: what cmd/verdi wires into `verdi serve` and
// `verdi mcp` (the loaders and forge their commands inject) is outside the
// closure, since the drivers serve the cores without it; a built binary's
// run logs through VERDI_GITLOG whatever its roots. A goroutine a handler
// starts on a context it keeps is a context, not a root, and so is
// outside this rule; the ritual-effect producer's per-run comparison of
// the observer's log with a process-wide VERDI_GITLOG covers what this
// syntactic rule cannot see.
func TestInProcessContextRoots(t *testing.T) {
	src, err := loadPackageClosure(verdiRepoRoot, contextRootStarts()...)
	if err != nil {
		t.Fatalf("loading the in-process servers' closure: %v", err)
	}
	allowed := allowedContextRoots()
	in := map[string]bool{}
	for _, p := range src.closure {
		in[p] = true
	}
	for _, want := range []string{"internal/workbench", "internal/mcpserve", "internal/gitx", "internal/commitdesign"} {
		if !in[want] {
			t.Errorf("closure: %s is not in the in-process servers' closure", want)
		}
	}

	findings, roots := checkContextRoots(src, allowed)
	for _, f := range findings {
		t.Error(f)
	}
	t.Logf("the in-process servers' closure: %d module packages, %d non-test files; context roots: %d: %s",
		len(src.closure), len(src.files), len(roots), strings.Join(roots, "; "))

	for _, m := range contextRootMutants() {
		t.Run("falsifier: "+m.name, func(t *testing.T) {
			got, _ := checkContextRoots(m.mutate(t, src), allowed)
			joined := strings.Join(got, "\n")
			for _, want := range m.wants {
				if !strings.Contains(joined, want) {
					t.Errorf("the guard missed the mutant %q (findings:\n%s\n); want a finding mentioning %q", m.name, joined, want)
				}
			}
		})
	}
}

// contextRootStarts are the packages whose servers the in-process drivers
// run: the workbench's handler and the MCP server.
func contextRootStarts() []string {
	return []string{"internal/workbench", "internal/mcpserve"}
}

// contextRootAllowance is one allowed context root: the file, the function
// (or package-level variable) that holds it, how many roots it holds, and
// why it drops no git call from the log.
type contextRootAllowance struct {
	file, holder string
	count        int
	reason       string
}

// allowedContextRoots is ledger SI-359 (14)'s list, and no other root. An
// allowance names its file and holder, never a whole package, so a root
// moved to another function of an allowed package is a finding.
func allowedContextRoots() []contextRootAllowance {
	return []contextRootAllowance{
		{"internal/evidence/fold.go", "Fold", 1, "a nil-Context fallback; every Fold caller sets Context"},
		{"internal/filelock/filelock.go", "var psLstart", 1, "reads ps, not git"},
	}
}

// checkContextRoots reports every context root in src that no allowance
// excuses, every allowance whose holder's count differs from its roots
// (one that excuses nothing included), and returns the roots it found, as
// "file:line (holder)", sorted.
func checkContextRoots(src seamSource, allowed []contextRootAllowance) (findings, roots []string) {
	type key struct{ file, holder string }
	found := map[key][]string{}
	for _, sf := range src.files {
		for _, r := range contextRootsIn(src.fset, sf) {
			k := key{sf.path, r.holder}
			at := fmt.Sprintf("%s:%d (%s)", sf.path, r.line, r.holder)
			found[k] = append(found[k], at)
			roots = append(roots, at)
		}
	}
	excused := map[key]bool{}
	for _, a := range allowed {
		k := key{a.file, a.holder}
		excused[k] = true
		if n := len(found[k]); n != a.count {
			findings = append(findings, fmt.Sprintf("allowed context root %s in %s (%s): want %d root(s), found %d %v; an allowance names exactly the roots it excuses (ledger SI-359 (14))",
				a.holder, a.file, a.reason, a.count, n, found[k]))
		}
	}
	var keys []key
	for k := range found {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].file != keys[j].file {
			return keys[i].file < keys[j].file
		}
		return keys[i].holder < keys[j].holder
	})
	for _, k := range keys {
		if !excused[k] {
			for _, at := range found[k] {
				findings = append(findings, fmt.Sprintf("context root at %s, in code the in-process workbench or MCP server reaches: a gitx call beneath it escapes the driver's observer; inherit the caller's context (context.WithoutCancel keeps its values) or allow it here with its reason (ledger SI-359 (7), (14))", at))
			}
		}
	}
	sort.Strings(roots)
	return findings, roots
}

// contextRoot is one reference to context.Background or context.TODO.
type contextRoot struct {
	holder string
	line   int
}

// contextRootsIn returns the context roots in one file, each with the
// function declaration or package-level variable that holds it.
func contextRootsIn(fset *token.FileSet, sf seamFile) []contextRoot {
	names, dot := map[string]bool{}, false
	for _, imp := range sf.file.Imports {
		if p, err := strconv.Unquote(imp.Path.Value); err != nil || p != "context" {
			continue
		}
		switch {
		case imp.Name == nil:
			names["context"] = true
		case imp.Name.Name == ".":
			dot = true
		case imp.Name.Name != "_":
			names[imp.Name.Name] = true
		}
	}
	if len(names) == 0 && !dot {
		return nil
	}
	isRoot := func(n ast.Node) bool {
		switch e := n.(type) {
		case *ast.SelectorExpr:
			x, ok := e.X.(*ast.Ident)
			return ok && names[x.Name] && (e.Sel.Name == "Background" || e.Sel.Name == "TODO")
		case *ast.Ident:
			return dot && (e.Name == "Background" || e.Name == "TODO")
		}
		return false
	}
	var out []contextRoot
	scan := func(holder string, n ast.Node) {
		ast.Inspect(n, func(n ast.Node) bool {
			if n != nil && isRoot(n) {
				out = append(out, contextRoot{holder: holder, line: fset.Position(n.Pos()).Line})
				return false
			}
			return true
		})
	}
	for _, d := range sf.file.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			scan(funcHolder(d), d)
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				var idents []string
				for _, id := range vs.Names {
					idents = append(idents, id.Name)
				}
				for _, v := range vs.Values {
					scan("var "+strings.Join(idents, ", "), v)
				}
			}
		}
	}
	return out
}

// funcHolder names a function declaration: its name, behind its
// receiver's type for a method.
func funcHolder(d *ast.FuncDecl) string {
	if d.Recv == nil || len(d.Recv.List) == 0 {
		return d.Name.Name
	}
	t := d.Recv.List[0].Type
	if st, ok := t.(*ast.StarExpr); ok {
		t = st.X
	}
	switch rt := t.(type) {
	case *ast.Ident:
		return rt.Name + "." + d.Name.Name
	case *ast.IndexExpr:
		if id, ok := rt.X.(*ast.Ident); ok {
			return id.Name + "." + d.Name.Name
		}
	case *ast.IndexListExpr:
		if id, ok := rt.X.(*ast.Ident); ok {
			return id.Name + "." + d.Name.Name
		}
	}
	return d.Name.Name
}

// loadPackageClosure parses every non-test file of starts and of every
// module package they import, transitively, from the module rooted at
// root (loadSeamSource's walk, from other starting packages).
func loadPackageClosure(root string, starts ...string) (seamSource, error) {
	module, err := modulePath(filepath.Join(root, "go.mod"))
	if err != nil {
		return seamSource{}, err
	}
	src := seamSource{fset: token.NewFileSet(), module: module}
	seen := map[string]bool{}
	queue := append([]string(nil), starts...)
	for len(queue) > 0 {
		pkg := queue[0]
		queue = queue[1:]
		if seen[pkg] {
			continue
		}
		seen[pkg] = true
		files, err := parsePackageDir(src.fset, root, pkg)
		if err != nil {
			return seamSource{}, err
		}
		for _, f := range files {
			for _, imp := range f.imports {
				if rel, ok := moduleRel(module, imp); ok && !seen[rel] {
					queue = append(queue, rel)
				}
			}
		}
		src.files = append(src.files, files...)
	}
	for pkg := range seen {
		src.closure = append(src.closure, pkg)
	}
	sort.Strings(src.closure)
	sort.Slice(src.files, func(i, j int) bool { return src.files[i].path < src.files[j].path })
	return src, nil
}

// contextRootMutant is one change to the parsed closure the guard must
// catch.
type contextRootMutant struct {
	name   string
	mutate func(t *testing.T, src seamSource) seamSource
	wants  []string
}

// contextRootMutants are the guard's falsifiers: the board's commit root
// restored (SI-359 (7)), a new root in a package the servers reach, under
// a renamed import and a dot import too, a second root inside an allowed
// holder, an allowed root moved to another function of its package (R5c2
// review R5C2R-3), and an allowance that excuses nothing.
func contextRootMutants() []contextRootMutant {
	const boardFile = "internal/workbench/board.go"
	return []contextRootMutant{
		{
			name: "the board's commit roots its own context again",
			mutate: func(t *testing.T, src seamSource) seamSource {
				return withSeamFileEdited(t, src, boardFile,
					"commitdesign.Run(context.WithoutCancel(r.Context()),", "commitdesign.Run(context.Background(),")
			},
			wants: []string{"context root at " + boardFile + ":", "(boardCommitHandler)"},
		},
		{
			name: "a new Background root in a reached package",
			mutate: func(t *testing.T, src seamSource) seamSource {
				return withSeamFile(t, src, "internal/commitdesign", "detached.go", `package commitdesign

import "context"

func detached() context.Context { return context.Background() }
`)
			},
			wants: []string{"context root at internal/commitdesign/detached.go:5 (detached)"},
		},
		{
			name: "a TODO root under a renamed import, in a method",
			mutate: func(t *testing.T, src seamSource) seamSource {
				return withSeamFile(t, src, "internal/mcpserve", "todo.go", `package mcpserve

import stdctx "context"

type later struct{}

func (*later) ctx() stdctx.Context { return stdctx.TODO() }
`)
			},
			wants: []string{"context root at internal/mcpserve/todo.go:7 (later.ctx)"},
		},
		{
			name: "a root through a dot import, held by a package-level variable",
			mutate: func(t *testing.T, src seamSource) seamSource {
				return withSeamFile(t, src, "internal/gitx", "dotctx.go", `package gitx

import . "context"

var rootContext = Background
`)
			},
			wants: []string{"context root at internal/gitx/dotctx.go:5 (var rootContext)"},
		},
		{
			name: "a second root inside an allowed holder",
			mutate: func(t *testing.T, src seamSource) seamSource {
				return withSeamFileEdited(t, src, "internal/evidence/fold.go",
					"ctx = context.Background()", "ctx = context.Background()\n\t\t_ = context.TODO()")
			},
			wants: []string{"allowed context root Fold in internal/evidence/fold.go", "want 1 root(s), found 2"},
		},
		{
			name: "Fold's root moved to a new function in internal/evidence",
			mutate: func(t *testing.T, src seamSource) seamSource {
				moved := withSeamFileEdited(t, src, "internal/evidence/fold.go", "ctx = context.Background()", "ctx = foldFallback()")
				return withSeamFile(t, moved, "internal/evidence", "fallback.go", `package evidence

import "context"

func foldFallback() context.Context { return context.Background() }
`)
			},
			wants: []string{"context root at internal/evidence/fallback.go:5 (foldFallback)", "allowed context root Fold in internal/evidence/fold.go", "want 1 root(s), found 0"},
		},
		{
			name: "an allowance that excuses nothing",
			mutate: func(t *testing.T, src seamSource) seamSource {
				return withSeamFileEdited(t, src, "internal/filelock/filelock.go",
					"context.WithTimeout(context.Background(), psTimeout)", "context.WithTimeout(context.WithoutCancel(nil), psTimeout)")
			},
			wants: []string{"allowed context root var psLstart in internal/filelock/filelock.go", "want 1 root(s), found 0"},
		},
	}
}
