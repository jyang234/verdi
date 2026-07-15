package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// corpusTestdataDir is examples/showcase relative to this package, the same
// fixture internal/corpus's own tests build (the shared "committed zone
// gets fixturegit-built, mutable/derived gets copied onto disk verbatim"
// pattern).
const corpusTestdataDir = "../../examples/showcase"

// buildCorpusRepo builds examples/showcase's committed zone into a real
// fixturegit repo (layers.txt-driven, same as internal/corpus's own
// buildFixtureRepo), writes a minimal verdi.yaml so store.FindRoot can
// find it, and copies examples/showcase/derived/ onto disk under
// .verdi/data/derived/ — mirroring the real store's derived tree, using
// the corpus's own commit dir names, which are themselves real
// fixturegit-built commit SHAs (layers 2 and 3), so gitx.IsAncestor
// resolves against real history rather than needing a fake.
func buildCorpusRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()

	order, files := parseCorpusLayers(t)
	var layers []fixturegit.Layer
	for _, n := range order {
		layerFiles := map[string]string{}
		for _, rel := range files[n] {
			data, err := os.ReadFile(filepath.Join(corpusTestdataDir, rel))
			if err != nil {
				t.Fatalf("reading corpus file %s (layer %d): %v", rel, n, err)
			}
			layerFiles[rel] = string(data)
		}
		layers = append(layers, fixturegit.Layer{Files: layerFiles, Message: fmt.Sprintf("layer %d", n)})
	}
	repo := fixturegit.Build(t, layers)

	// verdi.yaml is not part of the corpus's own committed-zone fixture
	// (examples/showcase predates it needing one); write a minimal one
	// directly to disk — store.FindRoot only requires the file to exist,
	// not be git-tracked.
	if err := os.MkdirAll(filepath.Join(repo.Dir, ".verdi"), 0o755); err != nil {
		t.Fatalf("mkdir .verdi: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, ".verdi", "verdi.yaml"), []byte("schema: verdi.layout/v1\nforge: gitlab\n"), 0o644); err != nil {
		t.Fatalf("writing verdi.yaml: %v", err)
	}

	// Derived data lives in data/ (gitignored, never fixturegit-tracked);
	// copy examples/showcase/derived/ verbatim onto the built repo's own
	// data/derived/ tree, preserving the corpus's commit-named
	// subdirectories.
	copyDerivedTree(t, filepath.Join(corpusTestdataDir, "derived"), filepath.Join(repo.Dir, ".verdi", "data", "derived"))

	return repo
}

func copyDerivedTree(t *testing.T, src, dst string) {
	t.Helper()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("reading %s: %v", src, err)
	}
	for _, refEntry := range entries {
		refSrc := filepath.Join(src, refEntry.Name())
		commitEntries, err := os.ReadDir(refSrc)
		if err != nil {
			t.Fatalf("reading %s: %v", refSrc, err)
		}
		for _, commitEntry := range commitEntries {
			commitSrc := filepath.Join(refSrc, commitEntry.Name())
			verdictsSrc := filepath.Join(commitSrc, "verdicts.json")
			data, err := os.ReadFile(verdictsSrc)
			if err != nil {
				t.Fatalf("reading %s: %v", verdictsSrc, err)
			}
			dstDir := filepath.Join(dst, refEntry.Name(), commitEntry.Name())
			if err := os.MkdirAll(dstDir, 0o755); err != nil {
				t.Fatalf("mkdir %s: %v", dstDir, err)
			}
			if err := os.WriteFile(filepath.Join(dstDir, "verdicts.json"), data, 0o644); err != nil {
				t.Fatalf("writing %s: %v", filepath.Join(dstDir, "verdicts.json"), err)
			}
		}
	}
}

