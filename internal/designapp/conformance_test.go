// Package designapp_test holds the Wave 6 Task 1 CLI/MCP adapter-
// conformance suite (contract: "Route CLI and MCP through the SAME six
// application methods, proven by ONE conformance suite" — mirroring
// internal/experimentapp/conformance_test.go's own precedent for the CSE
// application core). It is an external test package deliberately: it
// drives the real built CLI binary and the real in-process MCP server
// against the same hermetic Git repositories, which requires importing
// internal/mcpserve — impossible from package designapp (mcpserve
// imports designapp).
//
// This package builds cmd/verdi in a subprocess, so it is listed in the
// Makefile's CROSS_BINARY_PKGS (ADJ-68's gate-cache honesty guard,
// internal/specalign/gatecache_test.go, enforces that listing).
package designapp_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/designapp"
	"github.com/jyang234/verdi/internal/draftmutation"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/mcpserve"
	"github.com/jyang234/verdi/internal/store"
)

var (
	conformanceBinOnce sync.Once
	conformanceBin     string
	conformanceBinErr  error
)

// conformanceBinary builds cmd/verdi once for the whole test binary run
// (mirrors internal/experimentapp/conformance_test.go's own helper
// exactly).
func conformanceBinary(t *testing.T) string {
	t.Helper()
	conformanceBinOnce.Do(func() {
		dir, err := os.MkdirTemp("", "verdi-designapp-conformance-bin")
		if err != nil {
			conformanceBinErr = err
			return
		}
		bin := filepath.Join(dir, "verdi")
		cmd := exec.Command("go", "build", "-o", bin, "github.com/jyang234/verdi/cmd/verdi")
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &out
		if err := cmd.Run(); err != nil {
			conformanceBinErr = err
			return
		}
		conformanceBin = bin
	})
	if conformanceBinErr != nil {
		t.Fatalf("building cmd/verdi: %v", conformanceBinErr)
	}
	return conformanceBin
}

const conformanceSpec = `---
id: spec/sample
kind: spec
class: feature
title: Sample
owners: [platform-team]
problem: { text: "old problem", anchor: "#problem" }
outcome: { text: "old outcome", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "first", evidence: [static], anchor: "#ac-1" }
constraints:
  - { id: co-1, text: "bounded", anchor: "#co-1" }
---
# Sample

## Problem

Old prose stays.

## Outcome

Old prose stays.

## ac-1

First.

## co-1

Bounded.
`

