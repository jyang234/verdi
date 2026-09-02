// Built-binary end-to-end tests for `verdi context constitution` (Wave 6
// Task 3): the real compiled verdi binary as a real OS subprocess against a
// real, local fixturegit repository — mirroring context_e2e_test.go's own
// established style for this package's CLI-behavioral-path proofs.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/constitutionapp"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/mcpserve"
	"github.com/jyang234/verdi/internal/store"
)

// constitutionStoreFiles reads internal/constitutionapp's own testdata
// fixture (itself copied from internal/policyauthority/testdata/store) so
// this built-binary suite exercises the exact same committed constitution
// bytes constitutionapp's own package tests do, never a second ad hoc
// fixture.
func constitutionStoreFiles(t *testing.T) map[string]string {
	t.Helper()
	root := filepath.Join("..", "..", "internal", "constitutionapp", "testdata", "store")
	files := map[string]string{".verdi/verdi.yaml": "schema: verdi.layout/v1\n"}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("reading constitution fixture: %v", err)
	}
	return files
}

func buildConstitutionRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	return fixturegit.Build(t, []fixturegit.Layer{{Files: constitutionStoreFiles(t), Message: "adopt constitution"}})
}

func writeConstitutionRequestFile(t *testing.T, dir, name string, body map[string]interface{}) string {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing request file: %v", err)
	}
	return path
}

// TestContextConstitutionE2E_InspectHappyPath proves the real binary's
// inspect surface: exit 0, a decodable result naming both the accepted and
// proposed constitution states, and no worktree/HEAD side effect.
func TestContextConstitutionE2E_InspectHappyPath(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := buildConstitutionRepo(t)
	reqPath := writeConstitutionRequestFile(t, repo.Dir, "inspect.json", map[string]interface{}{
		"schema": constitutionapp.InspectRequestSchema,
	})

	headBefore := contextE2ECurrentHead(t, repo.Dir)

	stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, []string{"CI_DEFAULT_BRANCH=main"}, "context", "constitution", "inspect", "--request", reqPath)
	if code != 0 {
		t.Fatalf("exit = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty on success", stderr)
	}
	var result struct {
		Schema   string `json:"schema"`
		Accepted struct {
			Adopted bool `json:"adopted"`
		} `json:"accepted"`
		Proposed struct {
			Adopted bool `json:"adopted"`
		} `json:"proposed"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decoding stdout: %v\nstdout=%s", err, stdout)
	}
	if result.Schema != "verdi.constitution-inspect/v1" {
		t.Fatalf("schema = %q", result.Schema)
	}
	if !result.Accepted.Adopted || !result.Proposed.Adopted {
		t.Fatalf("expected both adopted, got %+v", result)
	}
	if got := contextE2ECurrentHead(t, repo.Dir); got != headBefore {
		t.Fatalf("HEAD changed: before=%s after=%s", headBefore, got)
	}
}

// TestContextConstitutionE2E_ProposeThenStaleHead proves the real binary's
// mutating propose surface end to end: a real branch/commit on the first
// call (exit 0), and a real stale-head verdict (exit 1) on a second call
// whose Expected.Head no longer names the branch's real HEAD.
func TestContextConstitutionE2E_ProposeThenStaleHead(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := buildConstitutionRepo(t)

	content, err := os.ReadFile(filepath.Join(repo.Dir, ".verdi", "policy", "overlays", "frontend-go-version.md"))
	if err != nil {
		t.Fatal(err)
	}
	retitled := strings.Replace(string(content), "Frontend Go version overlay", "Frontend Go version overlay (e2e)", 1)

	reqPath := writeConstitutionRequestFile(t, repo.Dir, "propose.json", map[string]interface{}{
		"schema":   constitutionapp.ProposeRequestSchema,
		"branch":   "policy/e2e-retitle",
		"kind":     "policy-overlay",
		"name":     "frontend-go-version",
		"content":  base64.StdEncoding.EncodeToString([]byte(retitled)),
		"expected": map[string]interface{}{"branch": "policy/e2e-retitle"},
	})

	stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, []string{"CI_DEFAULT_BRANCH=main"}, "context", "constitution", "propose", "--request", reqPath)
	if code != 0 {
		t.Fatalf("first propose exit = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	var first struct {
		ZeroEffect bool `json:"zero_effect"`
		Commit     string
	}
	if err := json.Unmarshal([]byte(stdout), &first); err != nil {
		t.Fatalf("decoding stdout: %v\nstdout=%s", err, stdout)
	}
	if first.ZeroEffect || first.Commit == "" {
		t.Fatalf("expected a real committed effect, got %+v", first)
	}

	staleReqPath := writeConstitutionRequestFile(t, repo.Dir, "propose-stale.json", map[string]interface{}{
		"schema":   constitutionapp.ProposeRequestSchema,
		"branch":   "policy/e2e-retitle",
		"kind":     "policy-overlay",
		"name":     "frontend-go-version",
		"content":  base64.StdEncoding.EncodeToString([]byte(retitled)),
		"expected": map[string]interface{}{"branch": "policy/e2e-retitle", "head": "0000000000000000000000000000000000000000"},
	})
	stdout, stderr, code = runVerdiBinary(t, bin, repo.Dir, []string{"CI_DEFAULT_BRANCH=main"}, "context", "constitution", "propose", "--request", staleReqPath)
	if code != 1 {
		t.Fatalf("second propose exit = %d, want 1 (stale-head)\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	var failure struct {
		Classification string `json:"classification"`
		Code           string `json:"code"`
	}
	if err := json.Unmarshal([]byte(stdout), &failure); err != nil {
		t.Fatalf("decoding failure stdout: %v\nstdout=%s", err, stdout)
	}
	if failure.Classification != "verdict" || failure.Code != "stale-head" {
		t.Fatalf("failure = %+v, want verdict/stale-head", failure)
	}
}

// TestContextConstitutionE2E_OutFile proves the --out path writes the exact
// canonical result to the named file and leaves stdout empty.
func TestContextConstitutionE2E_OutFile(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := buildConstitutionRepo(t)
	reqPath := writeConstitutionRequestFile(t, repo.Dir, "inspect.json", map[string]interface{}{
		"schema": constitutionapp.InspectRequestSchema,
	})
	outPath := filepath.Join(repo.Dir, "out.json")

	stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, []string{"CI_DEFAULT_BRANCH=main"}, "context", "constitution", "inspect", "--request", reqPath, "--out", outPath)
	if code != 0 {
		t.Fatalf("exit = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("stdout/stderr = %q/%q, want both empty with --out", stdout, stderr)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading --out file: %v", err)
	}
	var result struct {
		Schema string `json:"schema"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("decoding --out file: %v\ndata=%s", err, data)
	}
	if result.Schema != "verdi.constitution-inspect/v1" {
		t.Fatalf("schema = %q", result.Schema)
	}
}

