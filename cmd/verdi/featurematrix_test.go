package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/fixturegit"
)

// v2FixtureRoot is examples/showcase's own directory, relative to this
// package — the source for the round-four feature-fold fixture files
// (spec/escrow-autopay and its stories) that V1-P1 committed
// without wiring into layers.txt's shared fixturegit history (their
// frozen.commit stamps intentionally cite that shared history's existing
// HEAD — 7248a3f6..., proven by buildCorpusRepo(t).Head today — rather
// than pinning a new layer of their own). copyV2FeatureFixture mirrors
// buildCorpusRepo's own copyDerivedTree technique: these files are placed
// on the built repo's working tree verbatim, uncommitted, exactly like
// derived/ already is — storyresolve.LoadActiveSpec and index.Build both
// read straight off disk and neither cares whether a path is git-tracked.
const v2FixtureRoot = "../../examples/showcase/.verdi"

// copyV2FeatureFixture copies the named .verdi-relative directories from
// examples/showcase onto repoDir's own .verdi tree.
func copyV2FeatureFixture(t *testing.T, repoDir string, relDirs ...string) {
	t.Helper()
	for _, rel := range relDirs {
		src := filepath.Join(v2FixtureRoot, rel)
		dst := filepath.Join(repoDir, ".verdi", rel)
		copyTree(t, src, dst)
	}
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("reading %s: %v", src, err)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dst, err)
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			copyTree(t, s, d)
			continue
		}
		data, err := os.ReadFile(s)
		if err != nil {
			t.Fatalf("reading %s: %v", s, err)
		}
		if err := os.WriteFile(d, data, 0o644); err != nil {
			t.Fatalf("writing %s: %v", d, err)
		}
	}
}

