package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/matrixprojection"
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

// v2FeatureFixturePaths are the exact .verdi-relative dirs
// copyV2FeatureFixture's own call sites always pass, in order — factored
// out so commitV2FeatureFixture can `git add` precisely these paths
// (never a blanket `-A`, which would also stage buildCorpusRepo's own
// uncommitted derived/ tree).
var v2FeatureFixturePaths = []string{
	".verdi/specs/active/escrow-autopay",
	".verdi/specs/active/borrower-update-api",
	".verdi/specs/active/borrower-update-mobile",
	".verdi/specs/active/borrower-update-mobile-spike",
	".verdi/attestations/escrow-autopay",
}

// commitV2FeatureFixture lands the v2 feature fixture's own paths
// (v2FeatureFixturePaths) as a real git commit on repoDir's current
// branch — needed wherever a test wants internal/specstate's Git-derived
// state (Task 5) to actually resolve one of these candidates as landed,
// rather than the fixture's usual uncommitted-disk-copy shape (which
// resolves Proposed/RelationNew, fine for tests that don't care about a
// specific story's terminal state).
func commitV2FeatureFixture(t *testing.T, repoDir, message string) {
	t.Helper()
	args := append([]string{"add"}, v2FeatureFixturePaths...)
	add := exec.Command("git", args...)
	add.Dir = repoDir
	if out, err := add.CombinedOutput(); err != nil {
		t.Fatalf("git add %v: %v\n%s", v2FeatureFixturePaths, err, out)
	}
	commit := exec.Command("git", "-c", "user.name=t", "-c", "user.email=t@t", "commit", "--quiet", "--no-verify", "-m", message)
	commit.Dir = repoDir
	if out, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
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
// autopay-retry-policy ({ac-2, ac-3} — AC-3's own body prose: "plans
// against ac-2" too, a retry's own success/exhaustion is itself a
// mandate-adjacent state change ac-2's in-session guarantee also covers),
// and rewired its former implementing
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
// TestSupersededStoryRefs is supersededStoryRefs' direct table-driven unit
// test (I2). It pins the two properties the feature-close spec-stale
// condition depends on and that its map-flatten could silently break:
//
//   - dedup (L-N12): a story implementing two or more feature ACs appears
//     under each AC's group in supersededByAC but must flatten to exactly ONE
//     ref — the "one story across >=2 ACs" case asserts the exact singleton
//     set, so removing the seen-guard (which would emit one ref per AC) reds
//     it deterministically; and
//   - deterministic order (M9): the flatten walks a map, so its first-seen
//     order is Go-map-iteration-random — the output is sorted at the source.
//     The "unsorted single-AC group" case uses a one-key map (single-key
//     iteration IS deterministic) whose group is out of order, so the output
//     order is fully determined by the source sort alone: dropping the
//     sort.Strings reds it deterministically, not flakily.
func TestSupersededStoryRefs(t *testing.T) {
	cases := []struct {
		name string
		in   map[string][]string
		want []string
	}{
		{
			name: "empty map yields empty set",
			in:   map[string][]string{},
			want: nil,
		},
		{
			name: "single story under a single AC",
			in:   map[string][]string{"ac-1": {"spec/solo"}},
			want: []string{"spec/solo"},
		},
		{
			name: "one story across >=2 ACs is deduped to exactly one ref (seen-guard)",
			in: map[string][]string{
				"ac-1": {"spec/multi"},
				"ac-2": {"spec/multi"},
				"ac-3": {"spec/multi"},
			},
			want: []string{"spec/multi"},
		},
		{
			name: "unsorted single-AC group is sorted at the source (deterministic order)",
			in:   map[string][]string{"ac-1": {"spec/charlie", "spec/alpha", "spec/bravo"}},
			want: []string{"spec/alpha", "spec/bravo", "spec/charlie"},
		},
		{
			name: "dedup across ACs AND sorted",
			in: map[string][]string{
				"ac-2": {"spec/charlie", "spec/alpha"},
				"ac-1": {"spec/bravo", "spec/alpha"},
			},
			want: []string{"spec/alpha", "spec/bravo", "spec/charlie"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := supersededStoryRefs(tc.in)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("supersededStoryRefs(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestCmdMatrix_FeatureRef_Golden(t *testing.T) {
	repo := buildCorpusRepo(t)
	copyV2FeatureFixture(t, repo.Dir,
		"specs/active/escrow-autopay",
		"specs/active/borrower-update-api",
		"specs/active/borrower-update-mobile",
		"specs/active/borrower-update-mobile-spike",
		"attestations/escrow-autopay",
	)
	// discoverImplementingStories' Git-derived effective-state resolution
	// (Task 5) now refuses OPERATIONALLY (fix-round-1 finding 2) if the
	// default branch itself cannot be resolved at all — fixturegit repos
	// carry no origin remote, so this is required regardless of whether
	// any individual candidate here is actually landed (the v2 fixture
	// files above are copied to disk, uncommitted, on purpose — they
	// still resolve cleanly to Proposed, not Unproven, once the default
	// branch itself resolves).
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	got := runMatrixForTest(t, []string{"spec/escrow-autopay"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("cmdMatrix exit = %d, want 0; stderr=%q", got, stderr.String())
	}

	// The status line speaks the EFFECTIVE state (final fix wave I2): the
	// v2 fixture files are copied to disk UNCOMMITTED on purpose (see the
	// CI_DEFAULT_BRANCH note above), so the projector honestly resolves
	// Proposed — displayed in the legacy vocabulary as "draft" — even
	// though the raw persisted field CLAIMS accepted-pending-build. The
	// raw field lying about acceptance is exactly what effective-state
	// printing exists to stop.
	want := `feature: spec/escrow-autopay
status: draft

AC    STATUS     EVIDENCE             IMPLEMENTING STORIES         TEXT
ac-1  no-signal  attestation:present  -                            an autopay mandate is created against a submitted application's escrow account, tied to the payment method already on file
ac-2  pending    attestation:absent   spec/borrower-update-mobile  a borrower who edits an existing autopay mandate sees the change reflected in their account before they leave the session
ac-3  no-signal  attestation:absent   -                            a scheduled autopay charge that fails retries according to the declared retry policy instead of silently dropping

stubs: acceptance-time plan; current mapping computed below
STUB                  DECLARED ACS  LIVE STORIES  RECONCILIATION
autopay-mandate-api   ac-1, ac-2    -             unreconciled
autopay-retry-policy  ac-2, ac-3    -             unreconciled

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
	// See TestCmdMatrix_FeatureRef_Golden's identical note (fix-round-1
	// finding 2): the default branch must resolve for discoverImplementing
	// Stories to reach ANY per-candidate classification at all.
	t.Setenv("CI_DEFAULT_BRANCH", "main")

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
	t.Setenv("CI_DEFAULT_BRANCH", "main")
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

// TestDiscoverImplementingStories_UnresolvableDefaultBranch_OperationalError
// is fix-round-1 finding 2's proof for the feature-matrix consumer: when
// the default branch cannot be resolved at all, every implementing
// story's effective state is Unproven — this must refuse operationally
// (exit 2), naming the affected ref and the projector's own disclosure,
// never silently collapse to "not closed, not superseded" and render an
// ordinary-looking matrix.
func TestDiscoverImplementingStories_UnresolvableDefaultBranch_OperationalError(t *testing.T) {
	const featureSpecMD = `---
id: spec/matrix-unproven-fixture
kind: spec
class: feature
title: "Matrix unproven fixture"
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
	const storySpecMD = `---
id: spec/matrix-unproven-story
kind: spec
class: story
title: "Matrix unproven story"
owners: [platform-team]
status: accepted-pending-build
story: jira:MATRIX-UNPROVEN-1
problem: { text: "x", anchor: problem }
outcome: { text: "y", anchor: outcome }
links:
  - { type: implements, ref: "spec/matrix-unproven-fixture#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "the story's own obligation holds", evidence: [attestation] }
frozen: { at: 2024-01-01, commit: ` + gateFakeFrozenCommit + `}
---
# body
`
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml": "schema: verdi.layout/v1\nforge: github\n",
			".verdi/specs/active/matrix-unproven-fixture/spec.md": featureSpecMD,
			".verdi/specs/active/matrix-unproven-story/spec.md":   storySpecMD,
		},
		Message: "feature + implementing story, no default branch resolvable",
	}})
	t.Setenv("CI_DEFAULT_BRANCH", "")
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	got := runMatrixForTest(t, []string{"spec/matrix-unproven-fixture"}, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("cmdMatrix(unresolvable default branch) = %d, want 2 (operational); stdout=%q stderr=%q", got, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on an operational error", stdout.String())
	}
	if !bytesContains(stderr.Bytes(), "spec/matrix-unproven-story") {
		t.Fatalf("stderr = %q, want it to name the affected implementing story", stderr.String())
	}
	if !bytesContains(stderr.Bytes(), "no default branch could be resolved") {
		t.Fatalf("stderr = %q, want it to carry specstate's own disclosure", stderr.String())
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
	// Land the v2 fixture (fix-round-1 finding 1): the projector's own
	// legacy-terminal-status compatibility read only fires for a candidate
	// whose EXACT bytes are proven reachable from the default branch — an
	// uncommitted disk-only copy (this fixture's usual shape, matching how
	// derived/ data is planted) would resolve Proposed/RelationNew, never
	// Superseded. Mobile's own predecessor bytes are otherwise UNCHANGED
	// from the golden fixture (the brief's "unchanged superseded
	// predecessors" shape) — only its status line differs, and only the
	// successor's own supersedes edge would normally carry that signal;
	// this fixture has no such successor spec at all, so the legacy
	// status-field compatibility path is exactly what's under test here.
	commitV2FeatureFixture(t, repo.Dir, "land the v2 fixture, mobile pre-flipped to superseded")
	t.Setenv("CI_DEFAULT_BRANCH", "main")
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
autopay-retry-policy  ac-2, ac-3    -             unreconciled

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
	// The status the caller resolved (I2: effectiveMatrixStatus — a landed
	// legacy `status: superseded` projects Superseded via the projector's
	// compatibility reading, so the effective value here matches the raw
	// one this test used to pass through the spec literal).
	spec := &artifact.SpecFrontmatter{}
	record := matrixprojection.Record{
		Target:  matrixprojection.Target{Class: matrixprojection.ClassFeature, SpecRef: "spec/legacy-feature"},
		Feature: &matrixprojection.FeatureBody{ACs: []matrixprojection.FeatureAC{}},
	}
	printFeatureMatrix(&buf, spec, artifact.Status("superseded"), record, evidence.StubReconciliation{}, nil, nil, nil)

	if !strings.Contains(buf.String(), "\nstatus: superseded\n") {
		t.Fatalf("feature matrix must render the feature's own superseded status line; got:\n%s", buf.String())
	}
}
