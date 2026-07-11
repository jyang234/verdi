package forgetest

import (
	"context"
	"errors"
	"testing"

	"github.com/OWNER/verdi/internal/forge"
)

// Run executes the forge contract suite against the harness newHarness
// builds. newHarness is called once per subtest so each gets isolated
// state.
func Run(t *testing.T, newHarness func(t *testing.T) Harness) {
	t.Helper()
	t.Run("fetch evidence bundle happy path", func(t *testing.T) {
		testFetchHappyPath(t, newHarness(t))
	})
	t.Run("fetch evidence bundle no bundle", func(t *testing.T) {
		testFetchNoBundle(t, newHarness(t))
	})
	t.Run("generated attribute", func(t *testing.T) {
		testGeneratedAttribute(t, newHarness(t))
	})
	t.Run("list open mrs happy path", func(t *testing.T) {
		testListOpenMRsHappyPath(t, newHarness(t))
	})
	t.Run("list open mrs excludes other target branches", func(t *testing.T) {
		testListOpenMRsExcludesOtherTargets(t, newHarness(t))
	})
	t.Run("fetch file at ref happy path", func(t *testing.T) {
		testFetchFileAtRefHappyPath(t, newHarness(t))
	})
	t.Run("fetch file at ref not found", func(t *testing.T) {
		testFetchFileAtRefNotFound(t, newHarness(t))
	})
	t.Run("post comment round-trips byte-identical", func(t *testing.T) {
		testPostCommentByteIdenticalRoundTrip(t, newHarness(t))
	})
	t.Run("list comments never drops anchored or unanchored", func(t *testing.T) {
		testListCommentsAnchoredAndUnanchoredNeverDropped(t, newHarness(t))
	})
	t.Run("thread resolution matches native state", func(t *testing.T) {
		testGetThreadResolutionMatchesNativeState(t, newHarness(t))
	})
	t.Run("thread resolution unresolved by default", func(t *testing.T) {
		testGetThreadResolutionUnresolvedByDefault(t, newHarness(t))
	})
	t.Run("thread resolution excludes general comments", func(t *testing.T) {
		testGetThreadResolutionExcludesGeneralComments(t, newHarness(t))
	})
}

// testPostCommentByteIdenticalRoundTrip proves a posted token-bearing
// diff comment's Body survives byte-identical when re-listed (S6 Q3,
// live-verified on GitHub across post/resolve/push/force-push; exit
// criteria: "a posted token-bearing comment round-trips byte-identical in
// the comment body").
func testPostCommentByteIdenticalRoundTrip(t *testing.T, h Harness) {
	t.Helper()
	body := "[vd:ac-2] outcome AC reads implementation-scoped — reword?"
	created, err := h.Forge().PostComment(context.Background(), "1", body, &forge.CommentTarget{Path: "sample.py", Line: 9})
	if err != nil {
		t.Fatalf("PostComment: %v", err)
	}
	if created.Body != body {
		t.Fatalf("PostComment returned Body = %q, want %q", created.Body, body)
	}

	comments, err := h.Forge().ListComments(context.Background(), "1")
	if err != nil {
		t.Fatalf("ListComments: %v", err)
	}
	var found *forge.Comment
	for i := range comments {
		if comments[i].ID == created.ID {
			found = &comments[i]
		}
	}
	if found == nil {
		t.Fatalf("ListComments did not include the just-posted comment %q: %+v", created.ID, comments)
	}
	if found.Body != body {
		t.Errorf("re-listed Body = %q, want byte-identical %q (S6 Q3)", found.Body, body)
	}
	id, ok := forge.ParseCommentToken(found.Body)
	if !ok || id != "ac-2" {
		t.Errorf(`ParseCommentToken(re-listed body) = (%q, %v), want ("ac-2", true)`, id, ok)
	}
}

