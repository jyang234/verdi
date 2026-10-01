package reach_test

import (
	"context"
	"go/ast"
	"go/types"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/writescope/reach"
)

// lookupFunc finds a package-level function, or a method written
// "Type.Method", in one of the synthetic module's packages.
func lookupFunc(t *testing.T, prog *reach.Program, pkgPath, name string) *types.Func {
	t.Helper()
	pkg := prog.Package(pkgPath)
	if pkg == nil {
		t.Fatalf("no package %s", pkgPath)
	}
	if typ, method, ok := strings.Cut(name, "."); ok {
		tn, _ := pkg.Types.Scope().Lookup(typ).(*types.TypeName)
		if tn == nil {
			t.Fatalf("no type %s in %s", typ, pkgPath)
		}
		obj, _, _ := types.LookupFieldOrMethod(types.NewPointer(tn.Type()), true, pkg.Types, method)
		fn, _ := obj.(*types.Func)
		if fn == nil {
			t.Fatalf("no method %s in %s", name, pkgPath)
		}
		return fn
	}
	fn, _ := pkg.Types.Scope().Lookup(name).(*types.Func)
	if fn == nil {
		t.Fatalf("no function %s in %s", name, pkgPath)
	}
	return fn
}

// synthTargets is the synthetic mutating set: gitx.Mutate, gitx.Publish,
// gitx.Prune, and gitx.Other.
func synthTargets(t *testing.T, prog *reach.Program) map[types.Object]bool {
	t.Helper()
	return map[types.Object]bool{
		lookupFunc(t, prog, "example.com/synth/gitx", "Mutate"):  true,
		lookupFunc(t, prog, "example.com/synth/gitx", "Publish"): true,
		lookupFunc(t, prog, "example.com/synth/gitx", "Prune"):   true,
		lookupFunc(t, prog, "example.com/synth/gitx", "Other"):   true,
	}
}

// mustReach is Graph.Reach for an entry the test built the graph with.
func mustReach(t *testing.T, g *reach.Graph, e reach.Entry, targets map[types.Object]bool) []reach.Hit {
	t.Helper()
	hits, err := g.Reach(e, targets)
	if err != nil {
		t.Fatalf("Reach(%s): %v", e.Name, err)
	}
	return hits
}

func hitNames(hits []reach.Hit) []string {
	var out []string
	for _, h := range hits {
		out = append(out, h.Target.Name())
	}
	sort.Strings(out)
	return out
}

func testGraphReachResolvesEveryCallShape(t *testing.T, prog *reach.Program) {
	const app = "example.com/synth/app"
	tests := []struct {
		name string
		root string
		want []string
	}{
		{"direct call", "Direct", []string{"Mutate"}},
		{"call through an interface", "ViaInterface", []string{"Mutate"}},
		{"call through a closure", "ViaClosure", []string{"Mutate"}},
		{"call through a method value", "ViaMethodValue", []string{"Mutate"}},
		{"call through a function value", "ViaFuncValue", []string{"Mutate"}},
		{"call through a package-level variable", "ViaPackageVar", []string{"Mutate"}},
		{"standard-library callback through a converted interface", "NewHandler", []string{"Mutate"}},
		{"read-only path reaches no target", "ReadOnly", nil},
		{"the only caller of an otherwise unreached target", "Unreached", []string{"Other"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := reach.Entry{Surface: "test", Name: tt.root, Roots: []reach.Root{{Func: lookupFunc(t, prog, app, tt.root)}}}
			g, err := reach.Build(prog, []reach.Entry{entry})
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			got := hitNames(mustReach(t, g, entry, synthTargets(t, prog)))
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Fatalf("Reach(%s) = %v, want %v", tt.root, got, tt.want)
			}
		})
	}
}