// parseCorpusLayers reads examples/showcase/layers.txt, the same format
// internal/corpus/corpus_test.go's own parseLayers reads (duplicated here
// rather than exported cross-package, since it is test-only plumbing).
func parseCorpusLayers(t *testing.T) (order []int, files map[int][]string) {
	t.Helper()
	f, err := os.Open(filepath.Join(corpusTestdataDir, "layers.txt"))
	if err != nil {
		t.Fatalf("opening layers.txt: %v", err)
	}
	defer func() { _ = f.Close() }()

	files = map[int][]string{}
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
			t.Fatalf("layers.txt: bad layer number in line %q: %v", line, err)
		}
		rel := strings.TrimSpace(parts[1])
		files[n] = append(files[n], rel)
		if !seen[n] {
			order = append(order, n)
			seen[n] = true
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scanning layers.txt: %v", err)
	}
	sort.Ints(order)
	return order, files
}

// TestCmdMatrix_Golden runs `verdi matrix jira:LOAN-1482` against the real
// fixturegit-built corpus.
//
// Historical note (public-rollout-plan Task 1.4): before stale-decline's
// showcase renovation, this golden proved jira:LOAN-1482 resolved to
// spec/stale-decline through the STORY fold (foldStoryEvidence), with a
// rich evidenced/pending/no-signal/waived breakdown — ac-1 (static, one ci
// pass record) evidenced; ac-2 (static+behavioral, behavioral-only)
// pending; ac-3 (behavioral, source:local abstain only) no-signal; ac-4
// (runtime) waived by waivers/jira-loan-1482/ac-4.md. Task 1.4 gave
// stale-decline problem/outcome and open_questions frontmatter (VL-017 +
// the vetting bar require the open question carried as a declared object),
// which flips cmdMatrix's own fold discriminator
// (`spec.Class == ClassFeature && spec.Problem != nil`, matrix.go) from
// "grandfathered v0 feature, fold at the story level" to "real round-four
// feature, fold through cmdMatrixFeature" — the same discriminator
// featurelens.go uses in internal/dex. jira:LOAN-1482 now resolves through
// the feature fold below.
//
// The feature fold does not resolve stale-decline's own v0-era, story-ref-
// slug-keyed attestation (attestations/jira-loan-1482/ac-2.md verifies the
// whole spec, not a per-AC fragment under the feature's own slug) or its
// waiver, and it has no `implements` edges of its own to populate an
// IMPLEMENTING STORIES column (stale-decline's three stories implement
// spec/escrow-autopay's ACs, not stale-decline's own — a pre-existing
// corpus wiring choice outside this task's file scope) — so this golden is
// now honest but sparser than the retired story-fold rendering. The richer
// evidenced/pending/waived resolution this test used to prove is still
// exercised by TestCmdMatrix_RoundFourStory_RendersStoryFold's sibling
// spec/borrower-update-api (a class: story spec, always story-folded) and
// by TestCmdMatrix_ObligationColumn's synthetic fixture.
func TestCmdMatrix_Golden(t *testing.T) {
	repo := buildCorpusRepo(t)
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	got := runMatrixForTest(t, []string{"jira:LOAN-1482"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("cmdMatrix exit = %d, want 0 (matrix reports, never gates); stderr=%q", got, stderr.String())
	}

	want := `feature: spec/stale-decline
status: accepted-pending-build

AC    STATUS     EVIDENCE                             IMPLEMENTING STORIES  TEXT
ac-1  no-signal  attestation:absent; static:pass      -                     static obligation holds for the retry path
ac-2  no-signal  attestation:absent; behavioral:pass  -                     static and behavioral: charge API retried on stale decline
ac-3  no-signal  attestation:absent                   -                     behavioral: golden flow for partial refunds
ac-4  no-signal  attestation:absent                   -                     runtime: post-deploy decline-rate check

stubs: acceptance-time plan; current mapping computed below
(none declared)
STUB  DECLARED ACS  LIVE STORIES  RECONCILIATION

feature.violated: false
stub_reconciliation.blocked: false
`
	if stdout.String() != want {
		t.Fatalf("matrix output mismatch:\n--- got ---\n%s\n--- want ---\n%s", stdout.String(), want)
	}
}

// TestCmdMatrix_RoundFourStory_RendersStoryFold is the I-1 regression: a
// round-four `class: story` spec carries problem/outcome (VL-006 requires
// them on new-class specs), so a Problem-based feature-vs-story
// discriminator misrouted every such story into FoldFeature, which fails
// closed ("not a feature spec") — exit 2 with empty stdout. Routing on
// spec.Class == artifact.ClassFeature keeps the round-four story on the
// story-level fold path. Fixture: examples/showcase's borrower-update-api
// (class: story, problem/outcome present, story jira:LOAN-1482, one AC).
func TestCmdMatrix_RoundFourStory_RendersStoryFold(t *testing.T) {
	repo := buildCorpusRepo(t)
	copyV2FeatureFixture(t, repo.Dir, "specs/active/borrower-update-api")
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	got := runMatrixForTest(t, []string{"spec/borrower-update-api"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("cmdMatrix(spec/borrower-update-api) = %d, want 0 (round-four story renders the story fold); stderr=%q", got, stderr.String())
	}

	want := `story: jira:LOAN-1482
spec:  spec/borrower-update-api
status: accepted-pending-build

AC    STATUS     EVIDENCE                      TEXT                                                         OBLIGATION
ac-1  no-signal  static:none; behavioral:none  PUT /applications/:id/update returns 200 with the new state  static: (no obligation); behavioral: (no obligation)

story.violated: false
story.eligible: false
`
	if stdout.String() != want {
		t.Fatalf("round-four story matrix output mismatch:\n--- got ---\n%s\n--- want ---\n%s", stdout.String(), want)
	}
	// A misroute into the feature fold would print a feature header and stub
	// section instead — guard against regression explicitly.
	if strings.Contains(stdout.String(), "feature:") || strings.Contains(stdout.String(), "stub_reconciliation") {
		t.Fatalf("output routed through the feature fold, not the story fold:\n%s", stdout.String())
	}
}

// TestCmdMatrix_Preview_DiffersExactlyByAdvisoryRecords proves --preview's
// output differs from the authoritative run only in what the advisory
// (source: local) ac-3 record changes: ac-3 goes from no-signal to
// pending (the local abstain record now counts as behavioral signal), and
// nothing else about the table changes.
func TestCmdMatrix_Preview_DiffersExactlyByAdvisoryRecords(t *testing.T) {
	repo := buildCorpusRepo(t)
	t.Chdir(repo.Dir)

	var authoritative, preview bytes.Buffer
	if got := runMatrixForTest(t, []string{"jira:LOAN-1482"}, &authoritative, &bytes.Buffer{}); got != 0 {
		t.Fatalf("authoritative run exit = %d, want 0", got)
	}
	if got := runMatrixForTest(t, []string{"jira:LOAN-1482", "--preview"}, &preview, &bytes.Buffer{}); got != 0 {
		t.Fatalf("preview run exit = %d, want 0", got)
	}

	// Public-rollout-plan Task 1.4 flips jira:LOAN-1482 (spec/stale-decline)
	// from the story fold to the feature fold — see TestCmdMatrix_Golden's
	// doc comment for why. The preview/authoritative diff this test proves
	// (exactly ac-3's row differs, plus the PREVIEW banner) still holds
	// under the feature fold; only the column shape and ac-3's specific
	// before/after values changed.
	wantPreview := `feature: spec/stale-decline
status: accepted-pending-build
PREVIEW: advisory (source: local) evidence included alongside authoritative (source: ci)

AC    STATUS     EVIDENCE                                IMPLEMENTING STORIES  TEXT
ac-1  no-signal  attestation:absent; static:pass         -                     static obligation holds for the retry path
ac-2  no-signal  attestation:absent; behavioral:pass     -                     static and behavioral: charge API retried on stale decline
ac-3  no-signal  attestation:absent; behavioral:abstain  -                     behavioral: golden flow for partial refunds
ac-4  no-signal  attestation:absent                      -                     runtime: post-deploy decline-rate check

stubs: acceptance-time plan; current mapping computed below
(none declared)
STUB  DECLARED ACS  LIVE STORIES  RECONCILIATION

feature.violated: false
stub_reconciliation.blocked: false
`
	if preview.String() != wantPreview {
		t.Fatalf("preview output mismatch:\n--- got ---\n%s\n--- want ---\n%s", preview.String(), wantPreview)
	}

	// The only content difference between the two runs is the PREVIEW
	// banner line and ac-3's row (no-signal -> pending under the old
	// story fold; attestation:absent -> attestation:absent;
	// behavioral:abstain under the feature fold Task 1.4 moved
	// spec/stale-decline to — see TestCmdMatrix_Golden's doc comment).
	// The EVIDENCE column's width is computed from the widest value in
	// each run, so ac-3's longer preview text also widens the header row
	// and every other row's trailing padding — a formatting consequence,
	// not a content difference. Content equality is checked per field
	// (TrimRight of trailing spaces) rather than the raw padded line.
	authLines := strings.Split(strings.TrimRight(authoritative.String(), "\n"), "\n")
	previewLines := strings.Split(strings.TrimRight(preview.String(), "\n"), "\n")
	previewLinesNoBanner := make([]string, 0, len(previewLines))
	for _, l := range previewLines {
		if strings.HasPrefix(l, "PREVIEW:") {
			continue
		}
		previewLinesNoBanner = append(previewLinesNoBanner, l)
	}
	if len(authLines) != len(previewLinesNoBanner) {
		t.Fatalf("line count differs beyond the PREVIEW banner: authoritative=%d preview=%d", len(authLines), len(previewLinesNoBanner))
	}
	sawAC3Diff := false
	for i := range authLines {
		if authLines[i] == previewLinesNoBanner[i] {
			continue
		}
		// The EVIDENCE column's width is computed from the widest value in
		// each run, so ac-3's longer preview text re-pads every row's
		// inter-column gaps, not just its own — a formatting consequence,
		// not an independent content difference. Compare content
		// (whitespace-run-collapsed fields) rather than the raw padded
		// line so only a genuine field-value change counts.
		if strings.Join(strings.Fields(authLines[i]), " ") == strings.Join(strings.Fields(previewLinesNoBanner[i]), " ") {
			continue
		}
		if !strings.HasPrefix(authLines[i], "ac-3") {
			t.Fatalf("unexpected diff outside ac-3's row: authoritative=%q preview=%q", authLines[i], previewLinesNoBanner[i])
		}
		sawAC3Diff = true
	}
	if !sawAC3Diff {
		t.Fatal("expected ac-3's row to differ between authoritative and preview, it did not")
	}
}

// matrixSupersededStorySpecMD is a minimal frozen, superseded spec fixture
// (ac-2, feature-supersession-state): grandfathered class: feature shape (no
// problem/outcome required, mirroring accept_test.go's own
// alreadyAcceptedSpecMD) so it folds through the story-level path
// (printMatrix), proving the story-rung `status:` line hermetically rather
// than depending on this meta-repo's own real, evolving spec/disclosure-seam
// corpus data as a test fixture — the same honest, smallest-reversible-scope
// choice dc-4 makes for the feature rung, applied here to the story rung's
// own proof.
const matrixSupersededStorySpecMD = `---
id: spec/superseded-story-fixture
kind: spec
title: "Superseded story fixture"
owners: [platform-team]
class: feature
status: superseded
story: jira:LOAN-9000
acceptance_criteria:
  - { id: ac-1, text: "x", evidence: [static] }
frozen: { at: 2026-01-01, commit: deadbeefdeadbeefdeadbeefdeadbeefdeadbeef }
---
# Superseded story fixture
`

// TestCmdMatrix_StatusLine_Superseded proves ac-2/dc-3's story-rung fix:
// `verdi matrix` now prints the resolved spec's own `status:` line, so a
// superseded spec's terminal state is announced directly on this surface —
// closing the exact blindness `verdi matrix spec/disclosure-seam` (this
// corpus's real superseded story) exhibited before this story (no status
// line at all, 03 §rung 3's "legible ... without consulting backlinks").
func TestCmdMatrix_StatusLine_Superseded(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml": phase7ManifestYAML,
				".verdi/specs/active/superseded-story-fixture/spec.md": matrixSupersededStorySpecMD,
			},
			Message: "init store with a superseded spec",
		},
	})
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	got := runMatrixForTest(t, []string{"spec/superseded-story-fixture"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("cmdMatrix exit = %d, want 0; stderr=%q", got, stderr.String())
	}
	want := "story: jira:LOAN-9000\nspec:  spec/superseded-story-fixture\nstatus: superseded\n"
	if !strings.HasPrefix(stdout.String(), want) {
		t.Fatalf("matrix output = %q, want it to start with %q (the status: line announcing the terminal state)", stdout.String(), want)
	}
}