// conformanceStore builds one hermetic fixturegit repository — the
// existing internal/policyauthority ASD policy fixture, mode draft-write,
// on a checked-out design/sample branch, with conformanceSpec written at
// the active spec path — and returns its resolved checkout root. Every
// call builds an independent repo so CLI and MCP runs never share mutable
// state; fixturegit's fixed author/committer/date means identical layers
// still resolve the identical HEAD commit SHA regardless of which
// temporary directory holds this particular copy (only the checkout path
// itself differs — normalizeCheckout accounts for exactly that).
func conformanceStore(t *testing.T) string {
	t.Helper()
	files := map[string]string{".verdi/verdi.yaml": "schema: verdi.layout/v1\n", ".verdi/.gitignore": "data/\n"}
	source := filepath.Join("..", "policyauthority", "testdata", "store")
	if err := filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if entry.Name() == "go-toolchain.md" {
			data = bytes.Replace(data, []byte("mode: proposal-only"), []byte("mode: draft-write"), 1)
		}
		files[filepath.ToSlash(rel)] = string(data)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "adopt draft mutation policy"}})

	checkout := exec.Command("git", "checkout", "-b", "design/sample")
	checkout.Dir = repo.Dir
	if output, err := checkout.CombinedOutput(); err != nil {
		t.Fatalf("git checkout design/sample: %v\n%s", err, output)
	}

	resolved, err := filepath.EvalSymlinks(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.ToSlash(resolved)

	specDir := store.SpecDir(root, store.ZoneActive, "sample")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.SpecPath(root, store.ZoneActive, "sample"), []byte(conformanceSpec), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func runCLI(t *testing.T, bin, dir string, args ...string) (stdout string, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = filepath.FromSlash(dir)
	cmd.Env = append(os.Environ(), "CI_DEFAULT_BRANCH=main")
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	if err == nil {
		return out.String(), errOut.String(), 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return out.String(), errOut.String(), exitErr.ExitCode()
	}
	t.Fatalf("running verdi %v: %v\nstderr: %s", args, err, errOut.String())
	return "", "", 0
}

// callMCP drives one tools/call against a fresh in-process server rooted
// at root, over the exact NDJSON wire framing a real client would use
// (mcpserve.ServeConn) — never reaching into mcpserve's internals
// directly (mirrors internal/specalign/mcptools_test.go's own
// listMCPTools convention).
func callMCP(t *testing.T, root, tool string, args map[string]any) (text string, isError bool) {
	t.Helper()
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	srv := mcpserve.NewServer(root)
	reqBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": tool, "arguments": args},
	})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := mcpserve.ServeConn(context.Background(), bytes.NewReader(append(reqBody, '\n')), &out, srv); err != nil {
		t.Fatalf("ServeConn(tools/call %s): %v", tool, err)
	}
	var resp struct {
		Result struct {
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
		t.Fatalf("decoding tools/call response %q: %v", out.String(), err)
	}
	if resp.Error != nil {
		t.Fatalf("tools/call %s returned a JSON-RPC error: %s", tool, resp.Error.Message)
	}
	if len(resp.Result.Content) != 1 {
		t.Fatalf("tools/call %s: content = %+v, want exactly one item", tool, resp.Result.Content)
	}
	return resp.Result.Content[0].Text, resp.Result.IsError
}

// normalizeCheckout replaces every occurrence of checkout (the one
// legitimately per-instance field — the absolute temp-dir checkout path)
// with a fixed placeholder, so two independent, content-identical
// fixturegit repos compare byte-identical everywhere else.
func normalizeCheckout(s, checkout string) string {
	return strings.ReplaceAll(s, checkout, "<CHECKOUT>")
}

// assertConformant proves CLI and MCP produced the exact same typed
// result for the same operation over content-identical hermetic repos —
// CO-9's "Adapter conformance": "byte-identical ... resulting spec.md;
// provenance record; previous and resulting digests; semantic diff;
// warnings and error classifications."
func assertConformant(t *testing.T, op, cliOut, cliRoot, mcpOut, mcpRoot string) {
	t.Helper()
	gotCLI := normalizeCheckout(strings.TrimRight(cliOut, "\n"), cliRoot)
	gotMCP := normalizeCheckout(strings.TrimRight(mcpOut, "\n"), mcpRoot)
	if gotCLI != gotMCP {
		t.Fatalf("%s: CLI and MCP diverge\nCLI: %s\nMCP: %s", op, gotCLI, gotMCP)
	}
}

