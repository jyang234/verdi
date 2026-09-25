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
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/jyang234/verdi/internal/evidence"
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
//
// It also owns the process-lifetime cleanup for showcaseTemplateDir (see
// ensureShowcaseTemplate below): that directory is deliberately NOT a
// t.TempDir(), since it must outlive whichever single test happens to
// trigger the one-time build, so nothing but this function ever removes
// it. Both cleanups run as plain statements AFTER m.Run() returns and
// BEFORE os.Exit — os.Exit terminates the process immediately without
// running deferred calls, so a defer registered before it (as the
// pre-existing binary-tmp-dir cleanup below used to be) never fires; this
// is why both are ordinary post-Run() statements instead.
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

	verdiBinPath = filepath.Join(tmp, "verdi")
	cmd := exec.Command("go", "build", "-o", verdiBinPath, "./cmd/verdi")
	cmd.Dir = verdiRepoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "showcasealign: TestMain: building verdi binary: %v\n%s\n", err, out)
		os.Exit(2)
	}

	code := m.Run()
	_ = os.RemoveAll(tmp)
	if showcaseTemplateDir != "" {
		_ = os.RemoveAll(showcaseTemplateDir)
	}
	os.Exit(code)
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
//
// PROVISIONED ONCE PER PROCESS (lane R3, test-speed contract): buildShowcaseRepo
// itself is expensive (3 fixturegit.Build's worth of git spawns, plus
// attachHistoricalObligationQualityAncestry's `git fetch` of the
// obligation-quality adoption commit's full ancestry). Every one of this
// package's ~29 calling tests needs its own independently mutable copy —
// several attest, waive, init, gc, or otherwise commit into the store — so
// provisionShowcaseStore below builds it for real only once
// (ensureShowcaseTemplate) and gives every caller, including the very
// first, a byte-for-byte copy of that one build (cloneShowcaseRepoDir),
// following T1's cmd/verdi/countersign_lifecycle_contract_test.go
// cloneFixtureRepoDir exactly: a full working-directory copy — objects,
// refs (the refs/replace graft included, since it is just another ref
// under .git/refs), config, HEAD, and the working tree — then `git
// update-index -q --refresh` once, up front, so the copy's stat cache
// agrees with its own (different-mtime) files and every later `git
// diff-files`/`diff-index` in that copy reports a genuinely clean tree.
// TestProvisionShowcaseStoreCopyEquivalence below is this mechanism's own
// proof: a cached clone agrees with a completely independent, uncached
// buildShowcaseRepo call on refs, objects, config, the working tree, and
// the replace graft, and a mutation in one copy is invisible to another.
func provisionShowcaseStore(t *testing.T) (storeRoot string) {
	t.Helper()
	return cloneShowcaseRepoDir(t, ensureShowcaseTemplate(t)).Dir
}

// showcaseTemplateMu guards showcaseTemplate/showcaseTemplateDir.
// ensureShowcaseTemplate builds the store at most once per test process; a
// plain Mutex is used rather than sync.Once because buildShowcaseRepo can
// call t.Fatalf, whose runtime.Goexit unwinds through (and runs) any defer
// registered before it — including a hypothetical sync.Once's own internal
// "mark done" defer, which would leave the Once permanently reporting
// "done" with showcaseTemplate still nil, wedging every later caller
// behind a silently-broken cache instead of retrying. With a plain Mutex,
// showcaseTemplate is only ever assigned AFTER a build fully succeeds, so
// a Goexit mid-build leaves it nil (Unlock still runs, via its own defer,
// so no deadlock) and the next caller retries the build cleanly.
var (
	showcaseTemplateMu  sync.Mutex
	showcaseTemplateDir string
	showcaseTemplate    *fixturegit.Repo
)