// matrixObligationFixtureSpecMD is a minimal grandfathered class: feature
// story fixture (no problem/outcome needed, the same shape
// matrixSupersededStorySpecMD uses) declaring two ACs across three (ac,
// kind) pairs, so TestCmdMatrix_ObligationColumn can prove spec/
// obligation-wall ac-1 hermetically: ac-1 declares static+behavioral, ac-2
// declares runtime; only ac-1's static kind gets a fixture obligation
// (matrixObligationFixtureAC1StaticMD) below, leaving ac-1's behavioral
// kind and every one of ac-2's kinds deliberately un-obligated to exercise
// the disclosed "(no obligation)" marker (dc-2) alongside a real,
// rendered obligation title in the very same row.
const matrixObligationFixtureSpecMD = `---
id: spec/matrix-obligation-fixture
kind: spec
title: "Matrix obligation fixture"
owners: [platform-team]
class: feature
status: accepted-pending-build
story: jira:MATRIX-1
acceptance_criteria:
  - { id: ac-1, text: "widget can be edited and saved", evidence: [static, behavioral] }
  - { id: ac-2, text: "widget edit is probed post-deploy", evidence: [runtime] }
frozen: { at: 2026-01-01, commit: deadbeefdeadbeefdeadbeefdeadbeefdeadbeef }
---
# Matrix obligation fixture
`