// TestContextConstitution_CLIAndMCPRecordsAreByteIdentical proves the
// "byte-equivalent CLI/workbench-capable records" contract directly rather
// than by inspection: one fixture store, one root, both adapters, and a
// byte comparison of what each actually emits — the CLI's own
// dispatchConstitutionOp bytes (exactly what runConstitutionOp writes to
// stdout or --out) against the MCP tool's own text content item, for each of
// the three operations MCP registers.
func TestContextConstitution_CLIAndMCPRecordsAreByteIdentical(t *testing.T) {
	repo := buildConstitutionRepo(t)
	root, err := store.FindRoot(repo.Dir)
	if err != nil {
		t.Fatalf("store.FindRoot: %v", err)
	}

	ctx := context.Background()
	svc := constitutionapp.NewService()
	backend := &mcpserve.Backend{Root: root}

	cases := []struct {
		op      string
		request string
		call    func(json.RawMessage) map[string]any
	}{
		{"inspect", `{"schema":"` + constitutionapp.InspectRequestSchema + `"}`, func(a json.RawMessage) map[string]any { return backend.ConstitutionInspect(ctx, a) }},
		{"validate", `{"schema":"` + constitutionapp.ValidateRequestSchema + `"}`, func(a json.RawMessage) map[string]any { return backend.ConstitutionValidate(ctx, a) }},
		{"impact-review", `{"schema":"` + constitutionapp.ImpactReviewRequestSchema + `"}`, func(a json.RawMessage) map[string]any { return backend.ConstitutionImpactReview(ctx, a) }},
	}
	for _, c := range cases {
		t.Run(c.op, func(t *testing.T) {
			cliBytes, landed, typed, decodeErr := dispatchConstitutionOp(ctx, svc, root, c.op, []byte(c.request))
			if decodeErr != nil {
				t.Fatalf("CLI path: %v", decodeErr)
			}
			if typed != nil {
				t.Fatalf("CLI path: %v", typed)
			}
			if landed != nil {
				t.Fatalf("a read-only operation must report no landed effect, got %+v", landed)
			}
			mcpText := constitutionToolText(t, c.call(json.RawMessage(c.request)))
			if string(cliBytes) != mcpText {
				t.Fatalf("CLI and MCP records differ for %s:\nCLI = %q\nMCP = %q", c.op, string(cliBytes), mcpText)
			}
		})
	}
}

