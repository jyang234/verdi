//go:build linux

package execworkspace

// Linux-specific tests for default-deny network enforcement (design
// docs/superpowers/specs/2026-08-10-default-deny-network-enforcement-design.md
// §§3-5; ledger SI-75/SI-76). Lane contract test areas a (absent-grant
// leg), b (exact SysProcAttr tuple), c (nil Credential; nil SysProcAttr on
// allow), e (malformed refusal on Linux too), f (protected exec.Cmd
// fields for the deny leg — the allow leg's equivalent lives in the
// portable network_test.go).

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
)

// --- polarity: absent grant on Linux (test area a, absent leg) ---

func TestBuildProfile_NetworkPolarity_AbsentGrant_Linux(t *testing.T) {
	dir := t.TempDir()
	set := GrantSet{Grants: []Grant{{Kind: GrantProcessExecution, Argv0s: []string{"go"}}}}
	_, report, err := BuildProfile(dir, dir, set, nil)
	if err != nil {
		t.Fatalf("BuildProfile: unexpected error: %v", err)
	}
	if report.Network.Mode != NetworkDeny {
		t.Fatalf("report.Network.Mode = %v, want %v", report.Network.Mode, NetworkDeny)
	}
	if !report.Network.Configured {
		t.Fatalf("report.Network.Configured = false, want true on Linux")
	}
	if report.Network.Reason == "" {
		t.Fatalf("report.Network.Reason is empty")
	}
	for _, row := range report.Rows {
		if row.Kind == GrantNetwork {
			t.Fatalf("report.Rows contains a network row for an ABSENT grant: %+v", report.Rows)
		}
	}
}

func wantDenySysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUSER | syscall.CLONE_NEWNET,
		UidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Geteuid(), Size: 1},
		},
		GidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getegid(), Size: 1},
		},
		GidMappingsEnableSetgroups: false,
	}
}

// --- exact SysProcAttr tuple (test area b), nil Credential (test area c) ---

func TestProfileCommand_Deny_ExactSysProcAttrTuple(t *testing.T) {
	argv0 := allowedArgv0(t)
	p := Profile{
		AllowedArgv0s: []string{argv0},
		network:       NetworkEnforcement{Mode: NetworkDeny, Configured: true, Reason: "deny"},
	}
	cmd, _, cancel, err := p.Command(context.Background(), argv0)
	if err != nil {
		t.Fatalf("Command: %v", err)
	}
	defer cancel()

	want := wantDenySysProcAttr()
	if !reflect.DeepEqual(cmd.SysProcAttr, want) {
		t.Fatalf("cmd.SysProcAttr = %#v, want %#v (exact tuple, design §4)", cmd.SysProcAttr, want)
	}
	if cmd.SysProcAttr.Credential != nil {
		t.Fatalf("cmd.SysProcAttr.Credential = %#v, want nil (design §4)", cmd.SysProcAttr.Credential)
	}
	if cmd.ExtraFiles != nil {
		t.Fatalf("cmd.ExtraFiles = %#v, want nil (design §2)", cmd.ExtraFiles)
	}
}

// --- allow leg: nil SysProcAttr on Linux too (test area c) ---

func TestProfileCommand_Allow_NilSysProcAttr_Linux(t *testing.T) {
	argv0 := allowedArgv0(t)
	p := Profile{
		AllowedArgv0s: []string{argv0},
		network:       NetworkEnforcement{Mode: NetworkAllow, Configured: true, Reason: "allow"},
	}
	cmd, _, cancel, err := p.Command(context.Background(), argv0)
	if err != nil {
		t.Fatalf("Command: %v", err)
	}
	defer cancel()
	if cmd.SysProcAttr != nil {
		t.Fatalf("cmd.SysProcAttr = %#v, want nil for explicit ambient allow", cmd.SysProcAttr)
	}
	if cmd.ExtraFiles != nil {
		t.Fatalf("cmd.ExtraFiles = %#v, want nil", cmd.ExtraFiles)
	}
}

// --- malformed refusal on Linux too (test area e) ---

func TestProfileCommand_Deny_Unconfigured_RefusesOnLinuxToo(t *testing.T) {
	argv0 := allowedArgv0(t)
	p := Profile{
		AllowedArgv0s: []string{argv0},
		network:       NetworkEnforcement{Mode: NetworkDeny, Configured: false, Reason: "malformed"},
	}
	cmd, ctx, cancel, err := p.Command(context.Background(), argv0)
	if err == nil {
		if cancel != nil {
			cancel()
		}
		t.Fatalf("Command: want error, got nil (cmd=%v)", cmd)
	}
	var opErr *OperationalError
	if !errors.As(err, &opErr) {
		t.Fatalf("Command: want *OperationalError, got %T: %v", err, err)
	}
	if cmd != nil || ctx != nil || cancel != nil {
		t.Fatalf("Command: refused but returned cmd=%v ctx=%v cancel!=nil=%v", cmd, ctx, cancel != nil)
	}
}

// --- protected exec.Cmd fields exactly as constructed, deny leg (test
// area f; the allow leg's equivalent lives in network_test.go) ---

func TestProfileCommand_Deny_ProtectedFieldsExactlyAsConstructed(t *testing.T) {
	dir := t.TempDir()
	argv0 := filepath.Join(dir, "tool") // absolute: no PATH lookup involved
	envRoot := t.TempDir()
	set := GrantSet{Grants: []Grant{
		{Kind: GrantProcessExecution, Argv0s: []string{argv0}},
	}}
	profile, report, err := BuildProfile(dir, envRoot, set, map[string]string{"FOO": "bar"})
	if err != nil {
		t.Fatalf("BuildProfile: %v", err)
	}
	if report.Network.Mode != NetworkDeny || !report.Network.Configured {
		t.Fatalf("report.Network = %+v, want configured deny", report.Network)
	}

	cmd, _, cancel, err := profile.Command(context.Background(), argv0, "--flag")
	if err != nil {
		t.Fatalf("Command: %v", err)
	}
	defer cancel()

	if cmd.Path != argv0 {
		t.Fatalf("cmd.Path = %q, want %q", cmd.Path, argv0)
	}
	wantArgs := []string{argv0, "--flag"}
	if !reflect.DeepEqual(cmd.Args, wantArgs) {
		t.Fatalf("cmd.Args = %#v, want %#v", cmd.Args, wantArgs)
	}
	if !reflect.DeepEqual(cmd.Env, profile.Env()) {
		t.Fatalf("cmd.Env = %#v, want profile.Env()", cmd.Env)
	}
	want := wantDenySysProcAttr()
	if !reflect.DeepEqual(cmd.SysProcAttr, want) {
		t.Fatalf("cmd.SysProcAttr = %#v, want %#v", cmd.SysProcAttr, want)
	}
	if cmd.ExtraFiles != nil {
		t.Fatalf("cmd.ExtraFiles = %#v, want nil", cmd.ExtraFiles)
	}
	if cmd.Dir != "" {
		t.Fatalf("cmd.Dir = %q, want empty: this seam never sets it", cmd.Dir)
	}
}
