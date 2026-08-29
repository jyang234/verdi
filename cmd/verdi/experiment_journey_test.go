package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/experimentapp"
	"github.com/jyang234/verdi/internal/experimentrun"
	"github.com/jyang234/verdi/internal/fixturegit"
	forgefake "github.com/jyang234/verdi/internal/forge/fake"
	"github.com/jyang234/verdi/internal/instructionprojection"
	"github.com/jyang234/verdi/internal/policyconflict"
	"github.com/jyang234/verdi/internal/store"
)

func TestCompleteExperimentCLIJourneyBuiltBinary(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	bin := buildVerdiBinary(t)
	privateKey := ed25519.NewKeyFromSeed(wave5CFixtureEd25519Seed[:])
	seen := map[string]bool{}
	exits := map[int]bool{}
	callJSON := func(dir string, args ...string) (string, int) {
		t.Helper()
		if len(args) < 2 || args[0] != "experiment" {
			t.Fatalf("not an experiment command: %v", args)
		}
		operation := args[1]
		seen[operation] = true
		stdout, stderr, code := runExperimentBuiltBinary(t, bin, dir, nil, args...)
		if stderr != "" {
			t.Fatalf("%s stderr=%q (exit %d)", operation, stderr, code)
		}
		assertCLIJourneyCanonicalJSON(t, operation, stdout)
		var projected struct {
			Outcome experimentapp.Outcome `json:"outcome"`
		}
		if err := json.Unmarshal([]byte(stdout), &projected); err != nil {
			t.Fatalf("decode %s outcome: %v\n%s", operation, err, stdout)
		}
		if projected.Outcome.ExitCode() != code {
			t.Fatalf("%s exit=%d, typed outcome=%+v", operation, code, projected.Outcome)
		}
		exits[code] = true
		return stdout, code
	}
	base := func(operation, head string) []string {
		return []string{
			"experiment", operation, "--spike", "spec/request-path-spike",
			"--experiment", "request-path-v2", "--accepted-head", head, "--json",
		}
	}

	// Begin unlocked: author through the typed draft/candidate seams, then
	// make one direct Git edit and require explicit human reconciliation.
	repo := buildExperimentHumanRepo(t, privateKey.Public().(ed25519.PublicKey))
	experimentDir := filepath.Dir(wave5CDefinitionPath(repo.Dir))
	policyDir := filepath.Join(repo.Dir, ".verdi", "policy", "policies")
	basePolicy := mustReadWave5CFile(t, filepath.Join(policyDir, "experiment.md"))
	organizationPolicy := bytes.Replace(basePolicy, []byte("id: policy/experiment"), []byte("id: policy/organization"), 1)
	projectPolicy := bytes.Replace(basePolicy, []byte("id: policy/experiment"), []byte("id: policy/project"), 1)
	projectPolicy = bytes.Replace(projectPolicy, []byte("classes: [request-path-performance]"), []byte("classes: [request-path-performance, storage-throughput]"), 1)
	if bytes.Equal(projectPolicy, basePolicy) {
		t.Fatal("project policy fixture did not attempt the expected allowance weakening")
	}
	if err := os.Remove(filepath.Join(policyDir, "experiment.md")); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string][]byte{
		"organization.md": organizationPolicy,
		"project.md":      projectPolicy,
	} {
		if err := os.WriteFile(filepath.Join(policyDir, name), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	runGitForExperimentTest(t, repo.Dir, "add", "-A", ".verdi/policy/policies")
	runGitForExperimentTest(t, repo.Dir, "commit", "-q", "-m", "adopt layered journey policy")
	repo.Head = contextE2ECurrentHead(t, repo.Dir)
	newDefinition := wave5CProtectedDefinition(t, repo.Dir)
	newDefinition = bytes.Replace(newDefinition, []byte("#oq-cache\n"), []byte("#oq-cache-cli-journey\n"), 1)
	candidateRoot := t.TempDir()
	for _, name := range []string{"baseline.patch", "cache.patch"} {
		data := mustReadWave5CFile(t, filepath.Join(experimentDir, "candidates", name))
		target := filepath.Join(candidateRoot, "candidates", name)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.RemoveAll(experimentDir); err != nil {
		t.Fatal(err)
	}
	installCLIJourneyClosureTarget(t, repo)
	definitionInput := filepath.Join(t.TempDir(), "experiment-v2.yaml")
	weakeningProbeDefinition := bytes.Replace(newDefinition, []byte("class: request-path-performance"), []byte("class: storage-throughput"), 1)
	if bytes.Equal(weakeningProbeDefinition, newDefinition) {
		t.Fatal("definition fixture did not carry the expected experiment class")
	}
	if err := os.WriteFile(definitionInput, weakeningProbeDefinition, 0o600); err != nil {
		t.Fatal(err)
	}
	draftArgs := []string{
		"experiment", "draft-definition", "--spike", "spec/request-path-spike", "--experiment", "request-path-v2",
		"--accepted-head", repo.Head, "--definition", definitionInput, "--candidate-root", candidateRoot, "--json",
	}
	beforePolicyRefusal := contextE2EPorcelainStatus(t, repo.Dir)
	policyOut, policyCode := callJSON(repo.Dir, draftArgs...)
	if policyCode != 1 || !strings.Contains(policyOut, `"code":"policy-refused"`) || !strings.Contains(policyOut, "storage-throughput") || contextE2EPorcelainStatus(t, repo.Dir) != beforePolicyRefusal {
		t.Fatalf("real layered-policy refusal=%d/%q or mutated the worktree", policyCode, policyOut)
	}
	if _, err := os.Stat(experimentDir); !os.IsNotExist(err) {
		t.Fatalf("real layered-policy refusal created experiment state: %v", err)
	}
	repairedProjectPolicy := bytes.Replace(projectPolicy, []byte("classes: [request-path-performance, storage-throughput]"), []byte("classes: [request-path-performance]"), 1)
	repairedProjectPolicy = bytes.Replace(repairedProjectPolicy, []byte("observation_bytes: 262144"), []byte("observation_bytes: 131072"), 1)
	repairedProjectPolicy = bytes.Replace(repairedProjectPolicy, []byte("retained_artifact_bytes: 8388608"), []byte("retained_artifact_bytes: 4194304"), 1)
	if err := os.WriteFile(filepath.Join(policyDir, "project.md"), repairedProjectPolicy, 0o600); err != nil {
		t.Fatal(err)
	}
	runGitForExperimentTest(t, repo.Dir, "add", ".verdi/policy/policies/project.md")
	runGitForExperimentTest(t, repo.Dir, "commit", "-q", "-m", "narrow project experiment policy")
	repo.Head = contextE2ECurrentHead(t, repo.Dir)
	wave5CWriteProtectedInputs(t, repo.Dir)
	if err := os.WriteFile(definitionInput, newDefinition, 0o600); err != nil {
		t.Fatal(err)
	}
	draftArgs = []string{
		"experiment", "draft-definition", "--spike", "spec/request-path-spike", "--experiment", "request-path-v2",
		"--accepted-head", repo.Head, "--definition", definitionInput, "--candidate-root", candidateRoot, "--json",
	}
	if out, code := callJSON(repo.Dir, draftArgs...); code != 0 {
		t.Fatalf("draft-definition exit/output=%d/%q, want clean", code, out)
	}

	changedPatch := []byte("diff --git a/spikes/cache.go b/spikes/cache.go\nindex 1111111..2222222 100644\n--- a/spikes/cache.go\n+++ b/spikes/cache.go\n@@ -1 +1 @@\n-old\n+cli-journey\n")
	changedDefinition := bytes.Replace(newDefinition,
		[]byte("sha256:948705e2b8a093896358025d2b75282fbd1c36557c278881add34f4c75cbecc7"),
		[]byte(experimentRawDigest(changedPatch)), 1)
	inputDir := t.TempDir()
	patchInput := filepath.Join(inputDir, "cache.patch")
	definitionV2Input := filepath.Join(inputDir, "experiment-v2.yaml")
	if err := os.WriteFile(patchInput, changedPatch, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(definitionV2Input, changedDefinition, 0o600); err != nil {
		t.Fatal(err)
	}
	captureArgs := append(base("capture-candidate", repo.Head)[:len(base("capture-candidate", repo.Head))-1],
		"--candidate", "cache", "--patch", patchInput, "--definition", definitionV2Input, "--json")
	if _, code := callJSON(repo.Dir, captureArgs...); code != 0 {
		t.Fatalf("capture-candidate exit=%d, want clean", code)
	}

	runGitForExperimentTest(t, repo.Dir, "checkout", "-q", "-b", "journey-proposal")
	if err := os.WriteFile(filepath.Join(experimentDir, "human-note.txt"), []byte("direct Git reconciliation witness\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGitForExperimentTest(t, repo.Dir, "add", ".")
	runGitForExperimentTest(t, repo.Dir, "commit", "-q", "-m", "capture direct journey edit")
	beforeReview := contextE2EPorcelainStatus(t, repo.Dir)
	reviewOut, reviewCode := callJSON(repo.Dir, base("review-registration", repo.Head)...)
	if reviewCode != 1 || !strings.Contains(reviewOut, `"code":"direct-draft-unreconciled"`) || contextE2EPorcelainStatus(t, repo.Dir) != beforeReview {
		t.Fatalf("unreconciled review=%d/%q or mutated the worktree", reviewCode, reviewOut)
	}

	proofDir := t.TempDir()
	humanOperation := func(operation string) {
		t.Helper()
		args := base(operation, repo.Head)
		challengeOut, code := callJSON(repo.Dir, args...)
		if code != 1 {
			t.Fatalf("%s challenge exit=%d", operation, code)
		}
		challenge := decodeExperimentChallengeOutput(t, challengeOut).Challenge
		canonical, err := challenge.Canonical()
		if err != nil {
			t.Fatal(err)
		}
		proofPath := filepath.Join(proofDir, operation+".sig")
		if err := os.WriteFile(proofPath, ed25519.Sign(privateKey, canonical), 0o600); err != nil {
			t.Fatal(err)
		}
		withProof := append(args[:len(args)-1], "--human-proof", proofPath, "--json")
		if out, proofCode := callJSON(repo.Dir, withProof...); proofCode != 0 || !strings.Contains(out, `"classification":"clean"`) {
			t.Fatalf("%s proof exit=%d output=%q", operation, proofCode, out)
		}
	}
	humanOperation("reconcile-draft")
	humanOperation("propose-registration")

	// Accept that exact proposal, then install hermetic Linux-run evidence in
	// the same repository because the Darwin test host cannot execute the
	// production isolation runner.
	runGitForExperimentTest(t, repo.Dir, "add", ".")
	runGitForExperimentTest(t, repo.Dir, "commit", "-q", "-m", "propose journey registration")
	runGitForExperimentTest(t, repo.Dir, "checkout", "-q", "main")
	runGitForExperimentTest(t, repo.Dir, "merge", "-q", "--ff-only", "journey-proposal")
	repo.Head = contextE2ECurrentHead(t, repo.Dir)
	definition, err := experiment.DecodeDefinition(mustReadWave5CFile(t, wave5CDefinitionPath(repo.Dir)))
	if err != nil {
		t.Fatal(err)
	}
	resultDigest, targets := writeWave5CAcceptedRun(t, repo.Dir, definition, "run-alpha", 50)
	runGitForExperimentTest(t, repo.Dir, "add", ".")
	runGitForExperimentTest(t, repo.Dir, "commit", "-q", "-m", "accept journey result")
	repo.Head = contextE2ECurrentHead(t, repo.Dir)
	runGitForExperimentTest(t, repo.Dir, "checkout", "-q", "-b", "ratification")
	fixture := wave5CExperimentFixture{repo: repo, privateKey: privateKey, resultDigest: resultDigest, targets: targets}

	for _, operation := range []string{"inspect", "discover-capabilities", "validate-draft", "status"} {
		if _, code := callJSON(repo.Dir, base(operation, repo.Head)...); code != 0 {
			t.Fatalf("%s exit=%d, want clean", operation, code)
		}
	}
	explainArgs := append(base("explain-result", repo.Head)[:len(base("explain-result", repo.Head))-1], "--run", "run-alpha", "--json")
	if out, code := callJSON(repo.Dir, explainArgs...); code != 0 || !strings.Contains(out, `"winner":"cache"`) {
		t.Fatalf("explain-result exit/output=%d/%q", code, out)
	}

	// Both execution verbs fail before any runner or network activity when
	// one accepted input digest is substituted.
	mismatched := experimentrun.InputBindings{Schema: experimentrun.InputBindingSchema, Inputs: []experimentrun.InputBinding{
		{Slot: experimentrun.InputSlotContract, ID: "request-contract", Digest: "sha256:" + strings.Repeat("6", 64), Path: wave5CContractPath},
		{Slot: experimentrun.InputSlotWorkload, ID: "request-mix", Digest: "sha256:" + strings.Repeat("7", 64), Path: wave5CWorkloadPath},
	}}
	mismatchedBytes, err := experimentrun.EncodeInputBindings(mismatched)
	if err != nil {
		t.Fatal(err)
	}
	bindingsPath := filepath.Join(t.TempDir(), "mismatched.json")
	if err := os.WriteFile(bindingsPath, mismatchedBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	beforeExecution := contextE2EPorcelainStatus(t, repo.Dir)
	for _, operation := range []string{"start", "resume"} {
		args := append(base(operation, repo.Head)[:len(base(operation, repo.Head))-1], "--run", "run-refused", "--inputs", bindingsPath, "--json")
		out, code := callJSON(repo.Dir, args...)
		if code != 2 || !strings.Contains(out, `"code":"input-binding-invalid"`) {
			t.Fatalf("%s exit/output=%d/%q", operation, code, out)
		}
	}
	if after := contextE2EPorcelainStatus(t, repo.Dir); after != beforeExecution {
		t.Fatalf("execution refusals mutated worktree: before=%q after=%q", beforeExecution, after)
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "data", "execution")); !os.IsNotExist(err) {
		t.Fatalf("execution refusal left runner evidence: %v", err)
	}

	// Propose an authenticated ratification, prove proposal != acceptance,
	// then publish the capsule before attempting disposable cleanup.
	ratificationArgs := append(wave5CRatificationArgs(repo.Head, fixture.resultDigest, experiment.DispositionSelectRecommended, "", ""), "--json")
	challengeOut, challengeCode := callJSON(repo.Dir, ratificationArgs...)
	if challengeCode != 1 {
		t.Fatalf("ratification challenge exit=%d", challengeCode)
	}
	challengeBytes, err := decodeExperimentChallengeOutput(t, challengeOut).Challenge.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	ratificationProof := filepath.Join(t.TempDir(), "ratification.sig")
	if err := os.WriteFile(ratificationProof, ed25519.Sign(fixture.privateKey, challengeBytes), 0o600); err != nil {
		t.Fatal(err)
	}
	withRatificationProof := append(ratificationArgs[:len(ratificationArgs)-1], "--human-proof", ratificationProof, "--json")
	if out, code := callJSON(repo.Dir, withRatificationProof...); code != 0 || !strings.Contains(out, `"classification":"clean"`) {
		t.Fatalf("ratification proof exit/output=%d/%q", code, out)
	}
	beforeUnaccepted := contextE2EPorcelainStatus(t, repo.Dir)
	for _, operation := range []string{"publish-capsule", "release-workspaces"} {
		out, code := callJSON(repo.Dir, wave5CReleaseArgs(operation, repo.Head, true)...)
		if code != 1 || !strings.Contains(out, `"code":"ratification-not-accepted"`) {
			t.Fatalf("unaccepted %s exit/output=%d/%q", operation, code, out)
		}
	}
	if after := contextE2EPorcelainStatus(t, repo.Dir); after != beforeUnaccepted {
		t.Fatalf("unaccepted lifecycle refusals mutated worktree: before=%q after=%q", beforeUnaccepted, after)
	}

	runGitForExperimentTest(t, repo.Dir, "add", ".")
	runGitForExperimentTest(t, repo.Dir, "commit", "-q", "-m", "accept journey ratification")
	runGitForExperimentTest(t, repo.Dir, "checkout", "-q", "main")
	runGitForExperimentTest(t, repo.Dir, "merge", "-q", "--ff-only", "ratification")
	repo.Head = contextE2ECurrentHead(t, repo.Dir)
	publishOut, publishCode := callJSON(repo.Dir, wave5CReleaseArgs("publish-capsule", repo.Head, true)...)
	if publishCode != 0 || !strings.Contains(publishOut, `"CapsulePublished":true`) {
		t.Fatalf("publish-capsule exit/output=%d/%q", publishCode, publishOut)
	}
	manifestBytes := mustReadWave5CFile(t, wave5CCapsuleManifestPath(repo.Dir))
	if _, err := experiment.DecodeCapsuleManifest(manifestBytes); err != nil {
		t.Fatalf("published capsule does not strict-decode: %v", err)
	}
	for _, target := range fixture.targets {
		if _, err := os.Lstat(execworkspace.ReleasedPath(repo.Dir, target)); !os.IsNotExist(err) {
			t.Fatalf("capsule publication released workspace %s: %v", target, err)
		}
	}

	failingMarker := execworkspace.ReleasedPath(repo.Dir, fixture.targets[0])
	if err := os.MkdirAll(failingMarker, 0o755); err != nil {
		t.Fatal(err)
	}
	releaseOut, releaseCode := callJSON(repo.Dir, wave5CReleaseArgs("release-workspaces", repo.Head, true)...)
	if releaseCode != 2 || !strings.Contains(releaseOut, `"code":"workspace-release-failed"`) || !bytes.Equal(manifestBytes, mustReadWave5CFile(t, wave5CCapsuleManifestPath(repo.Dir))) {
		t.Fatalf("partial release exit/output=%d/%q", releaseCode, releaseOut)
	}
	if err := os.Remove(failingMarker); err != nil {
		t.Fatal(err)
	}
	retryOut, retryCode := callJSON(repo.Dir, wave5CReleaseArgs("release-workspaces", repo.Head, true)...)
	if retryCode != 0 || !strings.Contains(retryOut, `"classification":"clean"`) {
		t.Fatalf("release retry exit/output=%d/%q", retryCode, retryOut)
	}
	humanOut, humanErr, humanCode := runExperimentBuiltBinary(t, bin, repo.Dir, nil, wave5CReleaseArgs("release-workspaces", repo.Head, false)...)
	if humanCode != retryCode || humanErr != "" || !strings.HasPrefix(humanOut, "experiment release-workspaces: clean (clean)\n") {
		t.Fatalf("human/JSON parity=%d/%q/%q versus JSON %d", humanCode, humanOut, humanErr, retryCode)
	}

	// Closure consumes the capsule only after those exact bytes are accepted
	// in the same repository that carried the unlocked draft and every prior
	// authority transition. Keep the adopted real policy and its generated
	// instruction projection in that accepted tree, and exercise the real
	// conflict gate through its established hermetic no-conflict provider
	// seam. The gate still probes the real adopted store, strict-decodes the
	// real context request, seals the current branch/HEAD, and only then lets
	// the real CSE closure evidence provider run.
	if _, err := instructionprojection.Generate(repo.Dir); err != nil {
		t.Fatalf("generate retained journey policy projection: %v", err)
	}
	honestManifest := append([]byte(nil), manifestBytes...)
	tamperedManifest, err := experiment.DecodeCapsuleManifest(honestManifest)
	if err != nil {
		t.Fatal(err)
	}
	tamperedManifest.Selected = "baseline"
	tamperedBytes, err := experiment.EncodeCapsuleManifest(tamperedManifest)
	if err != nil {
		t.Fatalf("encode schema-valid tampered capsule: %v", err)
	}
	if err := os.WriteFile(wave5CCapsuleManifestPath(repo.Dir), tamperedBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	runGitForExperimentTest(t, repo.Dir, "add",
		".verdi/specs/active/request-path-spike/experiments/request-path-v2/selected/capsule-manifest.json",
		".verdi/policy/projections", "AGENTS.md")
	runGitForExperimentTest(t, repo.Dir, "commit", "-q", "-m", "accept adversarial journey capsule")
	repo.Head = contextE2ECurrentHead(t, repo.Dir)
	writeCloseExperimentWaiver(t, repo.Dir, repo.Head)
	writeCloseExperimentGateReportFor(t, repo.Dir, "request-path-spike", repo.Head)
	requestPath := contextLifecycleRequestFile(t, repo.Dir, "journey-close-context.json", "spec/request-path-spike", contextcompile.PhaseReview, nil)
	conflictCalls := 0
	conflictProvider := contextConflictProviderFunc(func(_ context.Context, request policyconflict.Request) (policyconflict.Result, error) {
		conflictCalls++
		accepted := request.Target.AcceptedContext
		if request.Target.Kind != policyconflict.TargetAcceptedContext || accepted == nil || accepted.Phase != contextcompile.PhaseReview || accepted.Spec != "spec/request-path-spike" {
			t.Fatalf("journey close conflict target=%+v, want accepted review context", request.Target)
		}
		return lifecycleConflictResult(policyconflict.VerdictPass), nil
	})
	runJourneyPreflight := func() (string, string, int) {
		t.Helper()
		cfg, err := store.Open(repo.Dir)
		if err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		code := runPreflightWithConflict(context.Background(), repo.Dir, "spec/request-path-spike", cfg.Manifest, cfg.Model, forgefake.New(), true, requestPath, conflictProvider, &stdout, &stderr)
		return stdout.String(), stderr.String(), code
	}
	beforeCloseRefusal := contextE2EPorcelainStatus(t, repo.Dir)
	closeOut, closeErr, closeCode := runJourneyPreflight()
	if closeCode != 1 || closeErr != "" || !strings.Contains(closeOut, "constitutional conflict: state: pass") || !strings.Contains(closeOut, "[FAIL] experiment request-path-v2") || !strings.Contains(closeOut, "close: --preflight: NOT READY") {
		t.Fatalf("combined conflict/CSE refusal exit/stdout/stderr=%d/%q/%q", closeCode, closeOut, closeErr)
	}
	if contextE2ECurrentHead(t, repo.Dir) != repo.Head || contextE2EPorcelainStatus(t, repo.Dir) != beforeCloseRefusal || !bytes.Equal(tamperedBytes, mustReadWave5CFile(t, wave5CCapsuleManifestPath(repo.Dir))) {
		t.Fatal("combined conflict/CSE preflight refusal mutated repository state")
	}
	if err := os.WriteFile(wave5CCapsuleManifestPath(repo.Dir), honestManifest, 0o600); err != nil {
		t.Fatal(err)
	}
	runGitForExperimentTest(t, repo.Dir, "add", ".verdi/specs/active/request-path-spike/experiments/request-path-v2/selected/capsule-manifest.json")
	runGitForExperimentTest(t, repo.Dir, "commit", "-q", "-m", "restore verified journey capsule")
	repo.Head = contextE2ECurrentHead(t, repo.Dir)
	writeCloseExperimentWaiver(t, repo.Dir, repo.Head)
	writeCloseExperimentGateReportFor(t, repo.Dir, "request-path-spike", repo.Head)
	closeOut, closeErr, closeCode = runJourneyPreflight()
	if closeCode != 0 || closeErr != "" || !strings.Contains(closeOut, "constitutional conflict: state: pass") || !strings.Contains(closeOut, "close: --preflight: READY") {
		t.Fatalf("close --preflight exit/stdout/stderr=%d/%q/%q", closeCode, closeOut, closeErr)
	}
	for _, policy := range []string{"organization.md", "project.md"} {
		runGitForExperimentTest(t, repo.Dir, "cat-file", "-e", "HEAD:.verdi/policy/policies/"+policy)
	}
	acceptedProjectPolicy := experimentGitOutput(t, repo.Dir, "show", "HEAD:.verdi/policy/policies/project.md")
	if strings.Contains(acceptedProjectPolicy, "storage-throughput") || !strings.Contains(acceptedProjectPolicy, "observation_bytes: 131072") || !strings.Contains(acceptedProjectPolicy, "retained_artifact_bytes: 4194304") {
		t.Fatalf("accepted project policy was not the honest narrowed layer:\n%s", acceptedProjectPolicy)
	}
	if conflictCalls != 2 {
		t.Fatalf("journey close conflict provider calls=%d, want one per CSE preflight", conflictCalls)
	}

	wantOperations := []string{
		"capture-candidate", "discover-capabilities", "draft-definition", "explain-result", "inspect",
		"propose-ratification", "propose-registration", "publish-capsule", "reconcile-draft",
		"release-workspaces", "resume", "review-registration", "start", "status", "validate-draft",
	}
	registeredOperations := make([]string, 0, len(experimentOperationUsage))
	for operation := range experimentOperationUsage {
		registeredOperations = append(registeredOperations, operation)
	}
	sort.Strings(registeredOperations)
	if !reflect.DeepEqual(registeredOperations, wantOperations) {
		t.Fatalf("production CLI experiment registry=%v, want fixed authority set %v", registeredOperations, wantOperations)
	}
	gotOperations := make([]string, 0, len(seen))
	for operation := range seen {
		gotOperations = append(gotOperations, operation)
	}
	sort.Strings(gotOperations)
	if !reflect.DeepEqual(gotOperations, registeredOperations) {
		t.Fatalf("CLI experiment operation set=%v, want every registered operation %v", gotOperations, registeredOperations)
	}
	if !exits[0] || !exits[1] || !exits[2] {
		t.Fatalf("CLI journey exit classes=%v, want 0/1/2", exits)
	}

	// Equivalent standalone fixture construction remains independent of
	// ambient Git identity; it is an adversarial determinism witness, never a
	// replacement for a stage of the continuous journey above.
	first, second := buildWave5CAcceptedResult(t, bin), buildWave5CAcceptedResult(t, bin)
	if second.repo.Head != first.repo.Head || second.resultDigest != first.resultDigest || !reflect.DeepEqual(second.targets, first.targets) {
		t.Fatalf("fixture identity drift: first=%s/%s/%v second=%s/%s/%v",
			first.repo.Head, first.resultDigest, first.targets, second.repo.Head, second.resultDigest, second.targets)
	}
}

func installCLIJourneyClosureTarget(t *testing.T, repo *fixturegit.Repo) {
	t.Helper()
	spike := strings.Replace(closeExperimentSpikeSpecMD, "id: spec/exp-spike", "id: spec/request-path-spike", 1)
	if spike == closeExperimentSpikeSpecMD {
		t.Fatal("close experiment spike fixture no longer has the expected id")
	}
	feature := strings.Replace(featureV1SpecMD,
		"frozen: { at: 2024-01-01, commit: 0000000000000000000000000000000000000a }",
		"open_questions:\n  - { id: oq-1, text: \"which candidate wins\", anchor: oq-1 }\n"+
			"frozen: { at: 2024-01-01, commit: 0000000000000000000000000000000000000a }", 1)
	if feature == featureV1SpecMD {
		t.Fatal("parent feature fixture no longer has the expected frozen block")
	}
	for name, content := range map[string]string{
		".verdi/verdi.yaml":                              "schema: verdi.layout/v1\nforge: github\nproviders:\n  jira:\n    mode: fake\n    base_url: https://example.atlassian.net\n    rollup_field: customfield_00000\n",
		".verdi/specs/active/loan-mgmt/spec.md":          feature,
		".verdi/specs/active/request-path-spike/spec.md": spike,
	} {
		closeExperimentWriteFixtureFile(t, repo.Dir, name, content)
	}
	runGitForExperimentTest(t, repo.Dir, "add", ".")
	runGitForExperimentTest(t, repo.Dir, "commit", "-q", "-m", "add journey closure target")
	repo.Head = contextE2ECurrentHead(t, repo.Dir)
}

func assertCLIJourneyCanonicalJSON(t *testing.T, operation, raw string) {
	t.Helper()
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		t.Fatalf("%s did not return JSON: %v\n%s", operation, err, raw)
	}
	want, err := canonjson.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if raw != string(want) {
		t.Fatalf("%s JSON is not canonical\ngot:  %q\nwant: %q", operation, raw, want)
	}
}
