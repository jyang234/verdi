// Package specalign is `make spec-align`'s home: the spec-alignment gate
// PLAN.md's "Owner amendment (build kickoff)" promises ("`make verify`
// grows to include e2e and a `spec-align` check by the end of the
// build") and the wave-7 integration task charges to this package.
//
// It is a test-only package (every .go file here is a _test.go file,
// following internal/corpus's and internal/svcfixcanned's own
// precedent) run via `go test ./internal/specalign/...` — that is what
// the Makefile's `spec-align` target invokes. It asserts, against THIS
// repo's own checkout (never a synthetic fixture — the whole point is
// self-hosting honesty):
//
//   - TestSelfHostedSpecFidelity (00 §How these documents are
//     maintained): each of the six specs under .verdi/specs/active/
//     is byte-identical to its docs/design/specs/ origin except the
//     single status: draft -> status: active line. SKIPS, loudly, when
//     the workspace layout (docs/ as a sibling of verdi/) is absent —
//     e.g. a CI checkout of verdi alone — rather than faking a pass.
//   - TestV0ThinSliceChecklist (00 §v0 thin slice checklist): one
//     named subtest per checklist bullet, each an executable assertion
//     that the bullet's shipped surface really exists on this repo.
//   - TestMCPToolInventory (05 §MCP server): the live server's
//     tools/list result is exactly the nine named tools (get_board
//     grown at V1-P9); TestMCPToolInventory_ListAnnotationsDocumentsReviewPopulation
//     additionally locks in that list_annotations' description documents
//     its mirrored review-sticky population, not just its existence.
//   - TestV0CLIVerbInventory (05 §CLI): every verb the spec (plus the
//     I-7/I-20 invented verbs) names responds per its v0 status — real
//     verbs never say "not implemented"; the four explicitly
//     out-of-v0 verbs (PLAN.md §5: close, gc, waivers,
//     verify-artifact) always do, with the out-of-scope message.
package specalign

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/lint"
)

// verdiRepoRoot and verdiBinPath are populated once by TestMain and read
// by every test in the package.
var (
	verdiRepoRoot string
	verdiBinPath  string
)

// TestMain resolves this repo's root robustly (via this source file's own
// compiled-in path, independent of the test binary's working directory)
// and builds the real verdi binary ONCE for every test in the package to
// exec against — build-then-exec, matching the Makefile's own lint-store
// convention ("the gate exercises the exact binary CI would ship"), never
// `go run` (which swallows child exit codes — the phase-1 defect
// PLAN.md's exit criteria comment records).
func TestMain(m *testing.M) {
	root, err := computeVerdiRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "specalign: TestMain: resolving verdi root:", err)
		os.Exit(2)
	}
	verdiRepoRoot = root

	tmp, err := os.MkdirTemp("", "verdi-specalign-bin-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "specalign: TestMain: mkdtemp:", err)
		os.Exit(2)
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	verdiBinPath = filepath.Join(tmp, "verdi")
	cmd := exec.Command("go", "build", "-o", verdiBinPath, "./cmd/verdi")
	cmd.Dir = verdiRepoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "specalign: TestMain: building verdi binary: %v\n%s\n", err, out)
		os.Exit(2)
	}

	os.Exit(m.Run())
}

// computeVerdiRoot resolves the verdi module root from THIS file's own
// path, recorded at compile time by runtime.Caller — robust regardless of
// the test binary's cwd or how `go test` was invoked (unlike relying on
// os.Getwd(), which `go test` happens to set to the package directory
// today but nothing guarantees generally).
func computeVerdiRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("runtime.Caller(0) failed")
	}
	// this file lives at <verdiRoot>/internal/specalign/helpers_test.go
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return "", fmt.Errorf("resolving verdi root from %s: %w", file, err)
	}
	return root, nil
}

// workspaceDocsDir is where the six specs' authoritative originals live
// per 00-index.md's own maintenance note: "resident at
// .verdi/specs/active/ ... " copied FROM docs/design/specs/, which lives
// OUTSIDE the verdi repo, workspace-relative (../docs/design/specs from
// verdi/). This is read-only-checked, never assumed present — see
// TestSelfHostedSpecFidelity's skip path.
func workspaceDocsDir(verdiRoot string) string {
	return filepath.Clean(filepath.Join(verdiRoot, "..", "docs", "design", "specs"))
}

// runBinary execs the once-built verdi binary with args, cwd=dir,
// capturing stdout/stderr separately and returning the process exit code
// (0 on success). A launch failure that is NOT an ExitError (binary
// missing, permissions, ...) is a test infrastructure failure, not a
// verb-behavior result, so it fails the calling test outright.
func runBinary(t *testing.T, dir string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(verdiBinPath, args...)
	cmd.Dir = dir
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return outBuf.String(), errBuf.String(), ee.ExitCode()
		}
		t.Fatalf("running verdi %v: %v", args, err)
	}
	return outBuf.String(), errBuf.String(), 0
}

// assertNotOutOfV0 fails the test if stderr contains dispatch.go's
// out-of-v0-scope message — the tell that a verb the spec (or the
// invention ledger) names as REAL for v0 has regressed into looking
// unimplemented/out-of-scope.
func assertNotOutOfV0(t *testing.T, verb, stderr string) {
	t.Helper()
	if strings.Contains(stderr, "not implemented") {
		t.Errorf("verdi %s: printed a \"not implemented\" message — this verb is IN v0 scope (05 §CLI) and must be real: %q", verb, stderr)
	}
}

// buildLintContext is lint.BuildContext — the duplicate this helper used
// to carry (cmd/verdi's then-unexported buildLintContext) was lifted into
// internal/lint itself for the disclosures-view enumeration
// (spec/disclosures-panel), so "lint wired" is now proven by literally
// the one shared context-construction path, not a maintained copy.
func buildLintContext(ctx context.Context, root string) lint.Context {
	return lint.BuildContext(ctx, root)
}
