package journey

import (
	"sort"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/specstate"
)

func TestObligationQualityBlockerKindParityWithArtifact(t *testing.T) {
	for _, kind := range []artifact.EvidenceKind{artifact.EvidenceStatic, artifact.EvidenceBehavioral, artifact.EvidenceRuntime, artifact.EvidenceAttestation} {
		id := obligationQualityBlockerID("ac-1", kind)
		if !blockerIDRe.MatchString(id) {
			t.Errorf("obligationQualityBlockerID(ac-1, %q) = %q, invalid blocker id", kind, id)
		}
	}
}

// TestLifecycleStateParityWithSpecstate proves journey's closed lifecycle
// state vocabulary is exactly internal/specstate.State's constant set —
// this package speaks the same vocabulary without importing specstate in
// production code (record.go never imports it; only this test does).
func TestLifecycleStateParityWithSpecstate(t *testing.T) {
	want := []string{
		string(specstate.Proposed),
		string(specstate.AcceptedPendingBuild),
		string(specstate.Superseded),
		string(specstate.Closed),
		string(specstate.Unproven),
	}
	sort.Strings(want)

	got := make([]string, 0, len(validState))
	for s := range validState {
		got = append(got, s)
	}
	sort.Strings(got)

	if len(got) != len(want) {
		t.Fatalf("journey valid lifecycle states = %v, specstate states = %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("journey valid lifecycle states[%d] = %q, want %q (specstate parity)", i, got[i], want[i])
		}
	}
}

// TestLifecycleRelationParityWithSpecstate is TestLifecycleStateParityWith
// Specstate's counterpart for specstate.Relation.
func TestLifecycleRelationParityWithSpecstate(t *testing.T) {
	want := []string{
		string(specstate.RelationNew),
		string(specstate.RelationExact),
		string(specstate.RelationDiverged),
		string(specstate.RelationUnproven),
	}
	sort.Strings(want)

	got := make([]string, 0, len(validLifecycleRelation))
	for r := range validLifecycleRelation {
		got = append(got, r)
	}
	sort.Strings(got)

	if len(got) != len(want) {
		t.Fatalf("journey valid lifecycle relations = %v, specstate relations = %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("journey valid lifecycle relations[%d] = %q, want %q (specstate parity)", i, got[i], want[i])
		}
	}
}