// testGraphReachHitsEveryTargetShape pins the reach half of R1-A4: a
// classified exported variable of function type and a classified interface
// method are targets a verb reaches by using them.
func testGraphReachHitsEveryTargetShape(t *testing.T, prog *reach.Program) {
	const app = "example.com/synth/app"
	targets := map[types.Object]bool{}
	for _, name := range []string{"gitx.StageAll", "(gitx.Stager).Stage"} {
		obj := prog.ObjectByName(name)
		if obj == nil {
			t.Fatalf("ObjectByName(%s) = nil", name)
		}
		targets[obj] = true
	}
	tests := []struct {
		root string
		want string
	}{
		{"ViaExportedVar", "StageAll"},
		{"ViaExportedInterface", "Stage"},
		{"Direct", ""},
	}
	for _, tt := range tests {
		t.Run(tt.root, func(t *testing.T) {
			entry := reach.Entry{Surface: "test", Name: tt.root, Roots: []reach.Root{{Func: lookupFunc(t, prog, app, tt.root)}}}
			g, err := reach.Build(prog, []reach.Entry{entry})
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			if got := strings.Join(hitNames(mustReach(t, g, entry, targets)), ","); got != tt.want {
				t.Fatalf("Reach(%s) = %q, want %q", tt.root, got, tt.want)
			}
		})
	}
}

// TestGraph_ReachFollowsGitxsOtherExportShapes: through gitx's exported
// function-typed variable and its interface's unexported implementation,
// a verb reaches the primitive they call.
func testGraphReachFollowsGitxsOtherExportShapes(t *testing.T, prog *reach.Program) {
	const app = "example.com/synth/app"
	tests := []struct {
		name string
		root string
		want []string
	}{
		{"exported variable", "ViaExportedVar", []string{"Mutate"}},
		{"exported interface behind a constructor", "ViaExportedInterface", []string{"Mutate"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := reach.Entry{Surface: "test", Name: tt.root, Roots: []reach.Root{{Func: lookupFunc(t, prog, app, tt.root)}}}
			g, err := reach.Build(prog, []reach.Entry{entry})
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			got := hitNames(mustReach(t, g, entry, synthTargets(t, prog)))
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Fatalf("Reach(%s) = %v, want %v", tt.root, got, tt.want)
			}
		})
	}
}

func testGraphUnreachedTargetStaysUnreached(t *testing.T, prog *reach.Program) {
	const app = "example.com/synth/app"
	var entries []reach.Entry
	for _, root := range []string{"Direct", "ViaInterface", "ViaClosure", "ViaMethodValue", "ViaFuncValue", "ViaPackageVar", "NewHandler", "ReadOnly"} {
		entries = append(entries, reach.Entry{Surface: "test", Name: root, Roots: []reach.Root{{Func: lookupFunc(t, prog, app, root)}}})
	}
	g, err := reach.Build(prog, entries)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	other := lookupFunc(t, prog, "example.com/synth/gitx", "Other")
	for _, e := range entries {
		for _, h := range mustReach(t, g, e, synthTargets(t, prog)) {
			if h.Target == other {
				t.Fatalf("%s reaches gitx.Other, which only the unlisted Unreached calls", e.Name)
			}
		}
	}
}

// testGraphReachCutsOnlyAtItsOwnDescendants pins R1-A3 (ledger SI-314
// (2)): traversal stops at the roots of the traversing entry's own
// descendants, never at an unrelated entry's root, so a verb that
// delegates to another verb's code reaches what that code reaches.
func testGraphReachCutsOnlyAtItsOwnDescendants(t *testing.T, prog *reach.Program) {
	const app = "example.com/synth/app"
	tests := []struct {
		name      string
		parent    string
		wantOuter string
	}{
		{"an unrelated entry's root is traversed", "", "Mutate"},
		{"a descendant's root is cut", "outer", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outer := reach.Entry{Surface: "test", Name: "outer", Roots: []reach.Root{{Func: lookupFunc(t, prog, app, "ViaInterface")}}}
			inner := reach.Entry{Surface: "test", Name: "inner", Parent: tt.parent, Roots: []reach.Root{{Func: lookupFunc(t, prog, app, "mutator.Do")}}}
			g, err := reach.Build(prog, []reach.Entry{outer, inner})
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			if got := strings.Join(hitNames(mustReach(t, g, outer, synthTargets(t, prog))), ","); got != tt.wantOuter {
				t.Fatalf("outer reaches %q, want %q", got, tt.wantOuter)
			}
			if got := strings.Join(hitNames(mustReach(t, g, inner, synthTargets(t, prog))), ","); got != "Mutate" {
				t.Fatalf("inner reaches %q, want Mutate", got)
			}
		})
	}
}

