package policyconflict

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/policyartifact"
)

func TestReportDeterminismRepeatedServiceEvaluation(t *testing.T) {
	judge := serviceNoConflictJudge()
	service, _ := newServiceFixture(t, judge)

	first, err := service.Evaluate(context.Background(), serviceAcceptedRequest())
	if err != nil {
		t.Fatalf("first Evaluate: %v", err)
	}
	second, err := service.Evaluate(context.Background(), serviceAcceptedRequest())
	if err != nil {
		t.Fatalf("second Evaluate: %v", err)
	}
	assertSameReportBytes(t, first.ReportBytes, second.ReportBytes)
	if first.Report.Digest != second.Report.Digest {
		t.Fatalf("report digests differ: %q vs %q", first.Report.Digest, second.Report.Digest)
	}
}

func TestReportDeterminismDisclosurePermutations(t *testing.T) {
	a := Disclosure{Code: contextcompile.DisclosureRepositoryRemoteUnknown, Witnesses: []string{"b", "c"}}
	b := Disclosure{Code: DisclosureSoloPrincipalCollapse, Witnesses: []string{"a"}}
	first, err := mergeReportDisclosures([]Disclosure{b, a}, []Disclosure{{Code: a.Code, Witnesses: []string{"a", "b"}}})
	if err != nil {
		t.Fatalf("merge first: %v", err)
	}
	second, err := mergeReportDisclosures([]Disclosure{{Code: a.Code, Witnesses: []string{"a", "b"}}}, []Disclosure{a, b})
	if err != nil {
		t.Fatalf("merge second: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("permuted disclosure merge differs: %+v vs %+v", first, second)
	}
	if len(first) != 2 || !reflect.DeepEqual(first[0].Witnesses, []string{"a", "b", "c"}) {
		t.Fatalf("merged disclosures = %+v, want sorted/deduplicated witnesses", first)
	}
}

func TestReportDeterminismCompilerDisclosureClassification(t *testing.T) {
	codes := []contextcompile.DisclosureCode{
		contextcompile.DisclosureActorResolutionUnproven,
		contextcompile.DisclosureRepositoryRemoteUnknown,
		contextcompile.DisclosureRepositoryBranchUnknown,
		contextcompile.DisclosureRepositoryHeadUnknown,
		contextcompile.DisclosureDefaultBranchUnknown,
		contextcompile.DisclosureDefaultRelationshipUnknown,
		contextcompile.DisclosureDirtyStateUnknown,
		contextcompile.DisclosureStagedStateUnknown,
		contextcompile.DisclosureFreshnessUnknown,
		contextcompile.DisclosureApplicabilityUnknown,
		contextcompile.DisclosureReviewResultDiffUnproven,
		contextcompile.DisclosureReviewEvidenceBundleUnproven,
		contextcompile.DisclosureReviewBuilderReceiptUnproven,
		contextcompile.DisclosureOpaqueHarnessVendorBase,
	}
	for _, code := range codes {
		t.Run(string(code), func(t *testing.T) {
			disclosures, err := compilerDisclosures([]contextcompile.DisclosureCode{code})
			if err != nil {
				t.Fatalf("compilerDisclosures: %v", err)
			}
			if len(disclosures) != 1 || disclosures[0].Code != code || disclosures[0].Witnesses == nil {
				t.Fatalf("disclosures = %+v, want code preserved with explicit witnesses", disclosures)
			}
			want := VerdictPass
			if blockingCompilerDisclosure(code) {
				want = VerdictBlockedUnproven
			}
			if got := reportVerdict(nil, nil, disclosures); got != want {
				t.Fatalf("reportVerdict = %q, want %q", got, want)
			}
		})
	}
}

func TestReportDeterminismViolatedOutranksUnproven(t *testing.T) {
	mechanical := []MechanicalEvaluation{{State: ProofViolatedWithWitness}}
	semantic := []SemanticEvaluation{{State: ProofUnproven}}
	if got := reportVerdict(mechanical, semantic, nil); got != VerdictBlockedViolated {
		t.Fatalf("reportVerdict = %q, want %q", got, VerdictBlockedViolated)
	}
}

func TestReportDeterminismRejectsFavorableVerdictForgery(t *testing.T) {
	base := mustReadFixture(t, "report.json")
	forged := forgedReport(t, base, []any{"verdict"}, string(VerdictPass))
	if _, err := DecodeReport(forged); err == nil || !strings.Contains(err.Error(), "derived verdict") {
		t.Fatalf("DecodeReport(freshly re-digested favorable lie) error = %v, want derived-verdict refusal", err)
	}

	report, err := DecodeReport(base)
	if err != nil {
		t.Fatalf("DecodeReport(fixture): %v", err)
	}
	report.Verdict = VerdictPass
	if _, err := EncodeReport(report); err == nil || !strings.Contains(err.Error(), "derived verdict") {
		t.Fatalf("EncodeReport(favorable lie) error = %v, want derived-verdict refusal", err)
	}
}

func TestDecodeReportRejectsSelfRedigestedMechanicalOutcomeForgery(t *testing.T) {
	base, err := DecodeReport(mustReadFixture(t, "report.json"))
	if err != nil {
		t.Fatalf("DecodeReport(fixture): %v", err)
	}
	row := violatedRow(t)
	if row.After.State != SolverUnsatisfiable {
		t.Fatalf("test setup: After.State = %q, want unsatisfiable", row.After.State)
	}
	base.Mechanical = []MechanicalEvaluation{row}
	base.Semantic = []SemanticEvaluation{}
	base.Disclosures = []Disclosure{}
	base.Verdict = VerdictBlockedViolated
	blocked, err := EncodeReport(base)
	if err != nil {
		t.Fatalf("EncodeReport(conforming blocked report): %v", err)
	}

	// Keep the exact unsatisfiable post-exemption proof, but lie that the row
	// is mechanically satisfiable and therefore that the report passes. The
	// public self-digest is recomputed over those canonical forged bytes, so
	// only independent cross-field proof validation can reject the lie.
	tree := setAtPath(t, blocked, []any{"mechanical", 0, "state"}, string(ProofProven))
	setAtPathIn(t, tree, []any{"mechanical", 0, "reasons"}, []any{string(ReasonMechanicalSatisfiable)})
	setAtPathIn(t, tree, []any{"verdict"}, string(VerdictPass))
	forged := redigestTopLevel(t, tree)
	if _, err := DecodeReport(forged); err == nil {
		t.Fatal("DecodeReport(self-redigested favorable mechanical outcome forgery): got nil error, want cross-field rejection")
	}
}

func TestMechanicalOutcomeValidationCoversConformingProofResults(t *testing.T) {
	satisfiable := evaluateOneRow(t, MechanicalInput{Claims: []contextcompile.TypedClaim{
		typedClaim(t, "policy-a", discreteClaim("c1", "level", policyartifact.OpEquals, []string{"gold"}, universalScope())),
	}})
	disjoint := evaluateOneRow(t, MechanicalInput{Claims: []contextcompile.TypedClaim{
		typedClaim(t, "policy-a", discreteClaim("c1", "level", policyartifact.OpEquals, []string{"gold"}, phaseScope("build"))),
		typedClaim(t, "policy-b", discreteClaim("c2", "level", policyartifact.OpEquals, []string{"silver"}, phaseScope("review"))),
	}})
	unsatisfiable := violatedRow(t)
	profile := mustDecodeProfile(t, rolePolicyYAML)
	unproven := evaluateOneRow(t, MechanicalInput{Claims: []contextcompile.TypedClaim{
		typedClaim(t, "policy-a", principalClaim("c1", "release", policyartifact.OpDifferentPrincipal, "author", "reviewer", universalScope())),
	}, Profile: profile})

	var silver TypedClaimRecord
	for _, claim := range unsatisfiable.Claims {
		if claim.PolicyID == "policy-b" {
			silver = claim
		}
	}
	effective, err := applyEffectiveExemptions(unsatisfiable, []ExemptionResolution{
		exemptionFor("ex-effective", allProvenResolution(), silver),
	})
	if err != nil {
		t.Fatalf("ApplyEffectiveExemptions(effective): %v", err)
	}
	rejected, err := applyEffectiveExemptions(unsatisfiable, []ExemptionResolution{
		rejectedExemption("ex-rejected", AuthorityResolution{
			Match: ProofProven, Freshness: ProofProven, Scope: ProofProven,
			Bound: ProofUnproven, Authorization: ProofProven,
		}),
	})
	if err != nil {
		t.Fatalf("ApplyEffectiveExemptions(rejected): %v", err)
	}

	partiallyRemoved := evaluateOneRow(t, MechanicalInput{Claims: []contextcompile.TypedClaim{
		typedClaim(t, "policy-a", discreteClaim("c1", "level", policyartifact.OpEquals, []string{"gold"}, universalScope())),
		typedClaim(t, "policy-b", discreteClaim("c2", "level", policyartifact.OpEquals, []string{"silver"}, universalScope())),
		typedClaim(t, "policy-c", discreteClaim("c3", "level", policyartifact.OpAllowedValues, []string{"gold", "silver"}, universalScope())),
	}})
	var harmless TypedClaimRecord
	for _, claim := range partiallyRemoved.Claims {
		if claim.PolicyID == "policy-c" {
			harmless = claim
		}
	}
	acceptedIneffective, err := applyEffectiveExemptions(partiallyRemoved, []ExemptionResolution{
		exemptionFor("ex-insufficient", allProvenResolution(), harmless),
	})
	if err != nil {
		t.Fatalf("ApplyEffectiveExemptions(insufficient): %v", err)
	}

	tests := []struct {
		name   string
		row    MechanicalEvaluation
		mutate func(*MechanicalEvaluation)
	}{
		{
			name: "satisfiable",
			row:  satisfiable,
			mutate: func(row *MechanicalEvaluation) {
				row.State = ProofUnproven
				row.Reasons = []ReasonCode{ReasonPrincipalRelationUnproven}
			},
		},
		{
			name: "disjoint",
			row:  disjoint,
			mutate: func(row *MechanicalEvaluation) {
				row.Scope.State = ScopeOverlap
			},
		},
		{
			name: "unsatisfiable",
			row:  unsatisfiable,
			mutate: func(row *MechanicalEvaluation) {
				row.State = ProofProven
				row.Reasons = []ReasonCode{ReasonMechanicalSatisfiable}
			},
		},
		{
			name: "unproven",
			row:  unproven,
			mutate: func(row *MechanicalEvaluation) {
				row.State = ProofProven
				row.Reasons = []ReasonCode{ReasonMechanicalSatisfiable}
			},
		},
		{
			name: "effective exemption",
			row:  effective,
			mutate: func(row *MechanicalEvaluation) {
				row.After.State = SolverUnsatisfiable
			},
		},
		{
			name: "rejected ineffective exemption",
			row:  rejected,
			mutate: func(row *MechanicalEvaluation) {
				row.Reasons = []ReasonCode{ReasonMechanicalConflict}
			},
		},
		{
			name: "accepted ineffective exemption",
			row:  acceptedIneffective,
			mutate: func(row *MechanicalEvaluation) {
				row.State = ProofProven
				row.Reasons = []ReasonCode{ReasonExemptionEffective}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := validateMechanicalEvaluation("row", test.row); err != nil {
				t.Fatalf("conforming row rejected: %v\nrow: %+v", err, test.row)
			}
			forged := test.row
			test.mutate(&forged)
			if err := validateMechanicalEvaluation("row", forged); err == nil {
				t.Fatalf("incoherent row accepted: %+v", forged)
			}
		})
	}
}

func TestReportDeterminismSemanticRequirement(t *testing.T) {
	if semanticEvaluationRequired(SemanticInput{Claims: nil, UnknownMechanicals: nil}) {
		t.Fatal("empty semantic input unexpectedly requires evaluation")
	}
	if semanticEvaluationRequired(SemanticInput{Claims: make([]contextcompile.ProseClaim, 1), UnknownMechanicals: []UnknownMechanicalWitness{}}) {
		t.Fatal("one prose claim unexpectedly requires a relation evaluation")
	}
	if !semanticEvaluationRequired(SemanticInput{Claims: make([]contextcompile.ProseClaim, 2), UnknownMechanicals: []UnknownMechanicalWitness{}}) {
		t.Fatal("two prose claims must require semantic evaluation")
	}
	if !semanticEvaluationRequired(SemanticInput{Claims: make([]contextcompile.ProseClaim, 1), UnknownMechanicals: []UnknownMechanicalWitness{{ID: "unknown"}}}) {
		t.Fatal("unknown mechanical witness must require semantic evaluation")
	}
}

func TestReportRedactionExcludesAmbientAndAbsoluteData(t *testing.T) {
	t.Setenv("VERDI_TEST_CREDENTIAL", "top-secret-credential")
	judge := serviceNoConflictJudge()
	service, repo := newServiceFixture(t, judge)

	result, err := service.Evaluate(context.Background(), serviceAcceptedRequest())
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	for _, forbidden := range [][]byte{
		[]byte(repo.Dir),
		[]byte("top-secret-credential"),
		[]byte("VERDI_TEST_CREDENTIAL"),
	} {
		if bytes.Contains(result.ReportBytes, forbidden) {
			t.Fatalf("report contains forbidden ambient/absolute data %q", forbidden)
		}
	}
}
