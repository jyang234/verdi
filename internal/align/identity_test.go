package align

import (
	"reflect"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// TestPreserveDispositions_CarriesForwardOnMatch proves PreserveDispositions
// (now a thin wrapper over the generic preserve helper, spec/shared-homes
// ac-5) still carries a prior disposition/note forward when a finding's
// identity (kind, id, text) is unchanged, and leaves a content-changed or
// brand-new finding undispositioned.
func TestPreserveDispositions_CarriesForwardOnMatch(t *testing.T) {
	existing := []artifact.Finding{
		{Kind: "boundary", ID: "boundary-a-b-via", Text: "holds", Disposition: artifact.FindingFixed, Note: "reviewed"},
	}
	newFindings := []artifact.Finding{
		{Kind: "boundary", ID: "boundary-a-b-via", Text: "holds"},    // same identity -> carries forward
		{Kind: "boundary", ID: "boundary-a-b-via", Text: "violated"}, // content changed -> undispositioned
		{Kind: "boundary", ID: "boundary-c-d-via", Text: "holds"},    // brand new -> undispositioned
	}

	got := PreserveDispositions(newFindings, existing)

	if len(got) != 3 {
		t.Fatalf("len(got) = %d, want 3 (order/length preserved)", len(got))
	}
	if got[0].Disposition != artifact.FindingFixed || got[0].Note != "reviewed" {
		t.Fatalf("got[0] = %+v, want disposition/note carried forward from existing", got[0])
	}
	if got[1].Disposition != "" || got[1].Note != "" {
		t.Fatalf("got[1] = %+v, want undispositioned (content changed, different identity)", got[1])
	}
	if got[2].Disposition != "" || got[2].Note != "" {
		t.Fatalf("got[2] = %+v, want undispositioned (new finding)", got[2])
	}
}

// TestPreserveDispositions_NilExisting proves a nil/empty existing slice
// (first run) leaves every new finding untouched — preserve's map-building
// loop over existing must handle the empty case cleanly.
func TestPreserveDispositions_NilExisting(t *testing.T) {
	newFindings := []artifact.Finding{{Kind: "boundary", ID: "x", Text: "y"}}
	got := PreserveDispositions(newFindings, nil)
	if !reflect.DeepEqual(got, newFindings) {
		t.Fatalf("got = %+v, want unchanged copy of newFindings (nil existing)", got)
	}
}

// TestPreserveConflictDispositions_CarriesForwardOnMatch is
// PreserveDispositions' decision-conflict-report analogue proof, matching
// by ConflictIdentity — the same generic preserve helper, a different
// identity function and finding type.
func TestPreserveConflictDispositions_CarriesForwardOnMatch(t *testing.T) {
	existing := []artifact.ConflictFinding{
		{Kind: "supersedes-conflict", ID: "conflict-1", Text: "adr-1 vs adr-2", Disposition: artifact.ConflictExempt, Note: "resolved"},
	}
	newFindings := []artifact.ConflictFinding{
		{Kind: "supersedes-conflict", ID: "conflict-1", Text: "adr-1 vs adr-2"}, // same identity
		{Kind: "supersedes-conflict", ID: "conflict-2", Text: "adr-3 vs adr-4"}, // new
	}

	got := PreserveConflictDispositions(newFindings, existing)

	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].Disposition != artifact.ConflictExempt || got[0].Note != "resolved" {
		t.Fatalf("got[0] = %+v, want disposition/note carried forward from existing", got[0])
	}
	if got[1].Disposition != "" || got[1].Note != "" {
		t.Fatalf("got[1] = %+v, want undispositioned (new finding)", got[1])
	}
}