// TestASDAdapterConformance proves get_board, get_design_context,
// get_design_capabilities, get_design_provenance, and
// prepare_design_review route CLI and MCP through the exact same
// designapp.Service method with byte-identical typed results.
// mutate_draft's own byte-identical CLI/MCP parity for the ASD wire
// contract is already proven in full by cmd/verdi/designmutate_test.go
// (built-binary) against draftmutation's pinned schema; this suite adds
// one further mutate_draft case proving MCP's own delegated-agent path
// resolves the identical typed result shape a CLI caller sees.
func TestASDAdapterConformance(t *testing.T) {
	bin := conformanceBinary(t)

	for _, tc := range []struct {
		op       string
		cliArgs  []string
		mcpTool  string
		mcpArgs  map[string]any
		wantCode int
	}{
		{op: "get_board", cliArgs: []string{"design", "board", "spec/sample"}, mcpTool: "get_board", mcpArgs: map[string]any{"ref": "spec/sample"}},
		{op: "get_design_context", cliArgs: []string{"design", "context", "spec/sample"}, mcpTool: "get_design_context", mcpArgs: map[string]any{"ref": "spec/sample"}},
		{op: "get_design_capabilities", cliArgs: []string{"design", "capabilities", "spec/sample"}, mcpTool: "get_design_capabilities", mcpArgs: map[string]any{"ref": "spec/sample"}},
		{op: "get_design_provenance", cliArgs: []string{"design", "provenance", "spec/sample"}, mcpTool: "get_design_provenance", mcpArgs: map[string]any{"ref": "spec/sample"}},
		{op: "prepare_design_review", cliArgs: []string{"design", "review", "spec/sample"}, mcpTool: "prepare_design_review", mcpArgs: map[string]any{"ref": "spec/sample"}},
	} {
		t.Run(tc.op, func(t *testing.T) {
			cliRoot := conformanceStore(t)
			mcpRoot := conformanceStore(t)

			cliOut, _, code := runCLI(t, bin, cliRoot, tc.cliArgs...)
			if code != 0 {
				t.Fatalf("CLI %v: exit %d, stdout=%s", tc.cliArgs, code, cliOut)
			}
			mcpOut, isError := callMCP(t, mcpRoot, tc.mcpTool, tc.mcpArgs)
			if isError {
				t.Fatalf("MCP %s: isError=true: %s", tc.mcpTool, mcpOut)
			}

			assertConformant(t, tc.op, cliOut, cliRoot, mcpOut, mcpRoot)
		})
	}

	t.Run("mutate_draft", func(t *testing.T) {
		cliRoot := conformanceStore(t)
		mcpRoot := conformanceStore(t)
		base := []byte(conformanceSpec)
		baseDigest := draftmutation.DigestBytes(base)
		baseSpecB64 := base64.StdEncoding.EncodeToString(base)
		operations := []map[string]any{{"op": "set-problem", "text": "conformance change", "anchor": "#problem"}}

		cliHead := gitHeadOf(t, cliRoot)
		cliRequest := map[string]any{
			"schema": "verdi.draftmutation/v1", "spec": "spec/sample",
			"base_digest": baseDigest, "base_spec_b64": baseSpecB64,
			"expected":   map[string]any{"checkout": cliRoot, "branch": "design/sample", "head": cliHead},
			"operations": operations,
		}
		cliRequestBytes, err := canonjson.Marshal(cliRequest)
		if err != nil {
			t.Fatal(err)
		}
		cliCmd := exec.Command(bin, "design", "mutate", "--request", "-", "--harness", "conformance")
		cliCmd.Dir = filepath.FromSlash(cliRoot)
		cliCmd.Env = append(os.Environ(), "CI_DEFAULT_BRANCH=main")
		cliCmd.Stdin = bytes.NewReader(cliRequestBytes)
		var cliOut bytes.Buffer
		cliCmd.Stdout = &cliOut
		if err := cliCmd.Run(); err != nil {
			t.Fatalf("verdi design mutate: %v", err)
		}

		mcpHead := gitHeadOf(t, mcpRoot)
		mcpArgs := map[string]any{
			"harness": "conformance", "schema": "verdi.draftmutation/v1", "spec": "spec/sample",
			"base_digest": baseDigest, "base_spec_b64": baseSpecB64,
			"expected":   map[string]any{"checkout": mcpRoot, "branch": "design/sample", "head": mcpHead},
			"operations": operations,
		}
		mcpOut, isError := callMCP(t, mcpRoot, "mutate_draft", mcpArgs)
		if isError {
			t.Fatalf("MCP mutate_draft: isError=true: %s", mcpOut)
		}

		assertConformant(t, "mutate_draft", cliOut.String(), cliRoot, mcpOut, mcpRoot)

		// CO-9's adapter conformance names the "provenance record" as its
		// own conformance object, alongside the result: two adapters that
		// return the same result but write different custody records have
		// not conformed. designprovenance.Entry carries no checkout, branch,
		// or HEAD (record.go) — its content is spec identity, digests,
		// attribution, policy digest, operations, changes, and excerpts — so
		// two content-identical repositories must produce byte-identical
		// sidecars, seal digest included. Path normalization is applied
		// anyway, so a future field carrying the checkout path would fail
		// here as a real divergence rather than as a false alarm.
		cliProvenance := readProvenanceSidecar(t, cliRoot)
		mcpProvenance := readProvenanceSidecar(t, mcpRoot)
		if len(cliProvenance) == 0 {
			t.Fatal("mutate_draft: CLI wrote no provenance sidecar")
		}
		gotCLI := normalizeCheckout(string(cliProvenance), cliRoot)
		gotMCP := normalizeCheckout(string(mcpProvenance), mcpRoot)
		if gotCLI != gotMCP {
			t.Fatalf("mutate_draft: provenance sidecars diverge\nCLI: %s\nMCP: %s", gotCLI, gotMCP)
		}
	})

}

