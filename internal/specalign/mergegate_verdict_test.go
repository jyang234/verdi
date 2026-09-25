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
