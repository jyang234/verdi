package reach_test

import (
	"go/ast"
	"go/types"
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
func synthTargets(t *testing.T, prog *reach.Program) map[*types.Func]bool {
	t.Helper()
	return map[*types.Func]bool{
		lookupFunc(t, prog, "example.com/synth/gitx", "Mutate"):  true,
		lookupFunc(t, prog, "example.com/synth/gitx", "Publish"): true,
		lookupFunc(t, prog, "example.com/synth/gitx", "Prune"):   true,
		lookupFunc(t, prog, "example.com/synth/gitx", "Other"):   true,
	}
}

// mustReach is Graph.Reach for an entry the test built the graph with.
func mustReach(t *testing.T, g *reach.Graph, e reach.Entry, targets map[*types.Func]bool) []reach.Hit {
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
		out = append(out, h.Func.Name())
	}
	sort.Strings(out)
	return out
}

func TestGraph_ReachResolvesEveryCallShape(t *testing.T) {
	prog := loadSynth(t)
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

func TestGraph_UnreachedTargetStaysUnreached(t *testing.T) {
	prog := loadSynth(t)
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
			if h.Func == other {
				t.Fatalf("%s reaches gitx.Other, which only the unlisted Unreached calls", e.Name)
			}
		}
	}
}

func TestGraph_ReachStopsAtAnotherEntrysRoot(t *testing.T) {
	prog := loadSynth(t)
	const app = "example.com/synth/app"
	outer := reach.Entry{Surface: "test", Name: "outer", Roots: []reach.Root{{Func: lookupFunc(t, prog, app, "ViaInterface")}}}
	inner := reach.Entry{Surface: "test", Name: "inner", Roots: []reach.Root{{Func: lookupFunc(t, prog, app, "mutator.Do")}}}
	g, err := reach.Build(prog, []reach.Entry{outer, inner})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got := hitNames(mustReach(t, g, outer, synthTargets(t, prog))); len(got) != 0 {
		t.Fatalf("outer reaches %v through inner's root; the most specific entry owns the mutation", got)
	}
	if got := hitNames(mustReach(t, g, inner, synthTargets(t, prog))); strings.Join(got, ",") != "Mutate" {
		t.Fatalf("inner reaches %v, want [Mutate]", got)
	}
}

func TestGraph_HitPathRunsFromRootToTarget(t *testing.T) {
	prog := loadSynth(t)
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

// TestGraph_ReachFailsOnRootsOutsideTheGraph pins R1-A7: an entry whose
// roots the graph has no node for is an error, never "reaches nothing".
func TestGraph_ReachFailsOnRootsOutsideTheGraph(t *testing.T) {
	prog := loadSynth(t)
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

func TestBuild_RejectsRootsOutsideTheModule(t *testing.T) {
	prog := loadSynth(t)
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
