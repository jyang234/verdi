package execworkspace

import (
	"context"
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
	// Explicit ambient network allow (never network-absent-deny): this test
	// is about Env() determinism, unrelated to network semantics, and an
	// explicit allow is the one grant set shape BuildProfile can construct
	// on every platform, including this darwin (unsupported-platform) dev
	// machine (lane contract task i).
	grants := GrantSet{Grants: []Grant{{Kind: GrantNetwork}}}
	declared := map[string]string{"FOO": "bar", "PATH": "/usr/bin"}

	p1, _, err := BuildProfile(dir, dir, grants, declared)
	if err != nil {
		t.Fatalf("BuildProfile: %v", err)
	}
	p2, _, err := BuildProfile(dir, dir, grants, declared)
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
	workspacePath := t.TempDir()
	dir := t.TempDir() // envRoot: deliberately NOT the workspace path (AD-13)
	// Explicit ambient network allow: constructible on every platform (lane
	// contract task i); this test is about Env() contents, not network.
	p, _, err := BuildProfile(workspacePath, dir, GrantSet{Grants: []Grant{{Kind: GrantNetwork}}}, map[string]string{"FOO": "bar"})
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
	// Explicit ambient network allow: constructible on every platform (lane
	// contract task i); this test is about env leakage, not network.
	p, _, err := BuildProfile(dir, dir, GrantSet{Grants: []Grant{{Kind: GrantNetwork}}}, map[string]string{"FOO": "bar"})
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
			_, _, err := BuildProfile(dir, dir, GrantSet{}, map[string]string{key: "smuggled"})
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
			_, _, err := BuildProfile(dir, dir, GrantSet{}, map[string]string{key: "v"})
			if err == nil {
				t.Fatalf("BuildProfile(%s): want error, got nil", name)
			}
		})
	}
}

func TestBuildProfile_CreatesDirs(t *testing.T) {
	dir := t.TempDir() // envRoot: the four dirs are created under it, not under the workspace
	// Explicit ambient network allow: constructible on every platform (lane
	// contract task i); this test is about directory creation, not network.
	_, _, err := BuildProfile(t.TempDir(), dir, GrantSet{Grants: []Grant{{Kind: GrantNetwork}}}, nil)
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
	// Explicit ambient network allow alongside timeouts: constructible on
	// every platform (lane contract task i); this test is about Timeout
	// wiring, not network. Adding it means the report also gains a network
	// row (kind-ordered ahead of timeouts), so the row assertion below
	// looks up by kind rather than assuming a single-row report.
	set := GrantSet{Grants: []Grant{{Kind: GrantNetwork}, {Kind: GrantTimeouts, Seconds: 90}}}
	p, report, err := BuildProfile(dir, dir, set, nil)
	if err != nil {
		t.Fatalf("BuildProfile: %v", err)
	}
	if p.Timeout != 90*time.Second {
		t.Fatalf("Timeout = %v, want 90s", p.Timeout)
	}
	if len(report.Rows) != 2 {
		t.Fatalf("report.Rows = %+v, want 2 rows (network, timeouts)", report.Rows)
	}
	byKind := map[GrantKind]EnforcementReportRow{}
	for _, row := range report.Rows {
		byKind[row.Kind] = row
	}
	if !byKind[GrantTimeouts].Applied || byKind[GrantTimeouts].Kind != GrantTimeouts {
		t.Fatalf("report = %+v, want an applied timeouts row", report.Rows)
	}
	if !byKind[GrantNetwork].Applied {
		t.Fatalf("report = %+v, want an applied network row", report.Rows)
	}
}

func TestBuildProfile_NoTimeoutGrantLeavesZeroTimeout(t *testing.T) {
	dir := t.TempDir()
	// Explicit ambient network allow: constructible on every platform (lane
	// contract task i); this test is about the absence of a timeouts
	// grant, not network.
	p, _, err := BuildProfile(dir, dir, GrantSet{Grants: []Grant{{Kind: GrantNetwork}}}, nil)
	if err != nil {
		t.Fatalf("BuildProfile: %v", err)
	}
	if p.Timeout != 0 {
		t.Fatalf("Timeout = %v, want 0", p.Timeout)
	}
}

