package reach_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/writescope/reach"
)

// synthDir is the synthetic module every reach test loads: its own go.mod
// under testdata, standard-library imports only, so loading it is hermetic.
func synthDir() string { return filepath.Join("testdata", "synth") }

// TestSynth loads the synthetic module once and runs, as subtests, every
// test that analyzes it: one type-check per test binary, shared by value,
// with no package-level state (docs/ground-rules.md).
func TestSynth(t *testing.T) {
	prog, err := reach.Load(context.Background(), synthDir(), reach.Targets()[0], "./...")
	if err != nil {
		t.Fatalf("Load(%s): %v", synthDir(), err)
	}
	for _, tc := range []struct {
		name string
		run  func(*testing.T, *reach.Program)
	}{
		{"Load_TypeChecksEveryModulePackage", testLoadTypeChecksEveryModulePackage},
		{"Load_ReportsTheInputsGoTestMustTrack", testLoadReportsTheInputsGoTestMustTrack},
		{"Graph_ReachResolvesEveryCallShape", testGraphReachResolvesEveryCallShape},
		{"Graph_ReachHitsEveryTargetShape", testGraphReachHitsEveryTargetShape},
		{"Graph_ReachFollowsGitxsOtherExportShapes", testGraphReachFollowsGitxsOtherExportShapes},
		{"Graph_UnreachedTargetStaysUnreached", testGraphUnreachedTargetStaysUnreached},
		{"Graph_ReachCutsOnlyAtItsOwnDescendants", testGraphReachCutsOnlyAtItsOwnDescendants},
		{"Graph_ReachCutsAtTheEntriesAHostServes", testGraphReachCutsAtTheEntriesAHostServes},
		{"Graph_HitPathRunsFromRootToTarget", testGraphHitPathRunsFromRootToTarget},
		{"Graph_ReachFailsOnRootsOutsideTheGraph", testGraphReachFailsOnRootsOutsideTheGraph},
		{"Build_RejectsAnEntryNamedTwice", testBuildRejectsAnEntryNamedTwice},
		{"Build_RejectsRootsOutsideTheModule", testBuildRejectsRootsOutsideTheModule},
		{"Build_AcceptsAGenericTypeNoModuleInterfaceMatches", testBuildAcceptsAGenericTypeNoModuleInterfaceMatches},
		{"CLIEntries_DeriveVerbsFromTheDispatcher", testCLIEntriesDeriveVerbsFromTheDispatcher},
		{"CLIEntries_Errors", testCLIEntriesErrors},
		{"PreDispatchEntry_ReachesWhatRunsForEveryVerb", testPreDispatchEntryReachesWhatRunsForEveryVerb},
		{"PreDispatchEntry_Errors", testPreDispatchEntryErrors},
		{"SwitchEntries_MatchTheInventorysOneSwitch", testSwitchEntriesMatchTheInventorysOneSwitch},
		{"SwitchEntries_Errors", testSwitchEntriesErrors},
		{"RouteEntries_DeriveRoutesAndActions", testRouteEntriesDeriveRoutesAndActions},
		{"RouteEntries_FailClosedOnUnresolvedFieldCalls", testRouteEntriesFailClosedOnUnresolvedFieldCalls},
		{"RouteEntries_FailClosedOnAnUnresolvableRegistration", testRouteEntriesFailClosedOnAnUnresolvableRegistration},
		{"RouteEntries_FailClosedOnUnfollowableFunctionValues", testRouteEntriesFailClosedOnUnfollowableFunctionValues},
		{"DisclosedBoundaries", testDisclosedBoundaries},
		{"StringKeyedMap", testStringKeyedMap},
		{"ExportedFuncs_ListsEveryCallableExport", testExportedFuncsListsEveryCallableExport},
		{"ExportedFuncs_Errors", testExportedFuncsErrors},
		{"ObjectByName_ResolvesEveryNameForm", testObjectByNameResolvesEveryNameForm},
		{"FuncByName_FindsOnlyDeclaredFunctions", testFuncByNameFindsOnlyDeclaredFunctions},
		{"GitDirWriters_FindsEveryWriteUnderTheGitDirectory", testGitDirWritersFindsEveryWriteUnderTheGitDirectory},
		{"GitDirWriters_Errors", testGitDirWritersErrors},
	} {
		t.Run(tc.name, func(t *testing.T) { tc.run(t, prog) })
	}
}

