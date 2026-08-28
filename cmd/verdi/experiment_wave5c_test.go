package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/experimentdecision"
	"github.com/jyang234/verdi/internal/experimenthuman"
	"github.com/jyang234/verdi/internal/fixturegit"
)

const (
	wave5CWorkloadPath = "inputs/workload.txt"
	wave5CContractPath = "inputs/contract.txt"
)

type wave5CExperimentFixture struct {
	repo         *fixturegit.Repo
	privateKey   ed25519.PrivateKey
	resultDigest string
	targets      []string
}

func TestExperimentRatificationCapsuleReleaseGrammarBuiltBinary(t *testing.T) {
	bin := buildVerdiBinary(t)
	for _, operation := range []string{"propose-ratification", "publish-capsule", "release-workspaces"} {
		t.Run(operation, func(t *testing.T) {
			stdout, stderr, code := runExperimentBuiltBinary(t, bin, t.TempDir(), nil, "experiment", operation)
			if code != 2 || stdout != "" || !strings.Contains(stderr, "--spike is required") ||
				!strings.Contains(stderr, "usage: verdi experiment "+operation) {
				t.Fatalf("exit/stdout/stderr = %d/%q/%q, want the closed operation-specific usage refusal", code, stdout, stderr)
			}
		})
	}

	base := []string{
		"experiment", "propose-ratification",
		"--spike", "spec/request-path-spike",
		"--experiment", "request-path-v2",
		"--accepted-head", strings.Repeat("a", 40),
	}
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "missing-result", args: append([]string{}, base...), want: "--result is required"},
		{name: "missing-disposition", args: append(append([]string{}, base...), "--result", "sha256:"+strings.Repeat("1", 64)), want: "--disposition is required"},
		{name: "unknown-release-flag", args: []string{"experiment", "release-workspaces", "--spike", "spec/request-path-spike", "--experiment", "request-path-v2", "--accepted-head", strings.Repeat("a", 40), "--human-proof", "proof.sig"}, want: "unknown flag --human-proof"},
	} {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, code := runExperimentBuiltBinary(t, bin, t.TempDir(), nil, test.args...)
			if code != 2 || stdout != "" || !strings.Contains(stderr, test.want) {
				t.Fatalf("exit/stdout/stderr = %d/%q/%q, want %q", code, stdout, stderr, test.want)
			}
		})
	}
}

