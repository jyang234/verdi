package execworkspace

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestBuildProfile_EnvDeterministic(t *testing.T) {
	dir := t.TempDir()
	grants := GrantSet{}
	declared := map[string]string{"FOO": "bar", "PATH": "/usr/bin"}

	p1, _, err := BuildProfile(dir, grants, declared)
	if err != nil {
		t.Fatalf("BuildProfile: %v", err)
	}
	p2, _, err := BuildProfile(dir, grants, declared)
	if err != nil {
		t.Fatalf("BuildProfile: %v", err)
	}

	e1, e2 := p1.Env(), p2.Env()
	if len(e1) != len(e2) {
		t.Fatalf("Env length differs: %v vs %v", e1, e2)
	}
	for i := range e1 {
		if e1[i] != e2[i] {
			t.Fatalf("Env not deterministic at index %d: %q vs %q", i, e1[i], e2[i])
		}
	}
	if !sort.StringsAreSorted(e1) {
		t.Fatalf("Env() = %v, want sorted", e1)
	}
}

func TestBuildProfile_ExactEnvContents(t *testing.T) {
	dir := t.TempDir()
	p, _, err := BuildProfile(dir, GrantSet{}, map[string]string{"FOO": "bar"})
	if err != nil {
		t.Fatalf("BuildProfile: %v", err)
	}
	want := map[string]string{
		"HOME":            filepath.Join(dir, ".home"),
		"XDG_CONFIG_HOME": filepath.Join(dir, ".home", ".config"),
		"XDG_CACHE_HOME":  filepath.Join(dir, ".home", ".cache"),
		"TMPDIR":          filepath.Join(dir, ".tmp"),
		"FOO":             "bar",
	}
	env := p.Env()
	if len(env) != len(want) {
		t.Fatalf("Env() = %v, want %d entries matching %v", env, len(want), want)
	}
	got := map[string]string{}
	for _, kv := range env {
		parts := strings.SplitN(kv, "=", 2)
		got[parts[0]] = parts[1]
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("Env()[%q] = %q, want %q", k, got[k], v)
		}
	}
}

func TestBuildProfile_NoProcessEnvLeakage(t *testing.T) {
	t.Setenv("VERDI_TEST_LEAK_PROBE", "should-never-appear")
	dir := t.TempDir()
	p, _, err := BuildProfile(dir, GrantSet{}, map[string]string{"FOO": "bar"})
	if err != nil {
		t.Fatalf("BuildProfile: %v", err)
	}
	for _, kv := range p.Env() {
		if strings.HasPrefix(kv, "VERDI_TEST_LEAK_PROBE=") {
			t.Fatalf("Env() leaked the real process environment: %v", p.Env())
		}
		if strings.Contains(kv, "should-never-appear") {
			t.Fatalf("Env() leaked the real process environment's value: %v", p.Env())
		}
	}
	// PATH must not silently inherit from the real process either, since it
	// was never declared.
	for _, kv := range p.Env() {
		if strings.HasPrefix(kv, "PATH=") {
			t.Fatalf("Env() set PATH without it being declared: %v", p.Env())
		}
	}
}

func TestBuildProfile_RejectsCollidingDeclaredEnvKey(t *testing.T) {
	dir := t.TempDir()
	for _, key := range []string{"HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "TMPDIR"} {
		t.Run(key, func(t *testing.T) {
			_, _, err := BuildProfile(dir, GrantSet{}, map[string]string{key: "smuggled"})
			if err == nil {
				t.Fatalf("BuildProfile: want error for declared env key %q colliding with a profile-owned key", key)
			}
		})
	}
}

func TestBuildProfile_RejectsInvalidDeclaredEnvKey(t *testing.T) {
	cases := map[string]string{
		"empty key":  "",
		"has equals": "FOO=BAR",
		"has NUL":    "FOO\x00BAR",
	}
	for name, key := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			_, _, err := BuildProfile(dir, GrantSet{}, map[string]string{key: "v"})
			if err == nil {
				t.Fatalf("BuildProfile(%s): want error, got nil", name)
			}
		})
	}
}