func TestBuildProfile_ProcessExecutionAllowlistSortedCopy(t *testing.T) {
	dir := t.TempDir()
	// Explicit ambient network allow alongside process-execution:
	// constructible on every platform (lane contract task i); this test is
	// about the AllowedArgv0s sorted copy, not network.
	set := GrantSet{Grants: []Grant{
		{Kind: GrantNetwork},
		{Kind: GrantProcessExecution, Argv0s: []string{"git", "go"}},
	}}
	p, _, err := BuildProfile(dir, dir, set, nil)
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
	// Explicit ambient network allow: constructible on every platform (lane
	// contract task i); this test is about the absence of a
	// process-execution grant, not network.
	p, _, err := BuildProfile(dir, dir, GrantSet{Grants: []Grant{{Kind: GrantNetwork}}}, nil)
	if err != nil {
		t.Fatalf("BuildProfile: %v", err)
	}
	if len(p.AllowedArgv0s) != 0 {
		t.Fatalf("AllowedArgv0s = %v, want empty (fact of absence, not an enforced empty allowlist)", p.AllowedArgv0s)
	}
}

// TestBuildProfile_NetworkAllowOnlyGrantSet_NoErrorOneRow is the migrated
// form of what used to be a literally EMPTY grant set (lane contract task
// i): since SI-75/SI-76, an empty GrantSet means an ABSENT network grant,
// which is a mandatory control this darwin (unsupported-platform) dev
// machine cannot configure — see
// TestBuildProfile_NetworkPolarity_AbsentGrant_EmptyGrantSet
// (network_unsupported_test.go) for that behavior. The smallest grant set
// BuildProfile can still construct successfully on every platform is
// "network allow and nothing else", which is what this test now pins:
// exactly one (network) row, no error.
func TestBuildProfile_NetworkAllowOnlyGrantSet_NoErrorOneRow(t *testing.T) {
	dir := t.TempDir()
	set := GrantSet{Grants: []Grant{{Kind: GrantNetwork}}}
	_, report, err := BuildProfile(dir, dir, set, nil)
	if err != nil {
		t.Fatalf("BuildProfile: unexpected error: %v", err)
	}
	if report == nil || len(report.Rows) != 1 {
		t.Fatalf("report = %+v, want non-nil report with exactly one (network) row", report)
	}
	if !report.Rows[0].Applied || report.Rows[0].Kind != GrantNetwork {
		t.Fatalf("report.Rows[0] = %+v, want an applied network row", report.Rows[0])
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
	_, report, err := BuildProfile(dir, dir, set, nil)
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
	// SI-75/SI-76: a PRESENT network grant is an explicit ambient allow,
	// always applied — it is no longer among AD-9's could-not-apply kinds.
	if !byKind[GrantNetwork].Applied {
		t.Fatalf("network row Applied = false, want true: a present grant is an explicit ambient allow")
	}
	if byKind[GrantNetwork].Reason == "" {
		t.Fatalf("network row Reason is empty, want it to name the explicit ambient permission")
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
	_, report, err := BuildProfile(dir, dir, set, nil)
	if err == nil {
		t.Fatalf("BuildProfile: want error, got nil")
	}
	var opErr *OperationalError
	if !errors.As(err, &opErr) {
		t.Fatalf("error is not *OperationalError: %v (%T)", err, err)
	}
	msg := err.Error()
	for _, name := range []string{"path-read", "path-write", "resource-ceilings"} {
		if !strings.Contains(msg, name) {
			t.Fatalf("error %q does not name unapplied kind %q", msg, name)
		}
	}
	if strings.Contains(msg, "process-execution") {
		t.Fatalf("error %q names process-execution, which WAS applied and must not be listed as unapplied", msg)
	}
	// SI-75/SI-76: the present network grant in this set is an explicit
	// ambient allow, always applied — it must not be listed as unapplied
	// either, exactly like process-execution above.
	if strings.Contains(msg, "network") {
		t.Fatalf("error %q names network, which WAS applied (explicit ambient allow) and must not be listed as unapplied", msg)
	}
	if report == nil || len(report.Rows) != 5 {
		t.Fatalf("report = %+v, want all 5 grant rows still returned alongside the error", report)
	}
}

func TestBuildProfile_RejectsInvalidGrantSet(t *testing.T) {
	dir := t.TempDir()
	set := GrantSet{Grants: []Grant{{Kind: GrantNetwork}, {Kind: GrantNetwork}}}
	if _, _, err := BuildProfile(dir, dir, set, nil); err == nil {
		t.Fatalf("BuildProfile: want error for invalid (duplicate-kind) grant set, got nil")
	}
}

func TestBuildProfile_RejectsEmptyWorkspacePath(t *testing.T) {
	if _, _, err := BuildProfile("", "", GrantSet{}, nil); err == nil {
		t.Fatalf("BuildProfile: want error for empty workspace path, got nil")
	}
}

// TestBuildProfile_RejectsNULInDeclaredEnvValue pins the value side of the
// declared-env contract. Env() renders "KEY=VALUE" entries destined for
// exec.Cmd.Env, where the OS truncates each entry at the first NUL: a value
// carrying one silently ships a DIFFERENT environment to the launched
// process than the profile reports, so it must fail closed exactly as an
// invalid key does.
func TestBuildProfile_RejectsNULInDeclaredEnvValue(t *testing.T) {
	cases := map[string]struct {
		value   string
		wantErr bool
	}{
		"clean value accepted":  {value: "ok", wantErr: false},
		"empty value accepted":  {value: "", wantErr: false},
		"embedded NUL rejected": {value: "has\x00nul", wantErr: true},
		"trailing NUL rejected": {value: "trail\x00", wantErr: true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			// Explicit ambient network allow: constructible on every
			// platform (lane contract task i) so the "accepted" cases
			// below reach err == nil here too; the "rejected" cases still
			// fail at the earlier NUL check regardless of the grant set.
			profile, _, err := BuildProfile(dir, dir, GrantSet{Grants: []Grant{{Kind: GrantNetwork}}}, map[string]string{"DECLARED": tc.value})
			if tc.wantErr && err == nil {
				t.Fatalf("BuildProfile(value %q) = env %v, want error", tc.value, profile.Env())
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("BuildProfile(value %q) = %v, want nil", tc.value, err)
			}
		})
	}
}

// TestBuildProfile_AD5_ErrorRenderingIsNotMislabelledAsMaterialization pins
// the rendered text of AD-5's operational error. OperationalError is this
// package's one shared retryable-failure type; its Error() must qualify the
// failure with the Op the caller supplied, not hard-code a "materialize:"
// prefix that mislabels an isolation-profile failure as a materialization
// one.
func TestBuildProfile_AD5_ErrorRenderingIsNotMislabelledAsMaterialization(t *testing.T) {
	dir := t.TempDir()
	// GrantPathRead, not GrantNetwork: this test pins the generic AD-5
	// Op-prefix rendering, unrelated to network semantics. Since SI-75/
	// SI-76 a present network grant is always applied (never
	// could-not-apply), so it no longer reaches this error path on its
	// own — path-read still has no v0 mechanism and keeps this test
	// meaningful (lane contract task i).
	set := GrantSet{Grants: []Grant{{Kind: GrantPathRead, Paths: []string{"src"}}}}
	_, _, err := BuildProfile(dir, dir, set, nil)
	if err == nil {
		t.Fatalf("BuildProfile: want error, got nil")
	}
	const wantPrefix = "execworkspace: isolation-profile: apply-grants: "
	if !strings.HasPrefix(err.Error(), wantPrefix) {
		t.Fatalf("error = %q, want prefix %q", err.Error(), wantPrefix)
	}
	if strings.Contains(err.Error(), "materialize") {
		t.Fatalf("error = %q, must not mention materialization: this failure is isolation-profile construction", err.Error())
	}
}

// TestOperationalError_RenderingIsOpQualified pins Error()'s generalized
// shape directly, for both the materialization Ops (which keep their exact
// historical rendering via a "materialize: " Op prefix) and a non-
// materialization Op.
func TestOperationalError_RenderingIsOpQualified(t *testing.T) {
	cases := map[string]struct {
		op   string
		want string
	}{
		"materialization op renders exactly as before": {
			op:   "materialize: acquire lock",
			want: "execworkspace: materialize: acquire lock: boom",
		},
		"isolation op is not mislabelled": {
			op:   "isolation-profile: apply-grants",
			want: "execworkspace: isolation-profile: apply-grants: boom",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := operationalError(tc.op, errors.New("boom"))
			if got := err.Error(); got != tc.want {
				t.Fatalf("Error() = %q, want %q", got, tc.want)
			}
			var target *OperationalError
			if !errors.As(error(err), &target) || target.Op != tc.op {
				t.Fatalf("errors.As did not reach the typed value with Op %q", tc.op)
			}
		})
	}
}

