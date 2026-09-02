package constitutionapp

import (
	"context"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyconflict"
)

func TestImpactReview_NoTargetsStillDiffsLayers(t *testing.T) {
	root := buildFixtureRepo(t)
	svc := testService()
	ctx := context.Background()

	content := strings.Replace(newOverlayContent(t, root), `values: ["1.25"]`, `values: ["1.25", "1.24"]`, 1)
	if _, typed := svc.Propose(ctx, root, ProposeRequest{
		Branch: "policy/widen-frontend", Kind: KindOverlay, Name: "frontend-go-version",
		Content: []byte(content), Expected: Expected{Branch: "policy/widen-frontend"},
	}); typed != nil {
		t.Fatalf("Propose: %v", typed)
	}

	review, typed := svc.ImpactReview(ctx, root, ImpactReviewRequest{})
	if typed != nil {
		t.Fatalf("ImpactReview: %v", typed)
	}
	if len(review.Conflicts) != 0 || len(review.AffectedConsumers) != 0 {
		t.Fatalf("expected no declared targets, got %+v / %+v", review.Conflicts, review.AffectedConsumers)
	}
	found := false
	for _, l := range review.Layers {
		if l.Kind == KindOverlay && l.Change == "changed" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a changed overlay layer, got %+v", review.Layers)
	}
}

// TestImpactReview_UnknownScopeTarget proves the required unknown-scope
// test scenario: a declared target whose Phase is outside its own declared
// Scope.Phases is a canonical, schema-valid request the compiler still
// cannot accept (contextcompile.PhaseScopeRefusal) — recorded as a
// per-target refusal, never fabricated as a pass and never aborting the
// whole review.
func TestImpactReview_UnknownScopeTarget(t *testing.T) {
	root := buildFixtureRepo(t)
	svc := testService()

	review, typed := svc.ImpactReview(context.Background(), root, ImpactReviewRequest{
		Targets: []ImpactTarget{{
			Spec:    "spec/does-not-matter",
			Phase:   contextcompile.PhaseBuild,
			Adapter: contextcompile.AdapterRef{ID: "codex", Version: "1"},
			Scope:   policyartifact.Scope{Phases: []string{"design"}, Environments: []string{}, Paths: []string{}, Refs: []string{}},
		}},
	})
	if typed != nil {
		t.Fatalf("ImpactReview: %v", typed)
	}
	if len(review.Conflicts) != 1 {
		t.Fatalf("expected exactly one target conflict row, got %d", len(review.Conflicts))
	}
	if review.Conflicts[0].Refusal == "" || !strings.Contains(review.Conflicts[0].Refusal, "unknown-scope") {
		t.Fatalf("expected an unknown-scope refusal, got %+v", review.Conflicts[0])
	}
	if len(review.AffectedConsumers) != 0 {
		t.Fatalf("a refused target must not count as an affected consumer, got %v", review.AffectedConsumers)
	}
}

// TestImpactReview_UnavailableJudgeTarget proves the required
// unavailable-judge test scenario: a target whose semantic evaluation is
// required (internal/policyconflict's own len(Claims)>=2 rule) but for
// which this package wires no Primary judge (conflict.go's disclosed v1
// limitation) completes as policyconflict.VerdictBlockedUnproven with
// policyconflict.ReasonJudgeUnavailable on the semantic row — a real,
// three-valued-unproven outcome internal/policyconflict itself already
// defines, never fabricated by this package.
func TestImpactReview_UnavailableJudgeTarget(t *testing.T) {
	root := buildConflictFixtureRepo(t)
	svc := testService()

	review, typed := svc.ImpactReview(context.Background(), root, ImpactReviewRequest{
		Targets: []ImpactTarget{{
			Spec:    "spec/operand-feature",
			Phase:   contextcompile.PhaseDesign,
			Adapter: contextcompile.AdapterRef{ID: "codex", Version: "1"},
			Scope:   policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{"cmd/"}, Refs: []string{}},
		}},
	})
	if typed != nil {
		t.Fatalf("ImpactReview: %v", typed)
	}
	if len(review.Conflicts) != 1 {
		t.Fatalf("expected exactly one target conflict row, got %d", len(review.Conflicts))
	}
	tc := review.Conflicts[0]
	if tc.Refusal != "" {
		t.Fatalf("expected a completed report, not a refusal: %q", tc.Refusal)
	}
	if tc.Report == nil {
		t.Fatal("expected a non-nil report")
	}
	if tc.Report.Verdict != policyconflict.VerdictBlockedUnproven {
		t.Fatalf("verdict = %q, want %q", tc.Report.Verdict, policyconflict.VerdictBlockedUnproven)
	}
	if len(tc.Report.Semantic) != 1 {
		t.Fatalf("expected one semantic row, got %d: %+v", len(tc.Report.Semantic), tc.Report.Semantic)
	}
	found := false
	for _, reason := range tc.Report.Semantic[0].Reasons {
		if reason == policyconflict.ReasonJudgeUnavailable {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected reason %q, got %v", policyconflict.ReasonJudgeUnavailable, tc.Report.Semantic[0].Reasons)
	}
	if len(review.AffectedConsumers) != 1 || review.AffectedConsumers[0] != "spec/operand-feature" {
		t.Fatalf("affected consumers = %v", review.AffectedConsumers)
	}
}