// TestCmdMatrix_FeatureRef_Golden is the exit criterion's "`verdi matrix
// spec/<feature>` output matches a golden showing frozen stubs paired
// with the computed live mapping under the 'acceptance-time plan; current
// mapping computed below' banner (05 §Lenses)".
//
// Fixture: spec/escrow-autopay (examples/showcase). public-rollout-plan
// Task 1.5 renamed its stubs to autopay-mandate-api ({ac-1, ac-2}) and
// autopay-retry-policy ({ac-3}), and rewired its former implementing
// stories (borrower-update-api, borrower-update-mobile) away to
// spec/stale-decline — the feature genuinely built breadth around
// (03 §The feature fold: escrow-autopay is the "accepted-pending-build,
// only unbuilt stubs" fixture, stale-decline the "accepted + built,
// evidence flowing" one; see cmd/verdi/matrix_test.go's TestCmdMatrix_Golden
// for the rich fold this same rewire produces there). Only
// borrower-update-mobile keeps a residual implements edge into this
// feature's own ac-2 — preserving the pending-supersession fixture below
// (spec/escrow-autopay-v2 amends exactly ac-2) — so ac-1 and ac-3 now
// honestly fold no-signal (zero implementing stories), ac-2 pending (one
// story, not yet closed/eligible). Neither declared stub realizes: no
// story's title-slug or implements-AC-set matches either one exactly.
// ac-1 still carries a real bound outcome attestation
// (attestations/escrow-autopay/ac-1.md) — present even though the fold
// reads no-signal, since an attestation alone was never sufficient
// without an implementing story (03 §The feature fold).
func TestCmdMatrix_FeatureRef_Golden(t *testing.T) {
	repo := buildCorpusRepo(t)
	copyV2FeatureFixture(t, repo.Dir,
		"specs/active/escrow-autopay",
		"specs/active/borrower-update-api",
		"specs/active/borrower-update-mobile",
		"specs/active/borrower-update-mobile-spike",
		"attestations/escrow-autopay",
	)
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	got := runMatrixForTest(t, []string{"spec/escrow-autopay"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("cmdMatrix exit = %d, want 0; stderr=%q", got, stderr.String())
	}

	want := `feature: spec/escrow-autopay
status: accepted-pending-build

AC    STATUS     EVIDENCE             IMPLEMENTING STORIES         TEXT
ac-1  no-signal  attestation:present  -                            an autopay mandate is created against a submitted application's escrow account, tied to the payment method already on file
ac-2  pending    attestation:absent   spec/borrower-update-mobile  a borrower who edits an existing autopay mandate sees the change reflected in their account before they leave the session
ac-3  no-signal  attestation:absent   -                            a scheduled autopay charge that fails retries according to the declared retry policy instead of silently dropping

stubs: acceptance-time plan; current mapping computed below
STUB                  DECLARED ACS  LIVE STORIES  RECONCILIATION
autopay-mandate-api   ac-1, ac-2    -             unreconciled
autopay-retry-policy  ac-3          -             unreconciled

feature.violated: false
stub_reconciliation.blocked: true
`
	if stdout.String() != want {
		t.Fatalf("matrix feature output mismatch:\n--- got ---\n%s\n--- want ---\n%s", stdout.String(), want)
	}
}

// TestCmdMatrix_FeatureRef_Negative_DanglingBinding proves cmdMatrix
// propagates a feature-fold error (a dangling evidence_for binding, 03
// §Declarations) as an operational exit 2 with stdout empty — the same
// "fails loudly, never a silent no-signal" discipline the story-level
// path already proves in TestCmdMatrix_Negative.
func TestCmdMatrix_FeatureRef_Negative_DanglingBinding(t *testing.T) {
	repo := buildCorpusRepo(t)
	copyV2FeatureFixture(t, repo.Dir,
		"specs/active/escrow-autopay",
		"specs/active/borrower-update-api",
		"specs/active/borrower-update-mobile",
		"specs/active/borrower-update-mobile-spike",
		"attestations/escrow-autopay",
	)

	derivedDir := filepath.Join(repo.Dir, ".verdi", "data", "derived", "spec--escrow-autopay", repo.Head)
	if err := os.MkdirAll(derivedDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", derivedDir, err)
	}
	bogus := `[{"schema":"verdi.evidence/v1","evidence_for":["ac-999"],"kind":"behavioral","verdict":"pass","witness":"w","provenance":{"source":"ci","pipeline":"1","commit":"` + repo.Head + `"},"digest":"sha256:` +
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" + `"}]`
	if err := os.WriteFile(filepath.Join(derivedDir, "verdicts.json"), []byte(bogus), 0o644); err != nil {
		t.Fatalf("writing verdicts.json: %v", err)
	}
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	got := runMatrixForTest(t, []string{"spec/escrow-autopay"}, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("cmdMatrix exit = %d, want 2 (operational error); stderr=%q", got, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on an operational error", stdout.String())
	}
	if !bytesContains(stderr.Bytes(), "ac-999") {
		t.Fatalf("stderr %q should name the dangling AC id", stderr.String())
	}
}

func bytesContains(b []byte, s string) bool {
	return bytes.Contains(b, []byte(s))
}

// TestDiscoverImplementingStories_ClosedStoryInArchive is the regression
// proof for discoverImplementingStories' "Defect fix" doc comment (found
// while building feature closure, spec/close-verb's deferred half): an
// implementing story that has already closed and moved to
// specs/archive/ must still be discoverable, not an operational error.
// Reproduces, in miniature, a real failure this repo's own store hit
// before the fix — `verdi matrix spec/true-closure` errored "loading
// implementing story spec/close-verb: ... no such file or directory"
// because all four of its implementing stories are already archived;
// after the storyresolve.LoadActiveSpec -> LoadSpec fix, the same command
// against the real repo succeeds and lists all four. This test pins that
// behavior with a minimal, hermetic fixture so it cannot silently regress.
func TestDiscoverImplementingStories_ClosedStoryInArchive(t *testing.T) {
	const featureSpecMD = `---
id: spec/matrix-closed-fixture
kind: spec
class: feature
title: "Matrix closed-story fixture"
owners: [platform-team]
status: accepted-pending-build
problem: { text: "x", anchor: problem }
outcome: { text: "y", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "the fixture outcome holds", evidence: [attestation] }
frozen: { at: 2024-01-01, commit: ` + gateFakeFrozenCommit + `}
---
# body
`
	const closedStorySpecMD = `---
