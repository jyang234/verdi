package execworkspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestPlanProfileIsPureAndActivationMatchesBuildProfile(t *testing.T) {
	root := t.TempDir()
	workspacePath := filepath.Join(root, "workspace")
	envRoot := filepath.Join(root, "environment")
	argv0 := filepath.Join(workspacePath, "evaluator")
	grants := GrantSet{Grants: []Grant{
		{Kind: GrantNetwork},
		{Kind: GrantProcessExecution, Argv0s: []string{argv0, "/bin/other"}},
		{Kind: GrantTimeouts, Seconds: 30},
	}}
	declaredEnv := map[string]string{"LANG": "C"}

	planned, plannedReport, err := PlanProfile(workspacePath, envRoot, grants, declaredEnv)
	if err != nil {
		t.Fatalf("PlanProfile: %v", err)
	}
	assertPathAbsent(t, workspacePath)
	assertPathAbsent(t, envRoot)

	wantEnv := []string{
		"HOME=" + filepath.Join(envRoot, ".home"),
		"LANG=C",
		"TMPDIR=" + filepath.Join(envRoot, ".tmp"),
		"XDG_CACHE_HOME=" + filepath.Join(envRoot, ".home", ".cache"),
		"XDG_CONFIG_HOME=" + filepath.Join(envRoot, ".home", ".config"),
	}
	if got := planned.Env(); !reflect.DeepEqual(got, wantEnv) {
		t.Fatalf("planned Env() = %#v, want %#v", got, wantEnv)
	}

	// Planning owns a clone of caller inputs. Mutation after receipt planning
	// must not alter either later activation or the fingerprint projection.
	grants.Grants[1].Argv0s[0] = "/mutated/evaluator"
	declaredEnv["LANG"] = "mutated"
	if got := planned.Env(); !reflect.DeepEqual(got, wantEnv) {
		t.Fatalf("planned Env() after caller mutation = %#v, want immutable %#v", got, wantEnv)
	}
	if got := planned.AllowedArgv0s; !reflect.DeepEqual(got, []string{"/bin/other", argv0}) {
		t.Fatalf("planned AllowedArgv0s after caller mutation = %#v, want cloned sorted allowlist", got)
	}

	cmd, runCtx, cancel, err := planned.Command(context.Background(), argv0)
	if err == nil {
		if cancel != nil {
			cancel()
		}
		t.Fatalf("planned Command returned cmd=%v ctx=%v, want fail-closed refusal", cmd, runCtx)
	}
	var opErr *OperationalError
	if !errors.As(err, &opErr) || !strings.Contains(err.Error(), "planned profile") {
		t.Fatalf("planned Command error = %T %q, want operational planned-profile refusal", err, err)
	}
	if cmd != nil || runCtx != nil || cancel != nil {
		t.Fatalf("planned Command refusal returned cmd=%v ctx=%v cancel!=nil=%v", cmd, runCtx, cancel != nil)
	}

	fingerprintInputs := FingerprintInputs{
		ToolVersions: map[string]string{"runtime": "go-test"},
		EnvVarNames:  []string{"LANG"},
		InputDigests: map[string]string{"fixture": "deadbeef"},
	}
	plannedFingerprint, err := CollectFingerprint(planned, fingerprintInputs)
	if err != nil {
		t.Fatalf("CollectFingerprint(planned): %v", err)
	}
	assertPathAbsent(t, envRoot)

	activated, err := planned.Activate()
	if err != nil {
		t.Fatalf("Activate: %v", err)
	}
	assertExactProfileDirectories(t, envRoot)
	if got := activated.Env(); !reflect.DeepEqual(got, wantEnv) {
		t.Fatalf("activated Env() = %#v, want planned projection %#v", got, wantEnv)
	}
	activatedFingerprint, err := CollectFingerprint(activated, fingerprintInputs)
	if err != nil {
		t.Fatalf("CollectFingerprint(activated): %v", err)
	}
	if !reflect.DeepEqual(activatedFingerprint, plannedFingerprint) {
		t.Fatalf("activated fingerprint = %q, want planned fingerprint %q", activatedFingerprint, plannedFingerprint)
	}

	cmd, runCtx, cancel, err = activated.Command(context.Background(), argv0, "--measure")
	if err != nil {
		t.Fatalf("activated Command: %v", err)
	}
	if cmd == nil || runCtx == nil || cancel == nil {
		t.Fatalf("activated Command returned cmd=%v ctx=%v cancel!=nil=%v, want launch construction", cmd, runCtx, cancel != nil)
	}
	cancel()

	// The activated projection is a clone, not mutable shared state owned by
	// the plan. A later activation of the exact plan must reproduce the receipt-
	// bound allowlist rather than a prior consumer mutation.
	activated.AllowedArgv0s[0] = "/mutated/activated"
	activatedAgain, err := planned.Activate()
	if err != nil {
		t.Fatalf("Activate again: %v", err)
	}
	if got := activatedAgain.AllowedArgv0s; !reflect.DeepEqual(got, []string{"/bin/other", argv0}) {
		t.Fatalf("second activation AllowedArgv0s = %#v, want immutable planned allowlist", got)
	}

	compatGrants := GrantSet{Grants: []Grant{
		{Kind: GrantNetwork},
		{Kind: GrantProcessExecution, Argv0s: []string{argv0, "/bin/other"}},
		{Kind: GrantTimeouts, Seconds: 30},
	}}
	built, builtReport, err := BuildProfile(workspacePath, envRoot, compatGrants, map[string]string{"LANG": "C"})
	if err != nil {
		t.Fatalf("BuildProfile: %v", err)
	}
	if !reflect.DeepEqual(builtReport, plannedReport) {
		t.Fatalf("BuildProfile report = %#v, want PlanProfile report %#v", builtReport, plannedReport)
	}
	if got := built.Env(); !reflect.DeepEqual(got, wantEnv) {
		t.Fatalf("BuildProfile Env() = %#v, want plan+activation Env() %#v", got, wantEnv)
	}
	builtFingerprint, err := CollectFingerprint(built, fingerprintInputs)
	if err != nil {
		t.Fatalf("CollectFingerprint(BuildProfile): %v", err)
	}
	if !reflect.DeepEqual(builtFingerprint, plannedFingerprint) {
		t.Fatalf("BuildProfile fingerprint = %q, want PlanProfile fingerprint %q", builtFingerprint, plannedFingerprint)
	}
}

