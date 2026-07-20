package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/fixturegit"
	forgepkg "github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/forge/fake"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/upstream"
)

const svcfixSrcDir = "../../testdata/svcfix"
const cannedSrcDir = "../../testdata/svcfix-canned"
const corpusSrcDir = "../../examples/showcase"
const bundleGoldenDir = "../../testdata/svcfix-canned/bundle-golden"

const testCommit = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
const testRef = "spec/stale-decline"

// buildTestStore assembles a minimal store root in a temp dir: a
// verdi.yaml, the stale-decline spec (copied from examples/showcase, whose
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

// buildTestStoreNoServices is buildTestStore's manifest without copying
// any service in — this repo's own self-hosted .verdi/ store has exactly
// this shape (zero discoverable .flowmap.yaml roots: verdi tracks its own
// feature/story specs, not a flowmap-bound service of itself), the real
// case sync_regen.go's regenerate() discloses honestly rather than
// erroring on.
func buildTestStoreNoServices(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	manifest := `schema: verdi.layout/v1
forge: github
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
	// The healthz-route addition (boundary-contract-branch.json vs the
	// base) is non-breaking, matching spike S1's captured `groundwork
	// diff` text output (no BREAKING marker) — exit 0.
	fr.Enqueue("groundwork", "diff", upstream.Result{ExitCode: 0})

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
//
// guide-claim: 7.2-verdi-sync
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

	code := runSync(context.Background(), root, testRef, testCommit, true, false, false, deps)
	if code != 0 {
		t.Fatalf("runSync(--or-regen) exit = %d, want 0; stderr=%s", code, stderr.String())
	}

	gotDir := filepath.Join(root, ".verdi", "data", "derived", "spec--stale-decline", testCommit)
	// The four required files plus toolchain.json: svcfix's canned graph
	// records a tool, so this regeneration writes the optional provenance
	// carrier too (spec/forge-transport ac-4/dc-4), byte-stably.
	names := append(append([]string{}, derivedFileNames...), derivedToolchainFile)
	for _, name := range names {
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

// buildProduceDeps assembles a produce-ready syncDeps against a fresh
// buildTestStore root: seeded upstream runner, canned go test output, and
// a fake forge whose CIContext reports pipeline "913" / job "7" — no
// network, no exec (CLAUDE.md).
func buildProduceDeps(t *testing.T) (root string, deps syncDeps) {
	t.Helper()
	root = buildTestStore(t)
	f := fake.New()
	f.SetCIContext(forgepkg.CIInfo{Pipeline: "913", Job: "7"})
	var stdout, stderr bytes.Buffer
	deps = syncDeps{
		Runner: seedRunner(t, root),
		Forge:  f,
		GoTest: fakeGoTest{output: []byte(svcfixGoTestJSON)},
		Stdout: &stdout,
		Stderr: &stderr,
	}
	return root, deps
}

func readProducedVerdicts(t *testing.T, root string) []artifact.Evidence {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "data", "derived", "spec--stale-decline", testCommit)
	var records []artifact.Evidence
	if err := decodeBundleFile(dir, "verdicts.json", &records); err != nil {
		t.Fatalf("decoding produced verdicts.json: %v", err)
	}
	return records
}

// TestRunSync_Produce_StampsSourceCI_AndPipelineJob proves --produce
// (spec/remote-and-ci dc-1), run inside a detected CI environment,
// assembles the same evidence internal/bundle would assemble for
// --or-regen (identical schema/evidence_for/kind/verdict/witness/producer/
// digest — verified against the committed local-regen golden) but stamps
// provenance.source: ci and pulls pipeline/job from the forge's CIContext,
// never provenance.source: local.
func TestRunSync_Produce_StampsSourceCI_AndPipelineJob(t *testing.T) {
	t.Setenv("CI", "true")
	root, deps := buildProduceDeps(t)

	code := runSync(context.Background(), root, testRef, testCommit, false, true, false, deps)
	stderrBuf := deps.Stderr.(*bytes.Buffer)
	if code != 0 {
		t.Fatalf("runSync(--produce) exit = %d, want 0; stderr=%s", code, stderrBuf.String())
	}

	got := readProducedVerdicts(t, root)
	golden := readCannedFile(t, bundleGoldenDir, "verdicts.json")
	var want []artifact.Evidence
	if err := artifact.DecodeStrictJSON(golden, &want); err != nil {
		t.Fatalf("decoding golden verdicts.json: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("produced %d records, want %d (golden)", len(got), len(want))
	}
	for i := range got {
		if got[i].Provenance.Source != artifact.SourceCI {
			t.Errorf("record %d provenance.source = %q, want ci", i, got[i].Provenance.Source)
		}
		if got[i].Provenance.Pipeline != "913" {
			t.Errorf("record %d provenance.pipeline = %q, want 913", i, got[i].Provenance.Pipeline)
		}
		if got[i].Provenance.Job != "7" {
			t.Errorf("record %d provenance.job = %q, want 7", i, got[i].Provenance.Job)
		}
		if got[i].Provenance.Commit != testCommit {
			t.Errorf("record %d provenance.commit = %q, want %s", i, got[i].Provenance.Commit, testCommit)
		}
		// Everything but provenance must match the local-regen golden
		// exactly: --produce and --or-regen share the same assembly
		// mechanics (regenerate/regenerateServices) and differ only in
		// the provenance they stamp.
		if got[i].Schema != want[i].Schema || got[i].Kind != want[i].Kind || got[i].Verdict != want[i].Verdict ||
			got[i].Witness != want[i].Witness || got[i].Producer != want[i].Producer || got[i].Digest != want[i].Digest ||
			strings.Join(got[i].EvidenceFor, ",") != strings.Join(want[i].EvidenceFor, ",") {
			t.Errorf("record %d non-provenance fields differ from golden:\ngot  %+v\nwant %+v", i, got[i], want[i])
		}
	}
}

// TestRunSync_Produce_DeterministicAcrossRuns proves --produce is
// byte-stable (co-1: "assembles a byte-stable bundle"): the same inputs,
// run twice into independent roots, produce byte-identical bundle files.
func TestRunSync_Produce_DeterministicAcrossRuns(t *testing.T) {
	t.Setenv("CI", "true")
	root1, deps1 := buildProduceDeps(t)
	root2, deps2 := buildProduceDeps(t)

	if code := runSync(context.Background(), root1, testRef, testCommit, false, true, false, deps1); code != 0 {
		t.Fatalf("first runSync(--produce) exit = %d, want 0", code)
	}
	if code := runSync(context.Background(), root2, testRef, testCommit, false, true, false, deps2); code != 0 {
		t.Fatalf("second runSync(--produce) exit = %d, want 0", code)
	}

	dir1 := filepath.Join(root1, ".verdi", "data", "derived", "spec--stale-decline", testCommit)
	dir2 := filepath.Join(root2, ".verdi", "data", "derived", "spec--stale-decline", testCommit)
	for _, name := range derivedFileNames {
		got1, err := os.ReadFile(filepath.Join(dir1, name))
		if err != nil {
			t.Fatalf("reading run1 %s: %v", name, err)
		}
		got2, err := os.ReadFile(filepath.Join(dir2, name))
		if err != nil {
			t.Fatalf("reading run2 %s: %v", name, err)
		}
		if string(got1) != string(got2) {
			t.Errorf("%s not byte-stable across two --produce runs:\n--- run1 ---\n%s\n--- run2 ---\n%s", name, got1, got2)
		}
	}
}

// TestRunSync_Produce_Negative_RefusesOutsideCI proves --produce, run
// with no detected CI environment and no --force-local, refuses (exit 2)
// rather than silently stamping source: ci from a plain developer
// laptop — dc-1's "a local --produce bundle must never reach a gate"
// starts with this refusal.
func TestRunSync_Produce_Negative_RefusesOutsideCI(t *testing.T) {
	t.Setenv("CI", "")
	t.Setenv("GITHUB_ACTIONS", "")
	root, deps := buildProduceDeps(t)

	code := runSync(context.Background(), root, testRef, testCommit, false, true, false, deps)
	stderrBuf := deps.Stderr.(*bytes.Buffer)
	if code != 2 {
		t.Fatalf("runSync(--produce) outside CI without --force-local: exit = %d, want 2; stderr=%s", code, stderrBuf.String())
	}
	if !strings.Contains(stderrBuf.String(), "--force-local") {
		t.Errorf("stderr = %q, want a mention of --force-local", stderrBuf.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".verdi", "data", "derived", "spec--stale-decline", testCommit, "verdicts.json")); err == nil {
		t.Error("refused --produce still wrote verdicts.json; want no bundle written")
	}
}

// TestRunSync_Produce_ForceLocalOverride_StampsSourceLocal proves
// --force-local lets --produce run outside CI for local testing, prints a
// disclosed NON-AUTHORITATIVE warning (mirroring rollup's --force-local
// precedent, I-32), AND — the true-closure fix — stamps the records
// source:LOCAL, never source:ci: no local invocation may emit an
// authoritative record, so a fabricated bundle can never fold as trusted.
func TestRunSync_Produce_ForceLocalOverride_StampsSourceLocal(t *testing.T) {
	t.Setenv("CI", "")
	t.Setenv("GITHUB_ACTIONS", "")
	root, deps := buildProduceDeps(t)

	code := runSync(context.Background(), root, testRef, testCommit, false, true, true, deps)
	stderrBuf := deps.Stderr.(*bytes.Buffer)
	if code != 0 {
		t.Fatalf("runSync(--produce --force-local) exit = %d, want 0; stderr=%s", code, stderrBuf.String())
	}
	if !strings.Contains(stderrBuf.String(), "NON-AUTHORITATIVE") {
		t.Errorf("stderr = %q, want a NON-AUTHORITATIVE disclosure", stderrBuf.String())
	}

	got := readProducedVerdicts(t, root)
	if len(got) == 0 {
		t.Fatal("expected produced records to assert their provenance on")
	}
	for i, r := range got {
		if r.Provenance.Source != artifact.SourceLocal {
			t.Errorf("record %d provenance.source = %q, want local under --force-local (only a genuine CI run may stamp source:ci, true-closure)", i, r.Provenance.Source)
		}
	}
}

// TestRunSync_Produce_Negative_MutuallyExclusiveWithOrRegen proves
// cmdSync's argument parsing rejects --or-regen combined with --produce
// before ever touching the filesystem (store.FindRoot) — the two flags
// express incompatible provenance intents.
func TestRunSync_Produce_Negative_MutuallyExclusiveWithOrRegen(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := cmdSync([]string{"--or-regen", "--produce"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("cmdSync(--or-regen --produce) exit = %d, want 2", code)
	}
	if stderr.Len() == 0 {
		t.Error("expected an explanatory stderr message")
	}
}

// TestRunSync_Produce_Negative_ForceLocalWithoutProduce proves --force-local
// alone (without --produce) is a usage error, not a silently ignored flag.
func TestRunSync_Produce_Negative_ForceLocalWithoutProduce(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := cmdSync([]string{"--force-local"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("cmdSync(--force-local) exit = %d, want 2", code)
	}
	if stderr.Len() == 0 {
		t.Error("expected an explanatory stderr message")
	}
}

// TestCmdSync_ProduceRuntime_Negative_MutuallyExclusive proves
// --produce-runtime is mutually exclusive with both --or-regen and
// --produce — three distinct provenance intents, never combinable
// (spec/runtime-evidence dc-1).
func TestCmdSync_ProduceRuntime_Negative_MutuallyExclusive(t *testing.T) {
	cases := [][]string{
		{"--produce-runtime", "--produce"},
		{"--produce-runtime", "--or-regen"},
	}
	for _, args := range cases {
		var stdout, stderr bytes.Buffer
		code := cmdSync(args, &stdout, &stderr)
		if code != 2 {
			t.Fatalf("cmdSync(%v) exit = %d, want 2", args, code)
		}
	}
}

// TestCmdSync_ProduceRuntime_Negative_ValueFlagsRequireMode proves
// --story/--ac/--verdict/--witness are rejected without --produce-runtime —
// they have no meaning for --produce/--or-regen.
func TestCmdSync_ProduceRuntime_Negative_ValueFlagsRequireMode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := cmdSync([]string{"--produce", "--story", "spec/x"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("cmdSync(--produce --story spec/x) exit = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--produce-runtime") {
		t.Errorf("stderr = %q, want a mention of --produce-runtime", stderr.String())
	}
}

// TestCmdSync_ProduceRuntime_Negative_InvalidVerdict proves an unrecognized
// --verdict value is a usage error, never silently coerced or ignored.
func TestCmdSync_ProduceRuntime_Negative_InvalidVerdict(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := cmdSync([]string{"--produce-runtime", "--story", "spec/x", "--ac", "ac-1", "--verdict", "maybe", "--witness", "w"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("cmdSync(--verdict maybe) exit = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "pass, fail, or abstain") {
		t.Errorf("stderr = %q, want a message naming the valid verdicts", stderr.String())
	}
}

// TestCmdSync_ProduceRuntime_Negative_MissingFlagValue proves a value flag
// given with no following value is a usage error rather than an index panic
// or a silently absorbed empty string.
func TestCmdSync_ProduceRuntime_Negative_MissingFlagValue(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := cmdSync([]string{"--produce-runtime", "--story"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("cmdSync(--story with no value) exit = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "requires a value") {
		t.Errorf("stderr = %q, want a 'requires a value' message", stderr.String())
	}
}

// TestRunSync_Produce_Negative_UnknownForgeCIContextError proves a forge
// CIContext error surfaces as an operational failure (exit 2), not a
// silently-empty pipeline/job stamp.
func TestRunSync_Produce_Negative_ForgeCIContextError(t *testing.T) {
	t.Setenv("CI", "true")
	root := buildTestStore(t)
	var stdout, stderr bytes.Buffer
	deps := syncDeps{
		Runner: seedRunner(t, root),
		Forge:  erroringForge{},
		GoTest: fakeGoTest{output: []byte(svcfixGoTestJSON)},
		Stdout: &stdout,
		Stderr: &stderr,
	}

	code := runSync(context.Background(), root, testRef, testCommit, false, true, false, deps)
	if code != 2 {
		t.Fatalf("runSync(--produce) with a CIContext error: exit = %d, want 2; stderr=%s", code, stderr.String())
	}
}

// TestRunSync_Produce_NoServicesDiscovered_StillSucceeds proves --produce
// against a store with zero discoverable services (this repo's own
// self-hosted .verdi/ store, in real life, once verify.yml's own
// `sync --produce` step runs it for real, round 6) still succeeds honestly
// with an empty-but-well-formed
// bundle, rather than failing with "nothing to regenerate" — hermetic:
// DiscoverServices finding nothing means the toolchain Runner is never
// invoked at all.
func TestRunSync_Produce_NoServicesDiscovered_StillSucceeds(t *testing.T) {
	t.Setenv("CI", "true")
	root := buildTestStoreNoServices(t)
	f := fake.New()
	f.SetCIContext(forgepkg.CIInfo{Pipeline: "913", Job: "7"})
	var stdout, stderr bytes.Buffer
	deps := syncDeps{
		Runner: upstream.NewFakeRunner(), // never called: no service to regenerate
		Forge:  f,
		GoTest: fakeGoTest{},
		Stdout: &stdout,
		Stderr: &stderr,
	}

	code := runSync(context.Background(), root, testRef, testCommit, false, true, false, deps)
	if code != 0 {
		t.Fatalf("runSync(--produce, no services) exit = %d, want 0; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "no services") {
		t.Errorf("stdout = %q, want a disclosed \"no services\" notice", stdout.String())
	}

	got := readProducedVerdicts(t, root)
	if len(got) != 0 {
		t.Errorf("verdicts = %+v, want none (no service to bind evidence to)", got)
	}
	dir := filepath.Join(root, ".verdi", "data", "derived", "spec--stale-decline", testCommit)
	testsData, err := os.ReadFile(filepath.Join(dir, "tests.json"))
	if err != nil {
		t.Fatalf("reading tests.json: %v", err)
	}
	if strings.Contains(string(testsData), "null") {
		t.Errorf("tests.json = %s, want no null fields (bundle.Assemble's never-null contract)", testsData)
	}
}

// TestRunSync_CI_PullsBundle proves plain `sync` (no --or-regen) pulls the
// bundle through the forge port and marks it materialized with source: ci
// already baked in (the forge just returns bytes a CI run already
// assembled with that provenance) — never touching the Runner at all.
//
// guide-claim: 7.2-ci-evidence-bundles
func TestRunSync_CI_PullsBundle(t *testing.T) {
	root := buildTestStore(t)
	f := fake.New()
	// The fetched artifact is the whole derived subtree CI uploaded, keyed
	// relative to data/derived/ — here the per-spec bundle for stale-decline.
	f.SeedBundle(testRef, testCommit, forgepkg.DerivedTree{
		"spec--stale-decline/" + testCommit + "/verdicts.json":      readCannedFile(t, bundleGoldenDir, "verdicts.json"),
		"spec--stale-decline/" + testCommit + "/tests.json":         readCannedFile(t, bundleGoldenDir, "tests.json"),
		"spec--stale-decline/" + testCommit + "/review.json":        readCannedFile(t, bundleGoldenDir, "review.json"),
		"spec--stale-decline/" + testCommit + "/boundary-diff.json": readCannedFile(t, bundleGoldenDir, "boundary-diff.json"),
	})

	var stdout, stderr bytes.Buffer
	deps := syncDeps{
		Runner: upstream.NewFakeRunner(), // never called: CI path never execs the toolchain
		Forge:  f,
		GoTest: fakeGoTest{},
		Stdout: &stdout,
		Stderr: &stderr,
	}

	code := runSync(context.Background(), root, testRef, testCommit, false, false, false, deps)
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

// seedGoldenTree builds the derived tree TestRunSync_CI_PullsBundle seeds —
// the per-spec golden bundle for stale-decline — plus, when toolchain is
// non-nil, a toolchain.json carrying those exact bytes, so the tool-pin
// intake tests below can vary only the provenance carrier.
func seedGoldenTree(t *testing.T, toolchain []byte) forgepkg.DerivedTree {
	t.Helper()
	tree := forgepkg.DerivedTree{
		"spec--stale-decline/" + testCommit + "/verdicts.json":      readCannedFile(t, bundleGoldenDir, "verdicts.json"),
		"spec--stale-decline/" + testCommit + "/tests.json":         readCannedFile(t, bundleGoldenDir, "tests.json"),
		"spec--stale-decline/" + testCommit + "/review.json":        readCannedFile(t, bundleGoldenDir, "review.json"),
		"spec--stale-decline/" + testCommit + "/boundary-diff.json": readCannedFile(t, bundleGoldenDir, "boundary-diff.json"),
	}
	if toolchain != nil {
		tree["spec--stale-decline/"+testCommit+"/toolchain.json"] = toolchain
	}
	return tree
}

// runSeededFetch drives runSync's plain fetch path (no --or-regen, no
// --produce) against a fake forge seeded with tree, returning the exit
// code and both output streams.
func runSeededFetch(t *testing.T, tree forgepkg.DerivedTree) (code int, stdout, stderr *bytes.Buffer) {
	t.Helper()
	root := buildTestStore(t)
	f := fake.New()
	f.SeedBundle(testRef, testCommit, tree)
	stdout, stderr = &bytes.Buffer{}, &bytes.Buffer{}
	deps := syncDeps{
		Runner: upstream.NewFakeRunner(), // never called: the CI-pull path never execs
		Forge:  f,
		GoTest: fakeGoTest{},
		Stdout: stdout,
		Stderr: stderr,
	}
	code = runSync(context.Background(), root, testRef, testCommit, false, false, false, deps)
	return code, stdout, stderr
}

// TestRunSync_CIFetch_ToolPin covers the I-4 secondary defense at
// fetched-bundle intake (spec/forge-transport ac-4/dc-4): a fetched
// toolchain.json whose recorded tool pseudo-version was built from
// verdi.yaml's pinned toolchain.commit is accepted silently; a mismatch is
// an operational refusal (exit 2) naming both the recorded tool string and
// the pinned commit; an ABSENT carrier (pre-carrier bundle, or the
// self-hosted producer, which runs no upstream tool) is a disclosed-
// unproven stdout notice — never a silent pass, never a spurious refusal;
// and a malformed carrier fails strict decode (operational).
// buildTestStore pins toolchain.commit cd38b1a56bb7deadbeef... — the same
// commit the canned graph's tool pseudo-version
// (v0.0.0-20260707202836-cd38b1a56bb7) truncates to 12 hex chars.
func TestRunSync_CIFetch_ToolPin(t *testing.T) {
	const pinnedTool = "v0.0.0-20260707202836-cd38b1a56bb7"
	const mismatchedTool = "v0.0.0-20260707202836-ffffffffffff"
	const pinnedCommit = "cd38b1a56bb7deadbeefdeadbeefdeadbeefdead"

	t.Run("matching pin accepted without notice", func(t *testing.T) {
		code, stdout, stderr := runSeededFetch(t, seedGoldenTree(t, []byte(`{"tool":"`+pinnedTool+`"}`+"\n")))
		if code != 0 {
			t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr.String())
		}
		if strings.Contains(stdout.String(), "disclosed-unproven") {
			t.Errorf("stdout = %q: a bundle WITH a matching toolchain.json must not print the disclosed-unproven notice", stdout.String())
		}
	})

	t.Run("mismatched pin refused naming both commits", func(t *testing.T) {
		code, _, stderr := runSeededFetch(t, seedGoldenTree(t, []byte(`{"tool":"`+mismatchedTool+`"}`+"\n")))
		if code != 2 {
			t.Fatalf("exit = %d, want 2 (operational refusal); stderr=%s", code, stderr.String())
		}
		if !strings.Contains(stderr.String(), mismatchedTool) {
			t.Errorf("stderr = %q, want the recorded tool string %q named", stderr.String(), mismatchedTool)
		}
		if !strings.Contains(stderr.String(), pinnedCommit) {
			t.Errorf("stderr = %q, want the pinned commit %q named", stderr.String(), pinnedCommit)
		}
	})

	t.Run("absent carrier is a disclosed-unproven notice, not a refusal", func(t *testing.T) {
		code, stdout, stderr := runSeededFetch(t, seedGoldenTree(t, nil))
		if code != 0 {
			t.Fatalf("exit = %d, want 0 (a pre-carrier bundle is never refused); stderr=%s", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "disclosed-unproven") {
			t.Errorf("stdout = %q, want the disclosed-unproven tool-pin notice (ac-4: never a silent skip)", stdout.String())
		}
	})

	t.Run("malformed carrier fails strict decode", func(t *testing.T) {
		code, _, stderr := runSeededFetch(t, seedGoldenTree(t, []byte(`{"tool":"`+pinnedTool+`","extra":1}`+"\n")))
		if code != 2 {
			t.Fatalf("exit = %d, want 2 (strict decode failure is operational); stderr=%s", code, stderr.String())
		}
		if !strings.Contains(stderr.String(), "toolchain.json") {
			t.Errorf("stderr = %q, want the failing file named", stderr.String())
		}
	})
}

// TestRunSync_CIFetch_ReachableByReaderFold is the true-closure keying
// regression test (Part 1's "B" test): a bundle FETCHED through the forge
// port and written by `verdi sync` — pulled while checked out on a GENUINE
// git branch whose slug differs from the spec ref — must land at exactly the
// per-spec key every fold reader looks under, and a reader fold (the exact
// evidence.LoadRecords + evidence.Fold every gate wraps) must reach it and
// fold the story to eligible.
//
// Before the fix this could never pass: sync wrote the fetched bundle to
// derived/RefSlug(gitRef)/ (here build--close-fixture) while readers read
// derived/RefSlug(spec.id)/ (spec--close-fixture) — disjoint keys — and the
// port collapsed the multi-verdicts.json artifact to a single bundle, or
// errored on the duplicate.
func TestRunSync_CIFetch_ReachableByReaderFold(t *testing.T) {
	repo := buildCloseFixtureRepo(t)
	ctx := context.Background()
	const specRef = "spec/close-fixture"

	// The authoritative (source: ci) records a genuine verdi-evidence CI
	// run would have produced and uploaded, keyed per spec — assembled
	// through the real self-hosted producer path so they are byte-for-byte
	// what CI serves.
	prov := artifact.EvidenceProvenance{Source: artifact.SourceCI, Pipeline: "913", Job: "7", Commit: repo.Head}
	bySpec, err := selfHostedEvidence(repo.Dir, prov)
	if err != nil {
		t.Fatalf("selfHostedEvidence: %v", err)
	}
	if len(bySpec[specRef]) == 0 {
		t.Fatalf("fixture produced no records for %s (bindings: %v)", specRef, bySpec)
	}

	// Build the derived tree exactly as CI uploads it: one per-spec subdir
	// per bound spec (keyed by spec ref) PLUS a branch-keyed whole-branch
	// bundle. The branch ref is deliberately distinct from every spec ref.
	branchRef := "build/close-fixture"
	tree := forgepkg.DerivedTree{
		store.RefSlug(branchRef) + "/" + repo.Head + "/verdicts.json": []byte("[]\n"),
	}
	for sref, recs := range bySpec {
		data, err := canonjson.Marshal(recs)
		if err != nil {
			t.Fatalf("marshaling %s records: %v", sref, err)
		}
		tree[store.RefSlug(sref)+"/"+repo.Head+"/verdicts.json"] = data
	}

	f := fake.New()
	f.SeedBundle(branchRef, repo.Head, tree)
	deps := syncDeps{
		Runner: upstream.NewFakeRunner(), // never called: the CI-pull path never execs
		Forge:  f,
		GoTest: fakeGoTest{},
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	}

	if code := runSync(ctx, repo.Dir, branchRef, repo.Head, false, false, false, deps); code != 0 {
		t.Fatalf("runSync(fetch) exit = %d, want 0; stderr=%s", code, deps.Stderr.(*bytes.Buffer).String())
	}

	// (1) The per-spec records landed at the READER key, not the branch key.
	readerRoot := filepath.Join(repo.Dir, ".verdi", "data", "derived", store.RefSlug(specRef))
	if _, err := os.Stat(filepath.Join(readerRoot, repo.Head, "verdicts.json")); err != nil {
		t.Fatalf("fetched per-spec bundle did not land at the reader key %s: %v", readerRoot, err)
	}

	// (2) A reader fold reaches those records and folds the story eligible.
	spec, _ := readSpec(t, repo.Dir, "close-fixture")
	records, err := evidence.LoadRecords(ctx, repo.Dir, readerRoot, repo.Head)
	if err != nil {
		t.Fatalf("LoadRecords at the reader key: %v", err)
	}
	result, err := evidence.Fold(evidence.Input{Spec: spec, Records: records, Preview: false, StoreRoot: repo.Dir, StorySlug: store.RefSlug(spec.Story)})
	if err != nil {
		t.Fatalf("Fold: %v", err)
	}
	if !result.Eligible {
		t.Errorf("reader fold over the forge-fetched bundle: eligible=false, want true (loaded %d authoritative records) — the pulled evidence was NOT reachable by the fold", len(records))
	}
}

// TestRunSync_ForceLocalRecords_IgnoredByAuthoritativeFold proves Part 2's
// trust floor with a witness: records a local (--force-local) --produce run
// wrote are stamped source: local and are therefore INVISIBLE to an
// authoritative fold (Preview false) — only a --preview fold sees them. A
// locally fabricated bundle can never fold as trusted.
func TestRunSync_ForceLocalRecords_IgnoredByAuthoritativeFold(t *testing.T) {
	repo := buildCloseFixtureRepo(t)
	ctx := context.Background()
	const specRef = "spec/close-fixture"

	// A local --produce run stamps source: local (Part 2). Exercise that
	// exact stamp via the producer with a local provenance.
	prov := artifact.EvidenceProvenance{Source: artifact.SourceLocal, Commit: repo.Head}
	if err := produceSelfHostedEvidence(repo.Dir, repo.Head, prov); err != nil {
		t.Fatalf("produceSelfHostedEvidence(local): %v", err)
	}

	spec, _ := readSpec(t, repo.Dir, "close-fixture")
	derivedRoot := filepath.Join(repo.Dir, ".verdi", "data", "derived", store.RefSlug(specRef))
	records, err := evidence.LoadRecords(ctx, repo.Dir, derivedRoot, repo.Head)
	if err != nil {
		t.Fatalf("LoadRecords: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected the local records to be on disk (LoadRecords returns both sources)")
	}

	// Authoritative fold (Preview false): the source: local records are
	// ignored, so the story cannot be eligible from them.
	auth, err := evidence.Fold(evidence.Input{Spec: spec, Records: records, Preview: false, StoreRoot: repo.Dir, StorySlug: store.RefSlug(spec.Story)})
	if err != nil {
		t.Fatalf("authoritative Fold: %v", err)
	}
	if auth.Eligible {
		t.Error("authoritative fold folded local (advisory) records as evidence — the trust boundary leaked")
	}

	// Preview fold: the same advisory records ARE folded in, proving they
	// were written correctly and are only gated by provenance, not absent.
	preview, err := evidence.Fold(evidence.Input{Spec: spec, Records: records, Preview: true, StoreRoot: repo.Dir, StorySlug: store.RefSlug(spec.Story)})
	if err != nil {
		t.Fatalf("preview Fold: %v", err)
	}
	if !preview.Eligible {
		t.Errorf("preview fold: eligible=false, want true (the advisory records should satisfy the ACs under --preview)")
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

	code := runSync(context.Background(), root, testRef, testCommit, false, false, false, deps)
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
	fr.Enqueue("groundwork", "diff", upstream.Result{ExitCode: 0}) // no boundary-contract change in this scenario

	var stdout, stderr bytes.Buffer
	deps := syncDeps{
		Runner: fr,
		Forge:  fake.New(),
		GoTest: fakeGoTest{output: []byte(svcfixGoTestJSON)},
		Stdout: &stdout,
		Stderr: &stderr,
	}

	code := runSync(context.Background(), root, testRef, testCommit, true, false, false, deps)
	if code != 1 {
		t.Fatalf("runSync with a BLOCK review: exit = %d, want 1; stderr=%s", code, stderr.String())
	}
}

// TestRunSync_Negative_BoundaryDiffCrossCheckDisagreement proves the I-3
// cross-check between verdi's own computed boundary-diff breaking verdict
// and `groundwork diff`'s exit code is actually wired into the regen path
// (not just unit-tested in isolation): if the fake `groundwork diff` here
// disagrees with the (non-breaking) route addition
// boundaryWriteRunner simulates, regeneration fails loudly rather than
// silently trusting its own computation.
func TestRunSync_Negative_BoundaryDiffCrossCheckDisagreement(t *testing.T) {
	root := buildTestStore(t)
	fr := upstream.NewFakeRunner()
	fr.Enqueue("flowmap", "graph", upstream.Result{Stdout: readCannedFile(t, cannedSrcDir, "graph.json"), ExitCode: 0})
	fr.Enqueue("flowmap", "boundary", upstream.Result{ExitCode: 0})
	fr.Enqueue("groundwork", "review", upstream.Result{Stdout: readCannedFile(t, cannedSrcDir, "review-structurally-clear.json"), ExitCode: 0})
	// Disagreement: the route addition is non-breaking, but this fake
	// `groundwork diff` claims exit 1 (breaking).
	fr.Enqueue("groundwork", "diff", upstream.Result{ExitCode: 1})

	runner := boundaryWriteRunner{
		Runner:         fr,
		svcDir:         filepath.Join(root, "svcfix"),
		branchContract: readCannedFile(t, cannedSrcDir, "boundary-contract-branch.json"),
	}

	var stdout, stderr bytes.Buffer
	deps := syncDeps{
		Runner: runner,
		Forge:  fake.New(),
		GoTest: fakeGoTest{output: []byte(svcfixGoTestJSON)},
		Stdout: &stdout,
		Stderr: &stderr,
	}

	code := runSync(context.Background(), root, testRef, testCommit, true, false, false, deps)
	if code != 2 {
		t.Fatalf("runSync with a boundary-diff cross-check disagreement: exit = %d, want 2; stderr=%s", code, stderr.String())
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
	code := runSync(context.Background(), root, testRef, testCommit, true, false, false, deps)
	if code != 2 {
		t.Fatalf("runSync with a forge error: exit = %d, want 2", code)
	}
}

// erroringForge is a minimal forgepkg.Forge whose FetchEvidenceBundle
// always fails with a plain (non-ErrNoBundle) error, to prove runSync
// treats that as operational regardless of --or-regen.
type erroringForge struct{}

func (erroringForge) FetchEvidenceBundle(ctx context.Context, ref, commit string) (forgepkg.DerivedTree, error) {
	return nil, errors.New("forge: simulated transport failure")
}
func (erroringForge) GeneratedAttribute() string { return "x-generated" }
func (erroringForge) CIContext(ctx context.Context) (forgepkg.CIInfo, error) {
	return forgepkg.CIInfo{}, errors.New("forge: simulated CIContext failure")
}
func (erroringForge) ListOpenMRs(ctx context.Context, targetBranch string) ([]forgepkg.OpenMR, error) {
	return nil, nil
}
func (erroringForge) FetchFileAtRef(ctx context.Context, ref, path string) ([]byte, error) {
	return nil, errors.New("forge: simulated transport failure")
}
func (erroringForge) ListComments(ctx context.Context, mrID string) ([]forgepkg.Comment, error) {
	return nil, nil
}
func (erroringForge) PostComment(ctx context.Context, mrID, body string, target *forgepkg.CommentTarget) (forgepkg.Comment, error) {
	return forgepkg.Comment{}, errors.New("forge: simulated transport failure")
}
func (erroringForge) GetThreadResolution(ctx context.Context, mrID string) ([]forgepkg.ThreadResolution, error) {
	return nil, nil
}

var _ forgepkg.Forge = erroringForge{}

// TestGithubIdentifier_CIEnvWinsOverResolvableOrigin proves spec/sync-
// local-flow ac-1's regression obligation with a REAL git remote in the
// loop, not just a hardcoded URL string (TestGithubOwnerRepo's own
// register, sync_helpers_test.go): the explicit CI env still wins
// byte-identically over a resolvable origin remote read from an actual
// fixturegit repository via gitx.RemoteURL — the same read cmdSync
// performs — proving the env-wins behavior end to end from git plumbing
// through to githubOwnerRepo's resolution, not merely that SOME
// identifier happens to resolve either way.
func TestGithubIdentifier_CIEnvWinsOverResolvableOrigin(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"a.txt": "1\n"}, Message: "layer 1"},
	})
	runGitCmd(t, repo.Dir, "remote", "add", "origin", "https://github.com/urlowner/urlrepo.git")

	remoteURL, err := gitx.RemoteURL(context.Background(), repo.Dir, "origin")
	if err != nil {
		t.Fatalf("gitx.RemoteURL: %v", err)
	}
	if remoteURL == "" {
		t.Fatal("gitx.RemoteURL returned empty for a configured origin remote")
	}

	t.Setenv("GITHUB_REPOSITORY_OWNER", "envowner")
	t.Setenv("GITHUB_REPOSITORY", "envowner/envrepo")

	owner, repoName, err := githubOwnerRepo(remoteURL, nil)
	if err != nil {
		t.Fatalf("githubOwnerRepo(%q): %v", remoteURL, err)
	}
	if owner != "envowner" || repoName != "envrepo" {
		t.Errorf("githubOwnerRepo(%q) = (%q, %q), want (envowner, envrepo) — the CI env must win byte-identically over a resolvable origin remote", remoteURL, owner, repoName)
	}
}

// TestCmdSync_LocalCheckout_RefusesNamingSources drives cmdSync — the
// real dispatch entry point `main` invokes, never runSync's testable core
// directly — over a scratch fixturegit repo with no CI environment
// variables set and no configured origin remote, proving spec/sync-
// local-flow ac-1's refusal: exit 2, naming every source tried. This
// built-binary register is deliberately chosen for the refusal path
// because it resolves and refuses BEFORE any network dial (co-1): a
// genuine successful forge round-trip stays proven only at the
// hermetic-fake integration register (this file's other Test* functions),
// never by letting this e2e test's built-binary entry point actually dial
// out.
func TestCmdSync_LocalCheckout_RefusesNamingSources(t *testing.T) {
	for _, v := range []string{
		"GITHUB_REPOSITORY_OWNER", "GITHUB_REPOSITORY", "GITHUB_TOKEN",
		"CI_COMMIT_REF_NAME", "GITHUB_HEAD_REF", "GITHUB_REF_NAME",
	} {
		t.Setenv(v, "")
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml": "schema: verdi.layout/v1\nforge: github\n",
		},
		Message: "store init",
	}})
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	code := cmdSync(nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("cmdSync in a local checkout with no CI env / no origin: exit = %d, want 2; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{"GITHUB_REPOSITORY_OWNER", "GITHUB_REPOSITORY", "origin"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr = %q, want it to name %q", stderr.String(), want)
		}
	}
}

// TestCmdSync_GitlabLocalCheckout_RefusesNamingProjectID is the gitlab
// counterpart to TestCmdSync_LocalCheckout_RefusesNamingSources (ADJ-69):
// it drives cmdSync over a scratch fixturegit repo whose manifest names
// `forge: gitlab` (DetectKind reaches gitlab with no CI env at all — the
// manifest wins) and with CI_PROJECT_ID unset, proving that a non-produce
// sync refuses with exit 2, naming CI_PROJECT_ID, BEFORE any network dial.
// The refusal lands at the buildForge construction seam, upstream of
// fetchAncestorBundle's FetchEvidenceBundle, so there is no
// gitlab.com/api/v4/projects//... egress with an empty :id — deb0dd3's own
// "only-refusing on branches that dial" rule, previously pinned for github
// alone.
func TestCmdSync_GitlabLocalCheckout_RefusesNamingProjectID(t *testing.T) {
	for _, v := range []string{
		"CI_PROJECT_ID", "CI_API_V4_URL", "CI_JOB_TOKEN",
		"GITHUB_REPOSITORY_OWNER", "GITHUB_REPOSITORY", "GITHUB_TOKEN",
		"CI_COMMIT_REF_NAME", "GITHUB_HEAD_REF", "GITHUB_REF_NAME",
	} {
		t.Setenv(v, "")
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml": "schema: verdi.layout/v1\nforge: gitlab\n",
		},
		Message: "store init",
	}})
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	code := cmdSync(nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("cmdSync in a gitlab local checkout with no CI_PROJECT_ID: exit = %d, want 2; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "CI_PROJECT_ID") {
		t.Errorf("stderr = %q, want it to name %q as the missing identity source", stderr.String(), "CI_PROJECT_ID")
	}
}

// cmdSyncProduceStore builds an env-less, origin-less store for forgeKind
// ("github" or "gitlab") as a real git repo (fixturegit) with a toolchain
// block but NO services, so a --produce/--produce-runtime run completes
// hermetically (an empty bundle, no upstream exec). It is the cmdSync
// register the below-the-seam fake-forge tests are blind to: cmdSync builds
// the REAL forge, so this is where the "no identifier refusal on paths that
// never dial" fixes — ADJ-43 (github) and ADJ-69 (gitlab) — are observable.
func cmdSyncProduceStore(t *testing.T, forgeKind string) {
	t.Helper()
	for _, v := range []string{
		"GITHUB_REPOSITORY_OWNER", "GITHUB_REPOSITORY", "GITHUB_TOKEN",
		"CI_PROJECT_ID", "CI_API_V4_URL", "CI_JOB_TOKEN",
		"CI_COMMIT_REF_NAME", "GITHUB_HEAD_REF", "GITHUB_REF_NAME",
	} {
		t.Setenv(v, "")
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml": "schema: verdi.layout/v1\nforge: " + forgeKind + "\ntoolchain:\n  module: github.com/jyang234/golang-code-graph\n  commit: cd38b1a56bb782177a207d741a39807821cf2c1c\n",
		},
		Message: "store init",
	}})
	t.Chdir(repo.Dir)
}

