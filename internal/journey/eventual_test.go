package journey

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/policyconflict"
)

// indexBlockers maps blockers by ID for by-ID assertions in the tests
// below — the eventual-blocker sibling of derive_test.go's blockerIDs.
func indexBlockers(blockers []Blocker) map[string]Blocker {
	out := make(map[string]Blocker, len(blockers))
	for _, b := range blockers {
		out[b.ID] = b
	}
	return out
}

func TestDeriveEventual_StubsAndFloor(t *testing.T) {
	owner := Owner{Declared: "platform-team", Attribution: governanceprincipal.NewUnauthenticatedAttribution()}
	in := eventualInput{
		Class: "feature", Owner: owner, LaterTransitions: []model.Transition{{Verb: "close"}},
		Stubs: &evidence.StubReconciliation{Stubs: []evidence.StubResult{{Slug: "board-tab", Bucket: evidence.StubUnreconciled}, {Slug: "core", Bucket: evidence.StubRealized, RealizedBy: []string{"spec/core"}}}},
		Fold:  &evidence.FeatureResult{ACs: []evidence.FeatureACResult{{ID: "ac-1", Floor: evidence.FloorResult{Satisfied: false, DeclaresAttestation: true, Attestation: evidence.AttestationAbsent}}, {ID: "ac-2", Floor: evidence.FloorResult{Satisfied: true}}}},
	}
	eb := deriveEventual(in)
	if !eb.Derived || len(eb.Disclosures) != 1 || !strings.Contains(eb.Disclosures[0], "no policy-conflict report") {
		t.Fatalf("eventual = %+v", eb)
	}
	ids := blockerIDs(eb.Items)
	want := []string{"outcome-floor/ac-1", "stub-unreconciled/board-tab"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
	byID := indexBlockers(eb.Items)
	if b := byID["stub-unreconciled/board-tab"]; b.Reason != ReasonStubUnreconciled || b.Class != ClassMechanical || b.Transition != "close" || !strings.Contains(b.ClearingCondition, "board-tab") || len(b.Witnesses) == 0 {
		t.Fatalf("stub blocker = %+v", b)
	}
	if b := byID["outcome-floor/ac-1"]; b.Reason != ReasonOutcomeFloorUnsatisfied || b.Class != ClassJudgmental || !strings.Contains(b.ClearingCondition, "attestations/") {
		t.Fatalf("floor blocker = %+v", b)
	}
	if err := (Blockers{Current: []Blocker{}, Eventual: eb}).validate(); err != nil {
		t.Fatal(err)
	}
}

// TestDeriveEventual_ClaimedQuestions proves the question-claimed-by-spike
// source (R-RR1-1): a declared open question named by a spike stub's
// resolves edge is a debt (the spike's own resolution is what will clear
// it); an open question no spike stub claims is NOT a journey blocker at
// all — it stays the readiness-shape concern it already is, so it is
// never counted twice (the plan's own R-RR1-1 note).
func TestDeriveEventual_ClaimedQuestions(t *testing.T) {
	owner := testOwner()
	spec := &artifact.SpecFrontmatter{
		OpenQuestions: []artifact.OpenQuestion{
			{ID: "oq-1", Text: "which route?", Anchor: "#oq-1"},
			{ID: "oq-2", Text: "unclaimed question", Anchor: "#oq-2"},
		},
		Stubs: []artifact.Stub{
			{Slug: "s", Spike: true, Resolves: []string{"oq-1"}},
		},
	}
	in := eventualInput{
		Class: "feature", Owner: owner, LaterTransitions: []model.Transition{{Verb: "close"}},
		Spec: spec,
	}
	eb := deriveEventual(in)
	ids := blockerIDs(eb.Items)
	want := []string{"question-claimed/oq-1"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids = %v, want %v (an unclaimed open question must produce NO item)", ids, want)
	}
	byID := indexBlockers(eb.Items)
	b := byID["question-claimed/oq-1"]
	if b.Reason != ReasonQuestionClaimedBySpike || b.Class != ClassMechanical || b.Transition != "close" {
		t.Fatalf("claimed-question blocker = %+v", b)
	}
	found := false
	for _, w := range b.Witnesses {
		if strings.Contains(w, "s") {
			found = true
		}
	}
	if !found {
		t.Fatalf("witnesses = %v, want at least one naming stub %q", b.Witnesses, "s")
	}
	if err := (Blockers{Current: []Blocker{}, Eventual: eb}).validate(); err != nil {
		t.Fatal(err)
	}
}

// TestDeriveEventual_LaterTransitionObligations proves the obligation and
// principal sources bound to a NON-candidate later transition: the SAME
// obligationBlockerID/obligationReason/transitionHasCountersign machinery
// deriveBlockers uses for current blockers, applied to a later transition
// instead — so the resulting IDs differ from any current blocker's ID by
// the verb segment alone, and a later transition that is ALSO (defensively)
// a candidate is skipped rather than duplicated.
func TestDeriveEventual_LaterTransitionObligations(t *testing.T) {
	owner := testOwner()
	acceptCandidate := model.Transition{
		Verb: "accept", From: "draft", To: "accepted",
		Obligations: []model.Obligation{{Scheme: "attestation", Kind: "author-vouch"}},
	}
	closeLater := model.Transition{
		Verb: "close", From: "accepted", To: "closed",
		Obligations: []model.Obligation{
			{Scheme: "attestation", Kind: "countersign", Count: 1},
			{Scheme: "behavioral", Kind: "fold-green"},
		},
	}
	in := eventualInput{
		Class:      "feature",
		Owner:      owner,
		Candidates: []model.Transition{acceptCandidate},
		// acceptCandidate is included AGAIN here (defensively) to prove a
		// later transition that duplicates a candidate is skipped, never
		// double-derived.
		LaterTransitions: []model.Transition{acceptCandidate, closeLater},
	}
	eb := deriveEventual(in)
	ids := blockerIDs(eb.Items)

	wantIDs := []string{
		"obligation-countersign-unproven/close/attestation/countersign",
		"obligation-fold-green-unproven/close/behavioral/fold-green",
		"principal-resolution-unproven/close",
	}
	for _, id := range wantIDs {
		found := false
		for _, got := range ids {
			if got == id {
				found = true
			}
		}
		if !found {
			t.Errorf("items = %v, want to contain %q", ids, id)
		}
	}
	unwantedID := "obligation-author-vouch-unproven/accept/attestation/author-vouch"
	for _, got := range ids {
		if got == unwantedID {
			t.Errorf("items = %v, must NOT duplicate the candidate transition's own obligation as an eventual item", ids)
		}
	}
	byID := indexBlockers(eb.Items)
	if b := byID["obligation-countersign-unproven/close/attestation/countersign"]; b.Class != ClassGovernance || b.Transition != "close" {
		t.Fatalf("later countersign obligation blocker = %+v", b)
	}
	if b := byID["obligation-fold-green-unproven/close/behavioral/fold-green"]; b.Class != ClassMechanical || b.Transition != "close" {
		t.Fatalf("later fold-green obligation blocker = %+v", b)
	}
	if b := byID["principal-resolution-unproven/close"]; b.Class != ClassGovernance || b.Transition != "close" {
		t.Fatalf("later principal-resolution blocker = %+v", b)
	}
	if err := (Blockers{Current: []Blocker{}, Eventual: eb}).validate(); err != nil {
		t.Fatal(err)
	}
}

// TestDeriveEventual_ConflictReport proves the three conflict-report
// sources (mechanical, semantic, exemption): each fires only when the
// report is supplied (R-RR1-2), only for a non-proven row/resolution, and
// a nil report yields none of them plus the fixed disclosure.
func TestDeriveEventual_ConflictReport(t *testing.T) {
	owner := testOwner()
	report := &policyconflict.Report{
		Mechanical: []policyconflict.MechanicalEvaluation{
			{
				ID:      "mech-1",
				State:   policyconflict.ProofViolatedWithWitness,
				Reasons: []policyconflict.ReasonCode{policyconflict.ReasonMechanicalConflict},
				Exemptions: []policyconflict.ExemptionResolution{
					{
						ID: "ex-1",
						Resolution: policyconflict.AuthorityResolution{
							Match: policyconflict.ProofProven, Freshness: policyconflict.ProofUnproven,
							Scope: policyconflict.ProofProven, Bound: policyconflict.ProofUnproven, Authorization: policyconflict.ProofProven,
						},
					},
				},
			},
		},
		Semantic: []policyconflict.SemanticEvaluation{
			{ID: "sem-1", State: policyconflict.ProofUnproven, Reasons: []policyconflict.ReasonCode{policyconflict.ReasonJudgeUnavailable}},
		},
	}

	t.Run("report supplied", func(t *testing.T) {
		in := eventualInput{Class: "feature", Owner: owner, LaterTransitions: []model.Transition{{Verb: "accept"}}, Conflict: report}
		eb := deriveEventual(in)
		if !eb.Derived {
			t.Fatal("Derived must be true")
		}
		ids := blockerIDs(eb.Items)
		want := []string{"conflict-mechanical/mech-1", "conflict-semantic/sem-1", "exemption-ineffective/ex-1"}
		if !reflect.DeepEqual(ids, want) {
			t.Fatalf("ids = %v, want %v", ids, want)
		}
		byID := indexBlockers(eb.Items)
		if b := byID["conflict-mechanical/mech-1"]; b.Reason != ReasonConflictMechanicalUnresolved || b.Class != ClassMechanical || b.Transition != "accept" {
			t.Fatalf("mechanical blocker = %+v", b)
		}
		if b := byID["conflict-semantic/sem-1"]; b.Reason != ReasonConflictSemanticUnresolved || b.Class != ClassJudgmental || b.Transition != "accept" {
			t.Fatalf("semantic blocker = %+v", b)
		}
		if b := byID["exemption-ineffective/ex-1"]; b.Reason != ReasonExemptionIneffective || b.Class != ClassGovernance || b.Transition != "accept" {
			t.Fatalf("exemption blocker = %+v", b)
		}
		for _, d := range eb.Disclosures {
			if strings.Contains(d, "no policy-conflict report") {
				t.Fatalf("disclosures = %v, must not disclose a missing report when one was supplied", eb.Disclosures)
			}
		}
		if err := (Blockers{Current: []Blocker{}, Eventual: eb}).validate(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("nil report", func(t *testing.T) {
		in := eventualInput{Class: "feature", Owner: owner, LaterTransitions: []model.Transition{{Verb: "accept"}}, Conflict: nil}
		eb := deriveEventual(in)
		if !eb.Derived {
			t.Fatal("Derived must be true even with no conflict report")
		}
		if len(eb.Items) != 0 {
			t.Fatalf("items = %v, want none of the conflict-derived items with a nil report", blockerIDs(eb.Items))
		}
		if len(eb.Disclosures) != 1 || !strings.Contains(eb.Disclosures[0], "no policy-conflict report") {
			t.Fatalf("disclosures = %v, want exactly the no-report disclosure", eb.Disclosures)
		}
	})
}

// TestDeriveEventual_StoryClassSkipsFeatureSources proves R-RR1-3's class
// gate: the three feature-only sources (stub reconciliation, the outcome
// floor, claimed questions) never fire for class story, even when their
// facts are (incorrectly) supplied.
func TestDeriveEventual_StoryClassSkipsFeatureSources(t *testing.T) {
	owner := testOwner()
	in := eventualInput{
		Class: "story", Owner: owner, LaterTransitions: []model.Transition{{Verb: "close"}},
		Stubs: &evidence.StubReconciliation{Stubs: []evidence.StubResult{{Slug: "board-tab", Bucket: evidence.StubUnreconciled}}},
		Fold:  &evidence.FeatureResult{ACs: []evidence.FeatureACResult{{ID: "ac-1", Floor: evidence.FloorResult{Satisfied: false}}}},
		Spec: &artifact.SpecFrontmatter{
			OpenQuestions: []artifact.OpenQuestion{{ID: "oq-1", Text: "x", Anchor: "#oq-1"}},
			Stubs:         []artifact.Stub{{Slug: "s", Spike: true, Resolves: []string{"oq-1"}}},
		},
	}
	eb := deriveEventual(in)
	if len(eb.Items) != 0 {
		t.Fatalf("items = %v, want none: class story must skip every feature-only source", blockerIDs(eb.Items))
	}
	if !eb.Derived {
		t.Fatal("Derived must still be true for class story")
	}
}

// TestDeriveEventual_OrderingAndUniqueness proves items are returned
// strictly ascending by ID, and that Blockers.validate refuses an eventual
// item whose ID collides with a current blocker's ID (the cross-section
// uniqueness rule record.go already enforces; this proves deriveEventual's
// own output composes with it correctly).
func TestDeriveEventual_OrderingAndUniqueness(t *testing.T) {
	owner := testOwner()
	in := eventualInput{
		Class: "feature", Owner: owner, LaterTransitions: []model.Transition{{Verb: "close"}},
		Stubs: &evidence.StubReconciliation{Stubs: []evidence.StubResult{
			{Slug: "zzz-last", Bucket: evidence.StubUnreconciled},
			{Slug: "aaa-first", Bucket: evidence.StubUnreconciled},
		}},
		Fold: &evidence.FeatureResult{ACs: []evidence.FeatureACResult{{ID: "ac-1", Floor: evidence.FloorResult{Satisfied: false}}}},
	}
	eb := deriveEventual(in)
	ids := blockerIDs(eb.Items)
	if !sort.StringsAreSorted(ids) {
		t.Fatalf("items not ascending: %v", ids)
	}
	if len(ids) < 2 {
		t.Fatalf("want multiple items to prove ordering, got %v", ids)
	}

	current := []Blocker{eb.Items[0]}
	if err := (Blockers{Current: current, Eventual: eb}).validate(); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("validate() = %v, want a duplicate-id refusal when an eventual item's ID equals a current blocker's ID", err)
	}
}
