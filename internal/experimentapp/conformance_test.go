// Package experimentapp_test holds the Wave 5B adapter-conformance suite
// (design §8: "CLI and MCP conformance tests feed semantically identical
// requests through both adapters and require byte-identical core result
// projections"). It is an external test package deliberately: it drives the
// real built CLI binary and the real in-process MCP server against the same
// hermetic Git repositories, which requires importing internal/mcpserve —
// impossible from package experimentapp (mcpserve imports it).
//
// This package builds cmd/verdi in a subprocess, so it is listed in the
// Makefile's CROSS_BINARY_PKGS (ADJ-68's gate-cache honesty guard,
// internal/specalign/gatecache_test.go, enforces that listing).
package experimentapp_test

import (
	"bytes"
	"context"
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
	"sync"
	"testing"

	"github.com/jyang234/verdi/internal/experimenthuman"
	"github.com/jyang234/verdi/internal/experimentrun"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/mcpserve"
)

var (
	conformanceBinOnce sync.Once
	conformanceBin     string
	conformanceBinErr  error
)

func conformanceBinary(t *testing.T) string {
	t.Helper()
	conformanceBinOnce.Do(func() {
		dir, err := os.MkdirTemp("", "verdi-conformance-bin")
		if err != nil {
			conformanceBinErr = err
			return
		}
		bin := filepath.Join(dir, "verdi")
		cmd := exec.Command("go", "build", "-o", bin, "github.com/jyang234/verdi/cmd/verdi")
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &out
		if err := cmd.Run(); err != nil {
			conformanceBinErr = fmt.Errorf("building verdi binary: %w\n%s", err, out.String())
			return
		}
		conformanceBin = bin
	})
	if conformanceBinErr != nil {
		t.Fatalf("conformanceBinary: %v", conformanceBinErr)
	}
	return conformanceBin
}

func conformanceCLI(t *testing.T, bin, dir string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &outBuf, &errBuf
	err := cmd.Run()
	if err == nil {
		return outBuf.String(), errBuf.String(), 0
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("run %v: %v", args, err)
	}
	return outBuf.String(), errBuf.String(), exitErr.ExitCode()
}

func conformanceMCP(t *testing.T, root string, args map[string]any) (text string, isError bool) {
	t.Helper()
	argBytes, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"experiment","arguments":` + string(argBytes) + `}}` + "\n"
	var out bytes.Buffer
	if err := mcpserve.ServeConn(context.Background(), strings.NewReader(req), &out, mcpserve.NewServer(root)); err != nil {
		t.Fatalf("ServeConn: %v", err)
	}
	var resp struct {
		Result *struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("decoding response %q: %v", out.String(), err)
	}
	if resp.Error != nil {
		t.Fatalf("tools/call returned a JSON-RPC error (typed application failures must remain tool results): %s", resp.Error.Message)
	}
	if resp.Result == nil || len(resp.Result.Content) != 1 {
		t.Fatalf("result has no single content item: %s", out.String())
	}
	return resp.Result.Content[0].Text, resp.Result.IsError
}

func conformanceGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func conformanceReadTestdata(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", filepath.FromSlash(name)))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(data)
}

// conformanceRepo replicates cmd/verdi's own experiment human-repo fixture
// (accepted v2 experiment, offline-human Ed25519 profile, host-shell-copy
// evaluator)
// — test helpers in another package's _test.go files cannot be imported.
func conformanceRepo(t *testing.T, publicKey ed25519.PublicKey) *fixturegit.Repo {
	t.Helper()
	capabilities := conformanceReadTestdata(t, "capabilities.json")
	definition := conformanceReadTestdata(t, "experiment-v2/experiment.yaml")
	// Bind the fully resolved host shell as the evaluator executable:
	// /bin/sh is itself a symlink on Linux runners, and the evaluator trust
	// boundary correctly refuses symlinked executables, so resolve the
	// chain to the real non-symlink regular executable and bind the digest
	// of its exact bytes. (A byte-copy at a test-owned path is NOT viable:
	// macOS kills relocated platform binaries with SIGKILL, witnessed
	// during the Wave 5B CI correction.) Mirrors cmd/verdi's experiment
	// fixture builder exactly.
	shellPath, err := filepath.EvalSymlinks("/bin/sh")
	if err != nil {
		t.Fatal(err)
	}
	shellBytes, err := os.ReadFile(shellPath)
	if err != nil {
		t.Fatal(err)
	}
	shellSum := sha256.Sum256(shellBytes)
	definition = strings.Replace(definition,
		`  argv: ["./tools/evaluator", "run"]`+"\n"+`  digest: sha256:3333333333333333333333333333333333333333333333333333333333333333`,
		fmt.Sprintf(`  argv: [%q, "tools/evaluator.sh", "run"]`, shellPath)+"\n"+`  digest: sha256:`+hex.EncodeToString(shellSum[:]), 1)
	profile := fmt.Sprintf(`---