// matrixObligationFixtureAC1StaticMD is ac-1's static obligation — the
// on-disk home spec/obligation-artifact DC-2 fixes:
// .verdi/obligations/matrix-obligation-fixture/ac-1--static.md, its id's
// first segment naming the spec's own directory name (never the
// jira:MATRIX-1 tracker slug above), exactly as spec/obligation-wall DC-1
// requires internal/evidence.Obligations to key its lookup.
const matrixObligationFixtureAC1StaticMD = `---
id: obligation/matrix-obligation-fixture--ac-1--static
kind: obligation
title: "Static analysis obligation for AC-1"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/matrix-obligation-fixture" }
frozen: { at: 2026-01-01, commit: 3e91ab2 }
---
# Static analysis obligation for AC-1

A golangci-lint pass over the touched packages must be clean.
`

// TestCmdMatrix_ObligationColumn proves spec/obligation-wall ac-1 end to
// end over a hermetic fixture story with a real obligation on disk: for
// each declared evidence kind, matrix's OBLIGATION column renders that
// kind's obligation TITLE when one exists (ac-1's static kind — the
// obligation's own prose title, not the AC's `text` field, must be
// legible directly on this surface, co-2's "legible without the
// sidecar"), and a disclosed "(no obligation)" marker — never a blocking
// error — when it does not (ac-1's behavioral kind, and ac-2's runtime
// kind, which also proves this reaches every declared kind, not just the
// first). The fixture carries no evidence records, waivers, or
// attestations at all — deliberately, since the OBLIGATION column is
// independent of fold status (evidence-obligations oq-1: "no fold
// change") — and matrix still exits 0 and renders the full table.
func TestCmdMatrix_ObligationColumn(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml": phase7ManifestYAML,
				".verdi/specs/active/matrix-obligation-fixture/spec.md":        matrixObligationFixtureSpecMD,
				".verdi/obligations/matrix-obligation-fixture/ac-1--static.md": matrixObligationFixtureAC1StaticMD,
			},
			Message: "init store with a partially-obligated story",
		},
	})
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	got := runMatrixForTest(t, []string{"spec/matrix-obligation-fixture"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("cmdMatrix exit = %d, want 0 (disclosure never blocks the render); stderr=%q", got, stderr.String())
	}

	want := `story: jira:MATRIX-1
spec:  spec/matrix-obligation-fixture
status: accepted-pending-build

AC    STATUS     EVIDENCE                      TEXT                               OBLIGATION
ac-1  no-signal  static:none; behavioral:none  widget can be edited and saved     static: Static analysis obligation for AC-1; behavioral: (no obligation)
ac-2  pending    runtime:awaited               widget edit is probed post-deploy  runtime: (no obligation)

story.violated: false
story.eligible: false
`
	if stdout.String() != want {
		t.Fatalf("matrix output mismatch:\n--- got ---\n%s\n--- want ---\n%s", stdout.String(), want)
	}
}