// ensureShowcaseTemplate returns the process-wide showcase store template,
// building it via buildShowcaseRepo on the first call only. buildShowcaseRepo
// itself returns a repo rooted in a t.TempDir() (fixturegit.Build's own
// construction — internal/fixturegit is a shared, non-test-only helper
// this lane does not touch), which is torn down when THAT ONE test
// finishes; this function copies it into its own directory with explicit
// TestMain-owned, process-lifetime cleanup (mirroring
// cmd/verdi/build_shared_test.go's buildVerdiBinary/buildDir for the
// shared built binary) before that happens, so the template survives every
// later test in the process, not just the one that happened to build it.
func ensureShowcaseTemplate(t *testing.T) *fixturegit.Repo {
	t.Helper()
	showcaseTemplateMu.Lock()
	defer showcaseTemplateMu.Unlock()
	if showcaseTemplate != nil {
		return showcaseTemplate
	}

	built := buildShowcaseRepo(t)

	dir, err := os.MkdirTemp("", "verdi-showcasealign-template-")
	if err != nil {
		t.Fatalf("ensureShowcaseTemplate: mkdtemp: %v", err)
	}
	showcaseTemplateDir = dir
	copyDirTree(t, built.Dir, dir)

	showcaseTemplate = &fixturegit.Repo{Dir: dir, Head: built.Head, Heads: append([]string(nil), built.Heads...)}
	return showcaseTemplate
}

// cloneShowcaseRepoDir returns an independent, freshly copied working
// directory of template — objects, refs (including any refs/replace
// graft), config, HEAD, and the working tree — in a fresh t.TempDir(),
// then refreshes git's index-stat cache against the copy. Twin of
// cmd/verdi/countersign_lifecycle_contract_test.go's cloneFixtureRepoDir
// (T1): a byte copy carries the ORIGINAL files' mtimes into
// .git/index's stat cache, so `git diff-files`/`diff-index` would
// misreport every tracked file as modified in the copy without this
// refresh.
func cloneShowcaseRepoDir(t *testing.T, template *fixturegit.Repo) *fixturegit.Repo {
	t.Helper()
	dir := t.TempDir()
	copyDirTree(t, template.Dir, dir)

	refresh := exec.Command("git", "update-index", "-q", "--refresh")
	refresh.Dir = dir
	if out, err := refresh.CombinedOutput(); err != nil {
		t.Fatalf("cloneShowcaseRepoDir: git update-index -q --refresh in %s: %v\n%s", dir, err, out)
	}
	return &fixturegit.Repo{Dir: dir, Head: template.Head, Heads: append([]string(nil), template.Heads...)}
}

// copyDirTree recursively byte-copies src's entire contents — files,
// directories, and symlinks, preserving permissions — into dst. Shared by
// ensureShowcaseTemplate (the once-per-process template copy) and
// cloneShowcaseRepoDir (every per-test clone from it); the walk itself is
// T1's cloneFixtureRepoDir WalkDir body, factored out so both copy sites
// use the identical logic rather than two hand-maintained twins.
func copyDirTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		switch {
		case entry.IsDir():
			return os.MkdirAll(target, info.Mode().Perm()|0o700)
		case info.Mode()&os.ModeSymlink != 0:
			linkDest, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(linkDest, target)
		default:
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(target, data, info.Mode().Perm())
		}
	})
	if err != nil {
		t.Fatalf("copyDirTree: copying %s to %s: %v", src, dst, err)
	}
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
	attachHistoricalObligationQualityAncestry(t, repo)
	writeLoansvcFixture(t, repo.Dir)
	provisionMutableZone(t, repo.Dir)
	return repo
}

// attachHistoricalObligationQualityAncestry supplies the one relationship the
// deterministic showcase reconstruction cannot encode in its stable fixture
// SHAs: this legacy corpus predates the owner merge that adopted obligation
// quality. Importing the exact local adoption object and grafting it after the
// reconstructed head lets production Git ancestry prove that historical fact;
// no runtime cutoff, fallback, or assumption is weakened.
func attachHistoricalObligationQualityAncestry(t *testing.T, repo *fixturegit.Repo) {
	t.Helper()
	fetch := exec.Command("git", "fetch", "--quiet", "--no-tags", verdiRepoRoot, evidence.ObligationQualityAdoptionCommit)
	fetch.Dir = repo.Dir
	if out, err := fetch.CombinedOutput(); err != nil {
		t.Fatalf("importing obligation-quality adoption %s into showcase fixture: %v\n%s", evidence.ObligationQualityAdoptionCommit, err, out)
	}
	graft := exec.Command("git", "replace", "--graft", evidence.ObligationQualityAdoptionCommit, repo.Head)
	graft.Dir = repo.Dir
	if out, err := graft.CombinedOutput(); err != nil {
		t.Fatalf("attaching historical showcase head %s before adoption %s: %v\n%s", repo.Head, evidence.ObligationQualityAdoptionCommit, err, out)
	}
	prove := exec.Command("git", "merge-base", "--is-ancestor", repo.Head, evidence.ObligationQualityAdoptionCommit)
	prove.Dir = repo.Dir
	if out, err := prove.CombinedOutput(); err != nil {
		t.Fatalf("proving historical showcase ancestry %s <= %s: %v\n%s", repo.Head, evidence.ObligationQualityAdoptionCommit, err, out)
	}
}

