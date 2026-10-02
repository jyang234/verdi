package readinesspilot

import (
	"testing"

	"github.com/jyang234/verdi/internal/journey"
)

// TestConcernHumanReview_TruthTable is SI-338 (3)'s rule as a truth table
// over every family of the closed vocabulary and every work class (the
// empty class of a non-journey row included): HumanReview is true exactly
// for review/role/* and for work class governance (principal resolution,
// countersign, exemption), and false otherwise — the judgmental rows
// (author vouch, outcome floor, semantic judge) included. It is a pure
// function of the id and the work class: state, area, blocking, timing,
// summary, and guidance never move it.
func TestConcernHumanReview_TruthTable(t *testing.T) {
	t.Parallel()

	ids := []string{
		"shape/problem",
		"shape/outcome",
		"shape/provenance",
		"shape/mutation",
		"shape/board",
		"shape/question/oq-1",
		"shape/board/question/a-01ARZ3NDEKTSV4RRFFQ69G5FAV",
		"shape/board/agent-task/a-01ARZ3NDEKTSV4RRFFQ69G5FAV",
		"success/criteria",
		"success/coverage/ac-1",
		"success/contributor/static",
		"success/blocker/obligation-quality/ac-1/static",
		"context/verdict",
		"context/mechanical/configuration:go-version#complete",
		"context/semantic/sha256-abc",
		"context/disclosure/repository-remote-unknown",
		"review/blocker/principal-resolution-unproven/close",
		"review/blocker/obligation-countersign-unproven/close/attestation/countersign",
		"review/blocker/obligation-author-vouch-unproven/merge/attestation/author-vouch",
		"review/blocker/outcome-floor/ac-1",
		"review/blocker/conflict-semantic/sha256-abc",
		"review/blocker/exemption-ineffective/legacy",
		"review/role/close/attestation/countersign",
		"review/role/merge/attestation/author-vouch",
		"review/action",
		"review/eventual-derivation",
	}
	classes := []journey.BlockerClass{
		"",
		journey.ClassMechanical,
		journey.ClassJudgmental,
		journey.ClassGovernance,
		journey.ClassExternalWait,
		journey.ClassUnknown,
	}
	families := map[concernFamily]bool{}
	for _, id := range ids {
		families[classifyConcern(id)] = true
		for _, class := range classes {
			want := class == journey.ClassGovernance || len(id) > len("review/role/") && id[:len("review/role/")] == "review/role/"
			for _, state := range []State{StateProven, StateViolated, StateUnproven} {
				c := Concern{ID: id, WorkClass: class, State: state, Blocking: state == StateViolated, Guidance: "any", Summary: "any"}
				if got := c.HumanReview(); got != want {
					t.Errorf("Concern{ID: %q, WorkClass: %q, State: %q}.HumanReview() = %v, want %v", id, class, state, got, want)
				}
			}
		}
	}
	for family := familyUnknown + 1; family < familyEnd; family++ {
		if !families[family] {
			t.Errorf("the truth table omits concern family %v", family)
		}
	}
}

// TestDeriveHumanReviewRows proves the rule over a derivation: the derived
// role row and the governance blocker read as human review, and the
// judgmental blockers (author vouch, outcome floor) do not.
func TestDeriveHumanReviewRows(t *testing.T) {
	t.Parallel()

	fixture := mustJourney(t)
	owner := fixture.Blockers.Current[0].Owner
	in := baseInput(t)
	in.Journey.Principals.Required = []journey.RequiredRole{
		{Transition: "close", Obligation: "attestation/countersign", Count: 1, Resolution: "unproven"},
	}
	in.Journey.Principals.Disclosures = []string{"authenticated principal resolution and profile-contributed requirements remain unproven"}
	in.Journey.Blockers.Eventual.Items = []journey.Blocker{
		{
			ID: "obligation-author-vouch-unproven/merge/attestation/author-vouch", Reason: journey.ReasonObligationAuthorVouchUnproven,
			Class: journey.ClassJudgmental, Witnesses: []string{"author vouch is absent"}, Owner: owner,
			ClearingCondition: "obligation attestation/author-vouch is proven for transition merge", Transition: "merge",
		},
		{
			ID: "obligation-countersign-unproven/close/attestation/countersign", Reason: journey.ReasonObligationCountersignUnproven,
			Class: journey.ClassGovernance, Witnesses: []string{"countersign is absent"}, Owner: owner,
			ClearingCondition: "obligation attestation/countersign is proven for transition close", Transition: "close",
		},
		fixture.Blockers.Eventual.Items[0],
	}
	if err := in.Journey.Validate(); err != nil {
		t.Fatalf("journey fixture Validate() = %v", err)
	}
	snapshot := mustDerive(t, in)
	want := map[string]bool{
		"review/role/close/attestation/countersign":                                      true,
		"review/blocker/obligation-countersign-unproven/close/attestation/countersign":   true,
		"review/blocker/obligation-author-vouch-unproven/merge/attestation/author-vouch": false,
		"review/blocker/outcome-floor/ac-2":                                              false,
		"review/action":                                                                  false,
		"shape/problem":                                                                  false,
	}
	for id, wantHuman := range want {
		if got := mustConcern(t, snapshot, id).HumanReview(); got != wantHuman {
			t.Errorf("derived %q HumanReview() = %v, want %v", id, got, wantHuman)
		}
	}
}