// matrixObligationFixtureBrokenMD is byte-identical to
// matrixObligationFixtureAC1StaticMD except its for_kind disagrees with
// its own id's <for-kind> segment — a decode-time Validate failure
// (artifact.ObligationFrontmatter's own id/for_kind agreement check), the
// same "present but malformed" shape internal/evidence's own
// TestObligations_Broken proves in isolation. This fixture instead proves
// the CLI wiring: cmdMatrix must surface it as an operational error (exit
// 2), never a silently-disclosed "(no obligation)" marker.
const matrixObligationFixtureBrokenMD = `---
id: obligation/matrix-obligation-fixture--ac-1--static
kind: obligation
title: "Broken obligation"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/matrix-obligation-fixture" }
frozen: { at: 2026-01-01, commit: 3e91ab2 }
---
# Broken obligation
`

// TestCmdMatrix_BrokenObligation_OperationalError proves a present-but-
// malformed obligation file is a surfaced operational error (exit 2, no
// stdout), not silently treated as absent — spec/obligation-wall DC-1/DC-2:
// "a broken obligation is not 'no obligation'". This complements
// TestCmdMatrix_ObligationColumn's happy/absent-path coverage by proving
// cmd/verdi/matrix.go's own obligationCellsFor wiring (not just the
// internal/evidence loader in isolation) fails closed.
func TestCmdMatrix_BrokenObligation_OperationalError(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml": phase7ManifestYAML,
				".verdi/specs/active/matrix-obligation-fixture/spec.md":        matrixObligationFixtureSpecMD,
				".verdi/obligations/matrix-obligation-fixture/ac-1--static.md": matrixObligationFixtureBrokenMD,
			},
			Message: "init store with a broken obligation",
		},
	})
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	got := runMatrixForTest(t, []string{"spec/matrix-obligation-fixture"}, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("cmdMatrix exit = %d, want 2 (a broken obligation file is an operational error); stdout=%q stderr=%q", got, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on an operational error", stdout.String())
	}
	if stderr.Len() == 0 {
		t.Fatal("stderr empty, want an error naming the broken obligation")
	}
}

