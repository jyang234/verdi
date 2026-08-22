package experimentrun

import (
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/experimentdecision"
)

func TestBuildCompleteResultBindsReceiptDecisionAndFinalInvocationDiagnostics(t *testing.T) {
	def, capabilities, capabilitiesBytes := decisionDefinition(t)
	receipt := decisionReceipt(t, def, capabilities, capabilitiesBytes, "run-1")
	observations := completeStorageObservations(t, def, "run-1")
	warmups := []experiment.WarmupFailure{{
		Candidate: "beta",
		Warmup:    1,
		Kind:      experiment.OutcomeCandidateTimeout,
		Witness:   "final invocation timeout",
	}}

	result, digest, err := buildCompleteResult(def, observations, receipt, warmups)
	if err != nil {
		t.Fatalf("buildCompleteResult: %v", err)
	}
	if result.Schema != experiment.ResultSchemaV2 || result.Decision == nil || result.Execution == nil {
		t.Fatalf("result envelope = %#v, want complete V2 envelope", result)
	}
	if result.Execution.WarmupDiagnostics.Authority != experiment.WarmupAuthorityNonDecisionDiagnostic || result.Execution.WarmupDiagnostics.Scope != experiment.WarmupScopeFinalInvocation {
		t.Fatalf("warmup diagnostic labels = %#v", result.Execution.WarmupDiagnostics)
	}
	if len(result.Execution.WarmupDiagnostics.Failures) != 1 || result.Execution.WarmupDiagnostics.Failures[0].Witness != "final invocation timeout" {
		t.Fatalf("warmup diagnostics = %#v, want exact final invocation failure", result.Execution.WarmupDiagnostics.Failures)
	}
	wantReceiptDigest, err := experiment.ExecutionReceiptDigest(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if result.Execution.ExecutionDigest != wantReceiptDigest {
		t.Fatalf("execution digest = %q, want receipt digest %q", result.Execution.ExecutionDigest, wantReceiptDigest)
	}
	if result.Execution.Isolation.Network != receipt.Network || len(result.Execution.Isolation.Disclosures) != 1 || result.Execution.Isolation.Disclosures[0] != experiment.IsolationWeaker {
		t.Fatalf("result isolation = %#v, want exact allowed-network receipt projection", result.Execution.Isolation)
	}
	if result.Decision.DefinitionDigest != receipt.ExperimentDigest || result.Decision.Run != receipt.Run {
		t.Fatalf("decision identity = {%q,%q}, receipt = {%q,%q}", result.Decision.DefinitionDigest, result.Decision.Run, receipt.ExperimentDigest, receipt.Run)
	}
	if err := experimentdecision.VerifyResult(def, observations, &receipt, result); err != nil {
		t.Fatalf("VerifyResult(recomputed V2): %v", err)
	}
	wantDigest, err := experiment.ResultDigest(result)
	if err != nil {
		t.Fatal(err)
	}
	if digest != wantDigest {
		t.Fatalf("returned result digest = %q, want whole-result digest %q", digest, wantDigest)
	}
}

func TestBuildCompleteResultPreservesMeasuredCandidateFailureEligibility(t *testing.T) {
	def, capabilities, capabilitiesBytes := decisionDefinition(t)
	receipt := decisionReceipt(t, def, capabilities, capabilitiesBytes, "run-1")
	for _, test := range []struct {
		kind    experiment.OutcomeKind
		witness string
	}{
		{kind: experiment.OutcomeCandidateCrash, witness: "candidate process crashed"},
		{kind: experiment.OutcomeCandidateTimeout, witness: "candidate process timed out"},
	} {
		t.Run(string(test.kind), func(t *testing.T) {
			observations := completeStorageObservations(t, def, "run-1")
			for i := range observations {
				if observations[i].Candidate != "beta" || observations[i].Round != 1 {
					continue
				}
				observations[i].Outcome = &experiment.CandidateOutcome{Kind: test.kind, Witness: &test.witness}
				observations[i].Guards = []experiment.GuardResult{}
				observations[i].Measurements = []experiment.Measurement{{
					ID: experiment.EvaluatorWallDurationMetricID, Value: experiment.NumberValue("1"), Unit: "ns", Source: experiment.SourceHarnessMeasured,
				}}
				observations[i].Disclosures = []string{experiment.PeakRSSUnavailableDisclosure}
			}

			result, _, err := buildCompleteResult(def, observations, receipt, []experiment.WarmupFailure{})
			if err != nil {
				t.Fatalf("buildCompleteResult: %v", err)
			}
			var beta *experiment.DecisionCandidate
			for i := range result.Decision.Candidates {
				if result.Decision.Candidates[i].ID == "beta" {
					beta = &result.Decision.Candidates[i]
				}
			}
			if beta == nil || beta.Eligible || len(beta.ExecutionFailures) != 1 || beta.ExecutionFailures[0].Kind != test.kind || beta.ExecutionFailures[0].Witness != test.witness {
				t.Fatalf("beta decision row = %#v, want ineligible with exact %s evidence", beta, test.kind)
			}
			if len(beta.Violations) != 0 {
				t.Fatalf("candidate failure became guard violations: %#v", beta.Violations)
			}
		})
	}
}

func decisionDefinition(t *testing.T) (experiment.Definition, experiment.Capabilities, []byte) {
	t.Helper()
	def, capabilities, _ := testDefinition(t, []string{"alpha", "beta"}, 1)
	capabilities.RequiresNetwork = true
	capabilitiesBytes, err := canonjson.Marshal(capabilities)
	if err != nil {
		t.Fatal(err)
	}
	def.Evaluator.CapabilitiesDigest = testDigestBytes(capabilitiesBytes)
	return relockDefinition(t, def), capabilities, capabilitiesBytes
}

func decisionReceipt(t *testing.T, def experiment.Definition, capabilities experiment.Capabilities, capabilitiesBytes []byte, run string) experiment.ExecutionReceipt {
	t.Helper()
	root := t.TempDir()
	inputs := writeResolvedInputs(t, root, def)
	authorization := testAuthorization(t, def, true)
	authorized := mustResolveAuthorization(t, def, capabilities, authorization)
	_, report, err := execworkspace.PlanProfile(root, root+"/environment", authorized.Grants, authorization.DeclaredEnv)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := buildExecutionReceipt(ReceiptInput{
		Definition:        def,
		Run:               run,
		Capabilities:      capabilities,
		CapabilitiesBytes: capabilitiesBytes,
		Authorization:     authorized,
		Inputs:            inputs,
		CandidatePatches:  candidatePatches(t, def),
		Fingerprint:       testFingerprint(t, def, capabilities, authorization, inputs),
		Enforcement:       *report,
		Versions: experiment.ReceiptVersions{
			Verdi: "v-test", RecommendationEngine: string(def.Algorithm),
		},
	}, linuxHostRuntimeFacts())
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}

func completeStorageObservations(t *testing.T, def experiment.Definition, run string) []experiment.Observation {
	t.Helper()
	schedule, err := DeriveSchedule(def)
	if err != nil {
		t.Fatal(err)
	}
	observations := make([]experiment.Observation, 0, len(measuredSchedule(schedule)))
	for i, scheduled := range measuredSchedule(schedule) {
		observation := storageObservation(t, def, run, scheduled)
		for j := range observation.Measurements {
			if observation.Measurements[j].ID == "latency" && scheduled.Candidate == "beta" {
				observation.Measurements[j].Value = experiment.NumberValue("5")
			}
		}
		if i%2 == 1 {
			observation.Disclosures = []string{}
		}
		observations = append(observations, observation)
	}
	return observations
}
