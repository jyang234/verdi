package constitutionimpact

import (
	"os"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyconflict"
)

// TestCompleteCoverageRefusesCallerOnlyPassingSubset is SI-179's semantic
// correction: a supplemental caller target cannot hide a registered consumer
// whose canonical evaluation is absent.
func TestCompleteCoverageRefusesCallerOnlyPassingSubset(t *testing.T) {
	passing := testConsumer("spec/passing", "local")
	omitted := testConsumer("spec/omitted", "local")
	passingID, err := passing.Identity()
	if err != nil {
		t.Fatal(err)
	}
	omittedID, err := omitted.Identity()
	if err != nil {
		t.Fatal(err)
	}
	planned, _, err := plannedConsumers("test", Inventory{Schema: InventorySchema, Consumers: []Consumer{passing, omitted}})
	if err != nil {
		t.Fatal(err)
	}
	plan := Plan{
		layerChanged: true,
		proposed: InventoryEvidence{
			Commit:             "0123456789abcdef0123456789abcdef01234567",
			ConstitutionDigest: "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		},
		consumers: planned,
	}
	passResult := testPassResult(t)

	coverage := plan.Complete(
		[]Evaluation{{
			ConsumerIdentity: passingID,
			Consumer:         passing,
			Result:           passResult,
		}},
		[]SupplementalTarget{{Consumer: passing, Result: passResult}},
	)

	if coverage.State != StateViolatedWithWitness {
		t.Fatalf("state = %q, want %q", coverage.State, StateViolatedWithWitness)
	}
	if !hasReason(coverage.Reasons, ReasonEvaluationOmitted, omittedID) {
		t.Fatalf("reasons = %#v, want %q for %s", coverage.Reasons, ReasonEvaluationOmitted, omittedID)
	}
	if len(coverage.Evaluations) != 2 {
		t.Fatalf("evaluations = %d, want one row per registered consumer", len(coverage.Evaluations))
	}
}

func testConsumer(spec, environment string) Consumer {
	return Consumer{
		Request: contextcompile.Request{
			Schema:  contextcompile.RequestSchema,
			Adapter: contextcompile.AdapterRef{ID: "codex", Version: "1"},
			Grants:  execworkspace.GrantSet{Grants: []execworkspace.Grant{}},
			Phase:   contextcompile.PhaseBuild,
			Scope: policyartifact.Scope{
				Phases: []string{policyartifact.PhaseBuild}, Environments: []string{},
				Paths: []string{}, Refs: []string{},
			},
			Spec: spec,
		},
		Environment:        environment,
		GovernedOperations: []string{"make-verify"},
	}
}

func testPassResult(t *testing.T) *policyconflict.Result {
	t.Helper()
	raw, err := os.ReadFile("../policyconflict/testdata/report.json")
	if err != nil {
		t.Fatal(err)
	}
	report, err := policyconflict.DecodeReport(raw)
	if err != nil {
		t.Fatal(err)
	}
	report.Semantic = []policyconflict.SemanticEvaluation{}
	report.Verdict = policyconflict.VerdictPass
	report.Digest = ""
	encoded, err := policyconflict.EncodeReport(report)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := policyconflict.DecodeReport(encoded)
	if err != nil {
		t.Fatal(err)
	}
	return &policyconflict.Result{Report: canonical, ReportBytes: encoded}
}

func hasReason(reasons []Reason, code ReasonCode, witness string) bool {
	for _, reason := range reasons {
		if reason.Code == code {
			for _, got := range reason.Witnesses {
				if got == witness {
					return true
				}
			}
		}
	}
	return false
}
