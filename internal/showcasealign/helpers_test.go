// Package showcasealign is `make lint-showcase` and `make
// showcase-coverage`'s home (docs/design/plans/2026-07-14-public-rollout-
// plan.md, workspace root, Phase 3 — story spec/showcase-drift-gate): the
// mechanical drift gate that keeps examples/showcase lint-clean and every
// shipped capability showcase-backed, mirroring internal/specalign's own
// self-hosting discipline but pointed at the showcase corpus instead of
// this repo's own .verdi/ store.
//
// It is a test-only package (every .go file here is a _test.go file,
// following internal/specalign's, internal/corpus's, and
// internal/svcfixcanned's own precedent) run via `go test
// ./internal/showcasealign/...`.
//
// As of Task 3.1 (this file + lintclean_test.go) this package provides:
//
//   - TestMain / verdiRepoRoot / verdiBinPath / runBinary: the
//     build-the-real-binary-once-then-exec harness, mirroring
//     internal/specalign/helpers_test.go line for line (same rationale:
//     exec the exact binary CI ships, never `go run`, which swallows
//     child exit codes — the phase-1 defect PLAN.md's exit criteria
//     comment records).
//   - provisionShowcaseStore: builds a real, git-backed showcase store —
//     see its own doc comment for exactly which construction it uses and
//     why.
//   - TestShowcaseLintClean (lintclean_test.go): `verdi lint` exits 0
//     against the provisioned store.
//
// TestShowcaseCoverage (Task 3.2) and TestReadmeExamplesFresh (Task 4.2)
// are later commits in this same story/feature; they are not yet part of
// this package, and `provisionShowcaseStore`/`runBinary` exist now
// specifically so those tasks can consume them without re-deriving the
// store construction.
package showcasealign

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
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
// exec against — build-then-exec, matching internal/specalign's own
// convention (never `go run`, which swallows child exit codes).
func TestMain(m *testing.M) {
	root, err := computeVerdiRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "showcasealign: TestMain: resolving verdi root:", err)
		os.Exit(2)
	}
	verdiRepoRoot = root

	tmp, err := os.MkdirTemp("", "verdi-showcasealign-bin-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "showcasealign: TestMain: mkdtemp:", err)
		os.Exit(2)
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	verdiBinPath = filepath.Join(tmp, "verdi")
	cmd := exec.Command("go", "build", "-o", verdiBinPath, "./cmd/verdi")
	cmd.Dir = verdiRepoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "showcasealign: TestMain: building verdi binary: %v\n%s\n", err, out)
		os.Exit(2)
	}

	os.Exit(m.Run())
}

// computeVerdiRoot resolves the verdi module root from THIS file's own
// path, recorded at compile time by runtime.Caller — robust regardless of
// the test binary's cwd or how `go test` was invoked. Twin of
// internal/specalign/helpers_test.go's computeVerdiRoot.
func computeVerdiRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("runtime.Caller(0) failed")
	}
	// this file lives at <verdiRoot>/internal/showcasealign/helpers_test.go
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return "", fmt.Errorf("resolving verdi root from %s: %w", file, err)
	}
	return root, nil
}

// runBinary execs the once-built verdi binary with args, cwd=dir,
// capturing stdout/stderr separately and returning the process exit code
// (0 on success). A launch failure that is NOT an ExitError (binary
// missing, permissions, ...) is a test infrastructure failure, not a
// verb-behavior result, so it fails the calling test outright. Same
// contract as internal/specalign/helpers_test.go's runBinary.
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

