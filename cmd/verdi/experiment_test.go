package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/experimentapp"
	"github.com/jyang234/verdi/internal/experimenthuman"
	"github.com/jyang234/verdi/internal/experimentrun"
	"github.com/jyang234/verdi/internal/fixturegit"
)

func runExperimentBuiltBinary(t *testing.T, bin, dir string, stdin []byte, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(stdin)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return stdout.String(), stderr.String(), 0
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("run %v: %v", args, err)
	}
	return stdout.String(), stderr.String(), exitErr.ExitCode()
}

func TestExperimentOperationGrammarBuiltBinary(t *testing.T) {
	bin := buildVerdiBinary(t)
	operations := []string{
		"inspect", "discover-capabilities", "validate-draft", "review-registration", "status", "explain-result",
		"draft-definition", "capture-candidate", "reconcile-draft", "propose-registration", "start", "resume",
	}
	for _, operation := range operations {
		t.Run(operation, func(t *testing.T) {
			stdout, stderr, code := runExperimentBuiltBinary(t, bin, t.TempDir(), nil, "experiment", operation)
			if code != 2 || stdout != "" || !strings.Contains(stderr, "--spike is required") || !strings.Contains(stderr, experimentOperationUsage[operation]) {
				t.Fatalf("exit/stdout/stderr = %d/%q/%q, want strict operation usage refusal", code, stdout, stderr)
			}
		})
	}

	common := []string{"experiment", "inspect", "--spike", "spec/example", "--experiment", "comparison", "--accepted-head", strings.Repeat("a", 40)}
	for _, suffix := range [][]string{{"--spike", "spec/duplicate"}, {"--unknown", "value"}, {"extra"}, {"--json", "--json"}} {
		_, stderr, code := runExperimentBuiltBinary(t, bin, t.TempDir(), nil, append(common, suffix...)...)
		if code != 2 || !strings.Contains(stderr, "usage: verdi experiment inspect") {
			t.Fatalf("strict grammar %v: exit=%d stderr=%q", suffix, code, stderr)
		}
	}
}

func TestExperimentTopLevelUsageRowBuiltBinary(t *testing.T) {
	bin := buildVerdiBinary(t)
	stdout, stderr, code := runExperimentBuiltBinary(t, bin, t.TempDir(), nil, "definitely-not-a-verb")
	if code != 2 || stdout != "" {
		t.Fatalf("exit/stdout = %d/%q, want top-level usage refusal", code, stdout)
	}
	if strings.Contains(stderr, "\t") {
		t.Fatalf("top-level usage contains a tab, legacy usage bytes must stay space-indented: %q", stderr)
	}
	if !strings.Contains(stderr, "\n       context, experiment\n") {
		t.Fatalf("top-level usage omits the experiment inventory row: %q", stderr)
	}
}

func TestExperimentInputBindingsBuiltBinary(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{".verdi/verdi.yaml": "schema: verdi.layout/v1\n", ".verdi/.gitignore": "data/\n"},
		Message: "binding transport fixture",
	}})
	digest := "sha256:" + strings.Repeat("1", 64)
	bindings := experimentrun.InputBindings{Schema: experimentrun.InputBindingSchema, Inputs: []experimentrun.InputBinding{
		{Slot: experimentrun.InputSlotContract, ID: "shared-input", Digest: digest, Path: "inputs/contract.txt"},
		{Slot: experimentrun.InputSlotWorkload, ID: "shared-input", Digest: digest, Path: "inputs/workload.txt"},
	}}
	canonical, err := experimentrun.EncodeInputBindings(bindings)
	if err != nil {
		t.Fatal(err)
	}
	canonicalPath := filepath.Join(repo.Dir, "bindings.json")
	if err := os.WriteFile(canonicalPath, canonical, 0o600); err != nil {
		t.Fatal(err)
	}
	malformedPath := filepath.Join(repo.Dir, "malformed.json")
	if err := os.WriteFile(malformedPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	base := []string{"experiment", "start", "--spike", "spec/example", "--experiment", "comparison", "--accepted-head", repo.Head, "--run", "run-1", "--inputs"}
	for _, test := range []struct {
		name  string
		input string
		stdin []byte
	}{
		{name: "malformed-file", input: malformedPath},
		{name: "noncanonical-stdin", input: "-", stdin: bytes.Replace(canonical, []byte(`{"inputs"`), []byte("{ \"inputs\""), 1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			before := contextE2EPorcelainStatus(t, repo.Dir)
			stdout, stderr, code := runExperimentBuiltBinary(t, bin, repo.Dir, test.stdin, append(base, test.input)...)
			if code != 2 || stdout != "" || !strings.Contains(stderr, "decoding --inputs") {
				t.Fatalf("exit/stdout/stderr = %d/%q/%q, want operational strict-decode refusal", code, stdout, stderr)
			}
			if after := contextE2EPorcelainStatus(t, repo.Dir); after != before {
				t.Fatalf("worktree changed on binding refusal: before=%q after=%q", before, after)
			}
			if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "data", "execution")); !os.IsNotExist(err) {
				t.Fatalf("runner evidence exists after binding refusal: %v", err)
			}
		})
	}

	for _, transport := range []struct {
		name  string
		input string
		stdin []byte
	}{{name: "file", input: canonicalPath}, {name: "stdin", input: "-", stdin: canonical}} {
		t.Run("canonical-identical-ids-"+transport.name, func(t *testing.T) {
			stdout, stderr, code := runExperimentBuiltBinary(t, bin, repo.Dir, transport.stdin, append(base, transport.input, "--json")...)
			if code != 2 || stderr != "" || strings.Contains(stdout, "input bindings are not canonical") {
				t.Fatalf("exit/stdout/stderr = %d/%q/%q, want shared decoder acceptance followed by accepted-tree operational result", code, stdout, stderr)
			}
			if !strings.Contains(stdout, `"code":"accepted-tree-invalid"`) {
				t.Fatalf("stdout = %q, want application accepted-tree result after binding decode", stdout)
			}
		})
	}
}

