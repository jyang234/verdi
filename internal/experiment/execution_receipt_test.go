package experiment

import (
	"strings"
	"testing"
)

func validExecutionReceipt(t *testing.T) ExecutionReceipt {
	t.Helper()
	workspaceRunID, err := WorkspaceRunID(digestOf("a"), "run-1", "facts-cache")
	if err != nil {
		t.Fatal(err)
	}
	return ExecutionReceipt{
		Schema: ExecutionReceiptSchema, ExperimentDigest: digestOf("a"), Run: "run-1",
		EnvironmentPolicy: "local-isolated-v1", AuthorityDigest: digestOf("b"), CapabilitiesDigest: digestOf("c"),
		ScheduleDigest: digestOf("d"), GrantsDigest: digestOf("e"),
		Fingerprint: ExecutionFingerprint{OS: "linux", Arch: "amd64", ToolVersions: map[string]string{"evaluator": "2.1.0", "verdi": "0.1.0"}, Env: map[string]*string{}, InputDigests: map[string]string{"workload": strings.TrimPrefix(digestOf("f"), "sha256:")}},
		Enforcement: []ReceiptEnforcement{{Kind: "process-execution", Applied: true, Reason: "argv allowlist applied"}, {Kind: "timeouts", Applied: true, Reason: "deadline applied"}},
		Network:     ReceiptNetwork{Mode: NetworkDeny, Configured: true, Reason: "network namespace configured"},
		Candidates:  []ReceiptCandidate{{ID: "facts-cache", BaseCommit: base40, PatchDigest: digestOf("9"), WorkspaceRunID: workspaceRunID, Materialization: WorkspaceIdentity{Shape: WorkspaceBasePlusPatch, RunID: workspaceRunID, CommitSHA: base40, PatchSHA256: strings.TrimPrefix(digestOf("9"), "sha256:")}}},
		Versions:    ReceiptVersions{Verdi: "0.1.0", RecommendationEngine: string(AlgorithmV1)}, Disclosures: []ReceiptDisclosure{DisclosureCPUAllocationUnproven, DisclosureMemoryAllocationUnproven},
	}
}