// TestASDCrossTransportClassificationParity proves the 0/1/2
// classification the application core computed survives BOTH adapters
// (CO-1: "The core distinguishes verdict failures from operational
// failures and makes every refusal explicit"; CO-9 names "error
// classifications" as an adapter-conformance object in its own right).
//
// The two transports carry the same fact differently and neither may
// discard it: the CLI projects it onto its exit code (1 verdict / 2
// operational) and MCP onto the machine-readable classification field of
// designapp's typed failure envelope, since a tool result has no exit-code
// channel. Code and detail must then agree byte-for-byte.
//
// BOTH a verdict and an operational failure are exercised: a suite that
// only ever drove one of them would pass against an adapter that hard-
// coded that one answer, which is precisely the defect this test exists
// to keep out.
func TestASDCrossTransportClassificationParity(t *testing.T) {
	bin := conformanceBinary(t)

	for _, tc := range []struct {
		name string
		// setup makes the failure reachable through both transports.
		setup   func(t *testing.T, root string)
		cliArgs []string
		mcpTool string
		mcpArgs map[string]any
		// wantExit is the CLI's projection; wantClassification is MCP's.
		// They are the same fact, asserted on each transport's own channel.
		wantExit           int
		wantClassification string
		wantCode           string
	}{
		{
			name:               "verdict",
			setup:              func(*testing.T, string) {},
			cliArgs:            []string{"design", "capabilities", "spec/does-not-exist"},
			mcpTool:            "get_design_capabilities",
			mcpArgs:            map[string]any{"ref": "spec/does-not-exist"},
			wantExit:           1,
			wantClassification: "verdict",
			wantCode:           "spec-not-found",
		},
		{
			// A repository whose default-branch ref points at a blob: the
			// ref still resolves, every tree read against it fails. That is
			// an unanswerable question, not a proven absence, so both
			// adapters must report an operational failure rather than the
			// verdict-free "not yet on the default branch".
			name:               "operational",
			setup:              breakConformanceDefaultBranch,
			cliArgs:            []string{"design", "review", "spec/sample"},
			mcpTool:            "prepare_design_review",
			mcpArgs:            map[string]any{"ref": "spec/sample"},
			wantExit:           2,
			wantClassification: "operational",
			wantCode:           "io-failure",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cliRoot := conformanceStore(t)
			mcpRoot := conformanceStore(t)
			tc.setup(t, cliRoot)
			tc.setup(t, mcpRoot)

			cliOut, cliErr, code := runCLI(t, bin, cliRoot, tc.cliArgs...)
			if code != tc.wantExit {
				t.Fatalf("CLI %v: exit %d, want %d\nstdout=%s\nstderr=%s", tc.cliArgs, code, tc.wantExit, cliOut, cliErr)
			}
			if cliOut != "" {
				t.Fatalf("CLI failure wrote to stdout: %q", cliOut)
			}

			mcpOut, isError := callMCP(t, mcpRoot, tc.mcpTool, tc.mcpArgs)
			if !isError {
				t.Fatalf("MCP %s: isError=false: %s", tc.mcpTool, mcpOut)
			}
			var failure struct {
				Schema         string `json:"schema"`
				Classification string `json:"classification"`
				Code           string `json:"code"`
				Detail         string `json:"detail"`
			}
			if err := json.Unmarshal([]byte(normalizeCheckout(strings.TrimRight(mcpOut, "\n"), mcpRoot)), &failure); err != nil {
				t.Fatalf("MCP %s failure is not the typed envelope: %v\n%s", tc.mcpTool, err, mcpOut)
			}
			if failure.Schema != designapp.FailureSchema {
				t.Fatalf("MCP failure schema = %q, want %q", failure.Schema, designapp.FailureSchema)
			}
			if failure.Classification != tc.wantClassification {
				t.Fatalf("MCP failure classification = %q, want %q (the CLI exited %d)", failure.Classification, tc.wantClassification, code)
			}
			if failure.Code != tc.wantCode {
				t.Fatalf("MCP failure code = %q, want %q", failure.Code, tc.wantCode)
			}

			// The remaining "<code>: <detail>" must be identical on both
			// transports — only the CLI's leading verb legitimately differs.
			cliDiagnostic := diagnosticAfterVerb(t, "CLI", normalizeCheckout(strings.TrimRight(cliErr, "\n"), cliRoot))
			if want := failure.Code + ": " + failure.Detail; cliDiagnostic != want {
				t.Fatalf("classification parity: CLI and MCP diverge\nCLI: %s\nMCP: %s", cliDiagnostic, want)
			}
		})
	}
}