// --- envRoot composition (whole-wave finding F2, controller decision AD-13) ---

func TestBuildProfile_RejectsEmptyEnvRoot(t *testing.T) {
	if _, _, err := BuildProfile(t.TempDir(), "", GrantSet{}, nil); err == nil {
		t.Fatalf("BuildProfile: want error for an empty env root (no silent default), got nil")
	}
}

// TestBuildProfile_RejectsRelativeRoots pins the absoluteness half of AD-13's
// required-caller-choice contract. A relative root resolves against the
// CALLING PROCESS's working directory, which this package neither chose nor
// controls: a relative envRoot would create the four profile-owned
// directories wherever that process happens to be standing and emit
// cwd-relative HOME/XDG_CONFIG_HOME/XDG_CACHE_HOME/TMPDIR values whose
// meaning silently changes if anything chdirs before the consumer launches.
// workspacePath carries the same looseness and feeds the same profile-owned
// decision (it is the unit territory envRoot is chosen relative to), so both
// roots must be absolute or the call fails closed — the same posture as the
// empty check beside it, never a silent resolution against an unowned cwd.
//
// The subtest chdirs into its own temp directory so that a build which
// wrongly SUCCEEDS pollutes only that directory, and so the
// nothing-was-created assertion below is exact.
func TestBuildProfile_RejectsRelativeRoots(t *testing.T) {
	cases := map[string]struct {
		workspacePath string
		envRoot       string
	}{
		"relative env root":        {envRoot: "relprobe-envroot"},
		"dot-relative env root":    {envRoot: filepath.Join(".", "relprobe-dotted")},
		"relative workspace path":  {workspacePath: "relprobe-workspace"},
		"both roots relative":      {workspacePath: "relprobe-ws", envRoot: "relprobe-env"},
		"parent-relative env root": {envRoot: filepath.Join("..", "relprobe-parent")},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cwd := t.TempDir()
			t.Chdir(cwd)

			workspacePath, envRoot := tc.workspacePath, tc.envRoot
			if workspacePath == "" {
				workspacePath = t.TempDir()
			}
			if envRoot == "" {
				envRoot = t.TempDir()
			}

			_, _, err := BuildProfile(workspacePath, envRoot, GrantSet{}, nil)
			if err == nil {
				t.Fatalf("BuildProfile(%q, %q): want a fail-closed error for a non-absolute root, got nil", workspacePath, envRoot)
			}
			if !strings.Contains(err.Error(), "AD-13") {
				t.Fatalf("error = %q, want the error to name AD-13", err.Error())
			}

			entries, readErr := os.ReadDir(cwd)
			if readErr != nil {
				t.Fatalf("ReadDir(cwd): %v", readErr)
			}
			if len(entries) != 0 {
				names := make([]string, 0, len(entries))
				for _, e := range entries {
					names = append(names, e.Name())
				}
				t.Fatalf("cwd contains %v, want nothing: a rejected root must never create profile-owned directories under the calling process's cwd", names)
			}
		})
	}
}