// testListCommentsAnchoredAndUnanchoredNeverDropped proves ListComments
// returns both a token-bearing (anchored) and a token-free (unanchored)
// comment in the same feed — the inbox tray's never-dropped guarantee
// starts at the port (05 §Review stickies and forge round-trip).
func testListCommentsAnchoredAndUnanchoredNeverDropped(t *testing.T, h Harness) {
	t.Helper()
	h.SeedComment(t, "2", forge.Comment{ID: "c1", ThreadID: "t1", Body: "[vd:ac-2] outcome AC reads implementation-scoped — reword?", Author: "reviewer"})
	h.SeedComment(t, "2", forge.Comment{ID: "c2", Body: "nit: this comment has no vd token, should land in the inbox tray", Author: "reviewer"})

	comments, err := h.Forge().ListComments(context.Background(), "2")
	if err != nil {
		t.Fatalf("ListComments: %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("ListComments returned %d comments, want 2 (both anchored and unanchored, never dropped): %+v", len(comments), comments)
	}
	var sawAnchored, sawUnanchored bool
	for _, c := range comments {
		if _, ok := forge.ParseCommentToken(c.Body); ok {
			sawAnchored = true
		} else {
			sawUnanchored = true
		}
	}
	if !sawAnchored || !sawUnanchored {
		t.Errorf("ListComments = %+v, want both a token-bearing and a token-free comment present", comments)
	}
}

// testGetThreadResolutionMatchesNativeState proves a resolved thread's
// state is reported accurately, including who resolved it.
func testGetThreadResolutionMatchesNativeState(t *testing.T, h Harness) {
	t.Helper()
	h.SeedComment(t, "3", forge.Comment{ID: "c1", ThreadID: "t1", Body: "[vd:ac-2] reword?"})
	h.SeedThreadResolution(t, "3", forge.ThreadResolution{ThreadID: "t1", Resolved: true, ResolvedBy: "reviewer"})

	threads, err := h.Forge().GetThreadResolution(context.Background(), "3")
	if err != nil {
		t.Fatalf("GetThreadResolution: %v", err)
	}
	if len(threads) != 1 {
		t.Fatalf("GetThreadResolution = %+v, want exactly 1 entry", threads)
	}
	if !threads[0].Resolved || threads[0].ResolvedBy != "reviewer" {
		t.Errorf("GetThreadResolution[0] = %+v, want Resolved=true ResolvedBy=\"reviewer\"", threads[0])
	}
}

// testGetThreadResolutionUnresolvedByDefault proves a freshly seeded
// diff-anchored thread starts unresolved — matching both real forges,
// where a new thread is never born resolved.
func testGetThreadResolutionUnresolvedByDefault(t *testing.T, h Harness) {
	t.Helper()
	h.SeedComment(t, "4", forge.Comment{ID: "c1", ThreadID: "t1", Body: "[vd:ac-2] reword?"})

	threads, err := h.Forge().GetThreadResolution(context.Background(), "4")
	if err != nil {
		t.Fatalf("GetThreadResolution: %v", err)
	}
	if len(threads) != 1 || threads[0].Resolved {
		t.Fatalf("GetThreadResolution = %+v, want exactly 1 unresolved entry", threads)
	}
}

// testGetThreadResolutionExcludesGeneralComments proves a general
// (non-diff) comment — ThreadID "" — never appears in
// GetThreadResolution: it belongs to no substantive/resolvable thread at
// all (05's "substantive" — comments.go's ThreadResolution doc comment).
func testGetThreadResolutionExcludesGeneralComments(t *testing.T, h Harness) {
	t.Helper()
	h.SeedComment(t, "5", forge.Comment{ID: "c1", Body: "General PR conversation comment, not tied to a diff line at all."})

	threads, err := h.Forge().GetThreadResolution(context.Background(), "5")
	if err != nil {
		t.Fatalf("GetThreadResolution: %v", err)
	}
	if len(threads) != 0 {
		t.Fatalf("GetThreadResolution = %+v, want none (a general/individual comment belongs to no resolvable thread)", threads)
	}
}

func testFetchHappyPath(t *testing.T, h Harness) {
	t.Helper()
	want := forge.EvidenceBundle{
		Verdicts:     []byte(`[{"schema":"verdi.evidence/v1"}]` + "\n"),
		Tests:        []byte(`{"schema":"verdi.tests/v1","suite":"pass"}` + "\n"),
		Review:       []byte(`[{"service":"svcfix","verdict":"STRUCTURALLY-CLEAR"}]` + "\n"),
		BoundaryDiff: []byte(`[]` + "\n"),
	}
	h.SeedBundle(t, "spec/stale-decline", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", want)

	got, err := h.Forge().FetchEvidenceBundle(context.Background(), "spec/stale-decline", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	if err != nil {
		t.Fatalf("FetchEvidenceBundle: %v", err)
	}
	if string(got.Verdicts) != string(want.Verdicts) {
		t.Errorf("Verdicts = %q, want %q", got.Verdicts, want.Verdicts)
	}
	if string(got.Tests) != string(want.Tests) {
		t.Errorf("Tests = %q, want %q", got.Tests, want.Tests)
	}
	if string(got.Review) != string(want.Review) {
		t.Errorf("Review = %q, want %q", got.Review, want.Review)
	}
	if string(got.BoundaryDiff) != string(want.BoundaryDiff) {
		t.Errorf("BoundaryDiff = %q, want %q", got.BoundaryDiff, want.BoundaryDiff)
	}
}

func testFetchNoBundle(t *testing.T, h Harness) {
	t.Helper()
	_, err := h.Forge().FetchEvidenceBundle(context.Background(), "spec/never-ran-ci", "0000000000000000000000000000000000000000")
	if err == nil {
		t.Fatal("FetchEvidenceBundle for a ref/commit with no CI run: want error, got nil")
	}
	if !errors.Is(err, forge.ErrNoBundle) {
		t.Fatalf("FetchEvidenceBundle error = %v, want errors.Is(err, forge.ErrNoBundle)", err)
	}
}

func testGeneratedAttribute(t *testing.T, h Harness) {
	t.Helper()
	got := h.Forge().GeneratedAttribute()
	if got != h.WantGeneratedAttribute() {
		t.Errorf("GeneratedAttribute() = %q, want %q", got, h.WantGeneratedAttribute())
	}
}

// testListOpenMRsHappyPath proves a seeded open MR targeting "main" is
// returned by ListOpenMRs(ctx, "main") with its source branch and title
// intact. The MR/PR's forge-native ID is deliberately not asserted here —
// GitLab (IID) and GitHub (PR number) assign it themselves and the two
// numbering spaces are unrelated (openmr.go's OpenMR.ID doc comment).
func testListOpenMRsHappyPath(t *testing.T, h Harness) {
	t.Helper()
	h.SeedOpenMR(t, "main", "design/loan-workflow-v2", "Supersede loan-workflow")

	mrs, err := h.Forge().ListOpenMRs(context.Background(), "main")
	if err != nil {
		t.Fatalf("ListOpenMRs: %v", err)
	}
	if len(mrs) != 1 {
		t.Fatalf("ListOpenMRs returned %d MRs, want 1: %+v", len(mrs), mrs)
	}
	if mrs[0].SourceBranch != "design/loan-workflow-v2" {
		t.Errorf("SourceBranch = %q, want %q", mrs[0].SourceBranch, "design/loan-workflow-v2")
	}
	if mrs[0].Title != "Supersede loan-workflow" {
		t.Errorf("Title = %q, want %q", mrs[0].Title, "Supersede loan-workflow")
	}
}

// testListOpenMRsExcludesOtherTargets proves an MR seeded against one
// target branch does not appear when a different target branch is queried
// — ListOpenMRs is scoped, not a blanket listing of every open MR.
func testListOpenMRsExcludesOtherTargets(t *testing.T, h Harness) {
	t.Helper()
	h.SeedOpenMR(t, "main", "design/loan-workflow-v2", "Supersede loan-workflow")

	mrs, err := h.Forge().ListOpenMRs(context.Background(), "release-1.0")
	if err != nil {
		t.Fatalf("ListOpenMRs: %v", err)
	}
	if len(mrs) != 0 {
		t.Fatalf("ListOpenMRs(release-1.0) = %+v, want none (MR was seeded against main)", mrs)
	}
}

// testFetchFileAtRefHappyPath proves a seeded file's exact bytes round-trip
// through FetchFileAtRef.
func testFetchFileAtRefHappyPath(t *testing.T, h Harness) {
	t.Helper()
	want := []byte("---\nid: spec/loan-workflow-v2\n---\nbody\n")
	h.SeedFile(t, "design/loan-workflow-v2", ".verdi/specs/active/loan-workflow-v2/spec.md", want)

	got, err := h.Forge().FetchFileAtRef(context.Background(), "design/loan-workflow-v2", ".verdi/specs/active/loan-workflow-v2/spec.md")
	if err != nil {
		t.Fatalf("FetchFileAtRef: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("FetchFileAtRef content = %q, want %q", got, want)
	}
}

// testFetchFileAtRefNotFound proves fetching a path never seeded at a ref
// wraps forge.ErrFileNotFound — the expected outcome for most open MRs,
// which don't touch the candidate spec path at all.
func testFetchFileAtRefNotFound(t *testing.T, h Harness) {
	t.Helper()
	_, err := h.Forge().FetchFileAtRef(context.Background(), "design/unrelated-branch", ".verdi/specs/active/never-seeded/spec.md")
	if err == nil {
		t.Fatal("FetchFileAtRef for a never-seeded path: want error, got nil")
	}
	if !errors.Is(err, forge.ErrFileNotFound) {
		t.Fatalf("FetchFileAtRef error = %v, want errors.Is(err, forge.ErrFileNotFound)", err)
	}
}