// TestCmdSync_ProduceForceLocal_CompletesWithoutIdentifierRefusal proves
// ADJ-43's fix at the cmdSync register: `verdi sync --produce --force-local`
// in an env-less, origin-less GitHub checkout never dials the forge (it only
// reads the CI environment via CIContext, a pure env read), so it must NOT
// hit the ac-1 identifier refusal — it completes exactly as it did before
// ac-1 (co-3 byte-identity). The pre-existing --produce tests inject a fake
// forge below the buildForge seam and so never observed this narrowing.
func TestCmdSync_ProduceForceLocal_CompletesWithoutIdentifierRefusal(t *testing.T) {
	cmdSyncProduceStore(t, "github")

	var stdout, stderr bytes.Buffer
	code := cmdSync([]string{"--produce", "--force-local"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("cmdSync(--produce --force-local) in an env-less/origin-less checkout: exit = %d, want 0 (completes as pre-ac-1); stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if strings.Contains(stderr.String(), "cannot identify the GitHub repository") {
		t.Errorf("stderr = %q, must NOT hit the ac-1 identifier refusal — --produce never dials the forge (ADJ-43)", stderr.String())
	}
}

// TestCmdSync_ProduceRuntimeForceLocal_CompletesWithoutIdentifierRefusal is
// the --produce-runtime variant: run with no --story/--ac it is the
// scheduled no-op ("nothing to report", exit 0), but it must still get PAST
// forge construction without the ac-1 identifier refusal — --produce-runtime
// only reads CIContext (nil-guarded), it never dials (ADJ-43).
func TestCmdSync_ProduceRuntimeForceLocal_CompletesWithoutIdentifierRefusal(t *testing.T) {
	cmdSyncProduceStore(t, "github")

	var stdout, stderr bytes.Buffer
	code := cmdSync([]string{"--produce-runtime", "--force-local"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("cmdSync(--produce-runtime --force-local) in an env-less/origin-less checkout: exit = %d, want 0 (scheduled no-op completes); stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if strings.Contains(stderr.String(), "cannot identify the GitHub repository") {
		t.Errorf("stderr = %q, must NOT hit the ac-1 identifier refusal — --produce-runtime never dials the forge (ADJ-43)", stderr.String())
	}
}

// TestCmdSync_GitlabProduceForceLocal_CompletesWithoutIdentifierRefusal is
// the gitlab counterpart proving ADJ-69's refusal does NOT leak onto the
// non-dialing --produce path: `verdi sync --produce --force-local` over a
// gitlab store with CI_PROJECT_ID unset builds an identifier-tolerant forge
// (buildForgeForCI, requireIdentifier=false) and completes exactly as it did
// before ADJ-69 — the empty project id is harmless where nothing dials
// (ADJ-43). It must reach exit 0 without hitting the gitlab identifier
// refusal, proving the requireIdentifier=false path stayed byte-identical.
func TestCmdSync_GitlabProduceForceLocal_CompletesWithoutIdentifierRefusal(t *testing.T) {
	cmdSyncProduceStore(t, "gitlab")

	var stdout, stderr bytes.Buffer
	code := cmdSync([]string{"--produce", "--force-local"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("cmdSync(--produce --force-local) in an env-less/origin-less gitlab checkout: exit = %d, want 0 (completes as pre-ADJ-69); stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if strings.Contains(stderr.String(), "cannot identify the GitLab project") {
		t.Errorf("stderr = %q, must NOT hit the ADJ-69 identifier refusal — --produce never dials the forge (ADJ-43)", stderr.String())
	}
}
