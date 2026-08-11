//go:build !linux

package execworkspace

// Unsupported-platform tests for default-deny network enforcement (design
// §1: "Darwin is unsupported for this control in the first increment ...
// There is no weaker fallback."; ledger SI-75/SI-76). Runs natively on this
// lane's darwin development machine — lane contract test areas a
// (absent-grant leg), d (unsupported-platform refusal), and the "combined
// failures" half of point 3.

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
)

func TestBuildProfile_NetworkPolarity_AbsentGrant_UnsupportedPlatform(t *testing.T) {
	dir := t.TempDir()
	set := GrantSet{Grants: []Grant{{Kind: GrantProcessExecution, Argv0s: []string{"go"}}}}
	_, report, err := BuildProfile(dir, dir, set, nil)
	if err == nil {
		t.Fatalf("BuildProfile: want an operational error on GOOS=%s (unsupported, no weaker fallback — design §1), got nil", runtime.GOOS)
	}
	var opErr *OperationalError
	if !errors.As(err, &opErr) {
		t.Fatalf("BuildProfile: want *OperationalError, got %T: %v", err, err)
	}
	if report == nil {
		t.Fatalf("BuildProfile: want the report returned alongside the error (AD-5 pattern), got nil")
	}
	if report.Network.Mode != NetworkDeny {
		t.Fatalf("report.Network.Mode = %v, want %v", report.Network.Mode, NetworkDeny)
	}
	if report.Network.Configured {
		t.Fatalf("report.Network.Configured = true, want false: GOOS=%s has no default-deny backend", runtime.GOOS)
	}
	if report.Network.Reason == "" {
		t.Fatalf("report.Network.Reason is empty")
	}
	if !strings.Contains(err.Error(), "network") {
		t.Fatalf("error = %q, want it to name the unconfigurable network control", err.Error())
	}

	var foundProcessExecRow bool
	for _, row := range report.Rows {
		if row.Kind == GrantNetwork {
			t.Fatalf("report.Rows contains a network row for an ABSENT grant: %+v (absence is the report's Network FIELD, never a row)", report.Rows)
		}
		if row.Kind == GrantProcessExecution {
			foundProcessExecRow = true
			if !row.Applied {
				t.Fatalf("process-execution row Applied = false, want true: network's absence must not widen into hiding an otherwise-normal row")
			}
		}
	}
	if !foundProcessExecRow {
		t.Fatalf("report.Rows has no process-execution row: %+v", report.Rows)
	}
}

func TestProfileCommand_UnsupportedPlatform_UnconfiguredDenyRefuses(t *testing.T) {
	argv0 := allowedArgv0(t)
	p := Profile{
		AllowedArgv0s: []string{argv0},
		network:       NetworkEnforcement{Mode: NetworkDeny, Configured: false, Reason: "unsupported"},
	}
	cmd, ctx, cancel, err := p.Command(context.Background(), argv0)
	if err == nil {
		if cancel != nil {
			cancel()
		}
		t.Fatalf("Command: want error for an unconfigured deny, got nil (cmd=%v)", cmd)
	}
	var opErr *OperationalError
	if !errors.As(err, &opErr) {
		t.Fatalf("Command: want *OperationalError, got %T: %v", err, err)
	}
	if cmd != nil || ctx != nil || cancel != nil {
		t.Fatalf("Command: refused but returned cmd=%v ctx=%v cancel!=nil=%v", cmd, ctx, cancel != nil)
	}
}

// --- combined failure: deny-unconfigured PLUS other could-not-apply rows
// name EVERYTHING in ONE deterministic error (lane contract point 3's last
// sentence) ---

func TestBuildProfile_CombinedFailure_NamesNetworkAndOtherUnappliedKinds(t *testing.T) {
	dir := t.TempDir()
	set := GrantSet{Grants: []Grant{
		{Kind: GrantPathRead, Paths: []string{"src"}},
		{Kind: GrantResourceCeilings, Ceilings: map[string]int{"cpu": 1}},
	}}
	_, report, err := BuildProfile(dir, dir, set, nil)
	if err == nil {
		t.Fatalf("BuildProfile: want ONE combined error, got nil")
	}
	var opErr *OperationalError
	if !errors.As(err, &opErr) {
		t.Fatalf("BuildProfile: want *OperationalError, got %T: %v", err, err)
	}
	msg := err.Error()
	for _, want := range []string{"network", "path-read", "resource-ceilings"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error %q does not name %q", msg, want)
		}
	}
	if report == nil || len(report.Rows) != 2 {
		t.Fatalf("report = %+v, want 2 rows (path-read, resource-ceilings) returned alongside the ONE combined error", report)
	}
	if report.Network.Mode != NetworkDeny || report.Network.Configured {
		t.Fatalf("report.Network = %+v, want deny/unconfigured", report.Network)
	}
}

// TestBuildProfile_NetworkPolarity_AbsentGrant_EmptyGrantSet pins that an
// entirely empty GrantSet still reaches the SAME mandatory-deny path as any
// other absent-network grant set: absence, not "no grants at all", is what
// triggers the control.
func TestBuildProfile_NetworkPolarity_AbsentGrant_EmptyGrantSet(t *testing.T) {
	dir := t.TempDir()
	_, report, err := BuildProfile(dir, dir, GrantSet{}, nil)
	if err == nil {
		t.Fatalf("BuildProfile: want an operational error for an empty grant set on an unsupported platform, got nil")
	}
	if report == nil || len(report.Rows) != 0 {
		t.Fatalf("report = %+v, want a non-nil, zero-row report (no OTHER grants requested)", report)
	}
	if report.Network.Mode != NetworkDeny || report.Network.Configured {
		t.Fatalf("report.Network = %+v, want deny/unconfigured", report.Network)
	}
}