func TestBuildProfile_CreatesDirs(t *testing.T) {
	dir := t.TempDir()
	_, _, err := BuildProfile(dir, GrantSet{}, nil)
	if err != nil {
		t.Fatalf("BuildProfile: %v", err)
	}
	for _, sub := range []string{
		filepath.Join(dir, ".home"),
		filepath.Join(dir, ".home", ".config"),
		filepath.Join(dir, ".home", ".cache"),
		filepath.Join(dir, ".tmp"),
	} {
		info, err := os.Stat(sub)
		if err != nil {
			t.Fatalf("stat %q: %v", sub, err)
		}
		if !info.IsDir() {
			t.Fatalf("%q is not a directory", sub)
		}
	}
}

func TestBuildProfile_TimeoutWired(t *testing.T) {
	dir := t.TempDir()
	set := GrantSet{Grants: []Grant{{Kind: GrantTimeouts, Seconds: 90}}}
	p, report, err := BuildProfile(dir, set, nil)
	if err != nil {
		t.Fatalf("BuildProfile: %v", err)
	}
	if p.Timeout != 90*time.Second {
		t.Fatalf("Timeout = %v, want 90s", p.Timeout)
	}
	if len(report.Rows) != 1 || !report.Rows[0].Applied || report.Rows[0].Kind != GrantTimeouts {
		t.Fatalf("report = %+v, want one applied timeouts row", report.Rows)
	}
}

func TestBuildProfile_NoTimeoutGrantLeavesZeroTimeout(t *testing.T) {
	dir := t.TempDir()
	p, _, err := BuildProfile(dir, GrantSet{}, nil)
	if err != nil {
		t.Fatalf("BuildProfile: %v", err)
	}
	if p.Timeout != 0 {
		t.Fatalf("Timeout = %v, want 0", p.Timeout)
	}
}

func TestBuildProfile_ProcessExecutionAllowlistSortedCopy(t *testing.T) {
	dir := t.TempDir()
	set := GrantSet{Grants: []Grant{{Kind: GrantProcessExecution, Argv0s: []string{"git", "go"}}}}
	p, _, err := BuildProfile(dir, set, nil)
	if err != nil {
		t.Fatalf("BuildProfile: %v", err)
	}
	want := []string{"git", "go"}
	if len(p.AllowedArgv0s) != len(want) {
		t.Fatalf("AllowedArgv0s = %v, want %v", p.AllowedArgv0s, want)
	}
	for i := range want {
		if p.AllowedArgv0s[i] != want[i] {
			t.Fatalf("AllowedArgv0s = %v, want %v", p.AllowedArgv0s, want)
		}
	}
}

func TestBuildProfile_NoProcessExecutionGrantLeavesNilAllowlist(t *testing.T) {
	dir := t.TempDir()
	p, _, err := BuildProfile(dir, GrantSet{}, nil)
	if err != nil {
		t.Fatalf("BuildProfile: %v", err)
	}
	if len(p.AllowedArgv0s) != 0 {
		t.Fatalf("AllowedArgv0s = %v, want empty (fact of absence, not an enforced empty allowlist)", p.AllowedArgv0s)
	}
}

func TestBuildProfile_EmptyGrantSet_NoErrorEmptyReport(t *testing.T) {
	dir := t.TempDir()
	_, report, err := BuildProfile(dir, GrantSet{}, nil)
	if err != nil {
		t.Fatalf("BuildProfile: unexpected error: %v", err)
	}
	if report == nil || len(report.Rows) != 0 {
		t.Fatalf("report = %+v, want non-nil empty report", report)
	}
}

