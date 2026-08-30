package experimentapp_test

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
)

func TestCompleteExperimentMCPParityJourney(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	privateKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x5c}, ed25519.SeedSize))
	repo := conformanceRepo(t, privateKey.Public().(ed25519.PublicKey))
	bin := conformanceBinary(t)
	seen := map[string]bool{}

	base := []string{"--spike", "spec/request-path-spike", "--experiment", "request-path-v2", "--accepted-head", repo.Head}
	mcpArgs := func(operation string, extra map[string]any) map[string]any {
		args := map[string]any{
			"operation": operation, "spike": "spec/request-path-spike",
			"experiment": "request-path-v2", "accepted_head": repo.Head,
		}
		for key, value := range extra {
			args[key] = value
		}
		return args
	}
	parity := func(operation string, cliExtra []string, mcpExtra map[string]any) {
		t.Helper()
		cliArgs := append([]string{"experiment", operation}, base...)
		cliArgs = append(cliArgs, cliExtra...)
		cliArgs = append(cliArgs, "--json")
		stdout, stderr, code := conformanceCLI(t, bin, repo.Dir, cliArgs...)
		if stderr != "" {
			t.Fatalf("CLI %s stderr=%q", operation, stderr)
		}
		text, isError := conformanceMCP(t, repo.Dir, mcpArgs(operation, mcpExtra))
		assertParity(t, operation, stdout, code, text, isError)
		assertJourneyCanonicalJSON(t, operation, text)
		seen[operation] = true
	}

	// Every read-only agent operation has byte-identical CLI/MCP projection.
	for _, operation := range []string{"inspect", "discover-capabilities", "validate-draft", "review-registration", "status"} {
		parity(operation, nil, nil)
	}
	parity("explain-result", []string{"--run", "run-missing"}, map[string]any{"run": "run-missing"})

	// Start and resume agree on verdict classification before registration.
	bindings := conformanceBindingsValue(t, "sha256:"+strings.Repeat("5", 64))
	bindingsPath := filepath.Join(t.TempDir(), "bindings.json")
	if err := os.WriteFile(bindingsPath, append(append([]byte(nil), bindings...), '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, operation := range []string{"start", "resume"} {
		parity(operation, []string{"--run", "run-journey", "--inputs", bindingsPath}, map[string]any{"run": "run-journey", "inputs": bindings})
	}

	// The two mutation members are accepted by the live MCP union. Drafting
	// uses a fresh id; malformed capture content is an application error, not
	// an adapter-class refusal, and neither fact can be mistaken for authority.
	definition := strings.Replace(conformanceGitShow(t, repo.Dir, repo.Head, ".verdi/specs/active/request-path-spike/experiments/request-path-v2/experiment.yaml"), "id: request-path-v2", "id: request-path-v3", 1)
	patches := map[string]string{
		"baseline": conformanceReadTestdata(t, "experiment-v2/candidates/baseline.patch"),
		"cache":    conformanceReadTestdata(t, "experiment-v2/candidates/cache.patch"),
	}
	draftText, draftError := conformanceMCP(t, repo.Dir, map[string]any{
		"operation": "draft-definition", "spike": "spec/request-path-spike", "experiment": "request-path-v3",
		"accepted_head": repo.Head, "definition": definition, "candidate_patches": patches,
	})
	if draftError || !strings.Contains(draftText, `"classification":"clean"`) {
		t.Fatalf("live MCP draft-definition=%q isError=%v", draftText, draftError)
	}
	assertJourneyCanonicalJSON(t, "draft-definition", draftText)
	seen["draft-definition"] = true
	newExperiment := filepath.Join(repo.Dir, ".verdi", "specs", "active", "request-path-spike", "experiments", "request-path-v3")
	if err := os.RemoveAll(newExperiment); err != nil {
		t.Fatal(err)
	}

	beforeCapture := conformanceGit(t, repo.Dir, "status", "--porcelain", "--untracked-files=all")
	captureText, captureError := conformanceMCP(t, repo.Dir, mcpArgs("capture-candidate", map[string]any{
		"candidate": "cache", "patch": patches["cache"], "definition": "not a definition\n",
	}))
	if !captureError || strings.Contains(captureText, "has no MCP path") || !strings.Contains(captureText, `"classification":"operational"`) {
		t.Fatalf("live MCP capture-candidate=%q isError=%v", captureText, captureError)
	}
	assertJourneyCanonicalJSON(t, "capture-candidate", captureText)
	seen["capture-candidate"] = true
	if after := conformanceGit(t, repo.Dir, "status", "--porcelain", "--untracked-files=all"); after != beforeCapture {
		t.Fatalf("refused capture mutated repository: before=%q after=%q", beforeCapture, after)
	}

	wantAgentSafe := []string{
		"capture-candidate", "discover-capabilities", "draft-definition", "explain-result", "inspect",
		"resume", "review-registration", "start", "status", "validate-draft",
	}
	gotAgentSafe := make([]string, 0, len(seen))
	for operation := range seen {
		gotAgentSafe = append(gotAgentSafe, operation)
	}
	sort.Strings(gotAgentSafe)
	if !reflect.DeepEqual(gotAgentSafe, wantAgentSafe) {
		t.Fatalf("live agent-safe union=%v, want exactly %v", gotAgentSafe, wantAgentSafe)
	}

	// After a genuine accepted lock, every human/lifecycle operation remains
	// structurally unavailable to agents and cannot touch the repository.
	acceptedHead := conformanceRegister(t, bin, repo, privateKey)
	repo.Head = acceptedHead
	beforeHumanOnly := conformanceGit(t, repo.Dir, "status", "--porcelain", "--untracked-files=all")
	for _, operation := range []string{
		"reconcile-draft", "propose-registration", "propose-ratification",
		"ratify", "capsule", "publish-capsule", "release", "release-workspaces", "closure", "close",
	} {
		text, isError := conformanceMCP(t, repo.Dir, mcpArgs(operation, nil))
		if !isError || !strings.Contains(text, operation) || (!strings.Contains(text, "human-only") && !strings.Contains(text, "not part of the Wave 5B agent surface")) {
			t.Fatalf("MCP agent call %s=%q isError=%v", operation, text, isError)
		}
	}
	if after := conformanceGit(t, repo.Dir, "status", "--porcelain", "--untracked-files=all"); after != beforeHumanOnly {
		t.Fatalf("human-only MCP refusals mutated repository: before=%q after=%q", beforeHumanOnly, after)
	}
}

func assertJourneyCanonicalJSON(t *testing.T, operation, raw string) {
	t.Helper()
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		t.Fatalf("%s result is not JSON: %v\n%s", operation, err, raw)
	}
	want, err := canonjson.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if raw != string(want) {
		t.Fatalf("%s result is not canonical JSON\ngot:  %q\nwant: %q", operation, raw, want)
	}
}
