package draftmutation

import (
	"reflect"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// diffTargets/diffWarningLabels mirror apply_test.go's own gotTargets/
// gotWarnings extraction so Diff's table below can compare against
// Apply's own single-operation result without duplicating a second
// assertion shape.
func diffTargets(changes []Change) []string {
	out := make([]string, len(changes))
	for i, c := range changes {
		out[i] = c.Target
	}
	return out
}

func diffWarningLabels(warnings []Warning) []string {
	out := make([]string, len(warnings))
	for i, w := range warnings {
		out[i] = string(w.Code) + "/" + w.Target
	}
	return out
}

// TestDiffMatchesApplyForOneOperation proves Diff(before, after) recovers
// exactly the Change/Warning classification Apply itself computed for a
// single, unambiguous operation — the one semantic-diff algorithm, reused
// rather than reimplemented (semanticdiff.go's doc comment).
func TestDiffMatchesApplyForOneOperation(t *testing.T) {
	tests := []struct {
		name       string
		operations []Operation
	}{
		{name: "added", operations: []Operation{{Op: OpAddConstraint, ID: "co-2", Text: "new bound", Anchor: "#co-2"}}},
		{name: "replaced", operations: []Operation{{Op: OpEditAC, ID: "ac-1", Text: "revised first", Evidence: []artifact.EvidenceKind{artifact.EvidenceStatic}, Anchor: "#ac-1"}}},
		{name: "removed", operations: []Operation{{Op: OpRemoveConstraint, ID: "co-1"}}},
		{name: "reordered", operations: []Operation{{Op: OpReorderAC, ID: "ac-1", AfterID: "ac-2"}}},
		{name: "relationship_added", operations: []Operation{{Op: OpAddContextRef, Ref: "spec/other@abcdef2"}}},
		{name: "relationship_removed", operations: []Operation{{Op: OpRemoveLink, Source: "spec", Type: artifact.LinkDependsOn, Ref: "spec/base"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := decodedRequest(t)
			req.Operations = tt.operations
			before := []byte(baseSpec)
			applied, err := Apply(before, req, testIdentity())
			if err != nil {
				t.Fatalf("Apply: %v", err)
			}

			gotChanges, gotWarnings, err := Diff(before, applied.Spec)
			if err != nil {
				t.Fatalf("Diff: %v", err)
			}
			if !reflect.DeepEqual(diffTargets(gotChanges), diffTargets(applied.Result.Changes)) {
				t.Fatalf("Diff targets = %v, want %v", diffTargets(gotChanges), diffTargets(applied.Result.Changes))
			}
			for i := range gotChanges {
				if gotChanges[i] != applied.Result.Changes[i] {
					t.Fatalf("Diff change[%d] = %+v, want %+v", i, gotChanges[i], applied.Result.Changes[i])
				}
			}
			if !reflect.DeepEqual(diffWarningLabels(gotWarnings), diffWarningLabels(applied.Result.Warnings)) {
				t.Fatalf("Diff warnings = %v, want %v", diffWarningLabels(gotWarnings), diffWarningLabels(applied.Result.Warnings))
			}
		})
	}
}

// TestDiffNoChange proves Diff(x, x) is the empty, non-nil report — never
// an error, never a fabricated change.
func TestDiffNoChange(t *testing.T) {
	before := []byte(baseSpec)
	changes, warnings, err := Diff(before, before)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if len(changes) != 0 || len(warnings) != 0 {
		t.Fatalf("Diff(x, x) = %v / %v, want both empty", changes, warnings)
	}
	if changes == nil || warnings == nil {
		t.Fatal("Diff(x, x) returned a nil slice, want non-nil empty")
	}
}

// TestDiffInvalidDocument proves a malformed before/after byte string is
// an operational decode error, never a silently empty diff.
func TestDiffInvalidDocument(t *testing.T) {
	valid := []byte(baseSpec)
	malformed := []byte("not a spec at all")

	if _, _, err := Diff(malformed, valid); err == nil {
		t.Fatal("Diff with malformed before bytes: want error, got nil")
	}
	if _, _, err := Diff(valid, malformed); err == nil {
		t.Fatal("Diff with malformed after bytes: want error, got nil")
	}
}