// TestProvisionShowcaseStoreCopyEquivalence is lane R3's own proof: caching
// the store's construction (ensureShowcaseTemplate/cloneShowcaseRepoDir)
// must never change what any of this package's other tests read. It
// checks a cached clone against a completely independent, uncached
// buildShowcaseRepo call on every axis provisionShowcaseStore's own doc
// comment promises equivalence for — refs (including the refs/replace
// graft), objects, config, the working tree (its directory set, empty
// directories included), and the index, whose stat
// cache must be fresh (`git diff-files` and `git diff-index HEAD` both
// empty) in the clone as in the independent build — and then
// checks that a mutation made in one clone (a working-tree edit, a commit,
// and an update-ref) never appears in a coexisting sibling clone or in a
// clone taken afterward from the shared template: not in its files,
// directories, refs, or objects.
func TestProvisionShowcaseStoreCopyEquivalence(t *testing.T) {
	t.Run("a cached clone agrees with a completely independent, uncached build", func(t *testing.T) {
		fresh := buildShowcaseRepo(t)          // never touches the shared template
		cachedDir := provisionShowcaseStore(t) // ensureShowcaseTemplate + cloneShowcaseRepoDir

		// index stat cache: checked FIRST, before anything below runs `git
		// status` (worktreeFingerprint does), because status silently
		// refreshes and rewrites a stale index and would hide exactly the
		// defect this checks for. diff-files and diff-index trust the
		// index's stat data without refreshing it, so a copy whose stat
		// cache still carried the template's mtimes would list every
		// tracked file here.
		assertShowcaseIndexClean(t, fresh.Dir)
		assertShowcaseIndexClean(t, cachedDir)

		// refs: every ref under .git/refs (branches AND the
		// refs/replace/<sha> graft), plus the symbolic HEAD pointer itself
		// (show-ref does not list HEAD).
		freshRefs := sortedShowcaseGitLines(t, fresh.Dir, "show-ref")
		cachedRefs := sortedShowcaseGitLines(t, cachedDir, "show-ref")
		if freshRefs != cachedRefs {
			t.Fatalf("git show-ref disagrees between an independent build and a cached clone:\nfresh:\n%s\ncached:\n%s", freshRefs, cachedRefs)
		}
		freshHead, err := os.ReadFile(filepath.Join(fresh.Dir, ".git", "HEAD"))
		if err != nil {
			t.Fatalf("reading fresh .git/HEAD: %v", err)
		}
		cachedHead, err := os.ReadFile(filepath.Join(cachedDir, ".git", "HEAD"))
		if err != nil {
			t.Fatalf("reading cached .git/HEAD: %v", err)
		}
		if string(freshHead) != string(cachedHead) {
			t.Fatalf(".git/HEAD disagrees: fresh=%q cached=%q", freshHead, cachedHead)
		}

		// objects: every object reachable from every ref, replace refs
		// resolved — the exact object graph a git command run against the
		// store would see, including (via the graft) the attached
		// historical obligation-quality adoption commit itself.
		freshObjects := sortedShowcaseGitLines(t, fresh.Dir, "rev-list", "--all", "--objects")
		cachedObjects := sortedShowcaseGitLines(t, cachedDir, "rev-list", "--all", "--objects")
		if freshObjects != cachedObjects {
			t.Fatalf("git rev-list --all --objects disagrees between an independent build and a cached clone")
		}

		// config
		freshCfg := sortedShowcaseGitLines(t, fresh.Dir, "config", "-l", "--local")
		cachedCfg := sortedShowcaseGitLines(t, cachedDir, "config", "-l", "--local")
		if freshCfg != cachedCfg {
			t.Fatalf("git config -l --local disagrees between an independent build and a cached clone:\nfresh:\n%s\ncached:\n%s", freshCfg, cachedCfg)
		}

		// working tree + index: worktreeFingerprint (cli_showcase_test.go,
		// TestCLIShowcaseVersion's own helper, reused rather than
		// reimplemented) hashes `git status --porcelain -uall` plus every
		// non-.git file's path, permissions, and content — deliberately
		// excluding .git's own bookkeeping, which the refs/objects/config
		// checks above already cover directly.
		freshDigest, freshStatus := worktreeFingerprint(t, fresh.Dir)
		cachedDigest, cachedStatus := worktreeFingerprint(t, cachedDir)
		if freshDigest != cachedDigest {
			t.Fatalf("working tree/index fingerprint disagrees between an independent build and a cached clone:\n%s", statusDelta(freshStatus, cachedStatus))
		}
		// directories: worktreeFingerprint hashes files only and git
		// status never lists an empty directory, so the present-but-empty
		// mutable zone (provisionMutableZone) is compared here instead.
		freshDirs := worktreeDirSet(t, fresh.Dir)
		cachedDirs := worktreeDirSet(t, cachedDir)
		if freshDirs != cachedDirs {
			t.Fatalf("working-tree directory set disagrees between an independent build and a cached clone:\n%s", statusDelta(freshDirs, cachedDirs))
		}

		// the replace graft must be FUNCTIONALLY identical, not merely
		// ref-identical: the historical ancestry proof
		// attachHistoricalObligationQualityAncestry already ran once, at
		// build time, must still hold after the copy.
		proveShowcaseAncestor(t, fresh.Dir)
		proveShowcaseAncestor(t, cachedDir)
	})

	t.Run("a mutation in one copy is invisible to a sibling copy and to the template", func(t *testing.T) {
		baseline := provisionShowcaseStore(t)
		baselineDigest, _ := worktreeFingerprint(t, baseline)
		baselineDirs := worktreeDirSet(t, baseline)
		baselineRefs := sortedShowcaseGitLines(t, baseline, "show-ref")

		// b is provisioned BEFORE a is mutated, so it is a live sibling:
		// anything a's edits reached through shared storage (a symlinked
		// or hard-linked .git/refs, objects, or file) would reach b too.
		a := provisionShowcaseStore(t)
		b := provisionShowcaseStore(t)

		manifestRel := filepath.Join(".verdi", "verdi.yaml")
		original, err := os.ReadFile(filepath.Join(a, manifestRel))
		if err != nil {
			t.Fatalf("reading %s in copy a: %v", manifestRel, err)
		}
		mutated := append(append([]byte{}, original...), []byte("# lane-r3 isolation probe\n")...)
		if err := os.WriteFile(filepath.Join(a, manifestRel), mutated, 0o644); err != nil {
			t.Fatalf("mutating copy a's %s: %v", manifestRel, err)
		}
		if aDigest, _ := worktreeFingerprint(t, a); aDigest == baselineDigest {
			t.Fatalf("test setup: mutating copy a did not change its own fingerprint — the probe mutation is not being observed")
		}

		// The mutation reaches a's git state too: committing it writes new
		// objects, moves the checked-out branch ref, and rewrites the index;
		// update-ref then adds a ref of its own.
		headBefore := strings.TrimSpace(showcaseGitOutput(t, a, "rev-parse", "HEAD"))
		showcaseGitOutput(t, a, "add", "--", manifestRel)
		showcaseGitOutput(t, a, "commit", "--quiet", "--no-verify", "-m", "lane-r3 isolation probe")
		probeCommit := strings.TrimSpace(showcaseGitOutput(t, a, "rev-parse", "HEAD"))
		showcaseGitOutput(t, a, "update-ref", "refs/heads/lane-r3-isolation-probe", headBefore)
		if aRefs := sortedShowcaseGitLines(t, a, "show-ref"); aRefs == baselineRefs {
			t.Fatalf("test setup: committing and update-ref in copy a did not change its git show-ref output — the probe is not being observed")
		}

		// c is taken AFTER all of a's mutations: matching the pristine
		// baseline proves the shared template itself was never touched.
		c := provisionShowcaseStore(t)

		for _, other := range []struct{ name, dir string }{
			{"sibling copy b", b},
			{"later copy c", c},
		} {
			// git state first: a leaked ref can point at an object this copy
			// lacks, and show-ref's own error then names that ref, where the
			// worktree checks below would only see git status exit 128.
			if refs := sortedShowcaseGitLines(t, other.dir, "show-ref"); refs != baselineRefs {
				t.Fatalf("%s's git show-ref differs from the pristine baseline — copy a's commit or update-ref leaked:\n%s", other.name, statusDelta(baselineRefs, refs))
			}
			probe := exec.Command("git", "cat-file", "-e", probeCommit)
			probe.Dir = other.dir
			err := probe.Run()
			if err == nil {
				t.Fatalf("%s holds copy a's probe commit %s — copies share an object store", other.name, probeCommit)
			}
			if _, ok := err.(*exec.ExitError); !ok {
				t.Fatalf("git cat-file -e %s in %s: %v", probeCommit, other.name, err)
			}
			manifest, err := os.ReadFile(filepath.Join(other.dir, manifestRel))
			if err != nil {
				t.Fatalf("reading %s in %s: %v", manifestRel, other.name, err)
			}
			if string(manifest) != string(original) {
				t.Fatalf("%s's %s carries copy a's mutation — copies are not independent", other.name, manifestRel)
			}
			if digest, status := worktreeFingerprint(t, other.dir); digest != baselineDigest {
				t.Fatalf("%s's fingerprint differs from the pristine baseline — copy a's mutation leaked:\n%s", other.name, status)
			}
			if dirs := worktreeDirSet(t, other.dir); dirs != baselineDirs {
				t.Fatalf("%s's directory set differs from the pristine baseline:\n%s", other.name, statusDelta(baselineDirs, dirs))
			}
		}
	})
}