// constitutionToolText extracts the single text content item from an MCP
// tool result, failing the test on an error result or any other shape.
func constitutionToolText(t *testing.T, result map[string]any) string {
	t.Helper()
	if isErr, ok := result["isError"]; ok && isErr == true {
		t.Fatalf("MCP tool returned an error result: %+v", result)
	}
	items, ok := result["content"].([]map[string]any)
	if !ok || len(items) != 1 {
		t.Fatalf("unexpected MCP tool result shape: %+v", result)
	}
	text, ok := items[0]["text"].(string)
	if !ok {
		t.Fatalf("MCP tool result carries no text content item: %+v", result)
	}
	return text
}

// TestContextConstitution_OutputFailureDisclosesLandedProposal proves the
// no-hidden-effect rule for the one MUTATING operation on this surface:
// `propose` creates a real branch and commit, and only THEN is the result
// rendered. An output-destination failure at that point (here an --out path
// that is a directory, so the atomic rename cannot land) used to return a
// bare exit 2 carrying nothing at all — the caller was told the command
// failed while a real branch and commit had already landed in its
// repository, discoverable only by inspecting Git by hand.
//
// The exit code stays 2 (the command genuinely did not deliver its result);
// what changes is that the landed identity travels with the diagnostic.
func TestContextConstitution_OutputFailureDisclosesLandedProposal(t *testing.T) {
	repo := buildConstitutionRepo(t)
	t.Chdir(repo.Dir)

	content, err := os.ReadFile(filepath.Join(repo.Dir, ".verdi", "policy", "overlays", "frontend-go-version.md"))
	if err != nil {
		t.Fatal(err)
	}
	retitled := strings.Replace(string(content), "Frontend Go version overlay", "Frontend Go version overlay (out-failure)", 1)
	reqPath := writeConstitutionRequestFile(t, repo.Dir, "propose-outfail.json", map[string]interface{}{
		"schema":   constitutionapp.ProposeRequestSchema,
		"branch":   "policy/out-failure",
		"kind":     "policy-overlay",
		"name":     "frontend-go-version",
		"content":  base64.StdEncoding.EncodeToString([]byte(retitled)),
		"expected": map[string]interface{}{"branch": "policy/out-failure"},
	})

	// An existing DIRECTORY at the --out path: atomicfile.Write's final
	// rename cannot replace it, so the write fails deterministically AFTER
	// Propose has already committed.
	outPath := filepath.Join(repo.Dir, "outdir")
	if err := os.Mkdir(outPath, 0o755); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr strings.Builder
	code := runConstitutionOp("propose", []string{"--request", reqPath, "--out", outPath}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (the result was not delivered)\nstdout: %s\nstderr: %s", code, stdout.String(), stderr.String())
	}

	// The mutation really landed: the branch exists and carries a commit.
	landedCommit := strings.TrimSpace(gitOutput(t, repo.Dir, "rev-parse", "policy/out-failure"))
	if landedCommit == "" {
		t.Fatal("test setup: expected Propose to have committed onto the proposal branch")
	}

	diagnostic := stderr.String()
	for _, want := range []string{"policy/out-failure", landedCommit, "zero_effect"} {
		if !strings.Contains(diagnostic, want) {
			t.Fatalf("the diagnostic hides the landed proposal (%q missing):\n%s", want, diagnostic)
		}
	}
}

// TestContextConstitutionE2E_UnversionedRequestRefused proves the request
// envelope's version is enforced on the real binary's own CLI path: a
// document with no schema field — exactly what every caller sent before this
// contract existed — is refused operationally (exit 2) before any store
// access, rather than read as whichever version this build implements.
func TestContextConstitutionE2E_UnversionedRequestRefused(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := buildConstitutionRepo(t)
	reqPath := writeConstitutionRequestFile(t, repo.Dir, "unversioned.json", map[string]interface{}{})

	stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, []string{"CI_DEFAULT_BRANCH=main"}, "context", "constitution", "inspect", "--request", reqPath)
	if code != 2 {
		t.Fatalf("exit = %d, want 2\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, constitutionapp.InspectRequestSchema) {
		t.Fatalf("stderr = %q, want the expected envelope version named", stderr)
	}
}

// TestContextConstitutionE2E_UnknownSubcommand_ExitTwo proves the usage
// contract: an unrecognized `context constitution` operation fails closed
// operationally before any store access.
func TestContextConstitutionE2E_UnknownSubcommand_ExitTwo(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := buildConstitutionRepo(t)

	stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, []string{"CI_DEFAULT_BRANCH=main"}, "context", "constitution", "bogus")
	if code != 2 {
		t.Fatalf("exit = %d, want 2\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
}
