package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	forgepkg "github.com/OWNER/verdi/internal/forge"
	"github.com/OWNER/verdi/internal/forge/fake"
	"github.com/OWNER/verdi/internal/upstream"
)

const svcfixSrcDir = "../../testdata/svcfix"
const cannedSrcDir = "../../testdata/svcfix-canned"
const corpusSrcDir = "../../testdata/corpus"
const bundleGoldenDir = "../../testdata/svcfix-canned/bundle-golden"

const testCommit = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
const testRef = "spec/stale-decline"

// buildTestStore assembles a minimal store root in a temp dir: a
// verdi.yaml, the stale-decline spec (copied from testdata/corpus, whose
// AC ids testdata/svcfix's verdi.bindings.yaml binds to), and a copy of
// testdata/svcfix as the one discovered service.
func buildTestStore(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	if err := os.CopyFS(filepath.Join(root, "svcfix"), os.DirFS(svcfixSrcDir)); err != nil {
		t.Fatalf("copying svcfix fixture: %v", err)
	}

	specDir := filepath.Join(root, ".verdi", "specs", "active", "stale-decline")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}
	specData, err := os.ReadFile(filepath.Join(corpusSrcDir, ".verdi", "specs", "active", "stale-decline", "spec.md"))
	if err != nil {
		t.Fatalf("reading corpus stale-decline spec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), specData, 0o644); err != nil {
		t.Fatalf("writing spec.md: %v", err)
	}

	manifest := `schema: verdi.layout/v1
forge: gitlab
services:
  discovery: flowmap
toolchain:
  module: github.com/jyang234/golang-code-graph
  commit: cd38b1a56bb7deadbeefdeadbeefdeadbeefdead
`
	if err := os.MkdirAll(filepath.Join(root, ".verdi"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".verdi", "verdi.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("writing verdi.yaml: %v", err)
	}
	return root
}

func readCannedFile(t *testing.T, dir, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("reading %s/%s: %v", dir, name, err)
	}
	return data
}

// fakeGoTest is a canned goTestRunner: a real `go test -json` capture from
// testdata/svcfix's own suite (both tests passing), matching
// internal/bundle's own realGoTestJSON fixture.
type fakeGoTest struct{ output []byte }

func (f fakeGoTest) RunGoTest(ctx context.Context, dir string) ([]byte, error) {
	return f.output, nil
}

const svcfixGoTestJSON = `
{"Action":"start","Package":"example.com/svcfix/internal/app"}
{"Action":"run","Package":"example.com/svcfix/internal/app","Test":"TestRefundFlow"}
{"Action":"pass","Package":"example.com/svcfix/internal/app","Test":"TestRefundFlow","Elapsed":0}
{"Action":"run","Package":"example.com/svcfix/internal/app","Test":"TestGetRefund"}
{"Action":"pass","Package":"example.com/svcfix/internal/app","Test":"TestGetRefund","Elapsed":0}
{"Action":"pass","Package":"example.com/svcfix/internal/app","Elapsed":0.288}
`

// boundaryWriteRunner wraps a Runner and simulates `flowmap boundary`'s
// real side effect (it writes .flowmap/boundary-contract.json in place —
// spike S1's "no stdout mode" finding) by writing branchContract to disk
// whenever a non-check boundary request passes through, since FakeRunner
// itself only returns canned Results and performs no filesystem I/O.
type boundaryWriteRunner struct {
	upstream.Runner
	svcDir         string
	branchContract []byte
}

func (r boundaryWriteRunner) Run(ctx context.Context, req upstream.Request) (upstream.Result, error) {
	res, err := r.Runner.Run(ctx, req)
	if err == nil && req.Bin == "flowmap" && req.Subcommand == "boundary" && !hasFlag(req.Flags, "-check") {
		_ = os.WriteFile(filepath.Join(r.svcDir, ".flowmap", "boundary-contract.json"), r.branchContract, 0o644)
	}
	return res, err
}