id: spec/matrix-closed-story
kind: spec
class: story
title: "Matrix closed story"
owners: [platform-team]
status: closed
story: jira:MATRIX-CLOSED-1
problem: { text: "x", anchor: problem }
outcome: { text: "y", anchor: outcome }
links:
  - { type: implements, ref: "spec/matrix-closed-fixture#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "the story's own obligation holds", evidence: [attestation] }
frozen: { at: 2024-01-01, commit: ` + gateFakeFrozenCommit + `}
---
# body
`
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml": "schema: verdi.layout/v1\nforge: github\n",
			".verdi/specs/active/matrix-closed-fixture/spec.md": featureSpecMD,
			".verdi/specs/archive/matrix-closed-story/spec.md":  closedStorySpecMD,
		},
		Message: "feature + already-closed implementing story in archive",
	}})
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	got := runMatrixForTest(t, []string{"spec/matrix-closed-fixture"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("cmdMatrix exit = %d, want 0 (a closed implementing story must be discoverable, not an operational error); stderr=%q", got, stderr.String())
	}
	if !bytesContains(stdout.Bytes(), "spec/matrix-closed-story") {
		t.Fatalf("stdout = %q, want it to list the closed implementing story spec/matrix-closed-story", stdout.String())
	}
}

// TestCmdMatrix_FeatureRef_SupersededStoryRendersTerminalMarker proves D-16's
// fold exclusion continues to hold (a superseded implementing story can never
// close, so it is still excluded from the feature fold's AC->story mapping
// and from stub reconciliation's live-story set — ac-2, whose sole implementer
// is mobile, falls back to no-signal once flipped), while ac-2
// (feature-supersession-state) amends the RENDERING: the superseded story
// is no longer silently dropped from the printed matrix — it appears in
// its former AC row tagged `[superseded]`, a terminal marker legible
// without consulting a `superseded-by` backlink (03 §rung 3). Starting
// from the golden fixture (public-rollout-plan Task 1.5: mobile's sole
// remaining implements edge into this feature is ac-2), flipping
// borrower-update-mobile to `superseded` on disk must therefore show it,
// marked, in ac-2's IMPLEMENTING STORIES cell (ac-1/ac-3 stay no-signal,
// unchanged — neither ever had an implementer), with
// feature.violated/stub_reconciliation.blocked unchanged from the golden
// (the visibility change carries no eligibility consequence).
func TestCmdMatrix_FeatureRef_SupersededStoryRendersTerminalMarker(t *testing.T) {
	repo := buildCorpusRepo(t)
	copyV2FeatureFixture(t, repo.Dir,
		"specs/active/escrow-autopay",
		"specs/active/borrower-update-api",
		"specs/active/borrower-update-mobile",
		"specs/active/borrower-update-mobile-spike",
		"attestations/escrow-autopay",
	)

	// Flip the on-disk (disposable) copy of borrower-update-mobile to
	// superseded — a status-only edit, frozen stamp preserved.
	mobilePath := filepath.Join(repo.Dir, ".verdi", "specs", "active", "borrower-update-mobile", "spec.md")
	raw, err := os.ReadFile(mobilePath)
	if err != nil {
		t.Fatalf("reading mobile spec: %v", err)
	}
	flipped := bytes.Replace(raw, []byte("status: accepted-pending-build"), []byte("status: superseded"), 1)
	if bytes.Equal(flipped, raw) {
		t.Fatal("test setup: mobile spec did not carry the expected status line to flip")
	}
	if err := os.WriteFile(mobilePath, flipped, 0o644); err != nil {
		t.Fatalf("writing flipped mobile spec: %v", err)
	}
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	got := runMatrixForTest(t, []string{"spec/escrow-autopay"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("cmdMatrix exit = %d, want 0; stderr=%q", got, stderr.String())
	}

	want := `feature: spec/escrow-autopay
status: accepted-pending-build

AC    STATUS     EVIDENCE             IMPLEMENTING STORIES                      TEXT
ac-1  no-signal  attestation:present  -                                         an autopay mandate is created against a submitted application's escrow account, tied to the payment method already on file
ac-2  no-signal  attestation:absent   spec/borrower-update-mobile [superseded]  a borrower who edits an existing autopay mandate sees the change reflected in their account before they leave the session
ac-3  no-signal  attestation:absent   -                                         a scheduled autopay charge that fails retries according to the declared retry policy instead of silently dropping

stubs: acceptance-time plan; current mapping computed below
STUB                  DECLARED ACS  LIVE STORIES  RECONCILIATION
autopay-mandate-api   ac-1, ac-2    -             unreconciled
autopay-retry-policy  ac-3          -             unreconciled

feature.violated: false
stub_reconciliation.blocked: true
`
	if stdout.String() != want {
		t.Fatalf("matrix feature output mismatch:\n--- got ---\n%s\n--- want ---\n%s", stdout.String(), want)
	}
}

// TestPrintFeatureMatrix_SupersededFeatureStatusLine is ac-2's feature-rung
// own-status proof on the matrix surface: a superseded FEATURE, pointed at
// by `verdi matrix`, announces its own terminal state directly (03 §rung 3,
// "without consulting backlinks") — the feature-rung mirror of
// TestCmdMatrix_StatusLine_Superseded's story-rung proof. Empty ACs/stubs
// keep it a focused rendering unit test: the only claim is the status line.
func TestPrintFeatureMatrix_SupersededFeatureStatusLine(t *testing.T) {
	var buf bytes.Buffer
	spec := &artifact.SpecFrontmatter{Status: artifact.Status("superseded")}
	result := evidence.FeatureResult{SpecRef: "spec/legacy-feature"}
	printFeatureMatrix(&buf, spec, result, evidence.StubReconciliation{}, nil, nil, false)

	if !strings.Contains(buf.String(), "\nstatus: superseded\n") {
		t.Fatalf("feature matrix must render the feature's own superseded status line; got:\n%s", buf.String())
	}
}
