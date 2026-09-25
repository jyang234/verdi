// Unit test of scripts/merge-gate-verdict.sh (SI-266), the committed decision
// the required `merge-gate` aggregator job runs. The workflow passes one
// `<job>=<result>` argument per gate job from `${{ needs.<job>.result }}`;
// GitHub's results are success, failure, cancelled, and skipped, and a job
// missing from `needs:` expands to an empty result. The script must exit 0
// only when it received at least one argument and every result is exactly
// `success`. The shape of the call itself is pinned by
// TestMergeGateAggregatorDecidesOverEveryGateJob.
package specalign

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// mergeGateVerdictScript is the committed decision the aggregator runs.
const mergeGateVerdictScript = "scripts/merge-gate-verdict.sh"

// runVerdictScript execs the script directly — as the workflow does, so the
// shebang and the executable bit are exercised — and returns its exit code
// and combined output.
func runVerdictScript(t *testing.T, args ...string) (int, string) {
	t.Helper()
	script := filepath.Join(verdiRepoRoot, filepath.FromSlash(mergeGateVerdictScript))
	cmd := exec.Command(script, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err == nil {
		return 0, out.String()
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode(), out.String()
	}
	t.Fatalf("running %s: %v", script, err)
	return -1, ""
}

// TestMergeGateVerdictScript covers success, each non-success result, an
// empty (missing) result, no results at all, and malformed arguments.
func TestMergeGateVerdictScript(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantCode int
		wantOut  []string
	}{
		{
			name:     "every gate job succeeded",
			args:     []string{"static=success", "test-cmd=success", "e2e=success"},
			wantCode: 0,
			wantOut:  []string{"static", "test-cmd", "e2e"},
		},
		{
			name:     "a single successful gate job",
			args:     []string{"static=success"},
			wantCode: 0,
		},
		{
			name:     "a failed gate job fails the gate",
			args:     []string{"static=success", "test-rest=failure", "e2e=success"},
			wantCode: 1,
			wantOut:  []string{"test-rest", "failure"},
		},
		{
			name:     "a cancelled gate job fails the gate",
			args:     []string{"static=success", "e2e=cancelled"},
			wantCode: 1,
			wantOut:  []string{"e2e", "cancelled"},
		},
		{
			name:     "a skipped gate job fails the gate",
			args:     []string{"test-cross=skipped", "static=success"},
			wantCode: 1,
			wantOut:  []string{"test-cross", "skipped"},
		},
		{
			name:     "an empty result (job missing from needs) fails the gate",
			args:     []string{"static=success", "spec-align="},
			wantCode: 1,
			wantOut:  []string{"spec-align"},
		},
		{
			name:     "an unknown or differently cased result fails the gate",
			args:     []string{"static=SUCCESS", "e2e=neutral"},
			wantCode: 1,
			wantOut:  []string{"static", "e2e"},
		},
		{
			name:     "a failure is reported even after a success of the same job",
			args:     []string{"static=success", "static=failure"},
			wantCode: 1,
		},
		{
			name:     "no results at all is refused",
			args:     nil,
			wantCode: 2,
		},
		{
			name:     "an argument without = is refused",
			args:     []string{"static=success", "e2e"},
			wantCode: 2,
			wantOut:  []string{"e2e"},
		},
		{
			name:     "an argument with an empty job name is refused",
			args:     []string{"=success"},
			wantCode: 2,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, out := runVerdictScript(t, tc.args...)
			if code != tc.wantCode {
				t.Errorf("exit = %d, want %d; output:\n%s", code, tc.wantCode, out)
			}
			for _, want := range tc.wantOut {
				if !strings.Contains(out, want) {
					t.Errorf("output does not name %q:\n%s", want, out)
				}
			}
		})
	}
}

// runCanary runs the aggregator's pinned canary step in dir the way a GitHub
// Actions `run:` step with no `shell:` runs on ubuntu-latest, `bash -e`, and
// returns its exit code and combined output.
func runCanary(t *testing.T, dir string) (int, string) {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatalf("bash is required to run the merge-gate canary step: %v", err)
	}
	cmd := exec.Command(bash, "--noprofile", "--norc", "-e", "-c", mergeGateCanaryRun)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err = cmd.Run()
	if err == nil {
		return 0, out.String()
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode(), out.String()
	}
	t.Fatalf("running the canary in %s: %v", dir, err)
	return -1, ""
}

// TestMergeGateVerdictCanary runs the canary step's pinned text against the
// real verdict script and against broken stand-ins. It must pass over the
// real script and fail over any script that exits 0 on a failing result,
// such as a loop whose failure flag is lost in a pipeline's subshell. A
// script that fails everything leaves the canary green: the verdict step
// after it then fails the required context on its own.
func TestMergeGateVerdictCanary(t *testing.T) {
	cases := []struct {
		name     string
		script   string // "" runs against the real repository
		wantCode int
		wantOut  string
	}{
		{
			name:     "the real script fails the probe, so the canary passes",
			wantCode: 0,
			wantOut:  "canary OK",
		},
		{
			name:     "a script that passes every result fails the canary",
			script:   "#!/bin/sh\nexit 0\n",
			wantCode: 1,
			wantOut:  "canary FAILED",
		},
		{
			name: "a failure flag lost in a pipeline subshell fails the canary",
			script: "#!/bin/sh\nfailed=0\n" +
				"printf '%s\\n' \"$@\" | while IFS= read -r arg; do\n" +
				"\t[ \"${arg#*=}\" = success ] || failed=1\n" +
				"done\n" +
				"exit \"$failed\"\n",
			wantCode: 1,
			wantOut:  "canary FAILED",
		},
		{
			name:     "a script that fails every result leaves the canary green",
			script:   "#!/bin/sh\nexit 1\n",
			wantCode: 0,
			wantOut:  "canary OK",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := verdiRepoRoot
			if tc.script != "" {
				dir = t.TempDir()
				path := filepath.Join(dir, filepath.FromSlash(mergeGateVerdictScript))
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(tc.script), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			code, out := runCanary(t, dir)
			if code != tc.wantCode {
				t.Errorf("canary exit = %d, want %d; output:\n%s", code, tc.wantCode, out)
			}
			if !strings.Contains(out, tc.wantOut) {
				t.Errorf("canary output does not contain %q:\n%s", tc.wantOut, out)
			}
		})
	}
}