// TestBuildProfile_AcceptsAbsoluteRoots is the positive half of the pair
// above: absolute roots remain accepted, so the new check rejects only the
// non-absolute case and does not narrow BuildProfile's contract further.
func TestBuildProfile_AcceptsAbsoluteRoots(t *testing.T) {
	// Explicit ambient network allow: constructible on every platform (lane
	// contract task i); this test is about absolute-root acceptance, not
	// network.
	set := GrantSet{Grants: []Grant{{Kind: GrantNetwork}}}
	if _, _, err := BuildProfile(t.TempDir(), t.TempDir(), set, nil); err != nil {
		t.Fatalf("BuildProfile(absolute, absolute) = %v, want nil", err)
	}
}

// TestBuildProfile_EnvRootSeparateFromWorkspacePath proves the four
// profile-owned directories are anchored on envRoot, NOT on workspacePath:
// that separation is the whole content of AD-13.
func TestBuildProfile_EnvRootSeparateFromWorkspacePath(t *testing.T) {
	workspacePath := t.TempDir()
	envRoot := t.TempDir()
	// Explicit ambient network allow: constructible on every platform (lane
	// contract task i); this test is about envRoot/workspacePath
	// separation, not network.
	p, _, err := BuildProfile(workspacePath, envRoot, GrantSet{Grants: []Grant{{Kind: GrantNetwork}}}, nil)
	if err != nil {
		t.Fatalf("BuildProfile: %v", err)
	}
	want := map[string]string{
		"HOME":            filepath.Join(envRoot, ".home"),
		"XDG_CONFIG_HOME": filepath.Join(envRoot, ".home", ".config"),
		"XDG_CACHE_HOME":  filepath.Join(envRoot, ".home", ".cache"),
		"TMPDIR":          filepath.Join(envRoot, ".tmp"),
	}
	got := map[string]string{}
	for _, kv := range p.Env() {
		parts := strings.SplitN(kv, "=", 2)
		got[parts[0]] = parts[1]
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("Env()[%q] = %q, want %q (anchored on envRoot)", k, got[k], v)
		}
		if info, serr := os.Stat(v); serr != nil || !info.IsDir() {
			t.Fatalf("profile-owned dir %q not created under envRoot: err=%v", v, serr)
		}
	}
	for _, sub := range []string{".home", ".tmp"} {
		if _, serr := os.Stat(filepath.Join(workspacePath, sub)); !os.IsNotExist(serr) {
			t.Fatalf("BuildProfile created %q inside the workspace path; envRoot must be the sole anchor (err=%v)", sub, serr)
		}
	}
}

