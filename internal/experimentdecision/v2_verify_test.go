package experimentdecision

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/experiment"
)

func receiptFor(t *testing.T, def experiment.Definition, run string) experiment.ExecutionReceipt {
	t.Helper()
	digest, err := experiment.DefinitionDigest(def)
	if err != nil {
		t.Fatal(err)
	}
	candidates := make([]experiment.ReceiptCandidate, 0, len(def.Candidates))
	for _, candidate := range def.Candidates {
		workspaceID, err := experiment.WorkspaceRunID(digest, run, candidate.ID)
		if err != nil {
			t.Fatal(err)
		}
		candidates = append(candidates, experiment.ReceiptCandidate{
			ID: candidate.ID, BaseCommit: candidate.Base, PatchDigest: candidate.Digest, WorkspaceRunID: workspaceID,
			Materialization: experiment.WorkspaceIdentity{Shape: experiment.WorkspaceBasePlusPatch, RunID: workspaceID, CommitSHA: candidate.Base, PatchSHA256: strings.TrimPrefix(candidate.Digest, "sha256:")},
		})
	}
	return experiment.ExecutionReceipt{
		Schema: experiment.ExecutionReceiptSchema, ExperimentDigest: digest, Run: run,
		EnvironmentPolicy: def.Execution.EnvironmentPolicy, AuthorityDigest: fixtureDigest("1"), CapabilitiesDigest: def.Evaluator.CapabilitiesDigest, ScheduleDigest: fixtureDigest("2"), GrantsDigest: fixtureDigest("3"),
		Fingerprint: experiment.ExecutionFingerprint{OS: "linux", Arch: "amd64", ToolVersions: map[string]string{"evaluator": "2.1.0", "verdi": "0.1.0"}, Env: map[string]*string{}, InputDigests: map[string]string{
			"contracts/equivalence.json":         strings.TrimPrefix(def.Contract.Digest, "sha256:"),
			"evaluator:" + def.Evaluator.Argv[0]: strings.TrimPrefix(def.Evaluator.Digest, "sha256:"),
			"inputs/workload.json":               strings.TrimPrefix(def.Workload.Digest, "sha256:"),
		}},
		Enforcement: []experiment.ReceiptEnforcement{{Kind: "process-execution", Applied: true, Reason: "allowlist applied"}, {Kind: "timeouts", Applied: true, Reason: "deadline applied"}},
		Network:     experiment.ReceiptNetwork{Mode: experiment.NetworkDeny, Configured: true, Reason: "network namespace configured"}, Candidates: candidates,
		Versions:    experiment.ReceiptVersions{Verdi: "0.1.0", RecommendationEngine: string(experiment.AlgorithmV1)},
		Disclosures: []experiment.ReceiptDisclosure{experiment.DisclosureCPUAllocationUnproven, experiment.DisclosureMemoryAllocationUnproven},
	}
}

