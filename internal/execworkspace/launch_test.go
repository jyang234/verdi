package execworkspace

import (
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// helperSleep drives TestExecworkspaceHelperSleep, the re-exec helper the
// deadline-enforcement test launches through Profile.Command. Registering it
// as a test flag keeps the helper hermetic: the parent needs no environment
// channel to the child, which matters because Profile.Command sets exactly
// the profile environment and nothing else.
var helperSleep = flag.Duration("execworkspace.helper.sleep", 0,
	"internal: when > 0, TestExecworkspaceHelperSleep sleeps this long (re-exec helper for the launch-deadline test)")

// TestExecworkspaceHelperSleep is not a test of this package; it is the child
// process the deadline test re-execs. It does nothing unless the parent passes
// -execworkspace.helper.sleep.
func TestExecworkspaceHelperSleep(t *testing.T) {
	if *helperSleep <= 0 {
		t.Skip("not the re-exec helper: -execworkspace.helper.sleep unset")
	}
	time.Sleep(*helperSleep)
}

// allowedArgv0 is an absolute, non-existent program path: absolute so
// exec.Command records it as cmd.Path verbatim without consulting PATH, and
// non-existent because every construction-level assertion below stops short
// of running anything.
func allowedArgv0(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "tool")
}

// --- fail-closed refusals ---

func TestProfileCommand_RefusesWithoutAnEnforceableAllowance(t *testing.T) {
	argv0 := allowedArgv0(t)

	cases := []struct {
		name    string
		profile Profile
		argv0   string
		wantIn  string
	}{
		{
			name:    "zero profile fails closed",
			profile: Profile{},
			argv0:   argv0,
			wantIn:  "no process-execution grant",
		},
		{
			name:    "timeout grant alone records no execution allowance",
			profile: Profile{Timeout: time.Second},
			argv0:   argv0,
			wantIn:  "no process-execution grant",
		},
		{
			name:    "argv0 outside the allowlist",
			profile: Profile{AllowedArgv0s: []string{"/bin/allowed"}},
			argv0:   "/bin/other",
			wantIn:  "not in the granted argv0 allowlist",
		},
		{
			name:    "empty allowlist is an allowance that permits nothing",
			profile: Profile{AllowedArgv0s: []string{}},
			argv0:   argv0,
			wantIn:  "not in the granted argv0 allowlist",
		},
		{
			name:    "prefix of an allowed entry is not a member",
			profile: Profile{AllowedArgv0s: []string{"/bin/allowed-tool"}},
			argv0:   "/bin/allowed",
			wantIn:  "not in the granted argv0 allowlist",
		},
		{
			name:    "empty argv0",
			profile: Profile{AllowedArgv0s: []string{"/bin/allowed"}},
			argv0:   "",
			wantIn:  "not in the granted argv0 allowlist",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd, ctx, cancel, err := tc.profile.Command(context.Background(), tc.argv0)
			if err == nil {
				if cancel != nil {
					cancel()
				}
				t.Fatalf("Command(%q): want fail-closed error, got nil (cmd=%v)", tc.argv0, cmd)
			}
			var opErr *OperationalError
			if !errors.As(err, &opErr) {
				t.Fatalf("Command(%q): want *OperationalError, got %T: %v", tc.argv0, err, err)
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Fatalf("Command(%q) error = %q, want it to contain %q", tc.argv0, err.Error(), tc.wantIn)
			}
			// A refusal never yields a usable launch, and never a live
			// context or cancel the caller might believe in.
			if cmd != nil {
				t.Fatalf("Command(%q): refused but returned a non-nil *exec.Cmd (%v) — never an unconstrained Cmd", tc.argv0, cmd)
			}
			if ctx != nil {
				t.Fatalf("Command(%q): refused but returned a non-nil context", tc.argv0)
			}
			if cancel != nil {
				t.Fatalf("Command(%q): refused but returned a non-nil cancel func", tc.argv0)
			}
		})
	}
}

func TestProfileCommand_RefusesNilContext(t *testing.T) {
	argv0 := allowedArgv0(t)
	p := Profile{AllowedArgv0s: []string{argv0}, Timeout: time.Second}

	//nolint:staticcheck // deliberately passing a nil context: the seam must
	// fail closed rather than panic inside context.WithTimeout.
	cmd, ctx, cancel, err := p.Command(nil, argv0)
	if err == nil {
		if cancel != nil {
			cancel()
		}
		t.Fatalf("Command(nil ctx): want fail-closed error, got nil (cmd=%v)", cmd)
	}
	var opErr *OperationalError
	if !errors.As(err, &opErr) {
		t.Fatalf("Command(nil ctx): want *OperationalError, got %T: %v", err, err)
	}
	if cmd != nil || ctx != nil || cancel != nil {
		t.Fatalf("Command(nil ctx): refused but returned cmd=%v ctx=%v cancel!=nil=%v", cmd, ctx, cancel != nil)
	}
}

