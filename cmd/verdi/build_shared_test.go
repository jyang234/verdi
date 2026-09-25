// build_shared_test.go centralizes the one thing every cmd/verdi test that
// execs the real built binary must never duplicate: the `go build` itself
// (lane T1 test-speed contract step 1). Every test in this package that
// builds the verdi binary with the plain, tag-free `go build -o <path> .`
// invocation must call buildVerdiBinary rather than shelling out to `go
// build` on its own — a second build site defeats the sync.Once sharing
// and pays for a full compile again.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

var (
	buildOnce sync.Once
	builtBin  string
	buildErr  error
	buildDir  string
)

// TestMain removes the shared binary's temp directory when this test
// binary's process exits, whether or not any test actually built it
// (buildVerdiBinary allocates buildDir lazily, on first use). Without this,
// buildVerdiBinary's directory — deliberately NOT a t.TempDir(), since a
// per-test temp dir would be removed at that one test's own cleanup and
// pull the shared binary out from under every later caller — would leak
// for the life of the machine.
func TestMain(m *testing.M) {
	code := m.Run()
	if buildDir != "" {
		// Best-effort: a leaked temp directory is an environmental cleanup
		// nuisance, never a test result, so its error is deliberately
		// discarded rather than changing the process exit code.
		_ = os.RemoveAll(buildDir)
	}
	os.Exit(code)
}

// buildVerdiBinary builds the real verdi binary once per test process
// (shared, via sync.Once, across every test in this package that execs the
// built binary) and returns its path. Every caller must treat the
// returned path as read-only: no test may mutate, move, or remove the
// shared binary.
func buildVerdiBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "verdi-bin")
		if err != nil {
			buildErr = err
			return
		}
		buildDir = dir
		bin := filepath.Join(dir, "verdi")
		cmd := exec.Command("go", "build", "-o", bin, ".")
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		if err := cmd.Run(); err != nil {
			buildErr = fmt.Errorf("building verdi binary: %w\n%s", err, out.String())
			return
		}
		builtBin = bin
	})
	if buildErr != nil {
		t.Fatalf("buildVerdiBinary: %v", buildErr)
	}
	return builtBin
}