// --- Showcase store construction ---
//
// provisionShowcaseStore(t) builds a real, git-backed showcase store and
// returns its root directory (a real working tree with .verdi/verdi.yaml
// at its root — store.FindRoot resolves it directly). Two constructions
// were evaluated for this task, by actually running each and inspecting
// `verdi lint`'s exit code and findings, per the task's own instruction
// to decide by running rather than by assumption:
//
//  1. A single-commit copy of examples/showcase/.verdi (the shape
//     cmd/e2eharness/provision.go's provisionStore builds, for e2e
//     browser tests that never assert lint-cleanliness). RULED OUT: the
//     showcase corpus's frozen artifacts (specs, ADRs, obligations, ...)
//     pin `frozen.commit` / predecessor object refs at specific
//     fixturegit commit SHAs (internal/corpus/corpus_test.go's
//     goldenHeads / goldenHeadsV2) that a single fresh commit cannot
//     contain as ancestors, so pin-resolution rules (VL-009, VL-003 via
//     the impacts/service-root git-history lookups, VL-015's
//     predecessor-object `git show <sha>:<path>` lookups) fail
//     structurally, independent of content quality. This is exactly the
//     38-finding failure 08-revision-notes.md's "Public rollout —
//     showcase ac-1 evidence" entry records for this same shape.
//  2. The fixturegit-history reconstruction internal/lint already proves
//     clean: internal/lint/harness_test.go's buildLintRepo (the v0
//     corpus, examples/showcase/layers.txt, via fixturegit — proved
//     clean by TestClean_CorpusLintsGreen) reconciled with
//     internal/lint/v2clean_test.go's buildV2FixtureCorpusRepo (the same
//     v0 layers PLUS the rung-4 loan-workflow/loan-workflow-v2
//     supersession pair and the escrow-autopay/borrower-update-* cluster
//     chained on top, each frozen SHA recomputed for THIS chain and
//     substituted in place of the golden literal — proved clean by
//     TestV2FixtureCorpus_LintsClean). 08-revision-notes.md's "Public
//     rollout" entry (2026-07-15, ADJ-16 class) ratifies this second
//     construction as showcase-corpus-renovation's ac-1 behavioral
//     evidence.
//
// CONSTRUCTION #2 WON (confirmed by running TestShowcaseLintClean to a
// PASS against the store buildShowcaseRepo below produces). It
// replicates buildV2FixtureCorpusRepo — which itself calls
// harness_test.go's parseCorpusLayers/setupLayer/writeLoansvcFixture/
// provisionMutableZone — byte-for-byte, because symbols defined in
// another package's _test.go files cannot be imported (the same
// constraint cmd/e2eharness/provision.go's own doc comment names for its
// ~20-line replica of committed-zone provisioning). If this replica and
// its internal/lint twins (harness_test.go, v2clean_test.go) ever
// diverge in behavior, that is a defect in one or the other.
//
// Known, deliberate gaps (matching the twins exactly, not a shortfall
// introduced here): rate-lock/rate-lock-v2 — a THIRD, still-independent
// fixturegit history (internal/artifact/v2fixture_test.go's goldenShaC/
// goldenShaD, chained after the same loan-workflow layers but never
// reconciled into a lint-clean single-repo proof by any existing test) —
// are absent from the built store, as is
// borrower-update-mobile/deviation-report.md (present on disk, but not
// part of buildV2FixtureCorpusRepo's own layerC either). The mutable zone
// is present-but-empty and no derived zone is written at all
// (buildLintRepo's own convention: VL-017 is the only rule that reads the
// mutable zone, and it keys off zone PRESENCE not content; no rule reads
// the derived zone). A future task needing real mutable/derived content,
// the rate-lock pair, or the deviation report must layer that in
// separately — disclosed here, not silently assumed.
func provisionShowcaseStore(t *testing.T) (storeRoot string) {
	t.Helper()
	return buildShowcaseRepo(t).Dir
}

// showcaseDir is examples/showcase, anchored at the repo root TestMain
// already resolved. internal/lint's own corpusDir constant is a
// "../../examples/showcase" relative literal (safe there because `go
// test` happens to set cwd to the package directory); this package
// anchors on verdiRepoRoot instead purely because every other helper here
// already needs it for building the binary — same target directory
// either way.
func showcaseDir() string {
	return filepath.Join(verdiRepoRoot, "examples", "showcase")
}

// setupGitAttributes is the repository-root .gitattributes VL-012
// requires (02 §Repository plumbing's literal example, gitlab-generated
// token to match forge: gitlab) — byte-identical to
// examples/showcase's own committed .gitattributes, which layers.txt does
// not track (it lists .verdi/ content only). Twin of
// internal/lint/harness_test.go's setupGitAttributes.
const setupGitAttributes = `.verdi/specs/*/*/board.json          gitlab-generated
.verdi/specs/*/*/rollup.json         gitlab-generated
.verdi/specs/*/*/deviation-report.md gitlab-generated
`

// loansvcFlowmapYAML and loansvcBoundaryContractJSON satisfy
// examples/showcase's own `impacts: { ref: svc/loansvc/boundary-contract }`
// link (stale-decline/spec.md). Twins of internal/lint/harness_test.go's
// same-named constants.
const loansvcFlowmapYAML = "version: 1\nservice: loansvc\n"
const loansvcBoundaryContractJSON = `{
  "service": "loansvc",
  "schema_version": "flowmap.boundary/v1",
  "entrypoints": { "http": [], "consumers": [] },
  "published": [],
  "consumed": [],
  "external_dependencies": [],
  "blind_spots": []
}
`