func testLoadTypeChecksEveryModulePackage(t *testing.T, prog *reach.Program) {
	if prog.Module != "example.com/synth" {
		t.Fatalf("Module = %q, want example.com/synth", prog.Module)
	}
	tests := []struct {
		path string
		obj  string
	}{
		{"example.com/synth/gitx", "Mutate"},
		{"example.com/synth/app", "ViaInterface"},
		{"example.com/synth/cli", "Run"},
		{"example.com/synth/web", "Register"},
		{"example.com/synth/strictweb", "Register"},
		{"example.com/synth/precli", "Run"},
		{"example.com/synth/hooks", "All"},
		{"example.com/synth/holes", "Register"},
		{"example.com/synth/disclosed", "Register"},
		{"example.com/synth/subhost", "NewSub"},
		{"example.com/synth/tools", "Server"},
		{"example.com/synth/gitdir", "Reconcile"},
		{"example.com/synth/other", "Write"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			pkg := prog.Package(tt.path)
			if pkg == nil {
				t.Fatalf("Package(%q) = nil", tt.path)
			}
			if len(pkg.Files) == 0 || pkg.Info == nil || pkg.Types == nil {
				t.Fatalf("Package(%q) is not type-checked from source: files=%d info=%v", tt.path, len(pkg.Files), pkg.Info != nil)
			}
			if pkg.Types.Scope().Lookup(tt.obj) == nil {
				t.Fatalf("Package(%q) scope has no %s", tt.path, tt.obj)
			}
		})
	}
	if got := prog.Package("net/http"); got != nil {
		t.Fatalf("Package(net/http) = %v, want nil: only the module's own packages are kept", got.Path)
	}
}

func TestLoad_Errors(t *testing.T) {
	linux := reach.Targets()[0]
	tests := []struct {
		name     string
		dir      string
		target   reach.Target
		patterns []string
	}{
		{"missing directory", filepath.Join("testdata", "no-such-module"), linux, []string{"./..."}},
		{"pattern matching nothing loadable", synthDir(), linux, []string{"example.com/synth/no-such-package"}},
		{"no patterns", synthDir(), linux, nil},
		{"incomplete target", synthDir(), reach.Target{GOOS: "linux"}, []string{"./..."}},
		{"unknown target", synthDir(), reach.Target{GOOS: "plan10", GOARCH: "amd64"}, []string{"./..."}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := reach.Load(context.Background(), tt.dir, tt.target, tt.patterns...); err == nil {
				t.Fatalf("Load(%s, %s, %v) succeeded, want an error", tt.dir, tt.target, tt.patterns)
			}
		})
	}
}

func TestTargets_CoverBothReleasePlatforms(t *testing.T) {
	got := map[string]bool{}
	for _, tg := range reach.Targets() {
		got[tg.String()] = true
	}
	for _, want := range []string{"linux/amd64", "darwin/arm64"} {
		if !got[want] {
			t.Errorf("Targets() lacks %s: %v", want, got)
		}
	}
}

func testLoadReportsTheInputsGoTestMustTrack(t *testing.T, prog *reach.Program) {
	inputs := prog.Inputs()
	want := map[string]bool{
		filepath.Join("testdata", "synth", "go.mod"): false,
		filepath.Join("testdata", "synth", "gitx"):   false,
		filepath.Join("testdata", "synth", "cli"):    false,
	}
	for _, in := range inputs {
		for w := range want {
			if strings.HasSuffix(in, w) {
				want[w] = true
			}
		}
	}
	for w, seen := range want {
		if !seen {
			t.Errorf("Inputs() lacks %s; got %v", w, inputs)
		}
	}
}