// testGraphReachCutsAtTheEntriesAHostServes pins the other half of R1-A3,
// as ledger SI-317 (1) corrects it: only a verb named a host (serve, mcp,
// context mcp in the module; here serve and mcp) stops at the entries of
// another surface whose site its code reaches; any other verb that builds
// the same server and drives it in process (qc, qi) reaches what the
// hosted entries do.
func testGraphReachCutsAtTheEntriesAHostServes(t *testing.T, prog *reach.Program) {
	cli, err := reach.CLIEntries(prog, "example.com/synth/cli", "Run", "cli")
	if err != nil {
		t.Fatalf("CLIEntries: %v", err)
	}
	pre, err := reach.PreDispatchEntry(prog, "example.com/synth/cli", "Run", "cli")
	if err != nil {
		t.Fatalf("PreDispatchEntry: %v", err)
	}
	routes, err := reach.RouteEntries(prog, "example.com/synth/web", "workbench")
	if err != nil {
		t.Fatalf("RouteEntries: %v", err)
	}
	tools, err := reach.SwitchEntries(prog, "example.com/synth/tools", "mcp", []string{"write_tool", "read_tool"})
	if err != nil {
		t.Fatalf("SwitchEntries: %v", err)
	}
	for i := range cli {
		cli[i].Host = cli[i].Name == "serve" || cli[i].Name == "mcp"
	}
	all := append(append(append(cli, pre), routes...), tools...)
	got := reachByName(t, prog, all)
	tests := []struct {
		entry string
		want  string
	}{
		{"serve", "Mutate"},      // the server's construction names app.Direct; every route it registers is served and cut (no Publish)
		{"mcp", ""},              // reaches the tool switch: every tool is served and cut
		{"qc", "Mutate,Publish"}, // R1-RR-B1: builds the same workbench but is no host, so the routes' work is its own
		{"qi", "Mutate"},         // R1-RR-B1: calls the same tool server but is no host
		{"ds", "Mutate"},         // delegation is traversed
		{"sub", ""},              // its own arms are cut
		{reach.PreDispatch, ""},
		{"/quick/thing/{name}/api/{action}", "Mutate,Publish"},
		{"/thing/{name}/api/{action}", ""},
		{"write_tool", "Mutate"},
	}
	for _, tt := range tests {
		t.Run(tt.entry, func(t *testing.T) {
			if got[tt.entry] != tt.want {
				t.Fatalf("%s reaches %q, want %q", tt.entry, got[tt.entry], tt.want)
			}
		})
	}
}

// TestBuild_RejectsAnEntryNamedTwice: descendants and hosts are found by
// name, so a name must mean one entry per surface.
func testBuildRejectsAnEntryNamedTwice(t *testing.T, prog *reach.Program) {
	e := reach.Entry{Surface: "test", Name: "twice", Roots: []reach.Root{{Func: lookupFunc(t, prog, "example.com/synth/app", "Direct")}}}
	if _, err := reach.Build(prog, []reach.Entry{e, e}); err == nil {
		t.Fatal("Build accepted two entries with one surface and name")
	}
}

func testGraphHitPathRunsFromRootToTarget(t *testing.T, prog *reach.Program) {
	entry := reach.Entry{Surface: "test", Name: "closure", Roots: []reach.Root{{Func: lookupFunc(t, prog, "example.com/synth/app", "ViaClosure")}}}
	g, err := reach.Build(prog, []reach.Entry{entry})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	hits := mustReach(t, g, entry, synthTargets(t, prog))
	if len(hits) != 1 {
		t.Fatalf("hits = %v, want one", hitNames(hits))
	}
	path := hits[0].Path
	if len(path) < 3 || path[0] != "app.ViaClosure" || path[len(path)-1] != "gitx.Mutate" {
		t.Fatalf("path = %v, want app.ViaClosure -> closure -> gitx.Mutate", path)
	}
}

