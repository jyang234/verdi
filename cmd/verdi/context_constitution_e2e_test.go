// Built-binary end-to-end tests for `verdi context constitution` (Wave 6
// Task 3): the real compiled verdi binary as a real OS subprocess against a
// real, local fixturegit repository — mirroring context_e2e_test.go's own
// established style for this package's CLI-behavioral-path proofs.
package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
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
	reqPath := writeConstitutionRequestFile(t, repo.Dir, "inspect.json", map[string]interface{}{})

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
	reqPath := writeConstitutionRequestFile(t, repo.Dir, "inspect.json", map[string]interface{}{})
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
