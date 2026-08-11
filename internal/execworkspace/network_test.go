package execworkspace

// Portable tests for default-deny network enforcement (design
// docs/superpowers/specs/2026-08-10-default-deny-network-enforcement-design.md
// §§2-5; ledger SI-75/SI-76): the present-grant (ambient allow) polarity
// leg is IDENTICAL on every platform, and the malformed/zero-mode refusal
// in Profile.Command holds on every platform too (design §5: "No state
// silently changes deny into allow"). The absent-grant polarity leg is
// platform-specific and lives in network_linux_test.go /
// network_unsupported_test.go instead (lane contract test area a).

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// --- polarity: present grant is ambient allow (test area a, present leg) ---

func TestBuildProfile_NetworkPolarity_PresentGrant_IsAmbientAllow(t *testing.T) {
	dir := t.TempDir()
	set := GrantSet{Grants: []Grant{
		{Kind: GrantNetwork},
		{Kind: GrantProcessExecution, Argv0s: []string{"go"}},
	}}
	profile, report, err := BuildProfile(dir, dir, set, nil)
	if err != nil {
		t.Fatalf("BuildProfile: unexpected error: %v", err)
	}
	if report.Network.Mode != NetworkAllow {
		t.Fatalf("report.Network.Mode = %v, want %v", report.Network.Mode, NetworkAllow)
	}
	if !report.Network.Configured {
		t.Fatalf("report.Network.Configured = false, want true: ambient allow needs no platform mechanism")
	}
	if report.Network.Reason == "" {
		t.Fatalf("report.Network.Reason is empty")
	}

	var networkRow *EnforcementReportRow
	for i := range report.Rows {
		if report.Rows[i].Kind == GrantNetwork {
			networkRow = &report.Rows[i]
		}
	}
	if networkRow == nil {
		t.Fatalf("report.Rows has no GrantNetwork row: %+v", report.Rows)
	}
	if !networkRow.Applied {
		t.Fatalf("network row Applied = false, want true: a present grant is an explicit ambient allow, never could-not-apply")
	}
	if networkRow.Reason == "" {
		t.Fatalf("network row Reason is empty")
	}

	cmd, _, cancel, cerr := profile.Command(context.Background(), "go")
	if cerr != nil {
		t.Fatalf("Command: %v", cerr)
	}
	defer cancel()
	if cmd.SysProcAttr != nil {
		t.Fatalf("cmd.SysProcAttr = %#v, want nil for explicit ambient allow", cmd.SysProcAttr)
	}
	if cmd.ExtraFiles != nil {
		t.Fatalf("cmd.ExtraFiles = %#v, want nil", cmd.ExtraFiles)
	}
}

// --- malformed/conflict refusal, portable (test area e) ---

func TestProfileCommand_RefusesMalformedNetworkState(t *testing.T) {
	argv0 := allowedArgv0(t)

	cases := map[string]NetworkEnforcement{
		"zero/unset mode":          {},
		"deny, unconfigured":       {Mode: NetworkDeny, Configured: false, Reason: "unconfigured"},
		"unknown mode, configured": {Mode: "block", Configured: true, Reason: "bogus"},
		"allow, unconfigured":      {Mode: NetworkAllow, Configured: false, Reason: "bogus"},
	}
	for name, net := range cases {
		t.Run(name, func(t *testing.T) {
			p := Profile{AllowedArgv0s: []string{argv0}, network: net}
			cmd, ctx, cancel, err := p.Command(context.Background(), argv0)
			if err == nil {
				if cancel != nil {
					cancel()
				}
				t.Fatalf("Command: want fail-closed error for network state %+v, got nil (cmd=%v)", net, cmd)
			}
			var opErr *OperationalError
			if !errors.As(err, &opErr) {
				t.Fatalf("Command: want *OperationalError, got %T: %v", err, err)
			}
			if cmd != nil || ctx != nil || cancel != nil {
				t.Fatalf("Command: refused but returned cmd=%v ctx=%v cancel!=nil=%v", cmd, ctx, cancel != nil)
			}
			// The refusal must keep naming the control and the reason
			// (networkNotLaunchableError, network.go): a future edit that
			// strips the operator-facing disclosure fails here rather than
			// silently degrading it.
			msg := err.Error()
			for _, want := range []string{"network", "no launchable mechanism"} {
				if !strings.Contains(msg, want) {
					t.Fatalf("Command: refusal error %q does not name %q", msg, want)
				}
			}
		})
	}
}

// TestProfileCommand_ZeroProfile_RefusesBeforeReachingNetworkCheck pins the
// check-ORDER half of the lane contract (point 4: "preserving existing
// refusal tests by ordering network check after existing checks is
// acceptable"): the zero Profile's AllowedArgv0s is nil, so it must still
// refuse at the PRE-EXISTING execution-allowance check, never reach the new
// network check — the reason launch_test.go's own refusal tests need no
// migration.
func TestProfileCommand_ZeroProfile_RefusesBeforeReachingNetworkCheck(t *testing.T) {
	cmd, ctx, cancel, err := (Profile{}).Command(context.Background(), "/bin/anything")
	if err == nil {
		if cancel != nil {
			cancel()
		}
		t.Fatalf("Command: want error, got nil (cmd=%v)", cmd)
	}
	if !strings.Contains(err.Error(), "no process-execution grant") {
		t.Fatalf("Command: error = %q, want it to fail at the pre-existing execution-allowance check first", err.Error())
	}
	if cmd != nil || ctx != nil || cancel != nil {
		t.Fatalf("Command: refused but returned cmd=%v ctx=%v cancel!=nil=%v", cmd, ctx, cancel != nil)
	}
}
