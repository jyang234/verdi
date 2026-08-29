package experimentrun

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/experiment"
)

func TestDeriveScheduleRotationTables(t *testing.T) {
	tests := []struct {
		name       string
		candidates []string
		warmups    int
		want       string
	}{
		{
			name:       "two candidates no warmups",
			candidates: []string{"alpha", "beta"},
			warmups:    0,
			want:       "measured-1:alpha,beta;measured-2:beta,alpha",
		},
		{
			name:       "two candidates multiple warmups",
			candidates: []string{"alpha", "beta"},
			warmups:    2,
			want:       "warmup-1:alpha,beta;warmup-2:beta,alpha;measured-1:alpha,beta;measured-2:beta,alpha",
		},
		{
			name:       "three candidates no warmups",
			candidates: []string{"alpha", "beta", "gamma"},
			warmups:    0,
			want:       "measured-1:alpha,beta,gamma;measured-2:beta,gamma,alpha",
		},
		{
			name:       "three candidates multiple warmups",
			candidates: []string{"alpha", "beta", "gamma"},
			warmups:    2,
			want:       "warmup-1:alpha,beta,gamma;warmup-2:beta,gamma,alpha;measured-1:gamma,alpha,beta;measured-2:alpha,beta,gamma",
		},
		{
			name:       "four candidates no warmups",
			candidates: []string{"alpha", "beta", "gamma", "delta"},
			warmups:    0,
			want:       "measured-1:alpha,beta,gamma,delta;measured-2:beta,gamma,delta,alpha",
		},
		{
			name:       "four candidates multiple warmups",
			candidates: []string{"alpha", "beta", "gamma", "delta"},
			warmups:    2,
			want:       "warmup-1:alpha,beta,gamma,delta;warmup-2:beta,gamma,delta,alpha;measured-1:gamma,delta,alpha,beta;measured-2:delta,alpha,beta,gamma",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def, _, _ := testDefinition(t, tt.candidates, tt.warmups)
			schedule, err := DeriveSchedule(def)
			if err != nil {
				t.Fatalf("DeriveSchedule(): %v", err)
			}
			if got := renderSchedule(schedule); got != tt.want {
				t.Fatalf("schedule = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestScheduleDigestDeterministicAndDerivationDoesNotAliasDefinition(t *testing.T) {
	def, _, _ := testDefinition(t, []string{"alpha", "beta", "gamma"}, 1)
	first, err := DeriveSchedule(def)
	if err != nil {
		t.Fatal(err)
	}
	second, err := DeriveSchedule(def)
	if err != nil {
		t.Fatal(err)
	}
	d1, err := ScheduleDigest(first)
	if err != nil {
		t.Fatal(err)
	}
	d2, err := ScheduleDigest(second)
	if err != nil {
		t.Fatal(err)
	}
	if d1 != d2 {
		t.Fatalf("ScheduleDigest() = %q, want deterministic %q", d2, d1)
	}
	first[0].Candidate = "mutated"
	if second[0].Candidate == "mutated" || def.Candidates[0].ID == "mutated" {
		t.Fatal("derived schedule aliases caller-owned state")
	}
}

func TestResolveAuthorizationFailsClosed(t *testing.T) {
	def, caps, capsBytes := testDefinition(t, []string{"alpha", "beta"}, 0)
	grants := testGrants(t, false, "./tools/evaluator", 30)
	grantBytes, err := execworkspace.EncodeGrantSet(grants)
	if err != nil {
		t.Fatal(err)
	}
	base := ExecutionAuthorization{
		EnvironmentPolicy: def.Execution.EnvironmentPolicy,
		AuthorityDigest:   digestText("authority"),
		GrantBytes:        grantBytes,
		DeclaredEnv:       map[string]string{"LANG": "C"},
		ObservationBytes:  4096,
	}

	if _, err := ResolveAuthorization(context.Background(), staticAuthorization{authorization: base}, def, caps); err != nil {
		t.Fatalf("ResolveAuthorization(valid): %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*ExecutionAuthorization, *experiment.Capabilities)
	}{
		{name: "mismatched environment policy", mutate: func(a *ExecutionAuthorization, _ *experiment.Capabilities) { a.EnvironmentPolicy = "other-policy" }},
		{name: "malformed authority digest", mutate: func(a *ExecutionAuthorization, _ *experiment.Capabilities) { a.AuthorityDigest = "unknown" }},
		{name: "nonpositive observation limit", mutate: func(a *ExecutionAuthorization, _ *experiment.Capabilities) { a.ObservationBytes = 0 }},
		{name: "noncanonical grant bytes", mutate: func(a *ExecutionAuthorization, _ *experiment.Capabilities) { a.GrantBytes = append(a.GrantBytes, ' ') }},
		{name: "timeout mismatch", mutate: func(a *ExecutionAuthorization, _ *experiment.Capabilities) {
			g := testGrants(t, false, "./tools/evaluator", 29)
			a.GrantBytes, _ = execworkspace.EncodeGrantSet(g)
		}},
		{name: "missing declared environment", mutate: func(a *ExecutionAuthorization, _ *experiment.Capabilities) { a.DeclaredEnv = map[string]string{} }},
		{name: "undeclared environment", mutate: func(a *ExecutionAuthorization, _ *experiment.Capabilities) { a.DeclaredEnv["EXTRA"] = "value" }},
		{name: "required network absent", mutate: func(a *ExecutionAuthorization, c *experiment.Capabilities) { c.RequiresNetwork = true }},
		{name: "network grant not required", mutate: func(a *ExecutionAuthorization, _ *experiment.Capabilities) {
			g := testGrants(t, true, "./tools/evaluator", 30)
			a.GrantBytes, _ = execworkspace.EncodeGrantSet(g)
		}},
		{name: "elevated capability unsupported", mutate: func(_ *ExecutionAuthorization, c *experiment.Capabilities) { c.RequiresElevated = true }},
		{name: "process grant excludes evaluator", mutate: func(a *ExecutionAuthorization, _ *experiment.Capabilities) {
			g := testGrants(t, false, "./tools/another-evaluator", 30)
			a.GrantBytes, _ = execworkspace.EncodeGrantSet(g)
		}},
		{name: "missing required guard capability", mutate: func(_ *ExecutionAuthorization, c *experiment.Capabilities) { c.Guards = nil }},
		{name: "unknown grant mechanism", mutate: func(a *ExecutionAuthorization, _ *experiment.Capabilities) {
			a.GrantBytes = []byte(`{"grants":[{"kind":"unknown"}],"schema":"verdi.execution-grants/v1"}` + "\n")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authorization := cloneAuthorization(base)
			capabilities := cloneCapabilities(caps)
			tt.mutate(&authorization, &capabilities)
			if _, err := ResolveAuthorization(context.Background(), staticAuthorization{authorization: authorization}, def, capabilities); err == nil {
				t.Fatal("ResolveAuthorization() = nil error")
			}
		})
	}

	if got := capsBytes; testDigestBytes(got) != def.Evaluator.CapabilitiesDigest {
		t.Fatal("test capabilities fixture lost registered digest parity")
	}
}

func TestResolveAuthorizationChecksFixedObserverMembershipAndPlatform(t *testing.T) {
	def, caps, _ := testDefinition(t, []string{"alpha", "beta"}, 0)
	def.Decision.PrimaryMetric = experiment.PrimaryMetric{ID: experiment.EvaluatorWallDurationMetricID, Type: experiment.MetricDuration, Unit: "ns", Aggregation: experiment.AggregationP95, Direction: experiment.DirectionLower}
	def = relockDefinition(t, def)
	base := testAuthorization(t, def, false)
	if _, err := ResolveAuthorization(context.Background(), staticAuthorization{authorization: base}, def, caps); err == nil {
		t.Fatal("ResolveAuthorization() accepted a fixed metric absent from capabilities.observers")
	}
	caps.Observers = []string{experiment.EvaluatorWallDurationMetricID}
	if _, err := ResolveAuthorization(context.Background(), staticAuthorization{authorization: base}, def, caps); err != nil {
		t.Fatalf("ResolveAuthorization(fixed wall observer): %v", err)
	}

	def.Decision.PrimaryMetric = experiment.PrimaryMetric{ID: experiment.EvaluatorPeakRSSMetricID, Type: experiment.MetricBytes, Unit: "bytes", Aggregation: experiment.AggregationP95, Direction: experiment.DirectionLower}
	def = relockDefinition(t, def)
	caps.Observers = []string{experiment.EvaluatorPeakRSSMetricID}
	_, err := ResolveAuthorization(context.Background(), staticAuthorization{authorization: base}, def, caps)
	if runtime.GOOS == "linux" && err != nil {
		t.Fatalf("ResolveAuthorization(linux peak RSS observer): %v", err)
	}
	if runtime.GOOS != "linux" && err == nil {
		t.Fatal("ResolveAuthorization() accepted unavailable peak RSS observer")
	}
}

func TestResolveInputsProvesExactProtectedRegularFiles(t *testing.T) {
	def, _, _ := testDefinition(t, []string{"alpha", "beta"}, 0)
	root := t.TempDir()
	contents := map[string]string{
		"inputs/workload.json":      "workload\n",
		"fixtures/request-log.json": "fixture\n",
		"contracts/behavioral.json": "contract\n",
	}
	for path, content := range contents {
		writeTestFile(t, root, path, content)
	}
	def.Workload.Digest = digestText(contents["inputs/workload.json"])
	def.Fixtures[0].Digest = digestText(contents["fixtures/request-log.json"])
	def.Contract.Digest = digestText(contents["contracts/behavioral.json"])
	def = relockDefinition(t, def)

	base := staticInputs{values: map[string]ResolvedInput{
		def.Workload.ID:    {ID: def.Workload.ID, Path: "inputs/workload.json", Digest: def.Workload.Digest},
		def.Fixtures[0].ID: {ID: def.Fixtures[0].ID, Path: "fixtures/request-log.json", Digest: def.Fixtures[0].Digest},
		def.Contract.ID:    {ID: def.Contract.ID, Path: "contracts/behavioral.json", Digest: def.Contract.Digest},
	}}
	resolved, err := ResolveInputs(context.Background(), base, root, def)
	if err != nil {
		t.Fatalf("ResolveInputs(valid): %v", err)
	}
	if resolved.Workload.Path != "inputs/workload.json" || len(resolved.Fixtures) != 1 || resolved.Contract.Path != "contracts/behavioral.json" {
		t.Fatalf("resolved = %+v", resolved)
	}

	tests := []struct {
		name   string
		mutate func(*staticInputs)
	}{
		{name: "wrong registered id", mutate: func(r *staticInputs) { v := r.values[def.Workload.ID]; v.ID = "other"; r.values[def.Workload.ID] = v }},
		{name: "wrong registered digest", mutate: func(r *staticInputs) {
			v := r.values[def.Workload.ID]
			v.Digest = digestText("different")
			r.values[def.Workload.ID] = v
		}},
		{name: "noncanonical path", mutate: func(r *staticInputs) {
			v := r.values[def.Workload.ID]
			v.Path = "inputs/../inputs/workload.json"
			r.values[def.Workload.ID] = v
		}},
		{name: "unprotected path", mutate: func(r *staticInputs) {
			v := r.values[def.Workload.ID]
			v.Path = "inputs/not-protected.json"
			r.values[def.Workload.ID] = v
		}},
		{name: "duplicate input path", mutate: func(r *staticInputs) {
			v := r.values[def.Fixtures[0].ID]
			v.Path = "inputs/workload.json"
			r.values[def.Fixtures[0].ID] = v
		}},
		{name: "missing input", mutate: func(r *staticInputs) { delete(r.values, def.Contract.ID) }},
		{name: "changed raw bytes", mutate: func(_ *staticInputs) { writeTestFile(t, root, "inputs/workload.json", "changed\n") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := cloneInputs(base)
			tt.mutate(&resolver)
			if _, err := ResolveInputs(context.Background(), resolver, root, def); err == nil {
				t.Fatal("ResolveInputs() = nil error")
			}
			if tt.name == "changed raw bytes" {
				writeTestFile(t, root, "inputs/workload.json", contents["inputs/workload.json"])
			}
		})
	}

	if err := os.Symlink(filepath.Join(root, "contracts", "behavioral.json"), filepath.Join(root, "contracts", "symlink.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "contracts"), filepath.Join(root, "linked-contracts")); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name string
		path string
	}{
		{name: "final component", path: "contracts/symlink.json"},
		{name: "parent component", path: "linked-contracts/behavioral.json"},
	} {
		t.Run(tt.name+" symlink", func(t *testing.T) {
			symlinkDefinition := def
			symlinkDefinition.ProtectedPaths = append(append([]string(nil), def.ProtectedPaths...), tt.path)
			symlinkDefinition = relockDefinition(t, symlinkDefinition)
			symlinkResolver := cloneInputs(base)
			v := symlinkResolver.values[def.Contract.ID]
			v.Path = tt.path
			symlinkResolver.values[def.Contract.ID] = v
			_, err := ResolveInputs(context.Background(), symlinkResolver, root, symlinkDefinition)
			if err == nil {
				t.Fatal("ResolveInputs() accepted a symlink")
			}
			if !strings.Contains(err.Error(), "traverses a symlink") {
				t.Fatalf("ResolveInputs() error = %q, want actual symlink traversal refusal", err)
			}
		})
	}
}

func TestPreflightEnvironmentRootRejectsEveryCollision(t *testing.T) {
	workspace := t.TempDir()
	path, err := PreflightEnvironmentRoot(workspace)
	if err != nil {
		t.Fatalf("PreflightEnvironmentRoot(absent): %v", err)
	}
	if path != filepath.Join(workspace, ".verdi-cse-environment") {
		t.Fatalf("environment path = %q", path)
	}

	for _, setup := range []struct {
		name string
		make func(t *testing.T, path string)
	}{
		{name: "regular file", make: func(t *testing.T, path string) {
			t.Helper()
			if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "empty directory", make: func(t *testing.T, path string) {
			t.Helper()
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "nonempty directory", make: func(t *testing.T, path string) {
			t.Helper()
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(path, "state"), []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "symlink", make: func(t *testing.T, path string) {
			t.Helper()
			target := filepath.Join(t.TempDir(), "target")
			if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, path); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(setup.name, func(t *testing.T) {
			root := t.TempDir()
			setup.make(t, filepath.Join(root, ".verdi-cse-environment"))
			if _, err := PreflightEnvironmentRoot(root); err == nil {
				t.Fatal("PreflightEnvironmentRoot() = nil error")
			}
		})
	}
}

func TestCandidateReceiptsUseExperimentScopedFullWorkspaceRunIDs(t *testing.T) {
	def, _, _ := testDefinition(t, []string{"zeta", "alpha"}, 0)
	digest, err := experiment.DefinitionDigest(def)
	if err != nil {
		t.Fatal(err)
	}
	patches := candidatePatches(t, def)
	rows, err := CandidateReceipts(def, digest, "run-1", patches)
	if err != nil {
		t.Fatalf("CandidateReceipts(): %v", err)
	}
	if rows[0].ID != "alpha" || rows[1].ID != "zeta" {
		t.Fatalf("receipt rows are not sorted: %+v", rows)
	}
	if len(rows[0].WorkspaceRunID) != 64 || rows[0].WorkspaceRunID == rows[1].WorkspaceRunID {
		t.Fatalf("workspace run ids = %q, %q", rows[0].WorkspaceRunID, rows[1].WorkspaceRunID)
	}
	other, err := CandidateReceipts(def, digestText("other"), "run-1", patches)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].WorkspaceRunID == other[0].WorkspaceRunID {
		t.Fatal("different experiments share a workspace run id")
	}
	patches["alpha"] = []byte("changed patch")
	if _, err := CandidateReceipts(def, digest, "run-1", patches); err == nil {
		t.Fatal("CandidateReceipts() accepted changed candidate patch bytes")
	}
}

func TestBuildExecutionReceiptStrictRoundTripDigestAndCloneSafety(t *testing.T) {
	def, caps, capsBytes := testDefinition(t, []string{"zeta", "alpha"}, 1)
	root := t.TempDir()
	resolved := writeResolvedInputs(t, root, def)
	auth := testAuthorization(t, def, false)
	input := ReceiptInput{
		Definition:        def,
		Run:               "run-1",
		Capabilities:      caps,
		CapabilitiesBytes: capsBytes,
		Authorization:     mustResolveAuthorization(t, def, caps, auth),
		Inputs:            resolved,
		CandidatePatches:  candidatePatches(t, def),
		Fingerprint:       testFingerprint(t, def, caps, auth, resolved),
		Enforcement: execworkspace.EnforcementReport{
			Rows: []execworkspace.EnforcementReportRow{
				{Kind: execworkspace.GrantProcessExecution, Applied: true, Reason: "allowlist applied"},
				{Kind: execworkspace.GrantTimeouts, Applied: true, Reason: "timeout applied"},
			},
			Network: execworkspace.NetworkEnforcement{Mode: execworkspace.NetworkDeny, Configured: true, Reason: "linux namespace configured"},
		},
		Versions: experiment.ReceiptVersions{Verdi: "v-test", RecommendationEngine: string(def.Algorithm)},
	}
	host := linuxHostRuntimeFacts()
	receipt, err := buildExecutionReceipt(input, host)
	if err != nil {
		t.Fatalf("BuildExecutionReceipt(): %v", err)
	}
	if receipt.Candidates[0].ID != "alpha" || receipt.Candidates[1].ID != "zeta" {
		t.Fatalf("receipt candidate order = %+v", receipt.Candidates)
	}
	encoded, err := experiment.EncodeExecutionReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := experiment.DecodeExecutionReceipt(encoded)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := experiment.ExecutionReceiptDigest(receipt)
	if err != nil {
		t.Fatal(err)
	}
	decodedDigest, err := experiment.ExecutionReceiptDigest(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if digest != decodedDigest {
		t.Fatalf("receipt digest = %q, decoded digest = %q", digest, decodedDigest)
	}
	if err := verifyExecutionReceipt(input, decoded, host); err != nil {
		t.Fatalf("VerifyExecutionReceipt(): %v", err)
	}

	input.Authorization.Authorization.DeclaredEnv["LANG"] = "mutated"
	input.CandidatePatches["alpha"] = []byte("mutated")
	input.Inputs.Workload.Path = "mutated/workload.json"
	if value := receipt.Fingerprint.Env["LANG"]; value == nil || *value != "C" {
		t.Fatal("receipt aliases authorization environment")
	}
	if receipt.Inputs.Workload.Path != "inputs/workload.json" {
		t.Fatalf("receipt input custody aliases resolved inputs: %+v", receipt.Inputs.Workload)
	}
	if err := verifyExecutionReceipt(input, receipt, host); err == nil {
		t.Fatal("VerifyExecutionReceipt() accepted changed candidate patch input")
	}
}

func TestVerifyExecutionReceiptRejectsChangedSlotPathsWhenDigestsEqual(t *testing.T) {
	def, caps, capsBytes := testDefinition(t, []string{"alpha", "beta"}, 0)
	def.Contract.Digest = def.Workload.Digest
	def = relockDefinition(t, def)
	root := t.TempDir()
	for relative, contents := range map[string]string{
		"inputs/workload.json":      "workload",
		"contracts/behavioral.json": "workload",
		"fixtures/request-log.json": "fixture",
	} {
		writeTestFile(t, root, relative, contents)
	}
	resolver := staticInputs{values: map[string]ResolvedInput{
		def.Workload.ID:    {ID: def.Workload.ID, Path: "inputs/workload.json", Digest: def.Workload.Digest},
		def.Fixtures[0].ID: {ID: def.Fixtures[0].ID, Path: "fixtures/request-log.json", Digest: def.Fixtures[0].Digest},
		def.Contract.ID:    {ID: def.Contract.ID, Path: "contracts/behavioral.json", Digest: def.Contract.Digest},
	}}
	resolved, err := ResolveInputs(context.Background(), resolver, root, def)
	if err != nil {
		t.Fatal(err)
	}
	auth := testAuthorization(t, def, false)
	input := ReceiptInput{
		Definition: def, Run: "run-1", Capabilities: caps, CapabilitiesBytes: capsBytes,
		Authorization: mustResolveAuthorization(t, def, caps, auth), Inputs: resolved,
		CandidatePatches: candidatePatches(t, def), Fingerprint: testFingerprint(t, def, caps, auth, resolved),
		Enforcement: execworkspace.EnforcementReport{
			Rows: []execworkspace.EnforcementReportRow{
				{Kind: execworkspace.GrantProcessExecution, Applied: true, Reason: "allowlist applied"},
				{Kind: execworkspace.GrantTimeouts, Applied: true, Reason: "timeout applied"},
			},
			Network: execworkspace.NetworkEnforcement{Mode: execworkspace.NetworkDeny, Configured: true, Reason: "linux namespace configured"},
		},
		Versions: experiment.ReceiptVersions{Verdi: "v-test", RecommendationEngine: string(def.Algorithm)},
	}
	host := linuxHostRuntimeFacts()
	receipt, err := buildExecutionReceipt(input, host)
	if err != nil {
		t.Fatal(err)
	}

	changed := cloneReceiptInput(input)
	changed.Inputs.Workload.Path, changed.Inputs.Contract.Path = changed.Inputs.Contract.Path, changed.Inputs.Workload.Path
	changed.Fingerprint = testFingerprint(t, def, caps, auth, changed.Inputs)
	if err := verifyExecutionReceipt(changed, receipt, host); err == nil {
		t.Fatal("VerifyExecutionReceipt() accepted exchanged workload/contract paths under one shared digest")
	}
}

func TestBuildExecutionReceiptRejectsMismatchedAuthorityInputs(t *testing.T) {
	def, caps, capsBytes := testDefinition(t, []string{"alpha", "beta"}, 0)
	root := t.TempDir()
	resolved := writeResolvedInputs(t, root, def)
	auth := testAuthorization(t, def, false)
	input := ReceiptInput{
		Definition:        def,
		Run:               "run-1",
		Capabilities:      caps,
		CapabilitiesBytes: capsBytes,
		Authorization:     mustResolveAuthorization(t, def, caps, auth),
		Inputs:            resolved,
		CandidatePatches:  candidatePatches(t, def),
		Fingerprint:       testFingerprint(t, def, caps, auth, resolved),
		Enforcement: execworkspace.EnforcementReport{
			Rows:    []execworkspace.EnforcementReportRow{{Kind: execworkspace.GrantProcessExecution, Applied: true, Reason: "allowlist applied"}, {Kind: execworkspace.GrantTimeouts, Applied: true, Reason: "timeout applied"}},
			Network: execworkspace.NetworkEnforcement{Mode: execworkspace.NetworkDeny, Configured: true, Reason: "linux namespace configured"},
		},
		Versions: experiment.ReceiptVersions{Verdi: "v-test", RecommendationEngine: string(def.Algorithm)},
	}
	tests := []struct {
		name   string
		mutate func(*ReceiptInput)
	}{
		{name: "capabilities bytes digest mismatch", mutate: func(in *ReceiptInput) {
			in.CapabilitiesBytes = append([]byte(nil), in.CapabilitiesBytes...)
			in.CapabilitiesBytes[0] = 'x'
		}},
		{name: "fingerprint omits declared environment", mutate: func(in *ReceiptInput) {
			in.Fingerprint = []byte(strings.Replace(string(in.Fingerprint), `"LANG":"C"`, `"LANG":null`, 1))
		}},
		{name: "fingerprint names unsupported platform", mutate: func(in *ReceiptInput) {
			in.Fingerprint = []byte(strings.Replace(string(in.Fingerprint), `"os":"linux"`, `"os":"darwin"`, 1))
		}},
		{name: "fingerprint extra input", mutate: func(in *ReceiptInput) {
			in.Fingerprint = []byte(strings.Replace(string(in.Fingerprint), `"input_digests":{`, `"input_digests":{"extra":"`+strings.Repeat("0", 64)+`",`, 1))
		}},
		{name: "receipt network mismatch", mutate: func(in *ReceiptInput) { in.Enforcement.Network.Mode = execworkspace.NetworkAllow }},
		{name: "unconfigured default deny", mutate: func(in *ReceiptInput) { in.Enforcement.Network.Configured = false }},
		{name: "unapplied enforcement", mutate: func(in *ReceiptInput) { in.Enforcement.Rows[0].Applied = false }},
		{name: "missing grant enforcement", mutate: func(in *ReceiptInput) { in.Enforcement.Rows = in.Enforcement.Rows[:1] }},
		{name: "wrong recommendation engine", mutate: func(in *ReceiptInput) { in.Versions.RecommendationEngine = "unknown" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := cloneReceiptInput(input)
			tt.mutate(&candidate)
			if _, err := buildExecutionReceipt(candidate, linuxHostRuntimeFacts()); err == nil {
				t.Fatal("BuildExecutionReceipt() = nil error")
			}
		})
	}
}

func TestBuildExecutionReceiptRejectsFingerprintHostForgeryAndMissingRuntime(t *testing.T) {
	def, caps, capsBytes := testDefinition(t, []string{"alpha", "beta"}, 0)
	root := t.TempDir()
	resolved := writeResolvedInputs(t, root, def)
	auth := testAuthorization(t, def, false)
	input := ReceiptInput{
		Definition:        def,
		Run:               "run-1",
		Capabilities:      caps,
		CapabilitiesBytes: capsBytes,
		Authorization:     mustResolveAuthorization(t, def, caps, auth),
		Inputs:            resolved,
		CandidatePatches:  candidatePatches(t, def),
		Fingerprint:       testFingerprint(t, def, caps, auth, resolved),
		Enforcement: execworkspace.EnforcementReport{
			Rows:    []execworkspace.EnforcementReportRow{{Kind: execworkspace.GrantProcessExecution, Applied: true, Reason: "allowlist applied"}, {Kind: execworkspace.GrantTimeouts, Applied: true, Reason: "timeout applied"}},
			Network: execworkspace.NetworkEnforcement{Mode: execworkspace.NetworkDeny, Configured: true, Reason: "linux namespace configured"},
		},
		Versions: experiment.ReceiptVersions{Verdi: "v-test", RecommendationEngine: string(def.Algorithm)},
	}
	for _, tt := range []struct {
		name      string
		mutate    func(*experiment.ExecutionFingerprint)
		build     func(ReceiptInput) (experiment.ExecutionReceipt, error)
		wantError string
	}{
		{name: "forged operating system", mutate: func(f *experiment.ExecutionFingerprint) {
			if runtime.GOOS == "linux" {
				f.OS = "darwin"
			} else {
				f.OS = "linux"
			}
		}, build: BuildExecutionReceipt, wantError: "does not match host"},
		{name: "missing runtime version", mutate: func(f *experiment.ExecutionFingerprint) {
			delete(f.ToolVersions, "runtime")
		}, build: func(input ReceiptInput) (experiment.ExecutionReceipt, error) {
			return buildExecutionReceipt(input, linuxHostRuntimeFacts())
		}, wantError: `tool version "runtime"`},
		{name: "forged architecture", mutate: func(f *experiment.ExecutionFingerprint) {
			f.Arch = "arm64"
		}, build: func(input ReceiptInput) (experiment.ExecutionReceipt, error) {
			return buildExecutionReceipt(input, linuxHostRuntimeFacts())
		}, wantError: "architecture"},
		{name: "forged runtime version", mutate: func(f *experiment.ExecutionFingerprint) {
			f.ToolVersions["runtime"] = "go-forged"
		}, build: func(input ReceiptInput) (experiment.ExecutionReceipt, error) {
			return buildExecutionReceipt(input, linuxHostRuntimeFacts())
		}, wantError: `tool version "runtime"`},
		{name: "matching unsupported host", mutate: func(f *experiment.ExecutionFingerprint) {
			f.OS = "darwin"
			f.Arch = "arm64"
		}, build: func(input ReceiptInput) (experiment.ExecutionReceipt, error) {
			return buildExecutionReceipt(input, hostRuntimeFacts{os: "darwin", arch: "arm64", runtimeVersion: runtime.Version()})
		}, wantError: "authoritative CSE execution requires linux"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			candidate := cloneReceiptInput(input)
			candidate.Fingerprint = mutateFingerprint(t, candidate.Fingerprint, tt.mutate)
			receipt, err := tt.build(candidate)
			if err == nil {
				t.Fatal("BuildExecutionReceipt() = nil error")
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("BuildExecutionReceipt() error = %q, want %q", err, tt.wantError)
			}
			if !reflect.DeepEqual(receipt, experiment.ExecutionReceipt{}) {
				t.Fatalf("BuildExecutionReceipt() receipt = %+v, want zero receipt", receipt)
			}
		})
	}

	candidate := cloneReceiptInput(input)
	candidate.Fingerprint = mutateFingerprint(t, candidate.Fingerprint, func(f *experiment.ExecutionFingerprint) {
		f.OS = runtime.GOOS
		f.Arch = runtime.GOARCH
		f.ToolVersions["runtime"] = runtime.Version()
	})
	receipt, err := BuildExecutionReceipt(candidate)
	if runtime.GOOS == "linux" {
		if err != nil {
			t.Fatalf("BuildExecutionReceipt() on linux: %v", err)
		}
		if reflect.DeepEqual(receipt, experiment.ExecutionReceipt{}) {
			t.Fatal("BuildExecutionReceipt() on linux returned a zero receipt")
		}
		return
	}
	if err == nil || !strings.Contains(err.Error(), "authoritative CSE execution requires linux") {
		t.Fatalf("BuildExecutionReceipt() error = %v, want unsupported-host refusal", err)
	}
	if !reflect.DeepEqual(receipt, experiment.ExecutionReceipt{}) {
		t.Fatalf("BuildExecutionReceipt() receipt = %+v, want zero receipt", receipt)
	}
}

type staticAuthorization struct{ authorization ExecutionAuthorization }

func (r staticAuthorization) ResolveExecutionAuthorization(context.Context, experiment.Definition, experiment.Capabilities) (ExecutionAuthorization, error) {
	return r.authorization, nil
}

type staticInputs struct{ values map[string]ResolvedInput }

func (r staticInputs) ResolveExperimentInput(_ context.Context, _ string, request ResolveInputRequest) (ResolvedInput, error) {
	value, ok := r.values[request.Ref.ID]
	if !ok {
		return ResolvedInput{}, os.ErrNotExist
	}
	return value, nil
}

func testDefinition(t *testing.T, candidateIDs []string, warmups int) (experiment.Definition, experiment.Capabilities, []byte) {
	t.Helper()
	patches := make(map[string][]byte, len(candidateIDs))
	candidates := make([]experiment.Candidate, 0, len(candidateIDs))
	base := strings.Repeat("a", 40)
	for _, id := range candidateIDs {
		patch := []byte("diff --git a/" + id + " b/" + id + "\n")
		patches[id] = patch
		candidates = append(candidates, experiment.Candidate{ID: id, Patch: "candidates/" + id + ".patch", Digest: testDigestBytes(patch), Base: base})
	}
	caps := experiment.Capabilities{
		Schema:           experiment.CapabilitiesSchemaV2,
		EvaluatorVersion: "evaluator-test/v1",
		ProtocolVersions: []string{experiment.EvaluatorProtocolSchema, experiment.ObservationSchemaV2},
		Metrics:          []experiment.CapabilityMetric{{ID: "latency", Type: experiment.MetricDuration, Unit: "ms", Direction: experiment.DirectionLower}, {ID: "memory", Type: experiment.MetricBytes, Unit: "bytes", Direction: experiment.DirectionLower}},
		Guards:           []string{"correctness"},
		Environment:      []string{"LANG"},
	}
	capsBytes, err := canonjson.Marshal(caps)
	if err != nil {
		t.Fatal(err)
	}
	def := experiment.Definition{
		Schema:     experiment.DefinitionSchema,
		ID:         "comparison",
		Spike:      "spec/comparison",
		Question:   "spec/question#oq-one",
		BaseCommit: base,
		Candidates: candidates,
		Evaluator: experiment.Evaluator{
			Argv:               []string{"./tools/evaluator", "run"},
			Digest:             digestText("evaluator"),
			CapabilitiesDigest: testDigestBytes(capsBytes),
		},
		Workload: experiment.ArtifactRef{ID: "workload", Digest: digestText("workload")},
		Fixtures: []experiment.ArtifactRef{{ID: "fixture", Digest: digestText("fixture")}},
		Contract: experiment.ArtifactRef{ID: "contract", Digest: digestText("contract")},
		Decision: experiment.DecisionSpec{
			PrimaryMetric:       experiment.PrimaryMetric{ID: "latency", Type: experiment.MetricDuration, Unit: "ms", Aggregation: experiment.AggregationP95, Direction: experiment.DirectionLower},
			Baseline:            candidateIDs[0],
			BaselineImprovement: threshold(0.1),
			CandidateSeparation: threshold(0.05),
			Guards:              []experiment.Guard{{ID: "correctness"}, {ID: "memory", MaximumRelativeToBaseline: pointer(1.2)}},
		},
		Execution:       experiment.Execution{Warmups: warmups, Rounds: 2, Order: experiment.OrderDeterministicRotation, TimeoutPerRound: "30s", EnvironmentPolicy: "isolated-v1"},
		Algorithm:       experiment.AlgorithmV1,
		RetentionPolicy: "standard",
		ProtectedPaths:  []string{"inputs/workload.json", "fixtures/request-log.json", "contracts/behavioral.json"},
	}
	return relockDefinition(t, def), caps, capsBytes
}

func relockDefinition(t *testing.T, def experiment.Definition) experiment.Definition {
	t.Helper()
	def.Lock = nil
	digest, err := experiment.DefinitionDigest(def)
	if err != nil {
		t.Fatal(err)
	}
	def.Lock = &experiment.Lock{DefinitionDigest: digest}
	return def
}

func testGrants(t *testing.T, network bool, argv0 string, seconds int) execworkspace.GrantSet {
	t.Helper()
	grants := []execworkspace.Grant{{Kind: execworkspace.GrantProcessExecution, Argv0s: []string{argv0}}, {Kind: execworkspace.GrantTimeouts, Seconds: seconds}}
	if network {
		grants = append([]execworkspace.Grant{{Kind: execworkspace.GrantNetwork}}, grants...)
	}
	return execworkspace.GrantSet{Grants: grants}
}

func testAuthorization(t *testing.T, def experiment.Definition, network bool) ExecutionAuthorization {
	t.Helper()
	bytes, err := execworkspace.EncodeGrantSet(testGrants(t, network, def.Evaluator.Argv[0], 30))
	if err != nil {
		t.Fatal(err)
	}
	return ExecutionAuthorization{EnvironmentPolicy: def.Execution.EnvironmentPolicy, AuthorityDigest: digestText("authority"), GrantBytes: bytes, DeclaredEnv: map[string]string{"LANG": "C"}, ObservationBytes: 4096}
}

func mustResolveAuthorization(t *testing.T, def experiment.Definition, caps experiment.Capabilities, authorization ExecutionAuthorization) AuthorizedExecution {
	t.Helper()
	got, err := ResolveAuthorization(context.Background(), staticAuthorization{authorization: authorization}, def, caps)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func writeResolvedInputs(t *testing.T, root string, def experiment.Definition) ResolvedInputs {
	t.Helper()
	paths := map[string]string{"inputs/workload.json": "workload", "fixtures/request-log.json": "fixture", "contracts/behavioral.json": "contract"}
	for path, content := range paths {
		writeTestFile(t, root, path, content)
	}
	resolver := staticInputs{values: map[string]ResolvedInput{
		def.Workload.ID:    {ID: def.Workload.ID, Path: "inputs/workload.json", Digest: def.Workload.Digest},
		def.Fixtures[0].ID: {ID: def.Fixtures[0].ID, Path: "fixtures/request-log.json", Digest: def.Fixtures[0].Digest},
		def.Contract.ID:    {ID: def.Contract.ID, Path: "contracts/behavioral.json", Digest: def.Contract.Digest},
	}}
	resolved, err := ResolveInputs(context.Background(), resolver, root, def)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func candidatePatches(t *testing.T, def experiment.Definition) map[string][]byte {
	t.Helper()
	patches := make(map[string][]byte, len(def.Candidates))
	for _, candidate := range def.Candidates {
		patch := []byte("diff --git a/" + candidate.ID + " b/" + candidate.ID + "\n")
		if got := testDigestBytes(patch); got != candidate.Digest {
			t.Fatalf("candidate patch %q digest = %q, want %q", candidate.ID, got, candidate.Digest)
		}
		patches[candidate.ID] = patch
	}
	return patches
}

func testFingerprint(t *testing.T, def experiment.Definition, caps experiment.Capabilities, authorization ExecutionAuthorization, inputs ResolvedInputs) []byte {
	t.Helper()
	fingerprint := experiment.ExecutionFingerprint{
		OS:   "linux",
		Arch: "amd64",
		ToolVersions: map[string]string{
			"evaluator":             caps.EvaluatorVersion,
			"recommendation-engine": string(def.Algorithm),
			"runtime":               runtime.Version(),
			"verdi":                 "v-test",
		},
		Env: map[string]*string{"LANG": pointer(authorization.DeclaredEnv["LANG"])},
		InputDigests: map[string]string{
			"evaluator:" + def.Evaluator.Argv[0]: strings.TrimPrefix(def.Evaluator.Digest, "sha256:"),
			inputs.Workload.Path:                 strings.TrimPrefix(inputs.Workload.Digest, "sha256:"),
			inputs.Fixtures[0].Path:              strings.TrimPrefix(inputs.Fixtures[0].Digest, "sha256:"),
			inputs.Contract.Path:                 strings.TrimPrefix(inputs.Contract.Digest, "sha256:"),
		},
	}
	bytes, err := canonjson.Marshal(fingerprint)
	if err != nil {
		t.Fatal(err)
	}
	return bytes
}

func linuxHostRuntimeFacts() hostRuntimeFacts {
	return hostRuntimeFacts{os: "linux", arch: "amd64", runtimeVersion: runtime.Version()}
}

func mutateFingerprint(t *testing.T, raw []byte, mutate func(*experiment.ExecutionFingerprint)) []byte {
	t.Helper()
	var fingerprint experiment.ExecutionFingerprint
	if err := json.Unmarshal(raw, &fingerprint); err != nil {
		t.Fatal(err)
	}
	mutate(&fingerprint)
	bytes, err := canonjson.Marshal(fingerprint)
	if err != nil {
		t.Fatal(err)
	}
	return bytes
}

func cloneAuthorization(in ExecutionAuthorization) ExecutionAuthorization {
	out := in
	out.GrantBytes = append([]byte(nil), in.GrantBytes...)
	out.DeclaredEnv = make(map[string]string, len(in.DeclaredEnv))
	for key, value := range in.DeclaredEnv {
		out.DeclaredEnv[key] = value
	}
	return out
}

func cloneCapabilities(in experiment.Capabilities) experiment.Capabilities {
	out := in
	out.ProtocolVersions = append([]string(nil), in.ProtocolVersions...)
	out.Metrics = append([]experiment.CapabilityMetric(nil), in.Metrics...)
	out.Guards = append([]string(nil), in.Guards...)
	out.Observers = append([]string(nil), in.Observers...)
	out.Environment = append([]string(nil), in.Environment...)
	return out
}

func cloneInputs(in staticInputs) staticInputs {
	out := staticInputs{values: make(map[string]ResolvedInput, len(in.values))}
	for key, value := range in.values {
		out.values[key] = value
	}
	return out
}

func cloneReceiptInput(in ReceiptInput) ReceiptInput {
	out := in
	out.Capabilities = cloneCapabilities(in.Capabilities)
	out.CapabilitiesBytes = append([]byte(nil), in.CapabilitiesBytes...)
	out.Authorization.Authorization = cloneAuthorization(in.Authorization.Authorization)
	out.Authorization.Grants.Grants = append([]execworkspace.Grant(nil), in.Authorization.Grants.Grants...)
	out.CandidatePatches = make(map[string][]byte, len(in.CandidatePatches))
	for id, patch := range in.CandidatePatches {
		out.CandidatePatches[id] = append([]byte(nil), patch...)
	}
	out.Fingerprint = append([]byte(nil), in.Fingerprint...)
	out.Enforcement.Rows = append([]execworkspace.EnforcementReportRow(nil), in.Enforcement.Rows...)
	return out
}

func renderSchedule(schedule []ScheduledAttempt) string {
	cycles := make([]string, 0)
	for i := 0; i < len(schedule); {
		cycle := schedule[i].Cycle
		candidates := make([]string, 0)
		for i < len(schedule) && schedule[i].Cycle == cycle {
			candidates = append(candidates, schedule[i].Candidate)
			i++
		}
		cycles = append(cycles, string(cycle.Kind)+"-"+strconv.Itoa(cycle.Number)+":"+strings.Join(candidates, ","))
	}
	return strings.Join(cycles, ";")
}

func writeTestFile(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func digestText(text string) string { return testDigestBytes([]byte(text)) }

func testDigestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func pointer[T any](value T) *T { return &value }

func threshold(value float64) experiment.Threshold { return experiment.Threshold{Relative: &value} }
