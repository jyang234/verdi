package policyconflict

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
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