func TestPlanProfileFailuresHaveNoFilesystemEffects(t *testing.T) {
	tests := []struct {
		name       string
		grants     GrantSet
		declared   map[string]string
		wantReport bool
	}{
		{
			name: "invalid grant set",
			grants: GrantSet{Grants: []Grant{
				{Kind: GrantNetwork},
				{Kind: GrantNetwork},
			}},
		},
		{
			name:     "invalid declared environment",
			grants:   GrantSet{Grants: []Grant{{Kind: GrantNetwork}}},
			declared: map[string]string{"HOME": "smuggled"},
		},
		{
			name: "required grant cannot be applied",
			grants: GrantSet{Grants: []Grant{
				{Kind: GrantNetwork},
				{Kind: GrantPathRead, Paths: []string{"input.txt"}},
			}},
			wantReport: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			workspacePath := filepath.Join(root, "workspace")
			envRoot := filepath.Join(root, "environment")

			planned, report, err := PlanProfile(workspacePath, envRoot, tc.grants, tc.declared)
			if err == nil {
				t.Fatalf("PlanProfile = (%#v, %#v, nil), want error", planned, report)
			}
			if (report != nil) != tc.wantReport {
				t.Fatalf("PlanProfile report nil = %v, want report present = %v", report == nil, tc.wantReport)
			}
			assertPathAbsent(t, workspacePath)
			assertPathAbsent(t, envRoot)

			if tc.wantReport {
				cmd, runCtx, cancel, commandErr := planned.Command(context.Background(), "/bin/tool")
				if commandErr == nil || !strings.Contains(commandErr.Error(), "planned profile") {
					t.Fatalf("planned Command after planning error = (%v, %v, cancel=%v, %v), want planned-profile refusal", cmd, runCtx, cancel != nil, commandErr)
				}
				if cmd != nil || runCtx != nil || cancel != nil {
					t.Fatalf("planned Command refusal returned cmd=%v ctx=%v cancel!=nil=%v", cmd, runCtx, cancel != nil)
				}
			}
		})
	}
}

