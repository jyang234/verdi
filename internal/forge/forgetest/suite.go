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