// parseCorpusLayers reads examples/showcase/layers.txt and returns, in
// ascending layer order, each layer's corpus-relative file paths as
// fixturegit Layers. Twin of internal/lint/harness_test.go's
// parseCorpusLayers (also internal/corpus/corpus_test.go's parseLayers).
func parseCorpusLayers(t *testing.T) []fixturegit.Layer {
	t.Helper()
	dir := showcaseDir()
	f, err := os.Open(filepath.Join(dir, "layers.txt"))
	if err != nil {
		t.Fatalf("opening layers.txt: %v", err)
	}
	defer func() { _ = f.Close() }()

	filesByLayer := map[int][]string{}
	var order []int
	seen := map[int]bool{}

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			t.Fatalf("layers.txt: malformed line %q", line)
		}
		var n int
		if _, err := fmt.Sscanf(parts[0], "%d", &n); err != nil {
			t.Fatalf("layers.txt: bad layer number in %q: %v", line, err)
		}
		rel := strings.TrimSpace(parts[1])
		filesByLayer[n] = append(filesByLayer[n], rel)
		if !seen[n] {
			order = append(order, n)
			seen[n] = true
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scanning layers.txt: %v", err)
	}
	sort.Ints(order)

	layers := make([]fixturegit.Layer, 0, len(order))
	for _, n := range order {
		files := map[string]string{}
		for _, rel := range filesByLayer[n] {
			data, err := os.ReadFile(filepath.Join(dir, rel))
			if err != nil {
				t.Fatalf("reading corpus file %s (layer %d): %v", rel, n, err)
			}
			files[rel] = string(data)
		}
		layers = append(layers, fixturegit.Layer{Files: files, Message: fmt.Sprintf("layer %d", n)})
	}
	return layers
}

// setupLayer is the fixed layer buildShowcaseRepo adds on top of the
// committed corpus: it carries ONLY the root .gitattributes (byte-
// identical to examples/showcase's committed .gitattributes). The store
// manifest is deliberately NOT re-added here — the committed
// .verdi/verdi.yaml that layers.txt already carries as layer 1 stands
// unmodified. Twin of internal/lint/harness_test.go's setupLayer.
func setupLayer() fixturegit.Layer {
	return fixturegit.Layer{
		Files: map[string]string{
			".gitattributes": setupGitAttributes,
		},
		Message: "lint test setup: root .gitattributes",
	}
}

// writeLoansvcFixture writes the untracked "loansvc" service discovery
// fixture into root's working tree (see loansvcFlowmapYAML's doc
// comment). Twin of internal/lint/harness_test.go's writeLoansvcFixture.
func writeLoansvcFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, filepath.Join(root, "loansvc", ".flowmap.yaml"), loansvcFlowmapYAML)
	writeTestFile(t, filepath.Join(root, "loansvc", ".flowmap", "boundary-contract.json"), loansvcBoundaryContractJSON)
}

// writeTestFile writes content to path, creating parent directories as
// needed, failing the test on error.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// provisionMutableZone creates an empty (present, but recording nothing)
// data/mutable/annotations/ directory in root's untracked working tree —
// the default posture every buildLintRepo-style test repo gets (01
// §Zones: the mutable zone is per-checkout, never committed). Twin of
// internal/lint/harness_test.go's provisionMutableZone.
func provisionMutableZone(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "data", "mutable", "annotations")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("provisioning mutable zone %s: %v", dir, err)
	}
}

// frozenLineRe and draftVariant strip a frozen: line and flip
// status: accepted-pending-build to status: draft, producing the
// pre-freeze draft content fixturegit needs to build the layer before the
// one that freezes it. Twins of internal/lint/v2clean_test.go's
// same-named declarations.
var frozenLineRe = regexp.MustCompile(`(?m)^frozen:.*\n`)

func draftVariant(content string) string {
	content = frozenLineRe.ReplaceAllString(content, "")
	return strings.Replace(content, "status: accepted-pending-build\n", "status: draft\n", 1)
}

// readShowcaseFile reads a examples/showcase file (relative to
// showcaseDir()). Twin of internal/lint/v2clean_test.go's
// readV2CorpusFile.
func readShowcaseFile(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(showcaseDir(), rel))
	if err != nil {
		t.Fatalf("reading corpus file %s: %v", rel, err)
	}
	return string(data)
}

// goldenShaAToken/goldenShaBToken are the golden literal SHAs
// examples/showcase's committed loan-workflow-v2/spec.md and
// reaffirmations/jira-loan-1483/ac-1.md cite (internal/artifact/
// v2fixture_test.go's goldenShaA/goldenShaB, from that package's own
// dedicated, unchained fixturegit history). Twins of
// internal/lint/v2clean_test.go's same-named constants.
const (
	goldenShaAToken = "b5117ecc69b6779ad75cde60d4aec206ece0950b"
	goldenShaBToken = "06a3f4cabb226fe9344e1645e27c344493b6b62b"
)

