package diagramverify

import (
	"context"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

func classificationOf(t *testing.T, results []Result, identity string) Result {
	t.Helper()
	for _, r := range results {
		if r.Identity == identity {
			return r
		}
	}
	t.Fatalf("no result for identity %q in %+v", identity, results)
	return Result{}
}

// TestCompare_ThreeWayTable is obligation ac-3--behavioral cases (1)-(3):
// exists, proposed-new (no base), and kept-but-gone (base-inherited,
// truth-dropped).
func TestCompare_ThreeWayTable(t *testing.T) {
	cases := []struct {
		name     string
		proposal []string
		base     []string
		truth    map[string]bool
		identity string
		want     Classification
	}{
		{
			name:     "exists: proposal element present in truth",
			proposal: []string{"GetRefund"},
			base:     nil,
			truth:    map[string]bool{"GetRefund": true},
			identity: "GetRefund",
			want:     Exists,
		},
		{
			name:     "proposed-new: no base, absent from truth",
			proposal: []string{"NewHandler"},
			base:     nil,
			truth:    map[string]bool{"GetRefund": true},
			identity: "NewHandler",
			want:     ProposedNew,
		},
		{
			name:     "kept-but-gone: base-inherited, truth dropped it",
			proposal: []string{"LegacyStep"},
			base:     []string{"LegacyStep"},
			truth:    map[string]bool{"GetRefund": true},
			identity: "LegacyStep",
			want:     KeptButGone,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			results := Compare(tc.proposal, tc.base, tc.truth)
			got := classificationOf(t, results, tc.identity)
			if got.Classification != tc.want {
				t.Errorf("Classification = %q, want %q", got.Classification, tc.want)
			}
		})
	}
}

// TestCompare_Rename_TwoIndependentFacts is obligation ac-3--behavioral
// case (4): a proposal that dropped node A and added node B relative to
// its base — a human would call it a rename — must render as the two
// independent facts kept-but-gone(A) + proposed-new(B), never a single
// combined "renamed" fact.
func TestCompare_Rename_TwoIndependentFacts(t *testing.T) {
	base := []string{"A"}
	proposal := []string{"B"} // A dropped, B added
	truth := map[string]bool{}

	results := Compare(proposal, base, truth)
	if len(results) != 2 {
		t.Fatalf("results = %+v, want exactly 2 (kept-but-gone A, proposed-new B)", results)
	}
	a := classificationOf(t, results, "A")
	if a.Classification != KeptButGone {
		t.Errorf("A classification = %q, want kept-but-gone", a.Classification)
	}
	b := classificationOf(t, results, "B")
	if b.Classification != ProposedNew {
		t.Errorf("B classification = %q, want proposed-new", b.Classification)
	}
	// No fourth "renamed" value anywhere: every result is one of the three
	// closed Classification values.
	for _, r := range results {
		if r.Classification != Exists && r.Classification != ProposedNew && r.Classification != KeptButGone {
			t.Errorf("unexpected classification %q", r.Classification)
		}
	}
}

// TestCompare_BaseElementStillTrue_NoDisclosure: a base identity the
// current proposal no longer draws, but truth STILL has, is not disclosed
// at all — the proposal simply chose not to depict something real, not a
// contradiction.
func TestCompare_BaseElementStillTrue_NoDisclosure(t *testing.T) {
	base := []string{"StillReal"}
	proposal := []string{} // proposal dropped it from the drawing
	truth := map[string]bool{"StillReal": true}

	results := Compare(proposal, base, truth)
	if len(results) != 0 {
		t.Fatalf("results = %+v, want none (nothing to disclose)", results)
	}
}

// TestCompare_Deterministic proves two calls over identical inputs
// produce byte-identical (order-identical) results — CLAUDE.md
// determinism discipline.
func TestCompare_Deterministic(t *testing.T) {
	proposal := []string{"A", "B", "C"}
	base := []string{"A", "X"}
	truth := map[string]bool{"A": true}

	r1 := Compare(proposal, base, truth)
	r2 := Compare(proposal, base, truth)
	if len(r1) != len(r2) {
		t.Fatalf("len mismatch: %d vs %d", len(r1), len(r2))
	}
	for i := range r1 {
		if r1[i] != r2[i] {
			t.Fatalf("results[%d] differ: %+v vs %+v", i, r1[i], r2[i])
		}
	}
}

// TestCompareWithWitness_Resolved is obligation ac-3--behavioral's
// fixturegit-backed witness-resolution test: a repository with a scripted
// history where a known commit removed a known identity string — the
// comparison must name that exact commit sha as witness. Neither this nor
// the companion absence test below claims the resolved witness is a
// verified CAUSE of the removal (dc-4) — only that it is the most recent
// commit whose diff touched the identity string under the service
// directory.
func TestCompareWithWitness_Resolved(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"svc/a.go": "package svc\n\nfunc LegacyStep() {}\n"}, Message: "add LegacyStep"},
		{Files: map[string]string{"svc/a.go": "package svc\n"}, Message: "remove LegacyStep"},
	})

	base := []string{"LegacyStep"}
	proposal := []string{"LegacyStep"} // proposal kept drawing it, unedited
	truth := map[string]bool{}         // truth no longer has it

	results, err := CompareWithWitness(context.Background(), repo.Dir, proposal, base, truth, "svc")
	if err != nil {
		t.Fatalf("CompareWithWitness: %v", err)
	}
	r := classificationOf(t, results, "LegacyStep")
	if r.Classification != KeptButGone {
		t.Fatalf("Classification = %q, want kept-but-gone", r.Classification)
	}
	if r.Witness == nil {
		t.Fatal("Witness = nil, want the removal commit sha")
	}
	if *r.Witness != repo.Heads[1] {
		t.Errorf("Witness = %q, want %q (the removal commit)", *r.Witness, repo.Heads[1])
	}
}

// TestCompareWithWitness_Unresolved: no commit in the fixture history ever
// touched the identity string — the witness is disclosed absent (nil),
// never guessed or fabricated.
func TestCompareWithWitness_Unresolved(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"svc/a.go": "package svc\n\nfunc Alpha() {}\n"}, Message: "add Alpha"},
	})

	base := []string{"NeverMentioned"}
	proposal := []string{"NeverMentioned"}
	truth := map[string]bool{}

	results, err := CompareWithWitness(context.Background(), repo.Dir, proposal, base, truth, "svc")
	if err != nil {
		t.Fatalf("CompareWithWitness: %v", err)
	}
	r := classificationOf(t, results, "NeverMentioned")
	if r.Classification != KeptButGone {
		t.Fatalf("Classification = %q, want kept-but-gone", r.Classification)
	}
	if r.Witness != nil {
		t.Fatalf("Witness = %q, want nil (unresolved)", *r.Witness)
	}
}