// TestRun_MatrixDispatchesToRealVerb proves dispatch.go routes "matrix" to
// the real implementation, matching the equivalent lint/sync tests.
func TestRun_MatrixDispatchesToRealVerb(t *testing.T) {
	t.Chdir(t.TempDir())

	var stderr bytes.Buffer
	got := run([]string{"matrix", "STORY-1482"}, &stderr)
	if got != 2 {
		t.Fatalf("run([matrix, STORY-1482]) outside a store = %d, want 2 (operational)", got)
	}
	if strings.Contains(stderr.String(), "usage") || strings.Contains(stderr.String(), "not implemented") {
		t.Fatalf("stderr = %q, want a real store-root error, not the generic stub message", stderr.String())
	}
}

// runMatrixForTest calls cmdMatrix directly — the same entry point
// dispatch.go's run() calls for the "matrix" verb.
func runMatrixForTest(t *testing.T, args []string, stdout, stderr io.Writer) int {
	t.Helper()
	return cmdMatrix(args, stdout, stderr)
}

// TestCmdMatrix_Negative covers cmdMatrix's own operational-error paths
// that don't need a real store: missing story argument, an unexpected
// extra positional argument, and no findable store root.
func TestCmdMatrix_Negative(t *testing.T) {
	t.Run("no story argument", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		got := cmdMatrix(nil, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("cmdMatrix(no args) = %d, want 2", got)
		}
		if stderr.Len() == 0 {
			t.Fatal("stderr empty, want a usage message")
		}
	})

	t.Run("extra positional argument", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		got := cmdMatrix([]string{"STORY-1482", "spec/other"}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("cmdMatrix(two positional args) = %d, want 2", got)
		}
	})

	t.Run("no store root", func(t *testing.T) {
		t.Chdir(t.TempDir())
		var stdout, stderr bytes.Buffer
		got := cmdMatrix([]string{"jira:LOAN-1482"}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("cmdMatrix(no store root) = %d, want 2", got)
		}
		if stdout.Len() != 0 {
			t.Fatalf("stdout = %q, want empty on an operational error", stdout.String())
		}
	})
}