func verifiableV2Run(t *testing.T) (experiment.Definition, []experiment.Observation, experiment.ExecutionReceipt, experiment.Result) {
	t.Helper()
	def := lockV2Definition(t)
	obs := measuredV2(happyObservations(t, def, "run-1",
		map[string][]float64{"baseline": {40, 42, 41}, "candidate-a": {18, 19, 17}},
		map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {108, 109, 107}},
	))
	core := mustEvaluate(t, def, obs)
	decision, err := experiment.DecisionFromResult(core, obs)
	if err != nil {
		t.Fatal(err)
	}
	receipt := receiptFor(t, def, "run-1")
	receiptDigest, err := experiment.ExecutionReceiptDigest(receipt)
	if err != nil {
		t.Fatal(err)
	}
	result, err := experiment.NewResultV2(decision, experiment.ResultExecution{
		ExecutionDigest:   receiptDigest,
		Isolation:         experiment.ResultIsolation{Network: receipt.Network, Disclosures: []experiment.IsolationDisclosure{}},
		WarmupDiagnostics: experiment.WarmupDiagnostics{Authority: experiment.WarmupAuthorityNonDecisionDiagnostic, Scope: experiment.WarmupScopeFinalInvocation, Failures: []experiment.WarmupFailure{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return def, obs, receipt, result
}

func TestVerifyResultV2DecisionOnlyRecomputeAndReceiptBinding(t *testing.T) {
	def, obs, receipt, result := verifiableV2Run(t)
	if err := VerifyResult(def, obs, &receipt, result); err != nil {
		t.Fatalf("VerifyResult(v2): %v", err)
	}

	forged := result
	decision := *result.Decision
	decision.Candidates = append([]experiment.DecisionCandidate(nil), result.Decision.Candidates...)
	primary := *decision.Candidates[1].Primary
	primary.Value = "1"
	decision.Candidates[1].Primary = &primary
	forged.Decision = &decision
	if err := VerifyResult(def, obs, &receipt, forged); err == nil {
		t.Fatalf("VerifyResult(v2 forged decision) = nil error")
	}

	diagnostic := result
	execution := *result.Execution
	execution.WarmupDiagnostics.Failures = []experiment.WarmupFailure{{Candidate: "candidate-a", Warmup: 1, Kind: experiment.OutcomeCandidateCrash, Witness: "warmup crash"}}
	diagnostic.Execution = &execution
	if err := VerifyResult(def, obs, &receipt, diagnostic); err != nil {
		t.Fatalf("VerifyResult(v2 diagnostic-only annex change): %v", err)
	}

	wrongReceipt := receipt
	wrongReceipt.ScheduleDigest = fixtureDigest("4")
	if err := VerifyResult(def, obs, &wrongReceipt, result); err == nil {
		t.Fatalf("VerifyResult(v2 wrong receipt) = nil error")
	}
	wrongBinding := receipt
	wrongBinding.CapabilitiesDigest = fixtureDigest("5")
	wrongBindingResult := result
	wrongBindingExecution := *result.Execution
	wrongBindingDigest, err := experiment.ExecutionReceiptDigest(wrongBinding)
	if err != nil {
		t.Fatal(err)
	}
	wrongBindingExecution.ExecutionDigest = wrongBindingDigest
	wrongBindingResult.Execution = &wrongBindingExecution
	if err := VerifyResult(def, obs, &wrongBinding, wrongBindingResult); err == nil {
		t.Fatalf("VerifyResult(v2 receipt not bound to definition) = nil error")
	}
	if err := VerifyResult(def, obs, nil, result); err == nil {
		t.Fatalf("VerifyResult(v2 missing receipt) = nil error")
	}
}

func TestVerifyResultV2RejectsWarmupFailuresOutsideScheduleOrder(t *testing.T) {
	def, obs, receipt, result := verifiableV2Run(t)
	outOfOrder := result
	execution := *result.Execution
	execution.WarmupDiagnostics.Failures = []experiment.WarmupFailure{
		{Candidate: "candidate-a", Warmup: 1, Kind: experiment.OutcomeCandidateCrash, Witness: "candidate-a failed first"},
		{Candidate: "baseline", Warmup: 1, Kind: experiment.OutcomeCandidateCrash, Witness: "baseline failed second"},
	}
	outOfOrder.Execution = &execution
	if err := outOfOrder.Validate(); err != nil {
		t.Fatalf("out-of-order annex must remain shape-valid: %v", err)
	}
	if err := VerifyResult(def, obs, &receipt, outOfOrder); err == nil {
		t.Fatalf("VerifyResult(out-of-order warmup failures) = nil error")
	}
}

func TestVerifyResultRejectsV2ResultOverV1ObservationsAtDirectPort(t *testing.T) {
	def := lockDefinition(t)
	observations := happyObservations(t, def, "run-1",
		map[string][]float64{"baseline": {40, 42, 41}, "candidate-a": {18, 19, 17}},
		map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {108, 109, 107}},
	)
	core := mustEvaluate(t, def, observations)
	decision, err := experiment.DecisionFromResult(core, observations)
	if err != nil {
		t.Fatal(err)
	}
	receipt := receiptFor(t, def, "run-1")
	receiptDigest, err := experiment.ExecutionReceiptDigest(receipt)
	if err != nil {
		t.Fatal(err)
	}
	result, err := experiment.NewResultV2(decision, experiment.ResultExecution{
		ExecutionDigest:   receiptDigest,
		Isolation:         experiment.ResultIsolation{Network: receipt.Network, Disclosures: []experiment.IsolationDisclosure{}},
		WarmupDiagnostics: experiment.WarmupDiagnostics{Authority: experiment.WarmupAuthorityNonDecisionDiagnostic, Scope: experiment.WarmupScopeFinalInvocation, Failures: []experiment.WarmupFailure{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyResult(def, observations, &receipt, result); err == nil {
		t.Fatalf("VerifyResult(v2 result over v1 observations) = nil error")
	}
}

func TestVerifyResultV1RetainsWholeResultRecomputeAndNilReceipt(t *testing.T) {
	def, obs, result := verifiableRun(t)
	if err := VerifyResult(def, obs, nil, result); err != nil {
		t.Fatalf("VerifyResult(v1): %v", err)
	}
	receipt := receiptFor(t, def, "run-1")
	if err := VerifyResult(def, obs, &receipt, result); err == nil {
		t.Fatalf("VerifyResult(v1 with receipt) = nil error")
	}
}