func hasFlag(flags []string, name string) bool {
	for _, f := range flags {
		if f == name {
			return true
		}
	}
	return false
}

// seedRunner builds a FakeRunner wrapped to simulate the boundary
// side-effect, primed with svcfix's real S1 captures.
func seedRunner(t *testing.T, root string) upstream.Runner {
	t.Helper()
	fr := upstream.NewFakeRunner()
	fr.Enqueue("flowmap", "graph", upstream.Result{Stdout: readCannedFile(t, cannedSrcDir, "graph.json"), ExitCode: 0})
	fr.Enqueue("flowmap", "boundary", upstream.Result{ExitCode: 0})
	fr.Enqueue("groundwork", "review", upstream.Result{Stdout: readCannedFile(t, cannedSrcDir, "review-structurally-clear.json"), ExitCode: 0})

	return boundaryWriteRunner{
		Runner:         fr,
		svcDir:         filepath.Join(root, "svcfix"),
		branchContract: readCannedFile(t, cannedSrcDir, "boundary-contract-branch.json"),
	}
}

// TestRunSync_OrRegen_MatchesGolden proves `sync --or-regen`, driven
// entirely by canned upstream output (no exec, no network), materializes a
// bundle byte-identical to testdata/svcfix-canned/bundle-golden/ — the
// property the exit criteria calls for.
func TestRunSync_OrRegen_MatchesGolden(t *testing.T) {
	root := buildTestStore(t)
	runner := seedRunner(t, root)

	var stdout, stderr bytes.Buffer
	deps := syncDeps{
		Runner: runner,
		Forge:  fake.New(), // unseeded: no CI bundle, forces the regen path
		GoTest: fakeGoTest{output: []byte(svcfixGoTestJSON)},
		Stdout: &stdout,
		Stderr: &stderr,
	}

	code := runSync(context.Background(), root, testRef, testCommit, true, deps)
	if code != 0 {
		t.Fatalf("runSync(--or-regen) exit = %d, want 0; stderr=%s", code, stderr.String())
	}

	gotDir := filepath.Join(root, ".verdi", "data", "derived", "spec--stale-decline", testCommit)
	for _, name := range derivedFileNames {
		got, err := os.ReadFile(filepath.Join(gotDir, name))
		if err != nil {
			t.Fatalf("reading materialized %s: %v", name, err)
		}
		want, err := os.ReadFile(filepath.Join(bundleGoldenDir, name))
		if err != nil {
			t.Fatalf("reading golden %s: %v", name, err)
		}
		if string(got) != string(want) {
			t.Errorf("%s differs from golden:\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
		}
	}
}

// TestRunSync_CI_PullsBundle proves plain `sync` (no --or-regen) pulls the
// bundle through the forge port and marks it materialized with source: ci
// already baked in (the forge just returns bytes a CI run already
// assembled with that provenance) — never touching the Runner at all.
func TestRunSync_CI_PullsBundle(t *testing.T) {
	root := buildTestStore(t)
	f := fake.New()
	f.SeedBundle(testRef, testCommit, forgepkg.EvidenceBundle{
		Verdicts:     readCannedFile(t, bundleGoldenDir, "verdicts.json"),
		Tests:        readCannedFile(t, bundleGoldenDir, "tests.json"),
		Review:       readCannedFile(t, bundleGoldenDir, "review.json"),
		BoundaryDiff: readCannedFile(t, bundleGoldenDir, "boundary-diff.json"),
	})

	var stdout, stderr bytes.Buffer
	deps := syncDeps{
		Runner: upstream.NewFakeRunner(), // never called: CI path never execs the toolchain
		Forge:  f,
		GoTest: fakeGoTest{},
		Stdout: &stdout,
		Stderr: &stderr,
	}

	code := runSync(context.Background(), root, testRef, testCommit, false, deps)
	if code != 0 {
		t.Fatalf("runSync exit = %d, want 0; stderr=%s", code, stderr.String())
	}

	gotDir := filepath.Join(root, ".verdi", "data", "derived", "spec--stale-decline", testCommit)
	data, err := os.ReadFile(filepath.Join(gotDir, "verdicts.json"))
	if err != nil {
		t.Fatalf("reading materialized verdicts.json: %v", err)
	}
	want := readCannedFile(t, bundleGoldenDir, "verdicts.json")
	if string(data) != string(want) {
		t.Errorf("materialized verdicts.json (source: ci pull) differs from golden")
	}
}

// TestRunSync_NoBundle_NoRegen_ExitsOperational proves plain `sync` with
// no CI bundle available and no --or-regen fails loudly (exit 2) rather
// than silently regenerating anyway.
func TestRunSync_NoBundle_NoRegen_ExitsOperational(t *testing.T) {
	root := buildTestStore(t)
	var stdout, stderr bytes.Buffer
	deps := syncDeps{
		Runner: upstream.NewFakeRunner(),
		Forge:  fake.New(), // unseeded
		GoTest: fakeGoTest{},
		Stdout: &stdout,
		Stderr: &stderr,
	}

	code := runSync(context.Background(), root, testRef, testCommit, false, deps)
	if code != 2 {
		t.Fatalf("runSync (no bundle, no --or-regen) exit = %d, want 2", code)
	}
	if stderr.Len() == 0 {
		t.Error("expected an explanatory stderr message")
	}
}

// TestRunSync_BlockingReview_ExitsOne proves a materialized bundle whose
// review verdicts BLOCK surfaces sync's own exit 1 (verdict failure),
// using the real BLOCK capture.
func TestRunSync_BlockingReview_ExitsOne(t *testing.T) {
	root := buildTestStore(t)
	fr := upstream.NewFakeRunner()
	fr.Enqueue("flowmap", "graph", upstream.Result{Stdout: readCannedFile(t, cannedSrcDir, "graph.json"), ExitCode: 0})
	fr.Enqueue("flowmap", "boundary", upstream.Result{ExitCode: 0})
	fr.Enqueue("groundwork", "review", upstream.Result{Stdout: readCannedFile(t, cannedSrcDir, "review-block.json"), ExitCode: 1})

	var stdout, stderr bytes.Buffer
	deps := syncDeps{
		Runner: fr,
		Forge:  fake.New(),
		GoTest: fakeGoTest{output: []byte(svcfixGoTestJSON)},
		Stdout: &stdout,
		Stderr: &stderr,
	}

	code := runSync(context.Background(), root, testRef, testCommit, true, deps)
	if code != 1 {
		t.Fatalf("runSync with a BLOCK review: exit = %d, want 1; stderr=%s", code, stderr.String())
	}
}

// TestRunSync_Negative_UnknownForgeError proves runSync surfaces a forge
// error (not ErrNoBundle) as an operational failure even with --or-regen.
func TestRunSync_Negative_ForgeError(t *testing.T) {
	root := buildTestStore(t)
	deps := syncDeps{
		Runner: upstream.NewFakeRunner(),
		Forge:  erroringForge{},
		GoTest: fakeGoTest{},
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	}
	code := runSync(context.Background(), root, testRef, testCommit, true, deps)
	if code != 2 {
		t.Fatalf("runSync with a forge error: exit = %d, want 2", code)
	}
}

// erroringForge is a minimal forgepkg.Forge whose FetchEvidenceBundle
// always fails with a plain (non-ErrNoBundle) error, to prove runSync
// treats that as operational regardless of --or-regen.
type erroringForge struct{}

func (erroringForge) FetchEvidenceBundle(ctx context.Context, ref, commit string) (*forgepkg.EvidenceBundle, error) {
	return nil, errors.New("forge: simulated transport failure")
}
func (erroringForge) GeneratedAttribute() string { return "x-generated" }
func (erroringForge) CIContext(ctx context.Context) (forgepkg.CIInfo, error) {
	return forgepkg.CIInfo{}, nil
}

var _ forgepkg.Forge = erroringForge{}
