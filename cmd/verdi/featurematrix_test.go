package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// v2FixtureRoot is testdata/corpus's own directory, relative to this
// package — the source for the round-four feature-fold fixture files
// (spec/accepted-pending-build and its stories) that V1-P1 committed
// without wiring into layers.txt's shared fixturegit history (their
// frozen.commit stamps intentionally cite that shared history's existing
// HEAD — 93ddc5bbb..., proven by buildCorpusRepo(t).Head today — rather
// than pinning a new layer of their own). copyV2FeatureFixture mirrors
// buildCorpusRepo's own copyDerivedTree technique: these files are placed
// on the built repo's working tree verbatim, uncommitted, exactly like
// derived/ already is — storyresolve.LoadActiveSpec and index.Build both
// read straight off disk and neither cares whether a path is git-tracked.
const v2FixtureRoot = "../../testdata/corpus/.verdi"

// copyV2FeatureFixture copies the named .verdi-relative directories from
// testdata/corpus onto repoDir's own .verdi tree.
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
// Fixture: spec/accepted-pending-build (testdata/corpus, V1-P1's round-
// four feature-fold fixture) declares three stubs (borrower-update-api,
// borrower-update-ui, borrower-update-audit-log) and three outcome ACs.
// Two real story specs (borrower-update-api, borrower-update-mobile)
// carry real `implements` edges into it; borrower-update-audit-log's
// stub has no implementing story at all (ac-3 folds no-signal — the fold
// exercising exactly the "left unreconciled" case PLAN-V1.md's fixture
// design names for this phase's negative stub-reconciliation case).
// ac-1 has a real bound outcome attestation
// (attestations/accepted-pending-build/ac-1.md); ac-2 and ac-3 have none.
// No story in the fixture is closed, so every stub reads unreconciled and
// every AC needing story bookkeeping reads pending — an honest, real-data
// snapshot of an in-flight feature, not a cherry-picked all-green case.
func TestCmdMatrix_FeatureRef_Golden(t *testing.T) {
	repo := buildCorpusRepo(t)
	copyV2FeatureFixture(t, repo.Dir,
		"specs/active/accepted-pending-build",
		"specs/active/borrower-update-api",
		"specs/active/borrower-update-mobile",
		"specs/active/borrower-update-mobile-spike",
		"attestations/accepted-pending-build",
	)
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	got := runMatrixForTest(t, []string{"spec/accepted-pending-build"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("cmdMatrix exit = %d, want 0; stderr=%q", got, stderr.String())
	}

	want := `feature: spec/accepted-pending-build

AC    STATUS     EVIDENCE             IMPLEMENTING STORIES                                   TEXT
ac-1  pending    attestation:present  spec/borrower-update-api, spec/borrower-update-mobile  a borrower can update their application
ac-2  pending    attestation:absent   spec/borrower-update-mobile                            a borrower can see the change reflected
ac-3  no-signal  attestation:absent   -                                                      support can audit every update

stubs: acceptance-time plan; current mapping computed below
STUB                       DECLARED ACS  LIVE STORIES                                           RECONCILIATION
borrower-update-api        ac-1          spec/borrower-update-api, spec/borrower-update-mobile  unreconciled
borrower-update-ui         ac-1, ac-2    spec/borrower-update-api, spec/borrower-update-mobile  unreconciled
borrower-update-audit-log  ac-3          -                                                      unreconciled

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
		"specs/active/accepted-pending-build",
		"specs/active/borrower-update-api",
		"specs/active/borrower-update-mobile",
		"specs/active/borrower-update-mobile-spike",
		"attestations/accepted-pending-build",
	)

	derivedDir := filepath.Join(repo.Dir, ".verdi", "data", "derived", "spec--accepted-pending-build", repo.Head)
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
	got := runMatrixForTest(t, []string{"spec/accepted-pending-build"}, &stdout, &stderr)
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

// TestCmdMatrix_FeatureRef_ExcludesSupersededStory proves D-16: a superseded
// implementing story is dropped from the feature fold's AC→story mapping
// entirely. Starting from the golden fixture, flipping borrower-update-mobile
// (which implements ac-1 and ac-2) to `superseded` on disk must remove it from
// every AC's implementing set — ac-2, which it was the sole implementer of,
// falls back to no-signal — so a rung-3 predecessor can never permanently
// hold a feature AC below `evidenced`.
func TestCmdMatrix_FeatureRef_ExcludesSupersededStory(t *testing.T) {
	repo := buildCorpusRepo(t)
	copyV2FeatureFixture(t, repo.Dir,
		"specs/active/accepted-pending-build",
		"specs/active/borrower-update-api",
		"specs/active/borrower-update-mobile",
		"specs/active/borrower-update-mobile-spike",
		"attestations/accepted-pending-build",
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
	got := runMatrixForTest(t, []string{"spec/accepted-pending-build"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("cmdMatrix exit = %d, want 0; stderr=%q", got, stderr.String())
	}
	out := stdout.String()
	if bytes.Contains([]byte(out), []byte("borrower-update-mobile")) {
		t.Fatalf("superseded story borrower-update-mobile must not appear in the feature mapping:\n%s", out)
	}
	// ac-2's only implementer was the now-superseded mobile story, so it must
	// fall back to no-signal with an empty implementing set.
	if !bytes.Contains([]byte(out), []byte("ac-2  no-signal")) {
		t.Fatalf("ac-2 should read no-signal once its sole (superseded) implementer is excluded:\n%s", out)
	}
}
