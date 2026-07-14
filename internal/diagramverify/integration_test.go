package diagramverify

import "testing"

// TestExtraction_ComparableIdentities_ExcludesAmbiguous proves dc-1's
// "an element whose parse was uncertain is excluded from the three-way
// comparison rather than guessed": an ambiguous node's RawID is dropped
// from ComparableNodeIdentities, and any edge touching it is dropped from
// ComparableEdgeIdentities too, while an unrelated node/edge pair survives
// untouched.
func TestExtraction_ComparableIdentities_ExcludesAmbiguous(t *testing.T) {
	collidingTruth := []string{
		"(*example.com/svcfix/internal/app.Service).GetRefund",
		"(*example.com/svcfix/internal/handler.Server).GetRefund",
		"(*example.com/svcfix/internal/app.Service).PublishRefund",
	}
	src := `flowchart LR
    GetRefund["GetRefund"]
    PublishRefund["PublishRefund"]
    GetRefund --> PublishRefund
`
	ext := Parse(src, collidingTruth)
	if ext.Coverage != CoveragePartial {
		t.Fatalf("Coverage = %q, want partial", ext.Coverage)
	}

	nodeIDs := ext.ComparableNodeIdentities()
	for _, id := range nodeIDs {
		if id == "GetRefund" {
			t.Errorf("ComparableNodeIdentities() = %v, want GetRefund excluded (ambiguous)", nodeIDs)
		}
	}
	if len(nodeIDs) != 1 || nodeIDs[0] != "PublishRefund" {
		t.Errorf("ComparableNodeIdentities() = %v, want [PublishRefund]", nodeIDs)
	}

	edgeIDs := ext.ComparableEdgeIdentities()
	if len(edgeIDs) != 0 {
		t.Errorf("ComparableEdgeIdentities() = %v, want none (the only edge touches the ambiguous node)", edgeIDs)
	}
}

// TestEndToEnd_ParseThenCompare wires Parse's extraction directly into
// Compare — the pipeline this package exists to provide, end to end over
// a from-scratch proposal: one node exists in truth, one is genuinely new
// design intent, and the edge between them (absent from truth) is
// proposed-new too.
func TestEndToEnd_ParseThenCompare(t *testing.T) {
	truthFQNs := []string{
		"(*example.com/svcfix/internal/app.Service).GetRefund",
	}
	src := `flowchart LR
    GetRefund["GetRefund"]
    NewStep["NewStep"]
    GetRefund --> NewStep
`
	ext := Parse(src, truthFQNs)
	if ext.Coverage != CoverageFull {
		t.Fatalf("Coverage = %q, want full", ext.Coverage)
	}

	truthNames := map[string]bool{"GetRefund": true}
	nodeResults := Compare(ext.ComparableNodeIdentities(), nil, truthNames)

	got := classificationOf(t, nodeResults, "GetRefund")
	if got.Classification != Exists {
		t.Errorf("GetRefund classification = %q, want exists", got.Classification)
	}
	got = classificationOf(t, nodeResults, "NewStep")
	if got.Classification != ProposedNew {
		t.Errorf("NewStep classification = %q, want proposed-new", got.Classification)
	}

	// No truth edges at all here, so the one drawn edge is proposed-new.
	edgeResults := Compare(ext.ComparableEdgeIdentities(), nil, map[string]bool{})
	got = classificationOf(t, edgeResults, EdgeIdentity("GetRefund", "NewStep"))
	if got.Classification != ProposedNew {
		t.Errorf("edge classification = %q, want proposed-new", got.Classification)
	}
}