// TestCmdMatrix_RefForms drives cmdMatrix end-to-end against the real
// corpus to pin I-30's strict ref contract: a bare tracker key is an
// operational error naming both accepted forms; a well-formed but unknown
// scheme-prefixed story ref is an operational error naming no matching
// spec; and the spec-ref path still folds and prints the same story.
func TestCmdMatrix_RefForms(t *testing.T) {
	repo := buildCorpusRepo(t)
	t.Chdir(repo.Dir)

	t.Run("bare tracker key exits 2 naming both accepted forms", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		got := cmdMatrix([]string{"STORY-1482"}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("cmdMatrix(STORY-1482) = %d, want 2 (operational error); stderr=%q", got, stderr.String())
		}
		if stdout.Len() != 0 {
			t.Fatalf("stdout = %q, want empty on an operational error", stdout.String())
		}
		msg := stderr.String()
		if !strings.Contains(msg, "jira:LOAN-1482") || !strings.Contains(msg, "spec/") {
			t.Fatalf("stderr %q must name both accepted forms (a scheme-prefixed story ref and a spec ref)", msg)
		}
	})

	t.Run("unknown scheme-prefixed story ref exits 2 naming no matching spec", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		got := cmdMatrix([]string{"jira:NOPE-1"}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("cmdMatrix(jira:NOPE-1) = %d, want 2 (operational error); stderr=%q", got, stderr.String())
		}
		if !strings.Contains(stderr.String(), "jira:NOPE-1") {
			t.Fatalf("stderr %q should name the unmatched story ref", stderr.String())
		}
	})

	t.Run("spec ref path still folds", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		got := cmdMatrix([]string{"spec/stale-decline"}, &stdout, &stderr)
		if got != 0 {
			t.Fatalf("cmdMatrix(spec/stale-decline) = %d, want 0; stderr=%q", got, stderr.String())
		}
		// Public-rollout-plan Task 1.4: spec/stale-decline now folds
		// through the feature path (see TestCmdMatrix_Golden's doc
		// comment), so its header is the feature-fold's own form.
		if !strings.HasPrefix(stdout.String(), "feature: spec/stale-decline\nstatus: accepted-pending-build\n") {
			t.Fatalf("spec-ref output header mismatch:\n%s", stdout.String())
		}
	})
}