schema: verdi.governance-profile/v1
id: solo-default
class: solo
applicable_transitions: [accept]
identity_trust_sources:
  - {id: offline-human, kind: identity-provider}
role_mappings:
  - {role: author, trust_source: offline-human, subjects: ["ed25519:%s"]}
ownership_sources: []
signature_requirements: []
required_approvers: []
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds: []
---
Hermetic offline-human profile.
`, base64.RawURLEncoding.EncodeToString(publicKey))
	policy := strings.ReplaceAll(conformanceReadTestdata(t, "policy/policies/experiment.md"), "./tools/evaluator", shellPath)
	script := "#!/bin/sh\nif [ \"$1\" = describe ]; then\n  printf '%s\\n' '" + strings.TrimSuffix(capabilities, "\n") + "'\n  exit 0\nfi\nexit 2\n"
	prefix := ".verdi/specs/active/request-path-spike/experiments/request-path-v2/"
	files := map[string]string{
		".verdi/verdi.yaml":                      "schema: verdi.layout/v1\n",
		".verdi/.gitignore":                      "data/\n",
		".verdi/policy/constitution.md":          conformanceReadTestdata(t, "policy/constitution.md"),
		".verdi/policy/profiles/solo-default.md": profile,
		".verdi/policy/policies/experiment.md":   policy,
		prefix + "experiment.yaml":               definition,
		prefix + "candidates/baseline.patch":     conformanceReadTestdata(t, "experiment-v2/candidates/baseline.patch"),
		prefix + "candidates/cache.patch":        conformanceReadTestdata(t, "experiment-v2/candidates/cache.patch"),
		prefix + "evaluator-capabilities.json":   capabilities,
		"tools/evaluator.sh":                     script,
	}
	return fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "accepted experiment fixture"}})
}

// conformanceRegister locks the fixture's registration through the real CLI
// human flow (challenge, detached Ed25519 signature, proposal merge) and
// returns the new accepted HEAD carrying the locked definition.
func conformanceRegister(t *testing.T, bin string, repo *fixturegit.Repo, privateKey ed25519.PrivateKey) string {
	t.Helper()
	conformanceGit(t, repo.Dir, "checkout", "-q", "-b", "proposal")
	experimentDir := filepath.Join(repo.Dir, ".verdi", "specs", "active", "request-path-spike", "experiments", "request-path-v2")
	if err := os.WriteFile(filepath.Join(experimentDir, "human-note.txt"), []byte("conformance direct edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	conformanceGit(t, repo.Dir, "add", ".")
	conformanceGit(t, repo.Dir, "commit", "-q", "-m", "direct edit")

	proofDir := t.TempDir()
	for _, operation := range []string{"reconcile-draft", "propose-registration"} {
		base := []string{"experiment", operation, "--spike", "spec/request-path-spike", "--experiment", "request-path-v2", "--accepted-head", repo.Head, "--json"}
		stdout, stderr, code := conformanceCLI(t, bin, repo.Dir, base...)
		if code != 1 || stderr != "" {
			t.Fatalf("%s challenge exit/stdout/stderr = %d/%q/%q", operation, code, stdout, stderr)
		}
		var challengeOut struct {
			Challenge experimenthuman.Challenge `json:"challenge"`
		}
		if err := json.Unmarshal([]byte(stdout), &challengeOut); err != nil {
			t.Fatalf("decode %s challenge: %v\n%s", operation, err, stdout)
		}
		challengeBytes, err := challengeOut.Challenge.Canonical()
		if err != nil {
			t.Fatal(err)
		}
		proof := filepath.Join(proofDir, operation+".sig")
		if err := os.WriteFile(proof, ed25519.Sign(privateKey, challengeBytes), 0o600); err != nil {
			t.Fatal(err)
		}
		stdout, stderr, code = conformanceCLI(t, bin, repo.Dir, append(base, "--human-proof", proof)...)
		if code != 0 || stderr != "" {
			t.Fatalf("%s with valid proof exit/stdout/stderr = %d/%q/%q", operation, code, stdout, stderr)
		}
	}
	conformanceGit(t, repo.Dir, "add", ".")
	conformanceGit(t, repo.Dir, "commit", "-q", "-m", "registration lock")
	conformanceGit(t, repo.Dir, "checkout", "-q", "main")
	conformanceGit(t, repo.Dir, "merge", "-q", "--ff-only", "proposal")
	return conformanceGit(t, repo.Dir, "rev-parse", "HEAD")
}

func conformanceBindingsValue(t *testing.T, workloadDigest string) json.RawMessage {
	t.Helper()
	bindings := experimentrun.InputBindings{Schema: experimentrun.InputBindingSchema, Inputs: []experimentrun.InputBinding{
		{Slot: experimentrun.InputSlotContract, ID: "request-contract", Digest: "sha256:" + strings.Repeat("6", 64), Path: "inputs/contract.txt"},
		{Slot: experimentrun.InputSlotWorkload, ID: "request-mix", Digest: workloadDigest, Path: "inputs/workload.txt"},
	}}
	canonical, err := experimentrun.EncodeInputBindings(bindings)
	if err != nil {
		t.Fatal(err)
	}
	return json.RawMessage(bytes.TrimSuffix(canonical, []byte("\n")))
}

// assertParity requires the exact same canonical typed-result bytes from
// both adapters and agreement between the CLI exit contract and the MCP
// isError flag.
func assertParity(t *testing.T, label, cliStdout string, cliCode int, mcpText string, mcpIsError bool) {
	t.Helper()
	if cliStdout != mcpText {
		t.Fatalf("%s: CLI and MCP typed result projections differ:\nCLI: %s\nMCP: %s", label, cliStdout, mcpText)
	}
	if mcpIsError != (cliCode != 0) {
		t.Fatalf("%s: MCP isError=%v disagrees with CLI exit %d", label, mcpIsError, cliCode)
	}
}

func TestExperimentAdapterConformance(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	bin := conformanceBinary(t)
	repo := conformanceRepo(t, publicKey)
	base := []string{"--spike", "spec/request-path-spike", "--experiment", "request-path-v2", "--accepted-head", repo.Head}
	mcpBase := map[string]any{"spike": "spec/request-path-spike", "experiment": "request-path-v2", "accepted_head": repo.Head}
	mcpArgs := func(operation string, extra map[string]any) map[string]any {
		args := map[string]any{"operation": operation}
		for key, value := range mcpBase {
			args[key] = value
		}
		for key, value := range extra {
			args[key] = value
		}
		return args
	}

	// Read parity on the accepted (still unlocked) experiment.
	for _, operation := range []string{"status", "review-registration", "validate-draft", "inspect"} {
		cliOut, cliErr, cliCode := conformanceCLI(t, bin, repo.Dir, append([]string{"experiment", operation}, append(base, "--json")...)...)
		if cliErr != "" {
			t.Fatalf("CLI %s stderr = %q", operation, cliErr)
		}
		text, isError := conformanceMCP(t, repo.Dir, mcpArgs(operation, nil))
		assertParity(t, "read "+operation, cliOut, cliCode, text, isError)
	}

	// Verdict-failure parity: execution before any accepted registration.
	matched := conformanceBindingsValue(t, "sha256:"+strings.Repeat("5", 64))
	matchedPath := filepath.Join(t.TempDir(), "matched.json")
	if err := os.WriteFile(matchedPath, append([]byte(matched), '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	cliOut, cliErr, cliCode := conformanceCLI(t, bin, repo.Dir, append([]string{"experiment", "start"}, append(base, "--run", "run-1", "--inputs", matchedPath, "--json")...)...)
	if cliErr != "" || cliCode != 1 || !strings.Contains(cliOut, `"code":"registration-not-accepted"`) {
		t.Fatalf("CLI pre-registration start = %d/%q/%q", cliCode, cliOut, cliErr)
	}
	text, isError := conformanceMCP(t, repo.Dir, mcpArgs("start", map[string]any{"run": "run-1", "inputs": matched}))
	assertParity(t, "verdict start", cliOut, cliCode, text, isError)

	// Successful mutation parity: the same draft-definition through each
	// adapter from identical state. Projections are byte-identical except
	// the two digest fields, which cover the provenance log where each
	// adapter honestly records its own attribution.
	newDefinition := strings.Replace(conformanceGitShow(t, repo.Dir, repo.Head, ".verdi/specs/active/request-path-spike/experiments/request-path-v2/experiment.yaml"), "id: request-path-v2", "id: request-path-v3", 1)
	candidateRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(candidateRoot, "candidates"), 0o755); err != nil {
		t.Fatal(err)
	}
	patches := map[string]string{}
	for _, candidate := range []string{"baseline", "cache"} {
		patch := conformanceReadTestdata(t, "experiment-v2/candidates/"+candidate+".patch")
		patches[candidate] = patch
		if err := os.WriteFile(filepath.Join(candidateRoot, "candidates", candidate+".patch"), []byte(patch), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	definitionPath := filepath.Join(candidateRoot, "experiment.yaml")
	if err := os.WriteFile(definitionPath, []byte(newDefinition), 0o600); err != nil {
		t.Fatal(err)
	}
	draftBase := []string{"experiment", "draft-definition", "--spike", "spec/request-path-spike", "--experiment", "request-path-v3", "--accepted-head", repo.Head, "--definition", definitionPath, "--candidate-root", candidateRoot, "--json"}
	newExperimentDir := filepath.Join(repo.Dir, ".verdi", "specs", "active", "request-path-spike", "experiments", "request-path-v3")
	cliOut, cliErr, cliCode = conformanceCLI(t, bin, repo.Dir, draftBase...)
	if cliErr != "" {
		t.Fatalf("CLI draft-definition stderr = %q", cliErr)
	}
	cliProvenance, err := os.ReadFile(filepath.Join(newExperimentDir, "mutation-provenance.jsonl"))
	if cliCode == 0 && err != nil {
		t.Fatalf("CLI draft-definition succeeded without provenance: %v", err)
	}
	if err := os.RemoveAll(newExperimentDir); err != nil {
		t.Fatal(err)
	}
	text, isError = conformanceMCP(t, repo.Dir, map[string]any{
		"operation": "draft-definition", "spike": "spec/request-path-spike", "experiment": "request-path-v3",
		"accepted_head": repo.Head, "definition": newDefinition, "candidate_patches": patches,
	})
	if isError != (cliCode != 0) {
		t.Fatalf("draft-definition: MCP isError=%v disagrees with CLI exit %d\nCLI: %s\nMCP: %s", isError, cliCode, cliOut, text)
	}
	var cliDraft, mcpDraft struct {
		Outcome          json.RawMessage
		AcceptedHead     string
		ExperimentPath   string
		ArtifactDigest   string
		ProvenanceDigest string
	}
	if err := json.Unmarshal([]byte(cliOut), &cliDraft); err != nil {
		t.Fatalf("decode CLI draft result: %v\n%s", err, cliOut)
	}
	if err := json.Unmarshal([]byte(text), &mcpDraft); err != nil {
		t.Fatalf("decode MCP draft result: %v\n%s", err, text)
	}
	if string(cliDraft.Outcome) != string(mcpDraft.Outcome) || cliDraft.AcceptedHead != mcpDraft.AcceptedHead || cliDraft.ExperimentPath != mcpDraft.ExperimentPath {
		t.Fatalf("draft-definition core projections diverge beyond attribution:\nCLI: %s\nMCP: %s", cliOut, text)
	}
	if cliCode == 0 {
		// The human-authored artifact-set digest excludes machine evidence,
		// so it is byte-identical across adapters; only the sealed
		// provenance digest differs, because provenance honestly records
		// each adapter's own attribution.
		if cliDraft.ArtifactDigest != mcpDraft.ArtifactDigest {
			t.Fatalf("draft-definition artifact digests must be identical across adapters:\nCLI: %s\nMCP: %s", cliOut, text)
		}
		if cliDraft.ProvenanceDigest == mcpDraft.ProvenanceDigest {
			t.Fatalf("draft-definition provenance digests must differ via adapter attribution:\nCLI: %s\nMCP: %s", cliOut, text)
		}
		mcpProvenance, err := os.ReadFile(filepath.Join(newExperimentDir, "mutation-provenance.jsonl"))
		if err != nil {
			t.Fatalf("MCP draft-definition succeeded without provenance: %v", err)
		}
		if !strings.Contains(string(cliProvenance), "verdi-cli") || !strings.Contains(string(mcpProvenance), "verdi-mcp") {
			t.Fatalf("adapter attribution not recorded honestly:\nCLI provenance: %s\nMCP provenance: %s", cliProvenance, mcpProvenance)
		}
		if err := os.RemoveAll(newExperimentDir); err != nil {
			t.Fatal(err)
		}
	}

	// Lock the registration through the real CLI human flow, then prove
	// operational-failure parity on the shared input-binding document.
	acceptedHead := conformanceRegister(t, bin, repo, privateKey)
	lockedBase := []string{"--spike", "spec/request-path-spike", "--experiment", "request-path-v2", "--accepted-head", acceptedHead}
	mismatched := conformanceBindingsValue(t, "sha256:"+strings.Repeat("7", 64))
	mismatchedPath := filepath.Join(t.TempDir(), "mismatched.json")
	if err := os.WriteFile(mismatchedPath, append([]byte(mismatched), '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, operation := range []string{"start", "resume"} {
		cliOut, cliErr, cliCode := conformanceCLI(t, bin, repo.Dir, append([]string{"experiment", operation}, append(lockedBase, "--run", "run-1", "--inputs", mismatchedPath, "--json")...)...)
		if cliErr != "" || cliCode != 2 || !strings.Contains(cliOut, `"code":"input-binding-invalid"`) {
			t.Fatalf("CLI %s mismatched bindings = %d/%q/%q", operation, cliCode, cliOut, cliErr)
		}
		text, isError := conformanceMCP(t, repo.Dir, map[string]any{
			"operation": operation, "spike": "spec/request-path-spike", "experiment": "request-path-v2",
			"accepted_head": acceptedHead, "run": "run-1", "inputs": mismatched,
		})
		assertParity(t, "operational "+operation, cliOut, cliCode, text, isError)
		if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "data", "execution")); !os.IsNotExist(err) {
			t.Fatalf("%s mismatched bindings left runner evidence: %v", operation, err)
		}
	}

	// Mutation-refusal parity on the locked definition.
	lockedDefinition := conformanceGitShow(t, repo.Dir, acceptedHead, ".verdi/specs/active/request-path-spike/experiments/request-path-v2/experiment.yaml")
	patchPath := filepath.Join(t.TempDir(), "baseline.patch")
	if err := os.WriteFile(patchPath, []byte(patches["baseline"]), 0o600); err != nil {
		t.Fatal(err)
	}
	lockedDefinitionPath := filepath.Join(t.TempDir(), "experiment.yaml")
	if err := os.WriteFile(lockedDefinitionPath, []byte(lockedDefinition), 0o600); err != nil {
		t.Fatal(err)
	}
	cliOut, cliErr, cliCode = conformanceCLI(t, bin, repo.Dir, append([]string{"experiment", "capture-candidate"}, append(lockedBase, "--candidate", "baseline", "--patch", patchPath, "--definition", lockedDefinitionPath, "--json")...)...)
	if cliErr != "" || cliCode == 0 {
		t.Fatalf("CLI capture-candidate against a locked definition must refuse: %d/%q/%q", cliCode, cliOut, cliErr)
	}
	text, isError = conformanceMCP(t, repo.Dir, map[string]any{
		"operation": "capture-candidate", "spike": "spec/request-path-spike", "experiment": "request-path-v2",
		"accepted_head": acceptedHead, "candidate": "baseline", "patch": patches["baseline"], "definition": lockedDefinition,
	})
	assertParity(t, "locked capture-candidate", cliOut, cliCode, text, isError)
}

func conformanceGitShow(t *testing.T, dir, commit, path string) string {
	t.Helper()
	return conformanceGitOutput(t, dir, "show", commit+":"+path)
}

func conformanceGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}