func TestExperimentRatificationCapsuleReleaseBuiltBinary(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	bin := buildVerdiBinary(t)
	fixture := buildWave5CAcceptedResult(t, bin)
	repo := fixture.repo

	base := wave5CRatificationArgs(repo.Head, fixture.resultDigest, experiment.DispositionSelectRecommended, "", "")
	stdout, stderr, code := runExperimentBuiltBinary(t, bin, repo.Dir, nil, append(base, "--json")...)
	if code != 1 || stderr != "" || !strings.Contains(stdout, `"code":"human-authorization-required"`) || !strings.Contains(stdout, experimentHumanPrompt) {
		t.Fatalf("ratification challenge exit/stdout/stderr = %d/%q/%q", code, stdout, stderr)
	}
	challenge := decodeExperimentChallengeOutput(t, stdout).Challenge
	challengeBytes, err := challenge.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	if challenge.Operation != experimenthuman.OperationProposeRatification || challenge.AcceptedHEAD != repo.Head {
		t.Fatalf("challenge identity = %+v, want propose-ratification at accepted HEAD %s", challenge, repo.Head)
	}
	wantInputDigest := wave5CRatificationInputDigest(t, fixture.resultDigest, experiment.DispositionSelectRecommended, "", "")
	if challenge.InputDigest != wantInputDigest {
		t.Fatalf("challenge input_digest = %q, want complete typed ratification digest %q", challenge.InputDigest, wantInputDigest)
	}

	humanOut, humanErr, humanCode := runExperimentBuiltBinary(t, bin, repo.Dir, nil, base...)
	if humanCode != 1 || humanErr != "" || !strings.Contains(humanOut, string(challengeBytes)) || !strings.Contains(humanOut, experimentHumanPrompt) {
		t.Fatalf("human challenge exit/stdout/stderr = %d/%q/%q, want exact challenge bytes and manual prompt", humanCode, humanOut, humanErr)
	}

	proofDir := t.TempDir()
	oldProofPath := filepath.Join(proofDir, "old.sig")
	if err := os.WriteFile(oldProofPath, ed25519.Sign(fixture.privateKey, challengeBytes), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, "ratification-context.txt"), []byte("new proposal context\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitForExperimentTest(t, repo.Dir, "add", "ratification-context.txt")
	runGitForExperimentTest(t, repo.Dir, "commit", "-q", "-m", "change ratification proposal head")
	stdout, stderr, code = runExperimentBuiltBinary(t, bin, repo.Dir, nil, append(append([]string{}, base...), "--human-proof", oldProofPath, "--json")...)
	if code != 1 || stderr != "" || !strings.Contains(stdout, `"code":"human-proof-invalid"`) {
		t.Fatalf("stale proof exit/stdout/stderr = %d/%q/%q, want a verdict", code, stdout, stderr)
	}
	if _, err := os.Lstat(wave5CRatificationPath(repo.Dir)); !os.IsNotExist(err) {
		t.Fatalf("stale proof wrote ratification bytes: %v", err)
	}

	stdout, stderr, code = runExperimentBuiltBinary(t, bin, repo.Dir, nil, append(base, "--json")...)
	if code != 1 || stderr != "" {
		t.Fatalf("refreshed challenge exit/stdout/stderr = %d/%q/%q", code, stdout, stderr)
	}
	currentChallenge := decodeExperimentChallengeOutput(t, stdout).Challenge
	currentBytes, err := currentChallenge.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	shortProofPath := filepath.Join(proofDir, "short.sig")
	if err := os.WriteFile(shortProofPath, make([]byte, ed25519.SignatureSize-1), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code = runExperimentBuiltBinary(t, bin, repo.Dir, nil, append(append([]string{}, base...), "--human-proof", shortProofPath, "--json")...)
	if code != 2 || stdout != "" || !strings.Contains(stderr, "want 64") {
		t.Fatalf("short proof exit/stdout/stderr = %d/%q/%q, want operational", code, stdout, stderr)
	}

	proofPath := filepath.Join(proofDir, "current.sig")
	if err := os.WriteFile(proofPath, ed25519.Sign(fixture.privateKey, currentBytes), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code = runExperimentBuiltBinary(t, bin, repo.Dir, nil, append(append([]string{}, base...), "--human-proof", proofPath, "--json")...)
	if code != 0 || stderr != "" || !strings.Contains(stdout, `"classification":"clean"`) {
		t.Fatalf("valid ratification proof exit/stdout/stderr = %d/%q/%q", code, stdout, stderr)
	}
	if raw, err := os.ReadFile(wave5CRatificationPath(repo.Dir)); err != nil {
		t.Fatalf("ratification proposal missing: %v", err)
	} else if record, decodeErr := experiment.DecodeRatification(raw); decodeErr != nil || record.Schema != experiment.RatificationSchemaV3 || record.Proof == nil {
		t.Fatalf("ratification proposal = %+v, err=%v; want strict v3 with retained proof", record, decodeErr)
	}

	releaseArgs := wave5CReleaseArgs("publish-capsule", repo.Head, true)
	stdout, stderr, code = runExperimentBuiltBinary(t, bin, repo.Dir, nil, releaseArgs...)
	if code != 1 || stderr != "" || !strings.Contains(stdout, `"code":"ratification-not-accepted"`) {
		t.Fatalf("unaccepted ratification release exit/stdout/stderr = %d/%q/%q, want proposed bytes refused", code, stdout, stderr)
	}

	runGitForExperimentTest(t, repo.Dir, "add", ".")
	runGitForExperimentTest(t, repo.Dir, "commit", "-q", "-m", "propose ratification")
	runGitForExperimentTest(t, repo.Dir, "checkout", "-q", "main")
	runGitForExperimentTest(t, repo.Dir, "merge", "-q", "--ff-only", "ratification")
	repo.Head = contextE2ECurrentHead(t, repo.Dir)

	manifestPath := wave5CCapsuleManifestPath(repo.Dir)
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code = runExperimentBuiltBinary(t, bin, repo.Dir, nil, wave5CReleaseArgs("publish-capsule", repo.Head, true)...)
	if code != 2 || stderr != "" || !strings.Contains(stdout, `"code":"capsule-publish-failed"`) {
		t.Fatalf("conflicting capsule exit/stdout/stderr = %d/%q/%q, want operational", code, stdout, stderr)
	}
	for _, target := range fixture.targets {
		if _, err := os.Lstat(execworkspace.ReleasedPath(repo.Dir, target)); !os.IsNotExist(err) {
			t.Fatalf("workspace %s was released before capsule publication succeeded: %v", target, err)
		}
	}
	if err := os.Remove(manifestPath); err != nil {
		t.Fatal(err)
	}

	failingMarker := execworkspace.ReleasedPath(repo.Dir, fixture.targets[0])
	if err := os.MkdirAll(failingMarker, 0o755); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code = runExperimentBuiltBinary(t, bin, repo.Dir, nil, wave5CReleaseArgs("publish-capsule", repo.Head, true)...)
	if code != 2 || stderr != "" || !strings.Contains(stdout, `"code":"workspace-release-failed"`) || !strings.Contains(stdout, `"CapsulePublished":true`) {
		t.Fatalf("partial release exit/stdout/stderr = %d/%q/%q, want published capsule plus operational failure", code, stdout, stderr)
	}
	if _, err := experiment.DecodeCapsuleManifest(mustReadWave5CFile(t, manifestPath)); err != nil {
		t.Fatalf("capsule was not safely published before partial release: %v", err)
	}
	for _, target := range fixture.targets[1:] {
		info, err := os.Lstat(execworkspace.ReleasedPath(repo.Dir, target))
		if err != nil || !info.Mode().IsRegular() {
			t.Fatalf("successful partial-release marker for %s = %v/%v", target, info, err)
		}
	}
	if err := os.Remove(failingMarker); err != nil {
		t.Fatal(err)
	}

	retryArgs := wave5CReleaseArgs("release-workspaces", repo.Head, true)
	firstJSON, firstErr, firstCode := runExperimentBuiltBinary(t, bin, repo.Dir, nil, retryArgs...)
	secondJSON, secondErr, secondCode := runExperimentBuiltBinary(t, bin, repo.Dir, nil, retryArgs...)
	if firstCode != 0 || secondCode != 0 || firstErr != "" || secondErr != "" || firstJSON != secondJSON {
		t.Fatalf("release retry results differ: first=%d/%q/%q second=%d/%q/%q", firstCode, firstJSON, firstErr, secondCode, secondJSON, secondErr)
	}
	if !strings.Contains(firstJSON, `"CapsulePublished":true`) || !strings.Contains(firstJSON, `"classification":"clean"`) {
		t.Fatalf("release retry JSON = %q, want clean typed capsule result", firstJSON)
	}
	publishJSON, publishErr, publishCode := runExperimentBuiltBinary(t, bin, repo.Dir, nil, wave5CReleaseArgs("publish-capsule", repo.Head, true)...)
	if publishCode != firstCode || publishErr != "" || publishJSON != firstJSON {
		t.Fatalf("publish/release typed projections differ: publish=%d/%q/%q release=%d/%q", publishCode, publishJSON, publishErr, firstCode, firstJSON)
	}
	humanOut, humanErr, humanCode = runExperimentBuiltBinary(t, bin, repo.Dir, nil, wave5CReleaseArgs("release-workspaces", repo.Head, false)...)
	if humanCode != firstCode || humanErr != "" || !strings.HasPrefix(humanOut, "experiment release-workspaces: clean (clean)\n") {
		t.Fatalf("release human parity = %d/%q/%q, want the JSON result's clean exit/classification", humanCode, humanOut, humanErr)
	}
}

func TestExperimentCapsuleReleaseNonSelectingBuiltBinary(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	bin := buildVerdiBinary(t)
	fixture := buildWave5CAcceptedResult(t, bin)
	repo := fixture.repo

	base := wave5CRatificationArgs(repo.Head, fixture.resultDigest, experiment.DispositionRejectAll, "", "")
	stdout, stderr, code := runExperimentBuiltBinary(t, bin, repo.Dir, nil, append(base, "--json")...)
	if code != 1 || stderr != "" {
		t.Fatalf("non-selecting challenge exit/stdout/stderr = %d/%q/%q", code, stdout, stderr)
	}
	challengeBytes, err := decodeExperimentChallengeOutput(t, stdout).Challenge.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	proofPath := filepath.Join(t.TempDir(), "reject-all.sig")
	if err := os.WriteFile(proofPath, ed25519.Sign(fixture.privateKey, challengeBytes), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code = runExperimentBuiltBinary(t, bin, repo.Dir, nil, append(append([]string{}, base...), "--human-proof", proofPath, "--json")...)
	if code != 0 || stderr != "" {
		t.Fatalf("non-selecting proposal exit/stdout/stderr = %d/%q/%q", code, stdout, stderr)
	}
	runGitForExperimentTest(t, repo.Dir, "add", ".")
	runGitForExperimentTest(t, repo.Dir, "commit", "-q", "-m", "reject all candidates")
	runGitForExperimentTest(t, repo.Dir, "checkout", "-q", "main")
	runGitForExperimentTest(t, repo.Dir, "merge", "-q", "--ff-only", "ratification")
	repo.Head = contextE2ECurrentHead(t, repo.Dir)

	publishJSON, publishErr, publishCode := runExperimentBuiltBinary(t, bin, repo.Dir, nil, wave5CReleaseArgs("publish-capsule", repo.Head, true)...)
	if publishCode != 0 || publishErr != "" || !strings.Contains(publishJSON, `"Disposition":"reject-all"`) || !strings.Contains(publishJSON, `"CapsulePublished":false`) {
		t.Fatalf("non-selecting publish exit/stdout/stderr = %d/%q/%q", publishCode, publishJSON, publishErr)
	}
	if _, err := os.Lstat(wave5CCapsuleManifestPath(repo.Dir)); !os.IsNotExist(err) {
		t.Fatalf("non-selecting ratification published a capsule: %v", err)
	}
	for _, target := range fixture.targets {
		info, err := os.Lstat(execworkspace.ReleasedPath(repo.Dir, target))
		if err != nil || !info.Mode().IsRegular() {
			t.Fatalf("non-selecting release marker for %s = %v/%v", target, info, err)
		}
	}
	releaseJSON, releaseErr, releaseCode := runExperimentBuiltBinary(t, bin, repo.Dir, nil, wave5CReleaseArgs("release-workspaces", repo.Head, true)...)
	if releaseCode != publishCode || releaseErr != "" || releaseJSON != publishJSON {
		t.Fatalf("non-selecting retry differs: publish=%d/%q release=%d/%q/%q", publishCode, publishJSON, releaseCode, releaseJSON, releaseErr)
	}
}

func buildWave5CAcceptedResult(t *testing.T, bin string) wave5CExperimentFixture {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	repo := buildExperimentHumanRepo(t, publicKey)
	wave5CBindProtectedInputs(t, repo)

	runGitForExperimentTest(t, repo.Dir, "checkout", "-q", "-b", "registration")
	registrationArgs := []string{
		"experiment", "propose-registration",
		"--spike", "spec/request-path-spike",
		"--experiment", "request-path-v2",
		"--accepted-head", repo.Head,
		"--json",
	}
	stdout, stderr, code := runExperimentBuiltBinary(t, bin, repo.Dir, nil, registrationArgs...)
	if code != 1 || stderr != "" {
		t.Fatalf("registration challenge exit/stdout/stderr = %d/%q/%q", code, stdout, stderr)
	}
	challengeBytes, err := decodeExperimentChallengeOutput(t, stdout).Challenge.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	proofPath := filepath.Join(t.TempDir(), "registration.sig")
	if err := os.WriteFile(proofPath, ed25519.Sign(privateKey, challengeBytes), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code = runExperimentBuiltBinary(t, bin, repo.Dir, nil, append(registrationArgs, "--human-proof", proofPath)...)
	if code != 0 || stderr != "" {
		t.Fatalf("registration proposal exit/stdout/stderr = %d/%q/%q", code, stdout, stderr)
	}
	runGitForExperimentTest(t, repo.Dir, "add", ".")
	runGitForExperimentTest(t, repo.Dir, "commit", "-q", "-m", "propose registration")
	runGitForExperimentTest(t, repo.Dir, "checkout", "-q", "main")
	runGitForExperimentTest(t, repo.Dir, "merge", "-q", "--ff-only", "registration")
	repo.Head = contextE2ECurrentHead(t, repo.Dir)

	definitionBytes := mustReadWave5CFile(t, wave5CDefinitionPath(repo.Dir))
	definition, err := experiment.DecodeDefinition(definitionBytes)
	if err != nil {
		t.Fatal(err)
	}
	resultDigest, targets := writeWave5CAcceptedRun(t, repo.Dir, definition, "run-alpha", 50)
	runGitForExperimentTest(t, repo.Dir, "add", ".")
	runGitForExperimentTest(t, repo.Dir, "commit", "-q", "-m", "accept experiment result")
	repo.Head = contextE2ECurrentHead(t, repo.Dir)
	runGitForExperimentTest(t, repo.Dir, "checkout", "-q", "-b", "ratification")

	return wave5CExperimentFixture{repo: repo, privateKey: privateKey, resultDigest: resultDigest, targets: targets}
}

func wave5CBindProtectedInputs(t *testing.T, repo *fixturegit.Repo) {
	t.Helper()
	workload := []byte("wave-5c workload\n")
	contract := []byte("wave-5c contract\n")
	definitionPath := wave5CDefinitionPath(repo.Dir)
	doc := string(mustReadWave5CFile(t, definitionPath))
	doc = strings.Replace(doc, "sha256:"+strings.Repeat("5", 64), experimentRawDigest(workload), 1)
	doc = strings.Replace(doc, "sha256:"+strings.Repeat("6", 64), experimentRawDigest(contract), 1)
	doc += "protected_paths:\n  - inputs/contract.txt\n  - inputs/workload.txt\n"
	if err := os.WriteFile(definitionPath, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{wave5CWorkloadPath: workload, wave5CContractPath: contract} {
		absolute := filepath.Join(repo.Dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absolute, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	runGitForExperimentTest(t, repo.Dir, "add", ".")
	runGitForExperimentTest(t, repo.Dir, "commit", "-q", "-m", "bind release inputs")
	repo.Head = contextE2ECurrentHead(t, repo.Dir)
}

func writeWave5CAcceptedRun(t *testing.T, root string, definition experiment.Definition, run string, cacheValue int) (string, []string) {
	t.Helper()
	definitionDigest, err := experiment.DefinitionDigest(definition)
	if err != nil {
		t.Fatal(err)
	}
	observations := make([]experiment.Observation, 0, len(definition.Candidates)*definition.Execution.Rounds)
	for _, candidate := range definition.Candidates {
		value := 100
		if candidate.ID == "cache" {
			value = cacheValue
		}
		for round := 1; round <= definition.Execution.Rounds; round++ {
			observations = append(observations, experiment.Observation{
				Schema: experiment.ObservationSchemaV2, ExperimentDigest: definitionDigest,
				Run: run, Candidate: candidate.ID, Round: round,
				Guards: []experiment.GuardResult{}, Measurements: []experiment.Measurement{{
					ID: "request-latency", Value: experiment.NumberValue(json.Number(strconv.Itoa(value))),
					Unit: "ms", Source: experiment.SourceEvaluatorMeasured,
				}}, Outcome: &experiment.CandidateOutcome{Kind: experiment.OutcomeCompleted}, Disclosures: []string{},
			})
		}
	}
	core, err := experimentdecision.Evaluate(definition, observations, experimentdecision.EnvironmentAttestation{PolicyID: definition.Execution.EnvironmentPolicy})
	if err != nil {
		t.Fatal(err)
	}
	decision, err := experiment.DecisionFromResult(core, observations)
	if err != nil {
		t.Fatal(err)
	}

	candidates := make([]experiment.ReceiptCandidate, 0, len(definition.Candidates))
	targets := make([]string, 0, len(definition.Candidates))
	for _, candidate := range definition.Candidates {
		workspaceRunID, err := experiment.WorkspaceRunID(definitionDigest, run, candidate.ID)
		if err != nil {
			t.Fatal(err)
		}
		identity := experiment.WorkspaceIdentity{
			Shape: experiment.WorkspaceBasePlusPatch, RunID: workspaceRunID,
			CommitSHA: candidate.Base, PatchSHA256: strings.TrimPrefix(candidate.Digest, "sha256:"),
		}
		candidates = append(candidates, experiment.ReceiptCandidate{
			ID: candidate.ID, BaseCommit: candidate.Base, PatchDigest: candidate.Digest,
			WorkspaceRunID: workspaceRunID, Materialization: identity,
		})
		patch := mustReadWave5CFile(t, filepath.Join(filepath.Dir(wave5CDefinitionPath(root)), filepath.FromSlash(candidate.Patch)))
		workspaceIdentity, err := execworkspace.NewPatchIdentity(workspaceRunID, candidate.Base, patch)
		if err != nil {
			t.Fatal(err)
		}
		target, err := workspaceIdentity.WorkspaceID()
		if err != nil {
			t.Fatal(err)
		}
		targets = append(targets, target)
	}
	receipt := experiment.ExecutionReceipt{
		Schema: experiment.ExecutionReceiptSchema, ExperimentDigest: definitionDigest, Run: run,
		EnvironmentPolicy: definition.Execution.EnvironmentPolicy,
		AuthorityDigest:   "sha256:" + strings.Repeat("1", 64), CapabilitiesDigest: definition.Evaluator.CapabilitiesDigest,
		ScheduleDigest: "sha256:" + strings.Repeat("2", 64), GrantsDigest: "sha256:" + strings.Repeat("3", 64),
		Fingerprint: experiment.ExecutionFingerprint{
			OS: "linux", Arch: "amd64", ToolVersions: map[string]string{"evaluator": "fixture/1.0.0", "verdi": "dev"}, Env: map[string]*string{},
			InputDigests: map[string]string{
				"evaluator:" + definition.Evaluator.Argv[0]: strings.TrimPrefix(definition.Evaluator.Digest, "sha256:"),
				wave5CWorkloadPath:                          strings.TrimPrefix(definition.Workload.Digest, "sha256:"),
				wave5CContractPath:                          strings.TrimPrefix(definition.Contract.Digest, "sha256:"),
			},
		},
		Enforcement: []experiment.ReceiptEnforcement{{Kind: "process-execution", Applied: true, Reason: "allowlist applied"}, {Kind: "timeouts", Applied: true, Reason: "deadline applied"}},
		Network:     experiment.ReceiptNetwork{Mode: experiment.NetworkDeny, Configured: true, Reason: "network namespace configured"},
		Candidates:  candidates, Versions: experiment.ReceiptVersions{Verdi: "dev", RecommendationEngine: string(experiment.AlgorithmV1)},
		Disclosures: []experiment.ReceiptDisclosure{experiment.DisclosureCPUAllocationUnproven, experiment.DisclosureMemoryAllocationUnproven},
	}
	receiptDigest, err := experiment.ExecutionReceiptDigest(receipt)
	if err != nil {
		t.Fatal(err)
	}
	result, err := experiment.NewResultV2(decision, experiment.ResultExecution{
		ExecutionDigest: receiptDigest,
		Isolation:       experiment.ResultIsolation{Network: receipt.Network, Disclosures: []experiment.IsolationDisclosure{}},
		WarmupDiagnostics: experiment.WarmupDiagnostics{
			Authority: experiment.WarmupAuthorityNonDecisionDiagnostic,
			Scope:     experiment.WarmupScopeFinalInvocation, Failures: []experiment.WarmupFailure{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	receiptBytes, err := experiment.EncodeExecutionReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	resultBytes, err := experiment.EncodeResult(result)
	if err != nil {
		t.Fatal(err)
	}
	var observationBytes bytes.Buffer
	for _, observation := range observations {
		encoded, err := experiment.EncodeObservation(observation)
		if err != nil {
			t.Fatal(err)
		}
		observationBytes.Write(encoded)
	}
	runDir := filepath.Join(filepath.Dir(wave5CDefinitionPath(root)), "runs", run)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{
		"execution.json": receiptBytes, "observations.jsonl": observationBytes.Bytes(), "result.json": resultBytes,
	} {
		if err := os.WriteFile(filepath.Join(runDir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	resultDigest, err := experiment.ResultDigest(result)
	if err != nil {
		t.Fatal(err)
	}
	return resultDigest, targets
}

func wave5CRatificationArgs(acceptedHead, resultDigest string, disposition experiment.Disposition, candidate, reason string) []string {
	args := []string{
		"experiment", "propose-ratification",
		"--spike", "spec/request-path-spike",
		"--experiment", "request-path-v2",
		"--accepted-head", acceptedHead,
		"--result", resultDigest,
		"--disposition", string(disposition),
	}
	if candidate != "" {
		args = append(args, "--candidate", candidate)
	}
	if reason != "" {
		args = append(args, "--reason", reason)
	}
	return args
}

func wave5CReleaseArgs(operation, acceptedHead string, jsonOutput bool) []string {
	args := []string{
		"experiment", operation,
		"--spike", "spec/request-path-spike",
		"--experiment", "request-path-v2",
		"--accepted-head", acceptedHead,
	}
	if jsonOutput {
		args = append(args, "--json")
	}
	return args
}

func wave5CRatificationInputDigest(t *testing.T, resultDigest string, disposition experiment.Disposition, candidate, reason string) string {
	t.Helper()
	digest, err := canonjson.Digest(struct {
		ResultDigest string                 `json:"result_digest"`
		Disposition  experiment.Disposition `json:"disposition"`
		Candidate    string                 `json:"candidate"`
		Reason       string                 `json:"reason"`
	}{ResultDigest: resultDigest, Disposition: disposition, Candidate: candidate, Reason: reason})
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

func wave5CDefinitionPath(root string) string {
	return filepath.Join(root, ".verdi", "specs", "active", "request-path-spike", "experiments", "request-path-v2", "experiment.yaml")
}

func wave5CRatificationPath(root string) string {
	return filepath.Join(filepath.Dir(wave5CDefinitionPath(root)), "ratification.yaml")
}

func wave5CCapsuleManifestPath(root string) string {
	return filepath.Join(filepath.Dir(wave5CDefinitionPath(root)), "selected", "capsule-manifest.json")
}

func mustReadWave5CFile(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return data
}