// breakConformanceDefaultBranch repoints the default branch at a real BLOB
// object, producing a repository where `git rev-parse --verify` still
// answers and every tree read fails. It reproduces
// internal/designapp's own breakDefaultBranchRef helper, which this
// external test package cannot reach across the package boundary (the same
// reason conformanceStore reproduces newTestStore).
func breakConformanceDefaultBranch(t *testing.T, root string) {
	t.Helper()
	cmd := exec.Command("git", "hash-object", "-w", filepath.Join(".verdi", "verdi.yaml"))
	cmd.Dir = filepath.FromSlash(root)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git hash-object: %v", err)
	}
	blob := strings.TrimSpace(string(out))
	// `git update-ref` refuses to point a branch at a non-commit, so the
	// loose ref file is written directly. A loose ref always wins over a
	// packed one.
	refPath := filepath.Join(filepath.FromSlash(root), ".git", "refs", "heads", "main")
	if err := os.MkdirAll(filepath.Dir(refPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(refPath, []byte(blob+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// readProvenanceSidecar returns the exact committed sidecar bytes for the
// conformance fixture spec, or nil when none was written.
func readProvenanceSidecar(t *testing.T, root string) []byte {
	t.Helper()
	raw, err := os.ReadFile(store.DesignProvenancePath(root, store.ZoneActive, "sample"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatalf("reading provenance sidecar: %v", err)
	}
	return raw
}

// diagnosticAfterVerb strips the leading "<verb-or-tool>: " prefix, the
// one part of a diagnostic line that legitimately differs between the two
// adapters, and returns the "<code>: <detail>" remainder.
func diagnosticAfterVerb(t *testing.T, adapter, line string) string {
	t.Helper()
	_, rest, found := strings.Cut(line, ": ")
	if !found {
		t.Fatalf("%s diagnostic %q does not carry a verb prefix", adapter, line)
	}
	return rest
}

// gitHeadOf resolves root's current HEAD commit, for constructing the
// mutate_draft request's own "expected" stale-safety assertion.
// fixturegit's fixed author/committer/date makes this identical across
// two independently built, content-identical repos.
func gitHeadOf(t *testing.T, root string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = filepath.FromSlash(root)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD: %v", err)
	}
	return strings.TrimSpace(string(out))
}