// testGraphReachFailsOnRootsOutsideTheGraph pins R1-A7: an entry whose
// roots the graph has no node for is an error, never "reaches nothing".
func testGraphReachFailsOnRootsOutsideTheGraph(t *testing.T, prog *reach.Program) {
	built := reach.Entry{Surface: "test", Name: "built", Roots: []reach.Root{{Func: lookupFunc(t, prog, "example.com/synth/app", "Direct")}}}
	g, err := reach.Build(prog, []reach.Entry{built})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if _, err := g.Reach(built, synthTargets(t, prog)); err != nil {
		t.Fatalf("Reach(built) = %v, want no error", err)
	}
	tests := []struct {
		name  string
		entry reach.Entry
	}{
		{"an arm the graph was not built with", reach.Entry{Surface: "test", Name: "stranger", Roots: []reach.Root{{Arm: &ast.CaseClause{}}}}},
		{"an entry the graph was not built for", reach.Entry{Surface: "test", Name: "stranger", Roots: built.Roots}},
		{"an empty root", reach.Entry{Surface: "test", Name: "empty", Roots: []reach.Root{{}}}},
		{"no roots", reach.Entry{Surface: "test", Name: "bare"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hits, err := g.Reach(tt.entry, synthTargets(t, prog))
			if err == nil {
				t.Fatalf("Reach(%s) = %v, nil; want an error, never \"reaches nothing\"", tt.entry.Name, hits)
			}
		})
	}
}

func testBuildRejectsRootsOutsideTheModule(t *testing.T, prog *reach.Program) {
	foreign := types.NewFunc(0, types.NewPackage("example.org/elsewhere", "elsewhere"), "F", types.NewSignatureType(nil, nil, nil, nil, nil, false))
	tests := []struct {
		name  string
		entry reach.Entry
	}{
		{"empty root", reach.Entry{Surface: "test", Name: "bad", Roots: []reach.Root{{}}}},
		{"function the module does not declare", reach.Entry{Surface: "test", Name: "bad", Roots: []reach.Root{{Func: foreign}}}},
		{"entry without roots", reach.Entry{Surface: "test", Name: "bare"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := reach.Build(prog, []reach.Entry{tt.entry}); err == nil {
				t.Fatal("Build accepted an entry whose roots name no module code")
			}
		})
	}
}

// TestBuild_FailsOnAGenericImplementationOfAModuleInterface pins R1-A5's
// tripwire: interface dispatch to a generic type is not modeled, so a
// module where a generic type with methods implements a module interface
// is refused rather than analyzed with the call missing.
func TestBuild_FailsOnAGenericImplementationOfAModuleInterface(t *testing.T) {
	prog, err := reach.Load(context.Background(), filepath.Join("testdata", "generic"), reach.Targets()[0], "./...")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	use := prog.FuncByName("g.Use")
	if use == nil {
		t.Fatal("no g.Use")
	}
	_, err = reach.Build(prog, []reach.Entry{{Surface: "test", Name: "use", Roots: []reach.Root{{Func: use}}}})
	if err == nil || !strings.Contains(err.Error(), "box") || !strings.Contains(err.Error(), "Sizer") {
		t.Fatalf("Build = %v, want an error naming the generic type box and the interface Sizer", err)
	}
}

// testBuildAcceptsAGenericTypeNoModuleInterfaceMatches: synth's pair is a
// generic type with methods that implement no module interface, so the
// tripwire stays quiet.
func testBuildAcceptsAGenericTypeNoModuleInterfaceMatches(t *testing.T, prog *reach.Program) {
	entry := reach.Entry{Surface: "test", Name: "swapped", Roots: []reach.Root{{Func: lookupFunc(t, prog, "example.com/synth/app", "Swapped")}}}
	if _, err := reach.Build(prog, []reach.Entry{entry}); err != nil {
		t.Fatalf("Build(synth) = %v, want nil: pair implements no module interface", err)
	}
}
