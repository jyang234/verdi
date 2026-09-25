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
//
// It also neutralizes CI_DEFAULT_BRANCH once, process-wide, before any test
// runs (lane R4 test-speed contract step 2): internal/specstate's
// ResolveDefaultBranch precedence chain checks CI_DEFAULT_BRANCH before the
// origin/HEAD symref pinFixtureDefaultBranch sets on a fixture, so an
// ambient value (a real GitLab runner, or a leftover value some other
// process exported into this one) would silently outrank the fixture's own
// git state and make the symref-pinned answer non-authoritative. Clearing
// it here — once, before m.Run(), never inside a running test — is exactly
// what internal/workbench's own testfixture_test.go neutralizeCIEnv does
// per-test; doing it once here (rather than 149 individual t.Setenv calls)
// is what lets the tests that only needed a resolvable default branch, not
// CI_DEFAULT_BRANCH itself, become t.Parallel()-eligible. A test whose own
// subject IS CI_DEFAULT_BRANCH handling (name-grepped
// "UnresolvableDefaultBranch"/"NoDefaultBranch"/"CIEnv"/"GitLabEnv", plus
// TestRunDesignStart_BasesOnDefaultBranch_NotHEAD,
// TestRunDesignStart_NoOriginRemote_DisclosedHeadBase,
// TestRunDesignStart_OriginExistsButUnresolvable_Exit2,
// TestRunClose_EffectiveStatePrecondition_RefusesBeforeMutation,
// TestObligationFrozenProbeBase_ResolvedNameUnresolvableRef_RefusesOperationally,
// and buildGateRepo's shared-fixture override family) keeps its own
// t.Setenv("CI_DEFAULT_BRANCH", ...) exactly as before, unaffected by this
// (t.Setenv scopes to that one test and is restored after it, same as
// always) — this process-wide default only changes what an UNPINNED test
// sees.
func TestMain(m *testing.M) {
	// Best-effort, like the buildDir cleanup below: os.Unsetenv on this
	// well-known variable name cannot meaningfully fail, and this runs
	// before m.Run() even starts, so there is no test result to attach an
	// error to.
	_ = os.Unsetenv("CI_DEFAULT_BRANCH")
	code := m.Run()
	if buildDir != "" {
		// Best-effort: a leaked temp directory is an environmental cleanup
		// nuisance, never a test result, so its error is deliberately
		// discarded rather than changing the process exit code.
		_ = os.RemoveAll(buildDir)
	}
	os.Exit(code)
}

// pinFixtureDefaultBranch points refs/remotes/origin/HEAD at
// refs/remotes/origin/main in a fixturegit-built repo (which always starts
// on branch "main" — fixturegit.Build's --initial-branch=main — and this
// never renames or deletes that branch away), making the default branch
// resolvable through internal/specstate's git-native precedence steps
// (gitx.DefaultBranch reads exactly this symref; git symbolic-ref only
// writes/reads the ref file itself, so this works even though the fixture
// carries no configured "origin" remote and refs/remotes/origin/main is
// never separately created — resolveBranchRef's own local-branch fallback
// then finds refs/heads/main) — the same mechanism
// internal/workbench/testfixture_test.go's setDefaultBranchSymref uses for
// its authoring fixtures, without that helper's own per-call
// neutralizeCIEnv (TestMain now does the CI_DEFAULT_BRANCH half of that
// once, process-wide; this package has no other test-suite-wide dependency
// on the other CI env vars neutralizeCIEnv also clears). This replaces a
// per-test t.Setenv("CI_DEFAULT_BRANCH", "main") pin (lane R4 test-speed
// contract step 2) so ordinary fixture-needs-a-resolvable-default-branch
// tests no longer need t.Setenv at all, and so become t.Parallel()-eligible
// wherever nothing else in them touches process-global state.
func pinFixtureDefaultBranch(t testing.TB, dir string) {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("pinFixtureDefaultBranch: symbolic-ref: %v\n%s", err, out)
	}
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