// showcaseGitOutput runs git in dir and returns its combined stdout+stderr,
// failing the calling test on a non-zero exit. Plumbing helper
// for TestProvisionShowcaseStoreCopyEquivalence.
func showcaseGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return string(out)
}

// sortedShowcaseGitLines runs showcaseGitOutput and returns its output with
// lines sorted, so two listings that are logically identical but were
// produced in a different on-disk or traversal order still compare equal.
func sortedShowcaseGitLines(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out := showcaseGitOutput(t, dir, args...)
	trimmed := strings.TrimRight(out, "\n")
	if trimmed == "" {
		return ""
	}
	lines := strings.Split(trimmed, "\n")
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// worktreeDirSet returns every directory under root, empty ones included,
// as sorted slash-separated relative paths, one per line — excluding root
// itself and the .git subtree (whose contents the refs/objects/config
// checks cover directly).
func worktreeDirSet(t *testing.T, root string) string {
	t.Helper()
	var dirs []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		switch rel {
		case ".":
			return nil
		case ".git":
			return fs.SkipDir
		}
		dirs = append(dirs, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("listing the working-tree directories under %s: %v", root, err)
	}
	sort.Strings(dirs)
	return strings.Join(dirs, "\n")
}

// assertShowcaseIndexClean fails the test unless both `git diff-files` and
// `git diff-index HEAD` report nothing in dir. Neither refreshes the index,
// so each reports every tracked file whose recorded stat data no longer
// matches the file on disk — the stale-cache state a byte copy leaves
// behind until cloneShowcaseRepoDir's `git update-index --refresh` runs.
func assertShowcaseIndexClean(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{{"diff-files"}, {"diff-index", "HEAD"}} {
		if out := showcaseGitOutput(t, dir, args...); out != "" {
			t.Fatalf("git %s in %s reports modified tracked files — the index stat cache is stale:\n%s", strings.Join(args, " "), dir, out)
		}
	}
}

// proveShowcaseAncestor re-runs attachHistoricalObligationQualityAncestry's
// own proof against dir: dir's current HEAD must still be provable as an
// ancestor of the obligation-quality adoption commit via the
// refs/replace graft, showing the graft itself works after a copy, not
// merely that its ref exists.
func proveShowcaseAncestor(t *testing.T, dir string) {
	t.Helper()
	head := strings.TrimSpace(showcaseGitOutput(t, dir, "rev-parse", "HEAD"))
	cmd := exec.Command("git", "merge-base", "--is-ancestor", head, evidence.ObligationQualityAdoptionCommit)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("proving historical ancestry %s <= %s in %s: %v\n%s", head, evidence.ObligationQualityAdoptionCommit, dir, err, out)
	}
}