type experimentChallengeOutput struct {
	Outcome   experimentapp.Outcome     `json:"outcome"`
	Challenge experimenthuman.Challenge `json:"challenge"`
}

func TestExperimentHumanProofBuiltBinary(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	repo := buildExperimentHumanRepo(t, publicKey)
	bin := buildVerdiBinary(t)
	runGitForExperimentTest(t, repo.Dir, "checkout", "-q", "-b", "proposal")
	experimentDir := filepath.Join(repo.Dir, ".verdi", "specs", "active", "request-path-spike", "experiments", "request-path-v2")
	if err := os.WriteFile(filepath.Join(experimentDir, "human-note.txt"), []byte("first direct edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitForExperimentTest(t, repo.Dir, "add", ".")
	runGitForExperimentTest(t, repo.Dir, "commit", "-q", "-m", "first direct edit")

	matchedBindings := experimentrun.InputBindings{Schema: experimentrun.InputBindingSchema, Inputs: []experimentrun.InputBinding{
		{Slot: experimentrun.InputSlotContract, ID: "request-contract", Digest: "sha256:" + strings.Repeat("6", 64), Path: "inputs/contract.txt"},
		{Slot: experimentrun.InputSlotWorkload, ID: "request-mix", Digest: "sha256:" + strings.Repeat("5", 64), Path: "inputs/workload.txt"},
	}}
	matchedDoc, err := experimentrun.EncodeInputBindings(matchedBindings)
	if err != nil {
		t.Fatal(err)
	}
	bindingDir := t.TempDir()
	matchedPath := filepath.Join(bindingDir, "matched.json")
	if err := os.WriteFile(matchedPath, matchedDoc, 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runExperimentBuiltBinary(t, bin, repo.Dir, nil,
		"experiment", "start", "--spike", "spec/request-path-spike", "--experiment", "request-path-v2",
		"--accepted-head", repo.Head, "--run", "run-1", "--inputs", matchedPath, "--json")
	if code != 1 || stderr != "" || !strings.Contains(stdout, `"code":"registration-not-accepted"`) {
		t.Fatalf("pre-registration start exit/stdout/stderr = %d/%q/%q, want registration-not-accepted verdict", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "data", "execution")); !os.IsNotExist(err) {
		t.Fatalf("runner evidence exists after pre-registration refusal: %v", err)
	}

	base := []string{"experiment", "reconcile-draft", "--spike", "spec/request-path-spike", "--experiment", "request-path-v2", "--accepted-head", repo.Head, "--json"}
	stdout, stderr, code = runExperimentBuiltBinary(t, bin, repo.Dir, nil, base...)
	if code != 1 || stderr != "" || !strings.Contains(stdout, `"code":"human-authorization-required"`) || !strings.Contains(stdout, experimentHumanPrompt) {
		t.Fatalf("challenge exit/stdout/stderr = %d/%q/%q", code, stdout, stderr)
	}
	challenge := decodeExperimentChallengeOutput(t, stdout).Challenge
	challengeBytes, err := challenge.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	if challenge.AcceptedHEAD != repo.Head {
		t.Fatalf("challenge accepted head = %q, want the exact accepted HEAD %q", challenge.AcceptedHEAD, repo.Head)
	}
	if challenge.Operation != experimenthuman.OperationReconcileDraft {
		t.Fatalf("challenge operation = %q, want %q", challenge.Operation, experimenthuman.OperationReconcileDraft)
	}
	if proposalHead := contextE2ECurrentHead(t, repo.Dir); challenge.ProposalHEAD != proposalHead {
		t.Fatalf("challenge proposal head = %q, want current proposal HEAD %q", challenge.ProposalHEAD, proposalHead)
	}

	humanOut, humanErr, humanCode := runExperimentBuiltBinary(t, bin, repo.Dir, nil, base[:len(base)-1]...)
	if humanCode != 1 || humanErr != "" {
		t.Fatalf("human challenge rendering exit/stderr = %d/%q", humanCode, humanErr)
	}
	if !strings.Contains(humanOut, "human-authorization-required") || !strings.Contains(humanOut, string(challengeBytes)) || !strings.Contains(humanOut, experimentHumanPrompt) {
		t.Fatalf("human challenge rendering must include the exact canonical challenge and manual signing prompt as data:\n%s", humanOut)
	}
	oldSignature := ed25519.Sign(privateKey, challengeBytes)

	// Change both the proposal HEAD and its human-artifact projection. The old
	// raw signature can only classify as invalid at the CLI boundary because
	// the CLI reconstructs the new current challenge and accepts no challenge
	// transport of its own.
	if err := os.WriteFile(filepath.Join(experimentDir, "human-note.txt"), []byte("second direct edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitForExperimentTest(t, repo.Dir, "add", ".")
	runGitForExperimentTest(t, repo.Dir, "commit", "-q", "-m", "second direct edit")
	proofDir := t.TempDir()
	oldProof := filepath.Join(proofDir, "old.sig")
	if err := os.WriteFile(oldProof, oldSignature, 0o600); err != nil {
		t.Fatal(err)
	}
	before := contextE2EPorcelainStatus(t, repo.Dir)
	oldArgs := append(append([]string{}, base...), "--human-proof", oldProof)
	stdout, stderr, code = runExperimentBuiltBinary(t, bin, repo.Dir, nil, oldArgs...)
	if code != 1 || stderr != "" || !strings.Contains(stdout, `"code":"human-proof-invalid"`) {
		t.Fatalf("old proof exit/stdout/stderr = %d/%q/%q", code, stdout, stderr)
	}
	if after := contextE2EPorcelainStatus(t, repo.Dir); after != before {
		t.Fatalf("old proof mutated worktree: before=%q after=%q", before, after)
	}

	wrongLength := filepath.Join(proofDir, "short.sig")
	if err := os.WriteFile(wrongLength, make([]byte, 63), 0o600); err != nil {
		t.Fatal(err)
	}
	shortArgs := append(append([]string{}, base...), "--human-proof", wrongLength)
	stdout, stderr, code = runExperimentBuiltBinary(t, bin, repo.Dir, nil, shortArgs...)
	if code != 2 || stdout != "" || !strings.Contains(stderr, "want 64") {
		t.Fatalf("short proof exit/stdout/stderr = %d/%q/%q", code, stdout, stderr)
	}
	if after := contextE2EPorcelainStatus(t, repo.Dir); after != before {
		t.Fatalf("short proof mutated worktree: before=%q after=%q", before, after)
	}

	stdout, stderr, code = runExperimentBuiltBinary(t, bin, repo.Dir, nil, base...)
	if code != 1 || stderr != "" {
		t.Fatalf("current challenge exit/stdout/stderr = %d/%q/%q", code, stdout, stderr)
	}
	current := decodeExperimentChallengeOutput(t, stdout).Challenge
	currentBytes, err := current.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	_, foreignPrivateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	foreignProof := filepath.Join(proofDir, "foreign.sig")
	if err := os.WriteFile(foreignProof, ed25519.Sign(foreignPrivateKey, currentBytes), 0o600); err != nil {
		t.Fatal(err)
	}
	foreignArgs := append(append([]string{}, base...), "--human-proof", foreignProof)
	stdout, stderr, code = runExperimentBuiltBinary(t, bin, repo.Dir, nil, foreignArgs...)
	if code != 1 || stderr != "" || !strings.Contains(stdout, `"code":"human-proof-invalid"`) {
		t.Fatalf("foreign-key proof exit/stdout/stderr = %d/%q/%q", code, stdout, stderr)
	}
	if after := contextE2EPorcelainStatus(t, repo.Dir); after != before {
		t.Fatalf("foreign-key proof mutated worktree: before=%q after=%q", before, after)
	}

	longProof := filepath.Join(proofDir, "long.sig")
	if err := os.WriteFile(longProof, make([]byte, 65), 0o600); err != nil {
		t.Fatal(err)
	}
	longArgs := append(append([]string{}, base...), "--human-proof", longProof)
	stdout, stderr, code = runExperimentBuiltBinary(t, bin, repo.Dir, nil, longArgs...)
	if code != 2 || stdout != "" || !strings.Contains(stderr, "exceeds 64") {
		t.Fatalf("long proof exit/stdout/stderr = %d/%q/%q", code, stdout, stderr)
	}
	if after := contextE2EPorcelainStatus(t, repo.Dir); after != before {
		t.Fatalf("long proof mutated worktree: before=%q after=%q", before, after)
	}

	dashArgs := append(append([]string{}, base...), "--human-proof", "-")
	stdout, stderr, code = runExperimentBuiltBinary(t, bin, repo.Dir, nil, dashArgs...)
	if code != 2 || stdout != "" || strings.Contains(stderr, "panic") || !strings.Contains(stderr, "signature file path") {
		t.Fatalf("dash proof exit/stdout/stderr = %d/%q/%q, want a clean operational file-path refusal", code, stdout, stderr)
	}
	if after := contextE2EPorcelainStatus(t, repo.Dir); after != before {
		t.Fatalf("dash proof mutated worktree: before=%q after=%q", before, after)
	}

	currentProof := filepath.Join(proofDir, "current.sig")
	if err := os.WriteFile(currentProof, ed25519.Sign(privateKey, currentBytes), 0o600); err != nil {
		t.Fatal(err)
	}
	currentArgs := append(append([]string{}, base...), "--human-proof", currentProof)
	stdout, stderr, code = runExperimentBuiltBinary(t, bin, repo.Dir, nil, currentArgs...)
	if code != 0 || stderr != "" || !strings.Contains(stdout, `"classification":"clean"`) {
		t.Fatalf("verified reconcile exit/stdout/stderr = %d/%q/%q", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(experimentDir, "mutation-provenance.jsonl")); err != nil {
		t.Fatalf("verified reconcile omitted provenance: %v", err)
	}

	registrationArgs := []string{"experiment", "propose-registration", "--spike", "spec/request-path-spike", "--experiment", "request-path-v2", "--accepted-head", repo.Head, "--json"}
	stdout, stderr, code = runExperimentBuiltBinary(t, bin, repo.Dir, nil, registrationArgs...)
	if code != 1 || stderr != "" {
		t.Fatalf("registration challenge exit/stdout/stderr = %d/%q/%q", code, stdout, stderr)
	}
	registrationChallenge := decodeExperimentChallengeOutput(t, stdout).Challenge
	registrationBytes, err := registrationChallenge.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	registrationProof := filepath.Join(proofDir, "registration.sig")
	if err := os.WriteFile(registrationProof, ed25519.Sign(privateKey, registrationBytes), 0o600); err != nil {
		t.Fatal(err)
	}
	registrationArgs = append(registrationArgs, "--human-proof", registrationProof)
	stdout, stderr, code = runExperimentBuiltBinary(t, bin, repo.Dir, nil, registrationArgs...)
	if code != 0 || stderr != "" || !strings.Contains(stdout, `"classification":"clean"`) {
		t.Fatalf("verified registration exit/stdout/stderr = %d/%q/%q", code, stdout, stderr)
	}

	runGitForExperimentTest(t, repo.Dir, "add", ".")
	runGitForExperimentTest(t, repo.Dir, "commit", "-q", "-m", "registration lock")
	runGitForExperimentTest(t, repo.Dir, "checkout", "-q", "main")
	runGitForExperimentTest(t, repo.Dir, "merge", "-q", "--ff-only", "proposal")
	acceptedHead := contextE2ECurrentHead(t, repo.Dir)

	mismatchedBindings := experimentrun.InputBindings{Schema: experimentrun.InputBindingSchema, Inputs: []experimentrun.InputBinding{
		{Slot: experimentrun.InputSlotContract, ID: "request-contract", Digest: "sha256:" + strings.Repeat("6", 64), Path: "inputs/contract.txt"},
		{Slot: experimentrun.InputSlotWorkload, ID: "request-mix", Digest: "sha256:" + strings.Repeat("7", 64), Path: "inputs/workload.txt"},
	}}
	mismatchedDoc, err := experimentrun.EncodeInputBindings(mismatchedBindings)
	if err != nil {
		t.Fatal(err)
	}
	mismatchedPath := filepath.Join(bindingDir, "mismatched.json")
	if err := os.WriteFile(mismatchedPath, mismatchedDoc, 0o600); err != nil {
		t.Fatal(err)
	}
	acceptedBefore := contextE2EPorcelainStatus(t, repo.Dir)
	for _, operation := range []string{"start", "resume"} {
		stdout, stderr, code = runExperimentBuiltBinary(t, bin, repo.Dir, nil,
			"experiment", operation, "--spike", "spec/request-path-spike", "--experiment", "request-path-v2",
			"--accepted-head", acceptedHead, "--run", "run-1", "--inputs", mismatchedPath, "--json")
		if code != 2 || stderr != "" || !strings.Contains(stdout, `"code":"input-binding-invalid"`) {
			t.Fatalf("%s mismatched bindings exit/stdout/stderr = %d/%q/%q, want operational input-binding-invalid", operation, code, stdout, stderr)
		}
		if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "data", "execution")); !os.IsNotExist(err) {
			t.Fatalf("%s runner evidence exists after mismatched-binding refusal: %v", operation, err)
		}
		if after := contextE2EPorcelainStatus(t, repo.Dir); after != acceptedBefore {
			t.Fatalf("%s mismatched bindings mutated worktree: before=%q after=%q", operation, acceptedBefore, after)
		}
	}
}

func experimentGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

func TestExperimentAcceptedPolicySymlinkModeBuiltBinary(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	repo := buildExperimentHumanRepo(t, publicKey)
	bin := buildVerdiBinary(t)

	// Real-Git probe: retype the still-valid policy blob to a committed
	// symlink entry (mode 120000) without changing its bytes, and commit an
	// unrelated symlink elsewhere in the same accepted tree.
	if err := os.Symlink("layers.txt", filepath.Join(repo.Dir, "docs-link")); err != nil {
		t.Fatal(err)
	}
	runGitForExperimentTest(t, repo.Dir, "add", "docs-link")
	policyBlob := experimentGitOutput(t, repo.Dir, "rev-parse", "HEAD:.verdi/policy/policies/experiment.md")
	runGitForExperimentTest(t, repo.Dir, "update-index", "--cacheinfo", "120000,"+policyBlob+",.verdi/policy/policies/experiment.md")
	runGitForExperimentTest(t, repo.Dir, "commit", "-q", "-m", "retype accepted policy entry")
	acceptedHead := contextE2ECurrentHead(t, repo.Dir)

	base := []string{"experiment", "reconcile-draft", "--spike", "spec/request-path-spike", "--experiment", "request-path-v2", "--accepted-head", acceptedHead, "--json"}
	stdout, stderr, code := runExperimentBuiltBinary(t, bin, repo.Dir, nil, base...)
	if code != 1 || stderr != "" || !strings.Contains(stdout, `"code":"human-authorization-required"`) {
		t.Fatalf("challenge exit/stdout/stderr = %d/%q/%q", code, stdout, stderr)
	}
	challengeBytes, err := decodeExperimentChallengeOutput(t, stdout).Challenge.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	proof := filepath.Join(t.TempDir(), "valid.sig")
	if err := os.WriteFile(proof, ed25519.Sign(privateKey, challengeBytes), 0o600); err != nil {
		t.Fatal(err)
	}
	before := contextE2EPorcelainStatus(t, repo.Dir)
	proofArgs := append(append([]string{}, base...), "--human-proof", proof)
	stdout, stderr, code = runExperimentBuiltBinary(t, bin, repo.Dir, nil, proofArgs...)
	if code != 2 || stdout != "" || !strings.Contains(stderr, "under .verdi/policy/") || !strings.Contains(stderr, "symlink") {
		t.Fatalf("retyped policy entry exit/stdout/stderr = %d/%q/%q, want operational symlink refusal naming the policy entry", code, stdout, stderr)
	}
	if after := contextE2EPorcelainStatus(t, repo.Dir); after != before {
		t.Fatalf("retyped policy entry mutated worktree: before=%q after=%q", before, after)
	}
}

func TestExperimentUnrelatedAcceptedSymlinkBuiltBinary(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	repo := buildExperimentHumanRepo(t, publicKey)
	bin := buildVerdiBinary(t)

	// A committed symlink OUTSIDE .verdi/policy must not be rejected by the
	// accepted-tree helper: the full human reconcile flow stays clean.
	if err := os.Symlink("layers.txt", filepath.Join(repo.Dir, "docs-link")); err != nil {
		t.Fatal(err)
	}
	runGitForExperimentTest(t, repo.Dir, "add", "docs-link")
	runGitForExperimentTest(t, repo.Dir, "commit", "-q", "-m", "unrelated accepted symlink")
	acceptedHead := contextE2ECurrentHead(t, repo.Dir)

	runGitForExperimentTest(t, repo.Dir, "checkout", "-q", "-b", "proposal")
	experimentDir := filepath.Join(repo.Dir, ".verdi", "specs", "active", "request-path-spike", "experiments", "request-path-v2")
	if err := os.WriteFile(filepath.Join(experimentDir, "human-note.txt"), []byte("direct edit beside a symlink\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitForExperimentTest(t, repo.Dir, "add", ".")
	runGitForExperimentTest(t, repo.Dir, "commit", "-q", "-m", "direct edit")

	base := []string{"experiment", "reconcile-draft", "--spike", "spec/request-path-spike", "--experiment", "request-path-v2", "--accepted-head", acceptedHead, "--json"}
	stdout, stderr, code := runExperimentBuiltBinary(t, bin, repo.Dir, nil, base...)
	if code != 1 || stderr != "" || !strings.Contains(stdout, `"code":"human-authorization-required"`) {
		t.Fatalf("challenge exit/stdout/stderr = %d/%q/%q", code, stdout, stderr)
	}
	challengeBytes, err := decodeExperimentChallengeOutput(t, stdout).Challenge.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	proof := filepath.Join(t.TempDir(), "valid.sig")
	if err := os.WriteFile(proof, ed25519.Sign(privateKey, challengeBytes), 0o600); err != nil {
		t.Fatal(err)
	}
	proofArgs := append(append([]string{}, base...), "--human-proof", proof)
	stdout, stderr, code = runExperimentBuiltBinary(t, bin, repo.Dir, nil, proofArgs...)
	if code != 0 || stderr != "" || !strings.Contains(stdout, `"classification":"clean"`) {
		t.Fatalf("reconcile beside unrelated symlink exit/stdout/stderr = %d/%q/%q, want clean completion", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(experimentDir, "mutation-provenance.jsonl")); err != nil {
		t.Fatalf("reconcile beside unrelated symlink omitted provenance: %v", err)
	}
}

func TestExperimentHumanKeyUnmappedBuiltBinary(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	repo := buildExperimentHumanRepoWithSubjects(t, []string{"user:alice"})
	bin := buildVerdiBinary(t)

	base := []string{"experiment", "reconcile-draft", "--spike", "spec/request-path-spike", "--experiment", "request-path-v2", "--accepted-head", repo.Head, "--json"}
	stdout, stderr, code := runExperimentBuiltBinary(t, bin, repo.Dir, nil, base...)
	if code != 1 || stderr != "" || !strings.Contains(stdout, `"code":"human-authorization-required"`) {
		t.Fatalf("challenge exit/stdout/stderr = %d/%q/%q", code, stdout, stderr)
	}
	challengeBytes, err := decodeExperimentChallengeOutput(t, stdout).Challenge.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	proof := filepath.Join(t.TempDir(), "unmapped.sig")
	if err := os.WriteFile(proof, ed25519.Sign(privateKey, challengeBytes), 0o600); err != nil {
		t.Fatal(err)
	}
	before := contextE2EPorcelainStatus(t, repo.Dir)
	proofArgs := append(append([]string{}, base...), "--human-proof", proof)
	stdout, stderr, code = runExperimentBuiltBinary(t, bin, repo.Dir, nil, proofArgs...)
	if code != 1 || stderr != "" || !strings.Contains(stdout, `"code":"human-key-unmapped"`) {
		t.Fatalf("unmapped proof exit/stdout/stderr = %d/%q/%q, want human-key-unmapped verdict", code, stdout, stderr)
	}
	if after := contextE2EPorcelainStatus(t, repo.Dir); after != before {
		t.Fatalf("unmapped proof mutated worktree: before=%q after=%q", before, after)
	}
}

func TestExperimentReadOperationClassificationBuiltBinary(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	repo := buildExperimentHumanRepo(t, publicKey)
	bin := buildVerdiBinary(t)
	before := contextE2EPorcelainStatus(t, repo.Dir)
	for _, test := range []struct {
		operation string
		extra     []string
	}{
		{operation: "inspect"},
		{operation: "discover-capabilities"},
		{operation: "validate-draft"},
		{operation: "review-registration"},
		{operation: "status"},
		{operation: "explain-result", extra: []string{"--run", "run-1"}},
	} {
		t.Run(test.operation, func(t *testing.T) {
			args := append([]string{"experiment", test.operation, "--spike", "spec/request-path-spike", "--experiment", "request-path-v2", "--accepted-head", repo.Head, "--json"}, test.extra...)
			stdout, stderr, code := runExperimentBuiltBinary(t, bin, repo.Dir, nil, args...)
			if stderr != "" {
				t.Fatalf("stderr = %q, want typed result on stdout only", stderr)
			}
			var result struct {
				Outcome experimentapp.Outcome
			}
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("decode %s result: %v\n%s", test.operation, err, stdout)
			}
			if code != result.Outcome.ExitCode() {
				t.Fatalf("%s exit = %d, want the typed classification %q exit %d\n%s", test.operation, code, result.Outcome.Classification, result.Outcome.ExitCode(), stdout)
			}
			if after := contextE2EPorcelainStatus(t, repo.Dir); after != before {
				t.Fatalf("%s mutated worktree: before=%q after=%q", test.operation, before, after)
			}
		})
	}
}

func TestExperimentDeterministicOutputBuiltBinary(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	repo := buildExperimentHumanRepo(t, publicKey)
	bin := buildVerdiBinary(t)
	args := []string{"experiment", "status", "--spike", "spec/request-path-spike", "--experiment", "request-path-v2", "--accepted-head", repo.Head}
	jsonArgs := append(append([]string{}, args...), "--json")

	firstJSON, _, firstJSONCode := runExperimentBuiltBinary(t, bin, repo.Dir, nil, jsonArgs...)
	secondJSON, _, secondJSONCode := runExperimentBuiltBinary(t, bin, repo.Dir, nil, jsonArgs...)
	if firstJSON != secondJSON || firstJSONCode != secondJSONCode {
		t.Fatalf("JSON output is not deterministic:\nfirst (%d): %q\nsecond (%d): %q", firstJSONCode, firstJSON, secondJSONCode, secondJSON)
	}
	if firstJSON == "" || !strings.Contains(firstJSON, `"classification"`) {
		t.Fatalf("JSON output omits the typed outcome: %q", firstJSON)
	}

	firstHuman, _, firstHumanCode := runExperimentBuiltBinary(t, bin, repo.Dir, nil, args...)
	secondHuman, _, secondHumanCode := runExperimentBuiltBinary(t, bin, repo.Dir, nil, args...)
	if firstHuman != secondHuman || firstHumanCode != secondHumanCode {
		t.Fatalf("human output is not deterministic:\nfirst (%d): %q\nsecond (%d): %q", firstHumanCode, firstHuman, secondHumanCode, secondHuman)
	}
	if !strings.HasPrefix(firstHuman, "experiment status: ") {
		t.Fatalf("human output = %q, want a deterministic rendering of the same typed result", firstHuman)
	}
	if firstHumanCode != firstJSONCode {
		t.Fatalf("human exit %d differs from JSON exit %d for the same operation", firstHumanCode, firstJSONCode)
	}
}

func decodeExperimentChallengeOutput(t *testing.T, data string) experimentChallengeOutput {
	t.Helper()
	var result experimentChallengeOutput
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		t.Fatalf("decode challenge output: %v\n%s", err, data)
	}
	return result
}

func buildExperimentHumanRepo(t *testing.T, publicKey ed25519.PublicKey) *fixturegit.Repo {
	t.Helper()
	return buildExperimentHumanRepoWithSubjects(t, []string{"ed25519:" + base64.RawURLEncoding.EncodeToString(publicKey)})
}

func buildExperimentHumanRepoWithSubjects(t *testing.T, subjects []string) *fixturegit.Repo {
	t.Helper()
	read := func(name string) string {
		data, err := os.ReadFile(filepath.Join("..", "..", "internal", "experimentapp", "testdata", filepath.FromSlash(name)))
		if err != nil {
			t.Fatalf("read experiment fixture %s: %v", name, err)
		}
		return string(data)
	}
	capabilities := read("capabilities.json")
	definition := read("experiment-v2/experiment.yaml")
	// Bind the fully resolved host shell as the evaluator executable:
	// /bin/sh is itself a symlink on Linux runners, and the evaluator trust
	// boundary correctly refuses symlinked executables, so resolve the
	// chain to the real non-symlink regular executable and bind the digest
	// of its exact bytes. (A byte-copy at a test-owned path is NOT viable:
	// macOS kills relocated platform binaries with SIGKILL, witnessed
	// during the Wave 5B CI correction.) Mirrors the conformance fixture
	// builder exactly.
	shellPath, err := filepath.EvalSymlinks("/bin/sh")
	if err != nil {
		t.Fatal(err)
	}
	shellBytes, err := os.ReadFile(shellPath)
	if err != nil {
		t.Fatal(err)
	}
	definition = strings.Replace(definition,
		`  argv: ["./tools/evaluator", "run"]`+"\n"+`  digest: sha256:3333333333333333333333333333333333333333333333333333333333333333`,
		fmt.Sprintf(`  argv: [%q, "tools/evaluator.sh", "run"]`, shellPath)+"\n"+`  digest: `+experimentRawDigest(shellBytes), 1)
	quoted := make([]string, len(subjects))
	for index, subject := range subjects {
		quoted[index] = fmt.Sprintf("%q", subject)
	}
	profile := fmt.Sprintf(`---
schema: verdi.governance-profile/v1
id: solo-default
class: solo
applicable_transitions: [accept]
identity_trust_sources:
  - {id: offline-human, kind: identity-provider}
role_mappings:
  - {role: author, trust_source: offline-human, subjects: [%s]}
ownership_sources: []
signature_requirements: []
required_approvers: []
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds: []
---
Hermetic offline-human profile.
`, strings.Join(quoted, ", "))
	policy := strings.ReplaceAll(read("policy/policies/experiment.md"), "./tools/evaluator", shellPath)
	script := "#!/bin/sh\nif [ \"$1\" = describe ]; then\n  printf '%s\\n' '" + strings.TrimSuffix(capabilities, "\n") + "'\n  exit 0\nfi\nexit 2\n"
	prefix := ".verdi/specs/active/request-path-spike/experiments/request-path-v2/"
	files := map[string]string{
		".verdi/verdi.yaml":                      "schema: verdi.layout/v1\n",
		".verdi/.gitignore":                      "data/\n",
		".verdi/policy/constitution.md":          read("policy/constitution.md"),
		".verdi/policy/profiles/solo-default.md": profile,
		".verdi/policy/policies/experiment.md":   policy,
		prefix + "experiment.yaml":               definition,
		prefix + "candidates/baseline.patch":     read("experiment-v2/candidates/baseline.patch"),
		prefix + "candidates/cache.patch":        read("experiment-v2/candidates/cache.patch"),
		prefix + "evaluator-capabilities.json":   capabilities,
		"tools/evaluator.sh":                     script,
	}
	return fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "accepted experiment fixture"}})
}

func experimentRawDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func runGitForExperimentTest(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

func TestExperimentInventoryBuiltBinary(t *testing.T) {
	bin := buildVerdiBinary(t)

	stdout, stderr, code := runExperimentBuiltBinary(t, bin, t.TempDir(), nil, "experiment")
	if code != 2 {
		t.Fatalf("exit = %d, want 2\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}

	for _, operation := range []string{
		"inspect",
		"discover-capabilities",
		"validate-draft",
		"review-registration",
		"status",
		"explain-result",
		"draft-definition",
		"capture-candidate",
		"reconcile-draft",
		"propose-registration",
		"start",
		"resume",
	} {
		if !strings.Contains(stderr, "  "+operation+"\n") {
			t.Errorf("inventory omits %q:\n%s", operation, stderr)
		}
	}
	for _, forbidden := range []string{"ratify", "capsule", "release", "closure"} {
		if strings.Contains(stderr, forbidden) {
			t.Errorf("inventory contains Wave 5C operation %q:\n%s", forbidden, stderr)
		}
	}
}
