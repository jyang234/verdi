package main

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"testing"
)

// runDesignSubBinary runs `verdi design <subcommand> <args...>` against
// dir with CI_DEFAULT_BRANCH pinned to main — the same hermetic
// default-branch override internal/designapp's own tests use, since a
// fixturegit repo carries no "origin" remote for specstate's fallback to
// resolve otherwise.
func runDesignSubBinary(t *testing.T, bin, dir string, args ...string) designMutateRun {
	t.Helper()
	cmd := exec.Command(bin, append([]string{"design"}, args...)...)
	cmd.Dir = filepath.FromSlash(dir)
	cmd.Env = commandEnvironment(map[string]string{"CI_DEFAULT_BRANCH": "main"})
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if err == nil {
		return designMutateRun{stdout: stdout.String(), stderr: stderr.String()}
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return designMutateRun{stdout: stdout.String(), stderr: stderr.String(), code: exitErr.ExitCode()}
	}
	t.Fatalf("running verdi design %v: %v", args, err)
	return designMutateRun{}
}

// TestDesignReadOnlySubcommandsBuiltBinary proves the five new
// designapp-backed CLI subcommands (Wave 6 Task 1) dispatch, produce
// canonical JSON on success, and map every closed *designapp.Error
// classification to the right exit code — the CLI half of the
// board/context/capabilities/provenance/review conformance pairing
// internal/designapp/conformance_test.go completes on the MCP side.
func TestDesignReadOnlySubcommandsBuiltBinary(t *testing.T) {
	bin := buildVerdiBinary(t)
	root, _, _ := designMutateStore(t)

	for _, tc := range []struct {
		name string
		args []string
		want string // one required top-level JSON key to spot-check
	}{
		{name: "board", args: []string{"board", "spec/sample"}, want: "spec"},
		{name: "context", args: []string{"context", "spec/sample"}, want: "current_draft"},
		{name: "capabilities", args: []string{"capabilities", "spec/sample"}, want: "policy_mode"},
		{name: "provenance", args: []string{"provenance", "spec/sample"}, want: "entries"},
		{name: "review", args: []string{"review", "spec/sample"}, want: "baseline"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := runDesignSubBinary(t, bin, root, tc.args...)
			if run.code != 0 || run.stderr != "" {
				t.Fatalf("verdi design %v: exit/stderr = %d/%q, stdout=%s", tc.args, run.code, run.stderr, run.stdout)
			}
			var decoded map[string]json.RawMessage
			if err := json.Unmarshal([]byte(run.stdout), &decoded); err != nil {
				t.Fatalf("decoding stdout as JSON: %v\n%s", err, run.stdout)
			}
			if _, ok := decoded[tc.want]; !ok {
				t.Fatalf("verdi design %v: stdout missing key %q: %s", tc.args, tc.want, run.stdout)
			}
			if !bytes.HasSuffix([]byte(run.stdout), []byte("\n")) {
				t.Fatalf("verdi design %v: stdout is not newline-terminated: %q", tc.args, run.stdout)
			}
		})
	}

	t.Run("missing spec is exit 1 verdict", func(t *testing.T) {
		run := runDesignSubBinary(t, bin, root, "board", "spec/does-not-exist")
		if run.code != 1 || run.stdout != "" {
			t.Fatalf("verdi design board (missing spec): exit/stdout = %d/%q, stderr=%s", run.code, run.stdout, run.stderr)
		}
	})

	t.Run("missing argument is exit 2 usage error", func(t *testing.T) {
		for _, sub := range []string{"board", "context", "capabilities", "provenance", "review"} {
			run := runDesignSubBinary(t, bin, root, sub)
			if run.code != 2 || run.stdout != "" {
				t.Fatalf("verdi design %s (no args): exit/stdout = %d/%q, stderr=%s", sub, run.code, run.stdout, run.stderr)
			}
		}
	})

	t.Run("context accepts repeated --child-story", func(t *testing.T) {
		run := runDesignSubBinary(t, bin, root, "context", "spec/sample", "--child-story", "spec/does-not-exist")
		// The named child story does not exist: a verdict refusal, never a
		// silently ignored flag.
		if run.code != 1 {
			t.Fatalf("verdi design context --child-story (missing): exit = %d, stdout=%s, stderr=%s", run.code, run.stdout, run.stderr)
		}
	})
}