// TestBuildProfile_EnvRootComposition_GcConvergence is the end-to-end proof of
// the composition property AD-13 requires BuildProfile's doc comment to
// disclose, pinned BOTH WAYS against the real production seam (Materializer +
// Releaser + GC):
//
//   - envRoot OUTSIDE the unit path: the workspace stays clean, so a released
//     unit converges through gc — reclaimed, then reclaim-orphaned.
//   - envRoot INSIDE the unit path: the profile's own .home/.tmp become
//     untracked content in the worktree, so gc keeps at rank 3 FOREVER. That
//     is the fail-closed behavior by design, not a bug — but it means a
//     consumer that wants a reclaimable workspace must place envRoot outside
//     the unit, which is exactly what the doc comment must say.
//
// The inside case is an ORDINARY keep-dirty (empty Detail), not finding F1's
// unevaluable keep: the unit here IS a real linked worktree, so the dirty
// predicate is genuinely evaluable and genuinely TRUE.
func TestBuildProfile_EnvRootComposition_GcConvergence(t *testing.T) {
	newReleasedWorkspace := func(t *testing.T, runID string) (storeRoot, repoDir, workspaceID, unitPath string) {
		t.Helper()
		repo := buildTestRepo(t)
		storeRoot = t.TempDir()
		m, err := NewMaterializer(storeRoot, repo.Dir, NewGitReconciler(storeRoot))
		if err != nil {
			t.Fatalf("NewMaterializer: %v", err)
		}
		id, ierr := NewExactIdentity(runID, repo.Head)
		if ierr != nil {
			t.Fatalf("NewExactIdentity: %v", ierr)
		}
		res, merr := m.Materialize(context.Background(), Request{Identity: id})
		if merr != nil {
			t.Fatalf("Materialize: %v", merr)
		}
		return storeRoot, repo.Dir, res.WorkspaceID, res.Path
	}

	t.Run("env root outside the unit path converges through gc", func(t *testing.T) {
		storeRoot, repoDir, workspaceID, unitPath := newReleasedWorkspace(t, "run-envroot-outside")
		envRoot := t.TempDir() // the consumer's own lifecycle territory
		// Explicit ambient network allow: constructible on every platform
		// (lane contract task i); this test is about gc convergence, not
		// network.
		p, _, err := BuildProfile(unitPath, envRoot, GrantSet{Grants: []Grant{{Kind: GrantNetwork}}}, nil)
		if err != nil {
			t.Fatalf("BuildProfile: %v", err)
		}
		home := filepath.Join(envRoot, ".home")
		if werr := os.WriteFile(filepath.Join(home, "state.json"), []byte("{}\n"), 0o644); werr != nil {
			t.Fatalf("writing profile-home state: %v", werr)
		}
		if !strings.Contains(strings.Join(p.Env(), " "), home) {
			t.Fatalf("Env() = %v, want HOME under %s", p.Env(), home)
		}

		if rerr := NewReleaser(storeRoot).Release(workspaceID); rerr != nil {
			t.Fatalf("Release: %v", rerr)
		}
		results1, _, gerr := GC(context.Background(), storeRoot, repoDir)
		if gerr != nil {
			t.Fatalf("GC run 1: %v", gerr)
		}
		if r := mustFindResult(t, results1, workspaceID); r.Outcome != Reclaimed {
			t.Fatalf("gc run 1 Outcome = %v (detail=%q), want Reclaimed — an envRoot outside the unit must leave the workspace reclaimable", r.Outcome, r.Detail)
		}
		results2, _, gerr := GC(context.Background(), storeRoot, repoDir)
		if gerr != nil {
			t.Fatalf("GC run 2: %v", gerr)
		}
		if r := mustFindResult(t, results2, workspaceID); r.Outcome != ReclaimOrphaned {
			t.Fatalf("gc run 2 Outcome = %v (detail=%q), want ReclaimOrphaned (converged)", r.Outcome, r.Detail)
		}
		// The consumer's own env root is untouched by gc: it was never inside
		// the unit, so it is that consumer's lifecycle to clean up.
		if _, serr := os.Stat(filepath.Join(home, "state.json")); serr != nil {
			t.Fatalf("gc deleted state under the caller-chosen envRoot: %v", serr)
		}
	})

	t.Run("env root inside the unit path keeps dirty by design", func(t *testing.T) {
		storeRoot, repoDir, workspaceID, unitPath := newReleasedWorkspace(t, "run-envroot-inside")
		// Explicit ambient network allow: constructible on every platform
		// (lane contract task i); this test is about gc convergence, not
		// network.
		if _, _, err := BuildProfile(unitPath, unitPath, GrantSet{Grants: []Grant{{Kind: GrantNetwork}}}, nil); err != nil {
			t.Fatalf("BuildProfile: %v", err)
		}
		if werr := os.WriteFile(filepath.Join(unitPath, ".home", "state.json"), []byte("{}\n"), 0o644); werr != nil {
			t.Fatalf("writing profile-home state: %v", werr)
		}
		if rerr := NewReleaser(storeRoot).Release(workspaceID); rerr != nil {
			t.Fatalf("Release: %v", rerr)
		}
		results, _, gerr := GC(context.Background(), storeRoot, repoDir)
		if gerr != nil {
			t.Fatalf("GC: %v", gerr)
		}
		r := mustFindResult(t, results, workspaceID)
		if r.Outcome != KeepDirty {
			t.Fatalf("Outcome = %v (detail=%q), want KeepDirty (profile env dirs inside the unit make it permanently keep-dirty by design)", r.Outcome, r.Detail)
		}
		if r.Detail != "" {
			t.Fatalf("Detail = %q, want empty: this is the ORDINARY rank-3 dirty keep (a real linked worktree, predicate evaluable and true), not the unevaluable-predicate keep", r.Detail)
		}
		if _, serr := os.Stat(unitPath); serr != nil {
			t.Fatalf("kept unit removed: %v", serr)
		}
	})
}
