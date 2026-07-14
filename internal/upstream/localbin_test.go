package upstream

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// localBinRunner execs prebuilt flowmap/groundwork binaries directly out of
// a directory (spike S1's captured bin/), rather than `go run …@pin` — no
// network needed, unlike RealRunner. It exists only for
// TestIntegration_LocalBinaries below.
type localBinRunner struct{ dir string }

func (r localBinRunner) Run(ctx context.Context, req Request) (Result, error) {
	bin := filepath.Join(r.dir, req.Bin)
	cmd := exec.CommandContext(ctx, bin, req.buildArgv()...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	res := Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}
	if runErr == nil {
		return res, nil
	}
	if exitErr, ok := runErr.(*exec.ExitError); ok {
		res.ExitCode = exitErr.ExitCode()
		return res, nil
	}
	return res, runErr
}

// TestIntegration_LocalBinaries is the OPTIONAL, non-hermetic integration
// test: it runs the real, prebuilt flowmap/groundwork binaries (spike S1's
// bin/) against testdata/svcfix end to end — graph, boundary generate +
// check, review, and the diff cross-check — proving this package's strict
// decoders and exec wrappers against the real toolchain, not just canned
// JSON. It is skipped, with a clear disclosed reason, unless
// VERDI_S1_BIN names a directory containing both binaries — this repo's
// CI and `make verify`/`make test` never set that variable, so this test
// never runs there and needs no network (CLAUDE.md: "No network in any
// test" — this test still execs no `go run`, only prebuilt local binaries,
// but it stays opt-in because those binaries are not committed to the
// repo).
func TestIntegration_LocalBinaries(t *testing.T) {
	dir := os.Getenv("VERDI_S1_BIN")
	if dir == "" {
		t.Skip("VERDI_S1_BIN not set: skipping the optional real-toolchain integration test (disclosed skip, not a silent pass — see localbin_test.go)")
	}
	if _, err := os.Stat(filepath.Join(dir, "flowmap")); err != nil {
		t.Skipf("VERDI_S1_BIN=%s has no flowmap binary: skipping (%v)", dir, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "groundwork")); err != nil {
		t.Skipf("VERDI_S1_BIN=%s has no groundwork binary: skipping (%v)", dir, err)
	}

	runner := localBinRunner{dir: dir}
	ctx := context.Background()
	svcDir, err := filepath.Abs("../../testdata/svcfix")
	if err != nil {
		t.Fatalf("resolving testdata/svcfix: %v", err)
	}

	base, err := RunGraph(ctx, runner, svcDir, "deadbeef", "")
	if err != nil {
		t.Fatalf("Graph: %v", err)
	}
	if len(base.Obligations) != 1 || base.Obligations[0].Status != ObligationSatisfied {
		t.Fatalf("base graph obligations = %+v, want one SATISFIED audit-before-publish", base.Obligations)
	}

	if err := BoundaryGenerate(ctx, runner, svcDir); err != nil {
		t.Fatalf("BoundaryGenerate: %v", err)
	}
	if err := BoundaryCheck(ctx, runner, svcDir); err != nil {
		t.Fatalf("BoundaryCheck: %v", err)
	}

	policyPath := filepath.Join(svcDir, "policy.json")
	baseGraphPath := filepath.Join(t.TempDir(), "base-graph.json")
	if err := os.WriteFile(baseGraphPath, readCanned(t, "graph.json"), 0o644); err != nil {
		t.Fatalf("writing base graph: %v", err)
	}

	review, err := RunReview(ctx, runner, policyPath, baseGraphPath, baseGraphPath, "")
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if review.Verdict != ReviewNoStructuralSignal && review.Verdict != ReviewStructurallyClear {
		t.Fatalf("Review(base vs itself) verdict = %q, want a clean verdict", review.Verdict)
	}

	baseContractPath := filepath.Join(svcDir, ".flowmap", "boundary-contract.json")
	selfContract := mustReadContract(t, baseContractPath)
	diffs := ComputeBoundaryDiff(selfContract, selfContract)
	if err := CrossCheckDiff(ctx, runner, baseContractPath, baseContractPath, HasBreaking(diffs)); err != nil {
		t.Fatalf("CrossCheckDiff: %v", err)
	}
}

// mustReadContract reads back this integration test's own generated
// boundary contract, so the diff cross-check above compares real,
// currently-on-disk contract state rather than a canned copy.
func mustReadContract(t *testing.T, path string) *BoundaryContract {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading generated boundary contract: %v", err)
	}
	c, err := DecodeBoundaryContract(data)
	if err != nil {
		t.Fatalf("decoding generated boundary contract: %v", err)
	}
	return c
}