// TestImpactReview_UnauthorizedExemptionTarget proves the required
// unauthorized-exemption test scenario: a mechanically PROVEN conflict
// (AC-3) whose only applicable exemption cannot authorize covering it stays
// blocked-violated, with the exemption resolution recorded and visible
// rather than silently applied.
//
// The failing authority substate is AUTHORIZATION, not scope: the fixture's
// exemption is deliberately declared at the same UNIVERSAL scope as the
// conflict itself (buildUnauthorizedExemptionFixtureRepo, fixture_test.go),
// so Match/Freshness/Scope/Bound are all trivially proven and the single
// isolated failure is its approval naming a role/principal pair the
// governing solo-default profile does not map.
func TestImpactReview_UnauthorizedExemptionTarget(t *testing.T) {
	root := buildUnauthorizedExemptionFixtureRepo(t)
	svc := testService()

	review, typed := svc.ImpactReview(context.Background(), root, ImpactReviewRequest{
		Targets: []ImpactTarget{{
			Spec:    "spec/operand-feature",
			Phase:   contextcompile.PhaseDesign,
			Adapter: contextcompile.AdapterRef{ID: "codex", Version: "1"},
			Scope:   policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}},
		}},
	})
	if typed != nil {
		t.Fatalf("ImpactReview: %v", typed)
	}
	if len(review.Conflicts) != 1 || review.Conflicts[0].Refusal != "" {
		t.Fatalf("expected exactly one completed target conflict, got %+v", review.Conflicts)
	}
	report := review.Conflicts[0].Report
	if report == nil {
		t.Fatal("expected a non-nil report")
	}
	if report.Verdict != policyconflict.VerdictBlockedViolated {
		t.Fatalf("verdict = %q, want %q (mechanical conflict): %+v", report.Verdict, policyconflict.VerdictBlockedViolated, report.Mechanical)
	}
	var row *policyconflict.MechanicalEvaluation
	for i := range report.Mechanical {
		if report.Mechanical[i].State == policyconflict.ProofViolatedWithWitness {
			row = &report.Mechanical[i]
		}
	}
	if row == nil {
		t.Fatalf("expected one violated-with-witness mechanical row, got %+v", report.Mechanical)
	}
	if len(row.Exemptions) == 0 {
		t.Fatalf("expected the universal-legacy-go exemption to be recorded as applicable but unauthorized, got none")
	}
	exemption := row.Exemptions[0]
	if exemption.Resolution.Authorization != policyconflict.ProofUnproven && exemption.Resolution.Authorization != policyconflict.ProofViolatedWithWitness {
		t.Fatalf("expected the exemption's Authorization substate to fail, got %+v", exemption.Resolution)
	}
	if !containsReasonCode(row.Reasons, policyconflict.ReasonExemptionIneffective) && !containsReasonCode(row.Reasons, policyconflict.ReasonMechanicalConflict) {
		t.Fatalf("reasons = %v, want mechanical-conflict and/or exemption-ineffective", row.Reasons)
	}
}

func containsReasonCode(reasons []policyconflict.ReasonCode, want policyconflict.ReasonCode) bool {
	for _, r := range reasons {
		if r == want {
			return true
		}
	}
	return false
}

func TestImpactReview_InputInvalid(t *testing.T) {
	svc := testService()
	if _, typed := svc.ImpactReview(context.Background(), "", ImpactReviewRequest{}); typed == nil || typed.Classification != ClassificationVerdict {
		t.Fatalf("expected verdict for missing root, got %+v", typed)
	}
	if _, typed := svc.ImpactReview(context.Background(), "x", ImpactReviewRequest{Targets: []ImpactTarget{{}}}); typed == nil || typed.Classification != ClassificationVerdict {
		t.Fatalf("expected verdict for a target missing spec, got %+v", typed)
	}
}