func TestExecutionReceiptExactCodecDigestAndUnionRules(t *testing.T) {
	receipt := validExecutionReceipt(t)
	b, err := EncodeExecutionReceipt(receipt)
	if err != nil {
		t.Fatalf("EncodeExecutionReceipt(): %v", err)
	}
	decoded, err := DecodeExecutionReceipt(b)
	if err != nil {
		t.Fatalf("DecodeExecutionReceipt(): %v", err)
	}
	d1, err := ExecutionReceiptDigest(receipt)
	if err != nil {
		t.Fatal(err)
	}
	d2, err := ExecutionReceiptDigest(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if d1 != d2 {
		t.Fatalf("receipt digests differ: %s vs %s", d1, d2)
	}

	for _, data := range [][]byte{
		append(append([]byte(nil), b...), []byte(`{}`)...),
		[]byte(strings.Replace(string(b), `{"authority_digest"`, `{ "authority_digest"`, 1)),
		[]byte(strings.Replace(string(b), `"run":"run-1"`, `"run":"run-1","run":"run-2"`, 1)),
		[]byte(strings.Replace(string(b), `"schema":"verdi.experiment-execution/v1"`, `"schema":"verdi.experiment-execution/v1","unknown":true`, 1)),
	} {
		if _, err := DecodeExecutionReceipt(data); err == nil {
			t.Fatalf("DecodeExecutionReceipt(mutated bytes) = nil error")
		}
	}

	bad := receipt
	bad.Network.Mode = NetworkAllow
	bad.Network.Configured = false
	if _, err := EncodeExecutionReceipt(bad); err == nil {
		t.Fatalf("EncodeExecutionReceipt(unconfigured allow) = nil error")
	}
	bad = validExecutionReceipt(t)
	bad.Candidates[0].WorkspaceRunID = strings.Repeat("0", 64)
	if _, err := EncodeExecutionReceipt(bad); err == nil {
		t.Fatalf("EncodeExecutionReceipt(forged workspace id) = nil error")
	}
	bad = validExecutionReceipt(t)
	bad.Fingerprint.Env = map[string]*string{"BAD=KEY": nil}
	if _, err := EncodeExecutionReceipt(bad); err == nil {
		t.Fatalf("EncodeExecutionReceipt(invalid fingerprint environment key) = nil error")
	}
}

func TestValidateExecutionReceiptBindingRejectsForgedControlsAndLockedInputs(t *testing.T) {
	_, capabilitiesDigest := capabilitiesAuthorityForState(t, []string{"behavioral-equivalence", "tenant-isolation"}, "fixture-evaluator/2.1.0")
	doc, digest := lockedV2DefinitionDoc(t, capabilitiesDigest)
	def := mustDecodeDefinition(t, doc)
	observations, _ := completeObservationsV2JSONL(t, digest, "run-1")
	receipt := executionReceiptForState(t, def, "run-1")
	receipt.Fingerprint.InputDigests = map[string]string{
		"contract:behavioral-equivalence-contract": strings.TrimPrefix(digestOf("7"), "sha256:"),
		"evaluator:./tools/cache-evaluator":        strings.TrimPrefix(digestOf("3"), "sha256:"),
		"fixture:request-log":                      strings.TrimPrefix(digestOf("6"), "sha256:"),
		"workload:representative-request-mix":      strings.TrimPrefix(digestOf("5"), "sha256:"),
	}
	if err := ValidateExecutionReceiptBinding(def, observations, receipt); err != nil {
		t.Fatalf("ValidateExecutionReceiptBinding(valid authoritative receipt): %v", err)
	}
	allowed := receipt
	allowed.Network = ReceiptNetwork{Mode: NetworkAllow, Configured: true, Reason: "policy-authorized ambient network"}
	allowed.Enforcement = append([]ReceiptEnforcement{{Kind: "network", Applied: true, Reason: "explicit network grant applied"}}, receipt.Enforcement...)
	if err := ValidateExecutionReceiptBinding(def, observations, allowed); err != nil {
		t.Fatalf("ValidateExecutionReceiptBinding(valid policy-authorized network receipt): %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*ExecutionReceipt)
	}{
		{name: "non-Linux platform", mutate: func(r *ExecutionReceipt) { r.Fingerprint.OS = "darwin" }},
		{name: "ambient network allow without grant row", mutate: func(r *ExecutionReceipt) {
			r.Network = ReceiptNetwork{Mode: NetworkAllow, Configured: true, Reason: "ambient network allowed"}
		}},
		{name: "empty enforcement", mutate: func(r *ExecutionReceipt) { r.Enforcement = []ReceiptEnforcement{} }},
		{name: "unapplied required control", mutate: func(r *ExecutionReceipt) { r.Enforcement[0].Applied = false }},
		{name: "missing allocation disclosures", mutate: func(r *ExecutionReceipt) { r.Disclosures = []ReceiptDisclosure{} }},
		{name: "changed workload digest", mutate: func(r *ExecutionReceipt) {
			r.Fingerprint.InputDigests["workload:representative-request-mix"] = strings.Repeat("0", 64)
		}},
		{name: "changed fixture identity", mutate: func(r *ExecutionReceipt) {
			delete(r.Fingerprint.InputDigests, "fixture:request-log")
			r.Fingerprint.InputDigests["fixture:other-log"] = strings.TrimPrefix(digestOf("6"), "sha256:")
		}},
		{name: "changed contract digest", mutate: func(r *ExecutionReceipt) {
			r.Fingerprint.InputDigests["contract:behavioral-equivalence-contract"] = strings.Repeat("1", 64)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			forged := receipt
			forged.Enforcement = append([]ReceiptEnforcement(nil), receipt.Enforcement...)
			forged.Disclosures = append([]ReceiptDisclosure(nil), receipt.Disclosures...)
			forged.Fingerprint.InputDigests = make(map[string]string, len(receipt.Fingerprint.InputDigests))
			for name, digest := range receipt.Fingerprint.InputDigests {
				forged.Fingerprint.InputDigests[name] = digest
			}
			tt.mutate(&forged)
			if err := ValidateExecutionReceiptBinding(def, observations, forged); err == nil {
				t.Fatalf("ValidateExecutionReceiptBinding(forged receipt) = nil error")
			}
		})
	}
}

func validDecisionV2() ResultDecision {
	return ResultDecision{Experiment: "cache-placement-v1", DefinitionDigest: digestOf("a"), Run: "run-1", Algorithm: AlgorithmV1, Verdict: VerdictProvenWinner, Winner: "facts-cache", Candidates: []DecisionCandidate{
		{ID: "baseline", Baseline: true, Eligible: true, ExecutionFailures: []CandidateExecutionFailure{}},
		{ID: "facts-cache", Baseline: false, Eligible: true, ExecutionFailures: []CandidateExecutionFailure{}},
	}, ObservationsDigest: digestOf("b")}
}

func TestResultV2DecisionAnnexPresenceAndReceiptBinding(t *testing.T) {
	receipt := validExecutionReceipt(t)
	executionDigest, err := ExecutionReceiptDigest(receipt)
	if err != nil {
		t.Fatal(err)
	}
	annex := ResultExecution{ExecutionDigest: executionDigest, Isolation: ResultIsolation{Network: receipt.Network, Disclosures: []IsolationDisclosure{}}, WarmupDiagnostics: WarmupDiagnostics{Authority: WarmupAuthorityNonDecisionDiagnostic, Scope: WarmupScopeFinalInvocation, Failures: []WarmupFailure{}}}
	res, err := NewResultV2(validDecisionV2(), annex)
	if err != nil {
		t.Fatalf("NewResultV2(): %v", err)
	}
	b, err := EncodeResult(res)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeResult(b); err != nil {
		t.Fatalf("DecodeResult(v2): %v", err)
	}
	for _, mutated := range [][]byte{
		append(append([]byte(nil), b...), []byte(`{}`)...),
		[]byte(strings.Replace(string(b), `{"decision"`, `{ "decision"`, 1)),
		[]byte(strings.Replace(string(b), `"schema":"verdi.experiment-result/v2"`, `"schema":"verdi.experiment-result/v2","schema":"verdi.experiment-result/v2"`, 1)),
		[]byte(strings.Replace(string(b), `"schema":"verdi.experiment-result/v2"`, `"schema":"verdi.experiment-result/v2","unknown":true`, 1)),
	} {
		if _, err := DecodeResult(mutated); err == nil {
			t.Fatalf("DecodeResult(mutated v2 bytes) = nil error")
		}
	}
	if err := ValidateResultReceipt(receipt, res); err != nil {
		t.Fatalf("ValidateResultReceipt(): %v", err)
	}

	freshResult := func(t *testing.T) Result {
		t.Helper()
		fresh, err := NewResultV2(validDecisionV2(), annex)
		if err != nil {
			t.Fatalf("NewResultV2(): %v", err)
		}
		return fresh
	}

	bad := freshResult(t)
	bad.Execution.ExecutionDigest = digestOf("0")
	if err := ValidateResultReceipt(receipt, bad); err == nil {
		t.Fatalf("ValidateResultReceipt(forged digest) = nil error")
	}
	bad = freshResult(t)
	bad.Decision = nil
	if _, err := EncodeResult(bad); err == nil {
		t.Fatalf("EncodeResult(v2 without decision) = nil error")
	}
	bad = freshResult(t)
	bad.Execution.WarmupDiagnostics.Authority = "decision"
	if err := bad.Validate(); err == nil {
		t.Fatalf("Result.Validate(forged diagnostic authority) = nil error")
	}
	bad = freshResult(t)
	bad.Execution.Isolation.Disclosures = []IsolationDisclosure{IsolationWeaker}
	if err := ValidateResultReceipt(receipt, bad); err == nil {
		t.Fatalf("ValidateResultReceipt(forged isolation) = nil error")
	}
	invalidUTF8 := string([]byte{0xff})
	if err := (Reason{Code: ReasonPracticalTie, Detail: invalidUTF8}).Validate(); err == nil {
		t.Fatalf("Reason.Validate(invalid UTF-8) = nil error")
	}
	bad = freshResult(t)
	bad.Execution.WarmupDiagnostics.Failures = []WarmupFailure{{Candidate: "facts-cache", Warmup: 1, Kind: OutcomeCandidateCrash, Witness: invalidUTF8}}
	if err := bad.Validate(); err == nil {
		t.Fatalf("Result.Validate(invalid UTF-8 warmup witness) = nil error")
	}
}
