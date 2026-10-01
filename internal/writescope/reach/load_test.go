package reach_test

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/jyang234/verdi/internal/writescope/reach"
)

// synthDir is the synthetic module every reach test loads: its own go.mod
// under testdata, standard-library imports only, so loading it is hermetic.
var synthDir = filepath.Join("testdata", "synth")

var (
	synthOnce sync.Once
	synthProg *reach.Program
	synthErr  error
)

// loadSynth loads the synthetic module once per test binary.
func loadSynth(t *testing.T) *reach.Program {
	t.Helper()
	synthOnce.Do(func() {
		synthProg, synthErr = reach.Load(context.Background(), synthDir, reach.Targets()[0], "./...")
	})
	if synthErr != nil {
		t.Fatalf("Load(%s): %v", synthDir, synthErr)
	}
	return synthProg
}

func TestLoad_TypeChecksEveryModulePackage(t *testing.T) {
	prog := loadSynth(t)
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
		{"pattern matching nothing loadable", synthDir, linux, []string{"example.com/synth/no-such-package"}},
		{"no patterns", synthDir, linux, nil},
		{"incomplete target", synthDir, reach.Target{GOOS: "linux"}, []string{"./..."}},
		{"unknown target", synthDir, reach.Target{GOOS: "plan10", GOARCH: "amd64"}, []string{"./..."}},
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
