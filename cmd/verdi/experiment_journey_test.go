package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/experimentapp"
	"github.com/jyang234/verdi/internal/experimentrun"
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
	draftRepo := buildExperimentHumanRepo(t, privateKey.Public().(ed25519.PublicKey))
	experimentDir := filepath.Dir(wave5CDefinitionPath(draftRepo.Dir))
	originalDefinition := mustReadWave5CFile(t, wave5CDefinitionPath(draftRepo.Dir))
	newDefinition := bytes.Replace(originalDefinition, []byte("id: request-path-v2"), []byte("id: request-path-v3"), 1)
	definitionInput := filepath.Join(t.TempDir(), "experiment-v3.yaml")
	if err := os.WriteFile(definitionInput, newDefinition, 0o600); err != nil {
		t.Fatal(err)
	}
	draftArgs := []string{
		"experiment", "draft-definition", "--spike", "spec/request-path-spike", "--experiment", "request-path-v3",
		"--accepted-head", draftRepo.Head, "--definition", definitionInput, "--candidate-root", experimentDir, "--json",
	}
	if _, code := callJSON(draftRepo.Dir, draftArgs...); code != 0 {
		t.Fatalf("draft-definition exit=%d, want clean", code)
	}

	changedPatch := []byte("diff --git a/spikes/cache.go b/spikes/cache.go\nindex 1111111..2222222 100644\n--- a/spikes/cache.go\n+++ b/spikes/cache.go\n@@ -1 +1 @@\n-old\n+cli-journey\n")
	changedDefinition := bytes.Replace(originalDefinition,
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
	captureArgs := append(base("capture-candidate", draftRepo.Head)[:len(base("capture-candidate", draftRepo.Head))-1],
		"--candidate", "cache", "--patch", patchInput, "--definition", definitionV2Input, "--json")
	if _, code := callJSON(draftRepo.Dir, captureArgs...); code != 0 {
		t.Fatalf("capture-candidate exit=%d, want clean", code)
	}

	runGitForExperimentTest(t, draftRepo.Dir, "checkout", "-q", "-b", "journey-proposal")
	if err := os.WriteFile(filepath.Join(experimentDir, "human-note.txt"), []byte("direct Git reconciliation witness\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGitForExperimentTest(t, draftRepo.Dir, "add", ".")
	runGitForExperimentTest(t, draftRepo.Dir, "commit", "-q", "-m", "capture direct journey edit")
	beforeReview := contextE2EPorcelainStatus(t, draftRepo.Dir)
	reviewOut, reviewCode := callJSON(draftRepo.Dir, base("review-registration", draftRepo.Head)...)
	if reviewCode != 1 || !strings.Contains(reviewOut, `"code":"direct-draft-unreconciled"`) || contextE2EPorcelainStatus(t, draftRepo.Dir) != beforeReview {
		t.Fatalf("unreconciled review=%d/%q or mutated the worktree", reviewCode, reviewOut)
	}

	proofDir := t.TempDir()
	humanOperation := func(operation string) {
		t.Helper()
		args := base(operation, draftRepo.Head)
		challengeOut, code := callJSON(draftRepo.Dir, args...)
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
		if out, proofCode := callJSON(draftRepo.Dir, withProof...); proofCode != 0 || !strings.Contains(out, `"classification":"clean"`) {
			t.Fatalf("%s proof exit=%d output=%q", operation, proofCode, out)
		}
	}
	humanOperation("reconcile-draft")
	humanOperation("propose-registration")

	// Continue from a deterministic accepted lock and retained result. This
	// helper itself uses the same built binary for the accepted lock; the
	// calls below cover the complete public operation set as one journey.
	fixture := buildWave5CAcceptedResult(t, bin)
	repo := fixture.repo
	acceptedResultHead := repo.Head
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

	wantOperations := []string{
		"capture-candidate", "discover-capabilities", "draft-definition", "explain-result", "inspect",
		"propose-ratification", "propose-registration", "publish-capsule", "reconcile-draft",
		"release-workspaces", "resume", "review-registration", "start", "status", "validate-draft",
	}
	gotOperations := make([]string, 0, len(seen))
	for operation := range seen {
		gotOperations = append(gotOperations, operation)
	}
	sort.Strings(gotOperations)
	if !reflect.DeepEqual(gotOperations, wantOperations) {
		t.Fatalf("CLI experiment operation set=%v, want exactly all 15 %v", gotOperations, wantOperations)
	}
	if !exits[0] || !exits[1] || !exits[2] {
		t.Fatalf("CLI journey exit classes=%v, want 0/1/2", exits)
	}

	// Equivalent fixture construction is independent of ambient Git identity
	// and pins the accepted HEAD, result digest, and workspace identities.
	second := buildWave5CAcceptedResult(t, bin)
	if second.repo.Head != acceptedResultHead || second.resultDigest != fixture.resultDigest || !reflect.DeepEqual(second.targets, fixture.targets) {
		t.Fatalf("fixture identity drift: first=%s/%s/%v second=%s/%s/%v",
			acceptedResultHead, fixture.resultDigest, fixture.targets, second.repo.Head, second.resultDigest, second.targets)
	}
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
