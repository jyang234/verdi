package evidence

import (
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

func featureSpecWithStubs(t *testing.T, stubs ...artifact.Stub) *artifact.SpecFrontmatter {
	t.Helper()
	return &artifact.SpecFrontmatter{
		Base:  artifact.Base{ID: "spec/loan-update", Kind: artifact.KindSpec, Title: "Loan update", Owners: []string{"platform-team"}},
		Class: artifact.ClassFeature,
		AcceptanceCriteria: []artifact.AcceptanceCriterion{
			{ID: "ac-1", Text: "t1", Evidence: []artifact.EvidenceKind{artifact.EvidenceAttestation}},
			{ID: "ac-2", Text: "t2", Evidence: []artifact.EvidenceKind{artifact.EvidenceAttestation}},
		},
		Stubs: stubs,
	}
}

// TestReconcileStubs_Buckets is the exit criterion's "a stub-reconciliation
// case per bucket (realized-by, withdrawn-with-note, unplanned-addition,
// unreconciled-blocks-closure)" — all four exercised in one feature so the
// bidirectional check's real shape is visible in one place too.
//
// guide-claim: 6.2-closure-reconciliation
// guide-claim: 8.2-stub-reconciliation-at-closure
func TestReconcileStubs_Buckets(t *testing.T) {
	spec := featureSpecWithStubs(t,
		artifact.Stub{Slug: "realized-stub", AcceptanceCriteria: []string{"ac-1"}},
		artifact.Stub{Slug: "withdrawn-stub", AcceptanceCriteria: []string{"ac-2"}},
		artifact.Stub{Slug: "unreconciled-stub", AcceptanceCriteria: []string{"ac-1", "ac-2"}},
	)

	in := StubReconcileInput{
		Spec: spec,
		Stories: []StubStory{
			// Closed, implements only ac-1: fully realizes realized-stub,
			// and partially (insufficiently) contributes to
			// unreconciled-stub.
			{SpecRef: "spec/story-realizer", ACIDs: []string{"ac-1"}, Closed: true},
			// Implements ac-2 (the other half unreconciled-stub needs) but
			// is NOT closed, so it can never realize anything — this is
			// what keeps unreconciled-stub genuinely unreconciled.
			{SpecRef: "spec/story-not-closed", ACIDs: []string{"ac-2"}, Closed: false},
			// Closed, implements ac-3 — not any stub's declared AC at
			// all: the unplanned-addition case.
			{SpecRef: "spec/story-surprise", ACIDs: []string{"ac-3"}, Closed: true},
		},
		Withdrawals: []StubWithdrawal{
			{Slug: "withdrawn-stub", Note: "descoped to a later feature"},
		},
	}

	got, err := ReconcileStubs(in)
	if err != nil {
		t.Fatalf("ReconcileStubs: %v", err)
	}

	byBucket := map[string]StubResult{}
	for _, r := range got.Stubs {
		byBucket[r.Slug] = r
	}

	realized := byBucket["realized-stub"]
	if realized.Bucket != StubRealized {
		t.Fatalf("realized-stub bucket = %s, want %s", realized.Bucket, StubRealized)
	}
	if len(realized.RealizedBy) == 0 {
		t.Fatal("realized-stub: RealizedBy is empty, want at least one contributing story")
	}

	withdrawn := byBucket["withdrawn-stub"]
	if withdrawn.Bucket != StubWithdrawnBucket {
		t.Fatalf("withdrawn-stub bucket = %s, want %s", withdrawn.Bucket, StubWithdrawnBucket)
	}
	if withdrawn.Note != "descoped to a later feature" {
		t.Fatalf("withdrawn-stub note = %q", withdrawn.Note)
	}

	unreconciled := byBucket["unreconciled-stub"]
	if unreconciled.Bucket != StubUnreconciled {
		t.Fatalf("unreconciled-stub bucket = %s, want %s (ac-2's only implementing story is not closed)", unreconciled.Bucket, StubUnreconciled)
	}
	if !got.Blocked {
		t.Fatal("Blocked = false, want true (unreconciled-stub blocks closure)")
	}

	if len(got.Unplanned) != 1 || got.Unplanned[0].SpecRef != "spec/story-surprise" {
		t.Fatalf("Unplanned = %+v, want exactly spec/story-surprise", got.Unplanned)
	}
}

// TestReconcileStubs_UnreconciledBlocksClosure is the dedicated negative
// case: a stub whose declared ACs are never fully covered by any closed
// story, and carries no withdrawal, blocks closure.
func TestReconcileStubs_UnreconciledBlocksClosure(t *testing.T) {
	spec := featureSpecWithStubs(t,
		artifact.Stub{Slug: "orphan-stub", AcceptanceCriteria: []string{"ac-1", "ac-2"}},
	)
	got, err := ReconcileStubs(StubReconcileInput{
		Spec: spec,
		Stories: []StubStory{
			{SpecRef: "spec/story-partial", ACIDs: []string{"ac-1"}, Closed: true}, // only covers ac-1, not ac-2
		},
	})
	if err != nil {
		t.Fatalf("ReconcileStubs: %v", err)
	}
	if len(got.Stubs) != 1 || got.Stubs[0].Bucket != StubUnreconciled {
		t.Fatalf("Stubs = %+v, want exactly one StubUnreconciled", got.Stubs)
	}
	if !got.Blocked {
		t.Fatal("Blocked = false, want true (an unreconciled stub blocks closure)")
	}
}

// TestReconcileStubs_RealizedByUnionOfStories proves "one or more named
// closed stories" — a stub can be realized by combining multiple stories'
// coverage, none of which alone covers the full declared AC set.
func TestReconcileStubs_RealizedByUnionOfStories(t *testing.T) {
	spec := featureSpecWithStubs(t,
		artifact.Stub{Slug: "split-stub", AcceptanceCriteria: []string{"ac-1", "ac-2"}},
	)
	got, err := ReconcileStubs(StubReconcileInput{
		Spec: spec,
		Stories: []StubStory{
			{SpecRef: "spec/story-half-a", ACIDs: []string{"ac-1"}, Closed: true},
			{SpecRef: "spec/story-half-b", ACIDs: []string{"ac-2"}, Closed: true},
		},
	})
	if err != nil {
		t.Fatalf("ReconcileStubs: %v", err)
	}
	if got.Blocked {
		t.Fatal("Blocked = true, want false (the union of two stories fully covers the stub)")
	}
	if got.Stubs[0].Bucket != StubRealized {
		t.Fatalf("bucket = %s, want %s", got.Stubs[0].Bucket, StubRealized)
	}
	if len(got.Stubs[0].RealizedBy) != 2 {
		t.Fatalf("RealizedBy = %v, want both contributing stories", got.Stubs[0].RealizedBy)
	}
}

// TestReconcileStubs_NotClosedNeverRealizes proves an implementing story
// that fully covers a stub's ACs but is not yet closed does not realize
// it — 03's bucket is "closed stories" specifically.
func TestReconcileStubs_NotClosedNeverRealizes(t *testing.T) {
	spec := featureSpecWithStubs(t, artifact.Stub{Slug: "s", AcceptanceCriteria: []string{"ac-1"}})
	got, err := ReconcileStubs(StubReconcileInput{
		Spec: spec,
		Stories: []StubStory{
			{SpecRef: "spec/story-open", ACIDs: []string{"ac-1"}, Closed: false},
		},
	})
	if err != nil {
		t.Fatalf("ReconcileStubs: %v", err)
	}
	if got.Stubs[0].Bucket != StubUnreconciled {
		t.Fatalf("bucket = %s, want %s (story not closed)", got.Stubs[0].Bucket, StubUnreconciled)
	}
}

// TestReconcileStubs_UnplannedAddition proves a closed implementing story
// that traces to no stub at all is recorded as an unplanned addition, not
// an error, and does not block closure by itself.
func TestReconcileStubs_UnplannedAddition(t *testing.T) {
	spec := featureSpecWithStubs(t, artifact.Stub{Slug: "planned-stub", AcceptanceCriteria: []string{"ac-1"}})
	got, err := ReconcileStubs(StubReconcileInput{
		Spec: spec,
		Stories: []StubStory{
			{SpecRef: "spec/story-planned", ACIDs: []string{"ac-1"}, Closed: true},
			{SpecRef: "spec/story-surprise", ACIDs: []string{"ac-2"}, Closed: true}, // ac-2 is not any stub's AC
		},
	})
	if err != nil {
		t.Fatalf("ReconcileStubs: %v", err)
	}
	if got.Blocked {
		t.Fatal("Blocked = true, want false (an unplanned addition is not itself an error)")
	}
	if len(got.Unplanned) != 1 || got.Unplanned[0].SpecRef != "spec/story-surprise" {
		t.Fatalf("Unplanned = %+v, want exactly spec/story-surprise", got.Unplanned)
	}
}

// TestReconcileStubs_FullCoverageWinsOverWithdrawal proves the documented
// precedence: a stub both fully covered by closed stories and separately
// declared withdrawn resolves to realized, never withdrawn (a stub that
// demonstrably shipped cannot honestly be recorded as not-built).
func TestReconcileStubs_FullCoverageWinsOverWithdrawal(t *testing.T) {
	spec := featureSpecWithStubs(t, artifact.Stub{Slug: "s", AcceptanceCriteria: []string{"ac-1"}})
	got, err := ReconcileStubs(StubReconcileInput{
		Spec: spec,
		Stories: []StubStory{
			{SpecRef: "spec/story-a", ACIDs: []string{"ac-1"}, Closed: true},
		},
		Withdrawals: []StubWithdrawal{{Slug: "s", Note: "descoped"}},
	})
	if err != nil {
		t.Fatalf("ReconcileStubs: %v", err)
	}
	if got.Stubs[0].Bucket != StubRealized {
		t.Fatalf("bucket = %s, want %s", got.Stubs[0].Bucket, StubRealized)
	}
}

// --- Negative paths ---

func TestReconcileStubs_Negative(t *testing.T) {
	t.Run("nil spec", func(t *testing.T) {
		if _, err := ReconcileStubs(StubReconcileInput{}); err == nil {
			t.Fatal("ReconcileStubs(nil spec): want error, got nil")
		}
	})

	t.Run("wrong class", func(t *testing.T) {
		spec := featureSpecWithStubs(t, artifact.Stub{Slug: "s", AcceptanceCriteria: []string{"ac-1"}})
		spec.Class = artifact.ClassStory
		if _, err := ReconcileStubs(StubReconcileInput{Spec: spec}); err == nil {
			t.Fatal("ReconcileStubs(story-class spec): want error, got nil")
		}
	})

	t.Run("no stubs at all: nothing to reconcile, never blocked", func(t *testing.T) {
		spec := featureSpecWithStubs(t)
		got, err := ReconcileStubs(StubReconcileInput{Spec: spec})
		if err != nil {
			t.Fatalf("ReconcileStubs: %v", err)
		}
		if got.Blocked || len(got.Stubs) != 0 {
			t.Fatalf("got = %+v, want empty and unblocked", got)
		}
	})
}