func TestProfileActivateFailureCannotLaunchAndBuildProfileKeepsCompatibility(t *testing.T) {
	root := t.TempDir()
	workspacePath := filepath.Join(root, "workspace")
	envRoot := filepath.Join(root, "environment")
	if err := os.WriteFile(envRoot, []byte("blocks directory creation"), 0o600); err != nil {
		t.Fatalf("WriteFile(envRoot): %v", err)
	}
	argv0 := filepath.Join(workspacePath, "evaluator")
	grants := GrantSet{Grants: []Grant{
		{Kind: GrantNetwork},
		{Kind: GrantProcessExecution, Argv0s: []string{argv0}},
	}}

	planned, report, err := PlanProfile(workspacePath, envRoot, grants, nil)
	if err != nil {
		t.Fatalf("PlanProfile: %v", err)
	}
	if report == nil {
		t.Fatal("PlanProfile report = nil, want enforcement projection")
	}

	activated, err := planned.Activate()
	if err == nil {
		t.Fatal("Activate = nil, want directory-creation error")
	}
	if !strings.Contains(err.Error(), "activate profile: creating") {
		t.Fatalf("Activate error = %q, want activation-qualified creation error", err)
	}
	assertProfileCannotLaunch(t, activated, argv0)
	assertProfileCannotLaunch(t, planned, argv0)

	built, builtReport, buildErr := BuildProfile(workspacePath, envRoot, grants, nil)
	if buildErr == nil {
		t.Fatal("BuildProfile = nil, want directory-creation error")
	}
	wantPrefix := "execworkspace: build profile: creating " + strconv.Quote(filepath.Join(envRoot, ".home"))
	if !strings.Contains(buildErr.Error(), wantPrefix) {
		t.Fatalf("BuildProfile error = %q, want preserved prefix %q", buildErr, wantPrefix)
	}
	if builtReport != nil {
		t.Fatalf("BuildProfile report = %#v, want nil on activation error for compatibility", builtReport)
	}
	assertProfileCannotLaunch(t, built, argv0)
}

func TestProfileActivateRejectsAProfileThatWasNotPlanned(t *testing.T) {
	activated, err := (Profile{}).Activate()
	if err == nil {
		t.Fatal("zero Profile Activate = nil, want fail-closed error")
	}
	var opErr *OperationalError
	if !errors.As(err, &opErr) {
		t.Fatalf("zero Profile Activate error = %T %v, want *OperationalError", err, err)
	}
	assertProfileCannotLaunch(t, activated, "/bin/tool")
}

func assertExactProfileDirectories(t *testing.T, envRoot string) {
	t.Helper()
	wantDirs := []string{
		envRoot,
		filepath.Join(envRoot, ".home"),
		filepath.Join(envRoot, ".home", ".cache"),
		filepath.Join(envRoot, ".home", ".config"),
		filepath.Join(envRoot, ".tmp"),
	}
	for _, path := range wantDirs {
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			t.Fatalf("profile directory %q = info %v, error %v; want directory", path, info, err)
		}
	}
	assertEntryNames(t, envRoot, []string{".home", ".tmp"})
	assertEntryNames(t, filepath.Join(envRoot, ".home"), []string{".cache", ".config"})
}

func assertEntryNames(t *testing.T, path string, want []string) {
	t.Helper()
	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatalf("ReadDir(%q): %v", path, err)
	}
	got := make([]string, len(entries))
	for i, entry := range entries {
		got[i] = entry.Name()
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadDir(%q) names = %#v, want exact %#v", path, got, want)
	}
}

func assertPathAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("Lstat(%q) error = %v, want path absent", path, err)
	}
}

func assertProfileCannotLaunch(t *testing.T, profile Profile, argv0 string) {
	t.Helper()
	cmd, runCtx, cancel, err := profile.Command(context.Background(), argv0)
	if err == nil {
		if cancel != nil {
			cancel()
		}
		t.Fatalf("Command returned cmd=%v ctx=%v, want fail-closed error", cmd, runCtx)
	}
	if cmd != nil || runCtx != nil || cancel != nil {
		t.Fatalf("Command refusal returned cmd=%v ctx=%v cancel!=nil=%v", cmd, runCtx, cancel != nil)
	}
}