// --- allowed construction ---

func TestProfileCommand_AllowedLaunchMatchesTheProfile(t *testing.T) {
	dir := t.TempDir()
	argv0 := filepath.Join(dir, "tool")

	cases := []struct {
		name         string
		grants       GrantSet
		wantDeadline bool
	}{
		{
			name: "process-execution grant only: no deadline added",
			grants: GrantSet{Grants: []Grant{
				{Kind: GrantProcessExecution, Argv0s: []string{argv0, filepath.Join(dir, "other")}},
			}},
			wantDeadline: false,
		},
		{
			name: "process-execution plus timeouts: deadline derived",
			grants: GrantSet{Grants: []Grant{
				{Kind: GrantProcessExecution, Argv0s: []string{argv0}},
				{Kind: GrantTimeouts, Seconds: 30},
			}},
			wantDeadline: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			envRoot := t.TempDir()
			profile, _, err := BuildProfile(dir, envRoot, tc.grants, map[string]string{"FOO": "bar"})
			if err != nil {
				t.Fatalf("BuildProfile: %v", err)
			}

			parent := context.Background()
			cmd, ctx, cancel, err := profile.Command(parent, argv0, "--flag", "value")
			if err != nil {
				t.Fatalf("Command: %v", err)
			}
			if cancel == nil {
				t.Fatal("Command returned a nil cancel func: a success always returns a callable cancel, never nil")
			}
			defer cancel()
			if cmd == nil || ctx == nil {
				t.Fatalf("Command returned cmd=%v ctx=%v, want both non-nil", cmd, ctx)
			}

			if cmd.Path != argv0 {
				t.Fatalf("cmd.Path = %q, want %q", cmd.Path, argv0)
			}
			wantArgs := []string{argv0, "--flag", "value"}
			if !reflect.DeepEqual(cmd.Args, wantArgs) {
				t.Fatalf("cmd.Args = %#v, want %#v", cmd.Args, wantArgs)
			}
			if !reflect.DeepEqual(cmd.Env, profile.Env()) {
				t.Fatalf("cmd.Env = %#v, want exactly profile.Env() = %#v (no inherited process environment)", cmd.Env, profile.Env())
			}
			if cmd.Dir != "" {
				t.Fatalf("cmd.Dir = %q, want empty: the working directory is the consumer's choice, never this seam's", cmd.Dir)
			}

			deadline, ok := ctx.Deadline()
			if ok != tc.wantDeadline {
				t.Fatalf("ctx deadline present = %v, want %v", ok, tc.wantDeadline)
			}
			if ok {
				if remaining := time.Until(deadline); remaining <= 0 || remaining > profile.Timeout {
					t.Fatalf("deadline remaining = %v, want within (0, %v]", remaining, profile.Timeout)
				}
			} else if ctx != parent {
				t.Fatal("with no timeout grant, Command must return the caller's own context unchanged")
			}
		})
	}
}

// TestProfileCommand_DeadlineActuallyKillsTheProcess proves the timeout grant
// is ENFORCED, not merely recorded: the seam's returned Cmd is bound to the
// derived context, so a child outliving the deadline is killed. Hermetic — the
// child is this very test binary, re-execed as the sleep helper above.
func TestProfileCommand_DeadlineActuallyKillsTheProcess(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Skipf("os.Executable unavailable: %v", err)
	}

	p := Profile{Timeout: 300 * time.Millisecond, AllowedArgv0s: []string{self}}
	cmd, _, cancel, err := p.Command(context.Background(), self,
		"-test.run=^TestExecworkspaceHelperSleep$",
		"-execworkspace.helper.sleep=60s",
	)
	if err != nil {
		t.Fatalf("Command: %v", err)
	}
	defer cancel()

	start := time.Now()
	runErr := cmd.Run()
	elapsed := time.Since(start)

	if runErr == nil {
		t.Fatal("the helper slept 60s past a 300ms deadline and exited cleanly: the deadline was not enforced")
	}
	if elapsed > 30*time.Second {
		t.Fatalf("child took %v to die against a 300ms deadline: the deadline was not enforced", elapsed)
	}
}
