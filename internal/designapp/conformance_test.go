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

func runCLI(t *testing.T, bin, dir string, args ...string) (stdout string, code int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = filepath.FromSlash(dir)
	cmd.Env = append(os.Environ(), "CI_DEFAULT_BRANCH=main")
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	if err == nil {
		return out.String(), 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return out.String(), exitErr.ExitCode()
	}
	t.Fatalf("running verdi %v: %v\nstderr: %s", args, err, errOut.String())
	return "", 0
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

			cliOut, code := runCLI(t, bin, cliRoot, tc.cliArgs...)
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
	})
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
