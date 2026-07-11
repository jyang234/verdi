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