func TestBuildProfile_ReportRowsExactForMixedGrantSet(t *testing.T) {
	dir := t.TempDir()
	set := GrantSet{Grants: []Grant{
		{Kind: GrantTimeouts, Seconds: 5},
		{Kind: GrantNetwork},
		{Kind: GrantProcessExecution, Argv0s: []string{"go"}},
		{Kind: GrantPathRead, Paths: []string{"src"}},
	}}
	_, report, err := BuildProfile(dir, set, nil)
	if err == nil {
		t.Fatalf("BuildProfile: want AD-5 error for could-not-apply rows, got nil")
	}
	if report == nil {
		t.Fatalf("BuildProfile: want report returned alongside the error, got nil")
	}
	if len(report.Rows) != 4 {
		t.Fatalf("report.Rows = %+v, want 4 rows (network, path-read, process-execution, timeouts)", report.Rows)
	}
	// Deterministic KIND order (GrantNetwork < GrantPathRead <
	// GrantProcessExecution < GrantTimeouts by declaration order), never
	// grant-list input order (which here put timeouts first).
	wantOrder := []GrantKind{GrantNetwork, GrantPathRead, GrantProcessExecution, GrantTimeouts}
	for i, row := range report.Rows {
		if row.Kind != wantOrder[i] {
			t.Fatalf("report.Rows[%d].Kind = %v, want %v (rows: %+v)", i, row.Kind, wantOrder[i], report.Rows)
		}
	}
	byKind := map[GrantKind]EnforcementReportRow{}
	for _, row := range report.Rows {
		byKind[row.Kind] = row
	}
	if byKind[GrantNetwork].Applied {
		t.Fatalf("network row Applied = true, want false")
	}
	if byKind[GrantNetwork].Reason == "" {
		t.Fatalf("network row Reason is empty, want it to name the missing mechanism")
	}
	if byKind[GrantPathRead].Applied {
		t.Fatalf("path-read row Applied = true, want false")
	}
	if !byKind[GrantProcessExecution].Applied {
		t.Fatalf("process-execution row Applied = false, want true")
	}
	if !byKind[GrantTimeouts].Applied {
		t.Fatalf("timeouts row Applied = false, want true")
	}
}

func TestBuildProfile_AD5_OperationalErrorNamesExactlyUnappliedKinds(t *testing.T) {
	dir := t.TempDir()
	set := GrantSet{Grants: []Grant{
		{Kind: GrantNetwork},
		{Kind: GrantPathRead, Paths: []string{"a"}},
		{Kind: GrantPathWrite, Paths: []string{"b"}},
		{Kind: GrantResourceCeilings, Ceilings: map[string]int{"cpu": 1}},
		{Kind: GrantProcessExecution, Argv0s: []string{"go"}},
	}}
	_, report, err := BuildProfile(dir, set, nil)
	if err == nil {
		t.Fatalf("BuildProfile: want error, got nil")
	}
	var opErr *OperationalError
	if !errors.As(err, &opErr) {
		t.Fatalf("error is not *OperationalError: %v (%T)", err, err)
	}
	msg := err.Error()
	for _, name := range []string{"network", "path-read", "path-write", "resource-ceilings"} {
		if !strings.Contains(msg, name) {
			t.Fatalf("error %q does not name unapplied kind %q", msg, name)
		}
	}
	if strings.Contains(msg, "process-execution") {
		t.Fatalf("error %q names process-execution, which WAS applied and must not be listed as unapplied", msg)
	}
	if report == nil || len(report.Rows) != 5 {
		t.Fatalf("report = %+v, want all 5 grant rows still returned alongside the error", report)
	}
}

func TestBuildProfile_RejectsInvalidGrantSet(t *testing.T) {
	dir := t.TempDir()
	set := GrantSet{Grants: []Grant{{Kind: GrantNetwork}, {Kind: GrantNetwork}}}
	if _, _, err := BuildProfile(dir, set, nil); err == nil {
		t.Fatalf("BuildProfile: want error for invalid (duplicate-kind) grant set, got nil")
	}
}

func TestBuildProfile_RejectsEmptyWorkspacePath(t *testing.T) {
	if _, _, err := BuildProfile("", GrantSet{}, nil); err == nil {
		t.Fatalf("BuildProfile: want error for empty workspace path, got nil")
	}
}