// buildShowcaseRepo builds the full v2-reconciled showcase corpus (v0's
// layers.txt layers + the lint-test setup layer, then the rung-4
// loan-workflow supersession pair as two more layers with their frozen
// SHAs recomputed for THIS chain, then a final layer with the
// escrow-autopay/borrower-update-* cluster and its attestation/
// reaffirmation/obligations) into one git-real repo, then overlays the
// loansvc discovery fixture and a present-but-empty mutable zone. Twin of
// internal/lint/v2clean_test.go's buildV2FixtureCorpusRepo — see
// provisionShowcaseStore's doc comment for why this construction (over a
// single-commit copy) is required, and for the known, deliberate gaps
// (rate-lock/rate-lock-v2, the borrower-update-mobile deviation report,
// real mutable/derived content) shared with that twin.
func buildShowcaseRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()

	base := parseCorpusLayers(t)
	base = append(base, setupLayer())

	v1Draft := draftVariant(readShowcaseFile(t, ".verdi/specs/active/loan-workflow/spec.md"))
	layerA := fixturegit.Layer{
		Files:   map[string]string{".verdi/specs/active/loan-workflow/spec.md": v1Draft},
		Message: "v2 corpus: loan-workflow v1 draft",
	}
	repoA := fixturegit.Build(t, append(append([]fixturegit.Layer{}, base...), layerA))
	shaA := repoA.Heads[len(repoA.Heads)-1]

	v1Frozen := strings.Replace(readShowcaseFile(t, ".verdi/specs/active/loan-workflow/spec.md"), goldenShaAToken, shaA, 1)
	v2Draft := draftVariant(readShowcaseFile(t, ".verdi/specs/active/loan-workflow-v2/spec.md"))
	layerB := fixturegit.Layer{
		Files: map[string]string{
			".verdi/specs/active/loan-workflow/spec.md":    v1Frozen,
			".verdi/specs/active/loan-workflow-v2/spec.md": v2Draft,
		},
		Message: "v2 corpus: loan-workflow v1 frozen + loan-workflow-v2 draft",
	}
	repoB := fixturegit.Build(t, append(append([]fixturegit.Layer{}, base...), layerA, layerB))
	shaB := repoB.Heads[len(repoB.Heads)-1]

	sub := func(rel string) string {
		content := readShowcaseFile(t, rel)
		content = strings.ReplaceAll(content, goldenShaAToken, shaA)
		content = strings.ReplaceAll(content, goldenShaBToken, shaB)
		return content
	}

	layerC := fixturegit.Layer{
		Files: map[string]string{
			".verdi/specs/active/loan-workflow-v2/spec.md":                  sub(".verdi/specs/active/loan-workflow-v2/spec.md"),
			".verdi/specs/active/escrow-autopay/spec.md":                    sub(".verdi/specs/active/escrow-autopay/spec.md"),
			".verdi/specs/active/escrow-autopay/layout.json":                sub(".verdi/specs/active/escrow-autopay/layout.json"),
			".verdi/specs/active/borrower-update-api/spec.md":               sub(".verdi/specs/active/borrower-update-api/spec.md"),
			".verdi/specs/active/borrower-update-mobile/spec.md":            sub(".verdi/specs/active/borrower-update-mobile/spec.md"),
			".verdi/specs/active/borrower-update-mobile-spike/spec.md":      sub(".verdi/specs/active/borrower-update-mobile-spike/spec.md"),
			".verdi/attestations/escrow-autopay/ac-1.md":                    sub(".verdi/attestations/escrow-autopay/ac-1.md"),
			".verdi/reaffirmations/jira-loan-1483/ac-1.md":                  sub(".verdi/reaffirmations/jira-loan-1483/ac-1.md"),
			".verdi/obligations/borrower-update-api/ac-1--static.md":        sub(".verdi/obligations/borrower-update-api/ac-1--static.md"),
			".verdi/obligations/borrower-update-api/ac-1--behavioral.md":    sub(".verdi/obligations/borrower-update-api/ac-1--behavioral.md"),
			".verdi/obligations/borrower-update-mobile/ac-1--static.md":     sub(".verdi/obligations/borrower-update-mobile/ac-1--static.md"),
			".verdi/obligations/borrower-update-mobile/ac-1--behavioral.md": sub(".verdi/obligations/borrower-update-mobile/ac-1--behavioral.md"),
			".verdi/obligations/borrower-update-mobile/ac-2--behavioral.md": sub(".verdi/obligations/borrower-update-mobile/ac-2--behavioral.md"),
		},
		Message: "v2 corpus: loan-workflow-v2 frozen + escrow-autopay cluster + reaffirmation + obligations",
	}

	layers := append(append([]fixturegit.Layer{}, base...), layerA, layerB, layerC)
	repo := fixturegit.Build(t, layers)
	writeLoansvcFixture(t, repo.Dir)
	provisionMutableZone(t, repo.Dir)
	return repo
}
