package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/designprovenance"
	"github.com/jyang234/verdi/internal/draftmutation"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/store"
)

const designMutateBaseSpec = `---
id: spec/sample
kind: spec
class: feature
title: Sample
owners: [platform-team]
problem: { text: "old problem", anchor: "#problem" }
outcome: { text: "old outcome", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "first", evidence: [static], anchor: "#ac-1" }
---
# Sample

## Problem

Old prose stays.

## Outcome

Old prose stays.

## ac-1

First.
`

func designMutateStore(t *testing.T) (root, head string, base []byte) {
	t.Helper()
	files := map[string]string{".verdi/verdi.yaml": "schema: verdi.layout/v1\n", ".verdi/.gitignore": "data/\n"}
	source := filepath.Join("..", "..", "internal", "policyauthority", "testdata", "store")
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
	root = filepath.ToSlash(resolved)
	base = []byte(designMutateBaseSpec)
	path := store.SpecPath(filepath.FromSlash(root), store.ZoneActive, "sample")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, base, 0o644); err != nil {
		t.Fatal(err)
	}
	return root, repo.Head, base
}

func designMutateRequest(t *testing.T, root, branch, head string, base []byte, operations []map[string]any) []byte {
	t.Helper()
	raw, err := canonjson.Marshal(map[string]any{
		"schema": draftmutation.RequestSchema, "spec": "spec/sample",
		"base_digest": draftmutation.DigestBytes(base), "base_spec_b64": base64.StdEncoding.EncodeToString(base),
		"expected":   map[string]any{"checkout": root, "branch": branch, "head": head},
		"operations": operations,
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

type designMutateRun struct {
	stdout string
	stderr string
	code   int
}

func runDesignMutateBinary(t *testing.T, bin, dir string, stdin []byte, extraEnv map[string]string, args ...string) designMutateRun {
	t.Helper()
	cmd := exec.Command(bin, append([]string{"design", "mutate"}, args...)...)
	cmd.Dir = filepath.FromSlash(dir)
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Env = commandEnvironment(extraEnv)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if err == nil {
		return designMutateRun{stdout: stdout.String(), stderr: stderr.String()}
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return designMutateRun{stdout: stdout.String(), stderr: stderr.String(), code: exitErr.ExitCode()}
	}
	t.Fatalf("running verdi design mutate: %v", err)
	return designMutateRun{}
}

func commandEnvironment(overrides map[string]string) []string {
	keys := make([]string, 0, len(overrides))
	for key := range overrides {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	env := make([]string, 0, len(os.Environ())+len(keys))
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")
		if _, replaced := overrides[key]; !replaced {
			env = append(env, value)
		}
	}
	for _, key := range keys {
		env = append(env, key+"="+overrides[key])
	}
	return env
}

func decodeMutationResult(t *testing.T, raw string) draftmutation.Result {
	t.Helper()
	var result draftmutation.Result
	if err := artifact.DecodeExactJSON([]byte(raw), &result); err != nil {
		t.Fatalf("decoding result: %v\n%s", err, raw)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("validating result: %v", err)
	}
	canonical, err := draftmutation.EncodeResult(result)
	if err != nil || string(canonical) != raw {
		t.Fatalf("result is not exact canonical JSON: %v\nwant: %s\ngot: %s", err, canonical, raw)
	}
	return result
}

func TestDesignMutateBuiltBinaryStdinFileAndSpoofResistance(t *testing.T) {
	bin := buildVerdiBinary(t)
	t.Run("stdin ignores actor and principal environment", func(t *testing.T) {
		root, head, base := designMutateStore(t)
		request := designMutateRequest(t, root, "design/sample", head, base, []map[string]any{{"op": "set-problem", "text": "stdin change", "anchor": "#problem"}})
		run := runDesignMutateBinary(t, bin, root, request, map[string]string{
			"CI_DEFAULT_BRANCH": "main", "VERDI_ACTOR_KIND": "human", "VERDI_PRINCIPAL_ID": "principal/forged", "VERDI_HUMAN": "1",
		}, "--request", "-", "--harness", "codex", "--session", "session-1")
		if run.code != 0 || run.stderr != "" {
			t.Fatalf("exit/stdout/stderr = %d\n%s\n%s", run.code, run.stdout, run.stderr)
		}
		result := decodeMutationResult(t, run.stdout)
		if result.Identity != (draftmutation.Identity{Checkout: root, Branch: "design/sample", Head: head, Spec: "spec/sample"}) {
			t.Fatalf("result identity = %+v", result.Identity)
		}
		logBytes, err := os.ReadFile(store.DesignProvenancePath(filepath.FromSlash(root), store.ZoneActive, "sample"))
		if err != nil {
			t.Fatal(err)
		}
		entries, err := designprovenance.DecodeLog(logBytes)
		if err != nil || len(entries) != 1 {
			t.Fatalf("provenance = %+v, %v", entries, err)
		}
		if !entries[0].Attribution.Unauthenticated || entries[0].Attribution.PrincipalID != "" || entries[0].Harness != "codex" || entries[0].Session != "session-1" {
			t.Fatalf("CLI provenance attribution = %+v", entries[0])
		}
	})

	t.Run("request file", func(t *testing.T) {
		root, head, base := designMutateStore(t)
		request := designMutateRequest(t, root, "design/sample", head, base, []map[string]any{{"op": "set-outcome", "text": "file change", "anchor": "#outcome"}})
		requestPath := filepath.Join(filepath.FromSlash(root), "mutation.json")
		if err := os.WriteFile(requestPath, request, 0o644); err != nil {
			t.Fatal(err)
		}
		run := runDesignMutateBinary(t, bin, root, nil, map[string]string{"CI_DEFAULT_BRANCH": "main"}, "--request", "mutation.json", "--harness", "script")
		if run.code != 0 || run.stderr != "" {
			t.Fatalf("exit/stdout/stderr = %d\n%s\n%s", run.code, run.stdout, run.stderr)
		}
		decodeMutationResult(t, run.stdout)
	})
}

func TestDesignMutateBuiltBinaryDiagnosticsAndExitCodes(t *testing.T) {
	bin := buildVerdiBinary(t)
	t.Run("required harness", func(t *testing.T) {
		root, head, base := designMutateStore(t)
		request := designMutateRequest(t, root, "design/sample", head, base, []map[string]any{{"op": "set-problem", "text": "changed", "anchor": "#problem"}})
		run := runDesignMutateBinary(t, bin, root, request, nil, "--request", "-")
		if run.code != 2 || run.stdout != "" || !strings.HasPrefix(run.stderr, "input-invalid:") {
			t.Fatalf("required harness = %+v", run)
		}
	})

	t.Run("malformed request actor field", func(t *testing.T) {
		root, head, base := designMutateStore(t)
		request := designMutateRequest(t, root, "design/sample", head, base, []map[string]any{{"op": "set-problem", "text": "changed", "anchor": "#problem"}})
		request = bytes.Replace(request, []byte(`"base_digest":`), []byte(`"actor":{"kind":"human"},"base_digest":`), 1)
		run := runDesignMutateBinary(t, bin, root, request, nil, "--request", "-", "--harness", "codex")
		if run.code != 2 || run.stdout != "" || !strings.HasPrefix(run.stderr, "input-invalid:") || !strings.Contains(run.stderr, "unknown field") {
			t.Fatalf("malformed request = %+v", run)
		}
	})

	t.Run("post-construction identity mismatch", func(t *testing.T) {
		root, _, base := designMutateStore(t)
		request := designMutateRequest(t, root, "design/sample", strings.Repeat("b", 40), base, []map[string]any{{"op": "set-problem", "text": "changed", "anchor": "#problem"}})
		run := runDesignMutateBinary(t, bin, root, request, nil, "--request", "-", "--harness", "codex")
		if run.code != 2 || run.stdout != "" || !strings.HasPrefix(run.stderr, "identity-invalid: identity=") || !strings.Contains(run.stderr, fmt.Sprintf(`"checkout":"%s"`, root)) || !strings.Contains(run.stderr, `"spec":"spec/sample"`) {
			t.Fatalf("identity mismatch = %+v", run)
		}
	})

	t.Run("pre-construction identity unavailable", func(t *testing.T) {
		dir, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		base := []byte(designMutateBaseSpec)
		request := designMutateRequest(t, filepath.ToSlash(dir), "design/sample", strings.Repeat("a", 40), base, []map[string]any{{"op": "set-problem", "text": "changed", "anchor": "#problem"}})
		run := runDesignMutateBinary(t, bin, dir, request, nil, "--request", "-", "--harness", "codex")
		if run.code != 2 || run.stdout != "" || !strings.HasPrefix(run.stderr, "identity-invalid: resolved_identity=unavailable expected=") || !strings.Contains(run.stderr, `"branch":"design/sample"`) {
			t.Fatalf("unavailable identity = %+v", run)
		}
	})

	t.Run("operation verdict", func(t *testing.T) {
		root, head, base := designMutateStore(t)
		request := designMutateRequest(t, root, "design/sample", head, base, []map[string]any{{"op": "remove-ac", "id": "ac-missing"}})
		run := runDesignMutateBinary(t, bin, root, request, map[string]string{"CI_DEFAULT_BRANCH": "main"}, "--request", "-", "--harness", "codex")
		if run.code != 1 || run.stdout != "" || !strings.HasPrefix(run.stderr, "operation-invalid: identity=") || !strings.Contains(run.stderr, `"head":"`+head+`"`) {
			t.Fatalf("operation verdict = %+v", run)
		}
	})
}

func TestDesignMutateBuiltBinaryStaleAndConcurrentCalls(t *testing.T) {
	bin := buildVerdiBinary(t)
	t.Run("stale", func(t *testing.T) {
		root, head, base := designMutateStore(t)
		request := designMutateRequest(t, root, "design/sample", head, base, []map[string]any{{"op": "set-outcome", "text": "typed", "anchor": "#outcome"}})
		current := bytes.Replace(base, []byte(`text: "old problem"`), []byte(`text: "direct problem"`), 1)
		if err := os.WriteFile(store.SpecPath(filepath.FromSlash(root), store.ZoneActive, "sample"), current, 0o644); err != nil {
			t.Fatal(err)
		}
		run := runDesignMutateBinary(t, bin, root, request, map[string]string{"CI_DEFAULT_BRANCH": "main"}, "--request", "-", "--harness", "codex")
		if run.code != 1 || run.stderr != "" {
			t.Fatalf("stale run = %+v", run)
		}
		var refusal draftmutation.StaleRefusal
		if err := artifact.DecodeExactJSON([]byte(run.stdout), &refusal); err != nil || refusal.Validate() != nil || refusal.Identity.Checkout != root || len(refusal.ChangedTargets) != 1 || refusal.ChangedTargets[0] != "problem" {
			t.Fatalf("stale refusal = %+v, %v", refusal, err)
		}
	})

	t.Run("two processes serialize to success then stale", func(t *testing.T) {
		root, head, base := designMutateStore(t)
		request := designMutateRequest(t, root, "design/sample", head, base, []map[string]any{{"op": "set-problem", "text": "concurrent", "anchor": "#problem"}})
		start := make(chan struct{})
		results := make(chan designMutateRun, 2)
		for range 2 {
			go func() {
				<-start
				results <- runDesignMutateBinary(t, bin, root, request, map[string]string{"CI_DEFAULT_BRANCH": "main"}, "--request", "-", "--harness", "codex")
			}()
		}
		close(start)
		first, second := <-results, <-results
		codes := []int{first.code, second.code}
		sort.Ints(codes)
		if codes[0] != 0 || codes[1] != 1 && codes[1] != 2 {
			t.Fatalf("concurrent results = %+v / %+v", first, second)
		}
		for _, run := range []designMutateRun{first, second} {
			switch run.code {
			case 0, 1:
				if run.stderr != "" || run.stdout == "" {
					t.Fatalf("success/stale concurrent output = %+v", run)
				}
			case 2:
				if run.stdout != "" || !strings.HasPrefix(run.stderr, "io-failure: identity=") || !strings.Contains(run.stderr, "global writer lock") {
					t.Fatalf("lock-refusal concurrent output = %+v", run)
				}
			}
		}
		logBytes, err := os.ReadFile(store.DesignProvenancePath(filepath.FromSlash(root), store.ZoneActive, "sample"))
		entries, decodeErr := designprovenance.DecodeLog(logBytes)
		if err != nil || decodeErr != nil || len(entries) != 1 {
			t.Fatalf("concurrent provenance = %+v, read=%v decode=%v", entries, err, decodeErr)
		}
	})
}

func TestDesignMutateBuiltBinaryRefusesServeWriterLock(t *testing.T) {
	bin := buildVerdiBinary(t)
	root, head, base := designMutateStore(t)
	request := designMutateRequest(t, root, "design/sample", head, base, []map[string]any{{"op": "set-problem", "text": "blocked", "anchor": "#problem"}})
	serve := exec.Command(bin, "serve", "--http", "127.0.0.1:0")
	serve.Dir = filepath.FromSlash(root)
	serve.Env = commandEnvironment(map[string]string{"CI_DEFAULT_BRANCH": "main"})
	var serveOutput bytes.Buffer
	serve.Stdout, serve.Stderr = &serveOutput, &serveOutput
	if err := serve.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = serve.Process.Kill()
		_ = serve.Wait()
	})
	waitForPointerFile(t, filepath.FromSlash(root), 5_000_000_000)

	run := runDesignMutateBinary(t, bin, root, request, map[string]string{"CI_DEFAULT_BRANCH": "main"}, "--request", "-", "--harness", "codex")
	if run.code != 2 || run.stdout != "" || !strings.HasPrefix(run.stderr, "io-failure: identity=") || !strings.Contains(run.stderr, "global writer lock") {
		t.Fatalf("serve contention = %+v\nserve output:\n%s", run, serveOutput.String())
	}
}
