package bundle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/upstream"
)

func testService(t *testing.T) ServiceBundle {
	t.Helper()
	in := JoinInput{
		ServiceName:      "svcfix",
		Graph:            loadSvcfixGraph(t),
		Bindings:         loadSvcfixBindings(t),
		KnownGoldenFlows: map[string]bool{"refund-flow": true},
		SpecACs:          specACs(),
		TestSummary:      passingTestSummary(),
		Provenance:       artifact.EvidenceProvenance{Source: artifact.SourceLocal, Commit: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"},
	}
	recs, err := BuildVerdicts(in)
	if err != nil {
		t.Fatalf("BuildVerdicts: %v", err)
	}

	review, err := upstream.DecodeReview(readFile(t, filepath.Join(cannedDir, "review-structurally-clear.json")))
	if err != nil {
		t.Fatalf("DecodeReview: %v", err)
	}

	base, err := upstream.DecodeBoundaryContract(readFile(t, filepath.Join(cannedDir, "boundary-contract-base.json")))
	if err != nil {
		t.Fatalf("DecodeBoundaryContract(base): %v", err)
	}
	branch, err := upstream.DecodeBoundaryContract(readFile(t, filepath.Join(cannedDir, "boundary-contract-branch.json")))
	if err != nil {
		t.Fatalf("DecodeBoundaryContract(branch): %v", err)
	}

	return ServiceBundle{
		ServiceName:  "svcfix",
		Verdicts:     recs,
		Review:       review,
		BoundaryDiff: upstream.ComputeBoundaryDiff(base, branch),
	}
}

// TestAssemble_Happy proves the four bundle files are written, are valid
// JSON each, and decode back through this module's own strict decoders.
func TestAssemble_Happy(t *testing.T) {
	dir := t.TempDir()
	svc := testService(t)

	if err := Assemble(dir, []ServiceBundle{svc}, passingTestSummary()); err != nil {
		t.Fatalf("Assemble: %v", err)
	}

	for _, name := range []string{"verdicts.json", "review.json", "boundary-diff.json", "tests.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("Assemble did not write %s: %v", name, err)
		}
	}

	verdicts, err := os.ReadFile(filepath.Join(dir, "verdicts.json"))
	if err != nil {
		t.Fatalf("reading verdicts.json: %v", err)
	}
	var decoded []artifact.Evidence
	if err := artifact.DecodeStrictJSON(verdicts, &decoded); err != nil {
		t.Fatalf("verdicts.json does not strict-decode as []artifact.Evidence: %v", err)
	}
	if len(decoded) != 2 {
		t.Fatalf("verdicts.json decoded to %d records, want 2", len(decoded))
	}
}

// TestAssemble_ByteIdentical proves two Assemble runs over the same
// logical input produce byte-identical output (canonjson determinism) —
// the property `verdi sync --or-regen` needs to match a canned golden.
func TestAssemble_ByteIdentical(t *testing.T) {
	dir1, dir2 := t.TempDir(), t.TempDir()
	svc1, svc2 := testService(t), testService(t)

	if err := Assemble(dir1, []ServiceBundle{svc1}, passingTestSummary()); err != nil {
		t.Fatalf("Assemble(1): %v", err)
	}
	if err := Assemble(dir2, []ServiceBundle{svc2}, passingTestSummary()); err != nil {
		t.Fatalf("Assemble(2): %v", err)
	}

	for _, name := range []string{"verdicts.json", "review.json", "boundary-diff.json", "tests.json"} {
		a, err := os.ReadFile(filepath.Join(dir1, name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		b, err := os.ReadFile(filepath.Join(dir2, name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		if string(a) != string(b) {
			t.Errorf("%s differs between two runs over the same input:\n--- run 1 ---\n%s\n--- run 2 ---\n%s", name, a, b)
		}
	}
}

func TestAssemble_EmptyArraysNotNull(t *testing.T) {
	dir := t.TempDir()
	if err := Assemble(dir, nil, &TestSummary{Schema: testsSchema, Suite: "pass"}); err != nil {
		t.Fatalf("Assemble(no services): %v", err)
	}
	for _, name := range []string{"verdicts.json", "review.json", "boundary-diff.json"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		got := string(data)
		if got != "[]\n" {
			t.Errorf("%s = %q, want an empty JSON array, not null", name, got)
		}
	}
}

func TestAssemble_Negative(t *testing.T) {
	t.Run("empty dir", func(t *testing.T) {
		if err := Assemble("", nil, &TestSummary{Schema: testsSchema, Suite: "pass"}); err == nil {
			t.Fatal("Assemble(\"\"): want error, got nil")
		}
	})
	t.Run("nonexistent dir", func(t *testing.T) {
		if err := Assemble(filepath.Join(t.TempDir(), "does-not-exist"), nil, &TestSummary{Schema: testsSchema, Suite: "pass"}); err == nil {
			t.Fatal("Assemble(nonexistent dir): want error, got nil")
		}
	})
	t.Run("nil tests summary", func(t *testing.T) {
		if err := Assemble(t.TempDir(), nil, nil); err == nil {
			t.Fatal("Assemble(nil tests): want error, got nil")
		}
	})
}

func TestMergeTestSummaries(t *testing.T) {
	a := &TestSummary{Schema: testsSchema, Suite: "pass", Packages: []PackageResult{{Package: "p1", Status: "pass", Tests: 1}}}
	b := &TestSummary{Schema: testsSchema, Suite: "fail", Packages: []PackageResult{{Package: "p2", Status: "fail", Tests: 1, Failures: 1}}}

	merged := MergeTestSummaries([]*TestSummary{a, b})
	if merged.Suite != "fail" {
		t.Errorf("Suite = %q, want fail (one input failed)", merged.Suite)
	}
	if len(merged.Packages) != 2 {
		t.Fatalf("Packages = %+v, want 2 entries", merged.Packages)
	}
}

func TestMergeTestSummaries_Empty(t *testing.T) {
	merged := MergeTestSummaries(nil)
	if merged == nil {
		t.Fatal("MergeTestSummaries(nil) = nil, want a non-nil empty summary")
	}
	if merged.Suite != "pass" {
		t.Errorf("Suite = %q, want pass for an empty merge", merged.Suite)
	}
}
