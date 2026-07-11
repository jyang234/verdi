package upstream

import "testing"

func mustDecodeContract(t *testing.T, file string) *BoundaryContract {
	t.Helper()
	c, err := DecodeBoundaryContract(readCanned(t, file))
	if err != nil {
		t.Fatalf("DecodeBoundaryContract(%s): %v", file, err)
	}
	return c
}

// TestComputeBoundaryDiff_Addition uses the two real captured contracts
// (base, and base + a GET /healthz route) to prove an addition is
// non-breaking — matching spike S1's captured `groundwork diff` text
// output verbatim: "+ route GET /healthz" (no BREAKING marker).
func TestComputeBoundaryDiff_Addition(t *testing.T) {
	base := mustDecodeContract(t, "boundary-contract-base.json")
	branch := mustDecodeContract(t, "boundary-contract-branch.json")

	diffs := ComputeBoundaryDiff(base, branch)
	if len(diffs) != 1 {
		t.Fatalf("ComputeBoundaryDiff = %+v, want exactly 1 entry", diffs)
	}
	want := BoundaryDiffEntry{Op: DiffAdd, Surface: SurfaceRoute, Name: "GET /healthz", Breaking: false}
	if diffs[0] != want {
		t.Fatalf("diffs[0] = %+v, want %+v", diffs[0], want)
	}
	if HasBreaking(diffs) {
		t.Error("HasBreaking = true, want false for a pure addition")
	}
}

// TestComputeBoundaryDiff_Removal swaps the same two real contracts (branch
// as base, base as branch) so the healthz route is now a removal — proving
// removals are breaking (I-3: "mark removals breaking"), matching spike
// S1's captured text output's "⚠ BREAKING" marker on its "-" line.
func TestComputeBoundaryDiff_Removal(t *testing.T) {
	withHealthz := mustDecodeContract(t, "boundary-contract-branch.json")
	without := mustDecodeContract(t, "boundary-contract-base.json")

	diffs := ComputeBoundaryDiff(withHealthz, without)
	if len(diffs) != 1 {
		t.Fatalf("ComputeBoundaryDiff = %+v, want exactly 1 entry", diffs)
	}
	want := BoundaryDiffEntry{Op: DiffRemove, Surface: SurfaceRoute, Name: "GET /healthz", Breaking: true}
	if diffs[0] != want {
		t.Fatalf("diffs[0] = %+v, want %+v", diffs[0], want)
	}
	if !HasBreaking(diffs) {
		t.Error("HasBreaking = false, want true for a removal")
	}
}

func TestComputeBoundaryDiff_NoChange(t *testing.T) {
	base := mustDecodeContract(t, "boundary-contract-base.json")
	same := mustDecodeContract(t, "boundary-contract-base.json")

	diffs := ComputeBoundaryDiff(base, same)
	if len(diffs) != 0 {
		t.Fatalf("ComputeBoundaryDiff(identical contracts) = %+v, want empty", diffs)
	}
}

// TestComputeBoundaryDiff_Dependency proves the "dependency" surface
// (external_dependencies), whose element shape has no direct S1 JSON
// capture — only the "+ dependency audit-svc (http)" text line — using a
// synthetic pair of contracts built from that same NamedResource shape.
func TestComputeBoundaryDiff_Dependency(t *testing.T) {
	base := &BoundaryContract{Service: "svcfix", SchemaVersion: boundaryContractSchema}
	branch := &BoundaryContract{
		Service:              "svcfix",
		SchemaVersion:        boundaryContractSchema,
		ExternalDependencies: []NamedResource{{Name: "audit-svc", Kind: "http"}},
	}

	diffs := ComputeBoundaryDiff(base, branch)
	if len(diffs) != 1 {
		t.Fatalf("ComputeBoundaryDiff = %+v, want exactly 1 entry", diffs)
	}
	want := BoundaryDiffEntry{Op: DiffAdd, Surface: SurfaceDependency, Name: "audit-svc (http)", Breaking: false}
	if diffs[0] != want {
		t.Fatalf("diffs[0] = %+v, want %+v", diffs[0], want)
	}
}

func TestComputeBoundaryDiff_Sorted(t *testing.T) {
	base := &BoundaryContract{Service: "svc", SchemaVersion: boundaryContractSchema}
	branch := &BoundaryContract{
		Service:       "svc",
		SchemaVersion: boundaryContractSchema,
		Entrypoints: ContractEntrypoints{HTTP: []HTTPEntrypoint{
			{Method: "POST", Route: "/z"},
			{Method: "GET", Route: "/a"},
		}},
		Published: []NamedResource{{Name: "topic-a"}},
	}
	diffs := ComputeBoundaryDiff(base, branch)
	for i := 1; i < len(diffs); i++ {
		if diffLess(diffs[i], diffs[i-1]) {
			t.Fatalf("diffs not sorted: %+v", diffs)
		}
	}
}
