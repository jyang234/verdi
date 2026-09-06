package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyconflict"
)

// --- fixture constants, verified against cmd/verdi/testdata/disposition-record/report.json ---

const (
	dispositionRecordFixtureInputID         = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	dispositionRecordFixtureAuthorityDigest = "sha256:7777777777777777777777777777777777777777777777777777777777777777"
	dispositionRecordFixtureClaim1ID        = "policy/example-policy#instruction-1"
	dispositionRecordFixtureClaim1Digest    = "sha256:9999999999999999999999999999999999999999999999999999999999999999"
	dispositionRecordFixtureClaim2ID        = "spec/example-story#ac-1"
	dispositionRecordFixtureClaim2Digest    = "sha256:8888888888888888888888888888888888888888888888888888888888888888"
	dispositionRecordFixtureApproverArg     = "policy-owner=principal/github-org/YWxpY2U"
	dispositionRecordFixtureApproverRole    = "policy-owner"
	dispositionRecordFixtureApproverID      = "principal/github-org/YWxpY2U"
)

// dispositionRecordFixtureReport returns the committed base fixture's raw
// bytes — a real verdi.policy-conflict-report/v1 document (copied from
// internal/policyconflict/testdata/report.json, this package's own
// hermetic copy per CLAUDE.md's "testdata/ is the only home for fixtures")
// whose one semantic row carries two claims sharing one authority digest,
// no primary/challenger judge exchange (a genuine human-fallback shape),
// and whose one mechanical row names no exemption.
func dispositionRecordFixtureReport(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "disposition-record", "report.json"))
	if err != nil {
		t.Fatalf("reading fixture report: %v", err)
	}
	// Self-check: the fixture is real, canonical policyconflict content —
	// never a hand-typed stand-in that merely looks like one.
	if _, err := policyconflict.DecodeReport(data); err != nil {
		t.Fatalf("test setup: fixture report does not strict-decode: %v", err)
	}
	return data
}

// writeDispositionRecordStoreRoot builds a minimal, real store root (the
// same bare "schema: verdi.layout/v1" manifest cmd/verdi/disposition_test.go's
// own writeDispositionStoreRoot uses) — `disposition record` touches only
// .verdi/policy/dispositions/, never git, never the target spec.
func writeDispositionRecordStoreRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, ".verdi", "verdi.yaml"), []byte("schema: verdi.layout/v1\n"))
	return root
}

// runDispositionRecordBinary execs the built verdi binary's "disposition
// record" verb with args, capturing stdout/stderr separately.
func runDispositionRecordBinary(t *testing.T, bin string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(bin, append([]string{"disposition", "record"}, args...)...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return outBuf.String(), errBuf.String(), ee.ExitCode()
		}
		t.Fatalf("running verdi disposition record %v: %v", args, err)
	}
	return outBuf.String(), errBuf.String(), 0
}

// dispositionRecordBaseArgs returns a complete, valid argument set against
// root and the fixture report at reportPath — every refusal test in the
// table below starts from a copy of this and mutates exactly one thing, so
// a refusal is provably attributable to that one change.
func dispositionRecordBaseArgs(root, reportPath, id string) []string {
	return []string{
		"--report", reportPath,
		"--row", dispositionRecordFixtureInputID,
		"--target-digest", dispositionRecordFixtureAuthorityDigest,
		"--conclusion", "no-conflict",
		"--compensating-control", "Human reviewed manually; no automated judge is configured.",
		"--expiry", "2099-12-31",
		"--approver", dispositionRecordFixtureApproverArg,
		"--id", id,
		"--title", "story-alpha claims coexist without conflict",
		"--owner", "platform-team",
		"--root", root,
	}
}

// TestCmdDispositionRecord_Positive drives the built binary end to end
// (obligation-shaped behavioral proof, mirroring disposition_test.go's own
// runDispositionBinary discipline): given a real policy-conflict report
// and a row selected by input_id, it writes one policy-disposition
// artifact whose witness fields are copied verbatim from the row, whose
// template identity/digest resolve through humanartifact.ResolveScaffold,
// whose origin is human-fallback (the fixture row carries no judgment),
// and whose remaining members come from the operands — then decodes and
// validates the written file through the frozen policyartifact decoder.
func TestCmdDispositionRecord_Positive(t *testing.T) {
	bin := buildVerdiBinary(t)
	root := writeDispositionRecordStoreRoot(t)
	reportPath := filepath.Join(root, "report.json")
	writeTestFile(t, reportPath, dispositionRecordFixtureReport(t))

	args := dispositionRecordBaseArgs(root, reportPath, "story-alpha-no-conflict")
	stdout, stderr, code := runDispositionRecordBinary(t, bin, args...)
	if code != 0 {
		t.Fatalf("verdi disposition record: exit = %d, want 0; stdout=%s stderr=%s", code, stdout, stderr)
	}

	destPath := filepath.Join(root, ".verdi", "policy", "dispositions", "story-alpha-no-conflict.md")
	raw, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("reading written disposition: %v", err)
	}
	d, err := policyartifact.DecodeDisposition(raw)
	if err != nil {
		t.Fatalf("DecodeDisposition on the written artifact: %v", err)
	}
	if err := d.Scope.Validate(); err != nil {
		t.Fatalf("written disposition scope does not validate: %v", err)
	}

	if d.ID != "policy-disposition/story-alpha-no-conflict" {
		t.Fatalf("ID = %q", d.ID)
	}
	if d.Title != "story-alpha claims coexist without conflict" {
		t.Fatalf("Title = %q", d.Title)
	}
	if len(d.Owners) != 1 || d.Owners[0] != "platform-team" {
		t.Fatalf("Owners = %v", d.Owners)
	}
	if d.Witness.InputID != dispositionRecordFixtureInputID {
		t.Fatalf("Witness.InputID = %q, want %q", d.Witness.InputID, dispositionRecordFixtureInputID)
	}
	if d.Witness.TargetDigest != dispositionRecordFixtureAuthorityDigest {
		t.Fatalf("Witness.TargetDigest = %q, want %q", d.Witness.TargetDigest, dispositionRecordFixtureAuthorityDigest)
	}
	if len(d.Witness.Claims) != 2 {
		t.Fatalf("Witness.Claims = %+v, want 2 entries copied verbatim from the report row", d.Witness.Claims)
	}
	if d.Witness.Claims[0].ID != dispositionRecordFixtureClaim1ID || d.Witness.Claims[0].Digest != dispositionRecordFixtureClaim1Digest {
		t.Fatalf("Witness.Claims[0] = %+v, want id/digest matching the report row's first claim", d.Witness.Claims[0])
	}
	if d.Witness.Claims[1].ID != dispositionRecordFixtureClaim2ID || d.Witness.Claims[1].Digest != dispositionRecordFixtureClaim2Digest {
		t.Fatalf("Witness.Claims[1] = %+v, want id/digest matching the report row's second claim", d.Witness.Claims[1])
	}
	if len(d.Witness.Exemptions) != 0 {
		t.Fatalf("Witness.Exemptions = %+v, want empty (the fixture's one mechanical row names none)", d.Witness.Exemptions)
	}
	if d.Conclusion != policyartifact.DispositionNoConflict {
		t.Fatalf("Conclusion = %q, want no-conflict", d.Conclusion)
	}
	if d.Origin != policyartifact.DispositionHumanFallback {
		t.Fatalf("Origin = %q, want human-fallback (the fixture row carries no judgment)", d.Origin)
	}
	if d.Judgment != nil {
		t.Fatalf("Judgment = %+v, want none", d.Judgment)
	}
	if len(d.CompensatingControls) != 1 || d.CompensatingControls[0] != "Human reviewed manually; no automated judge is configured." {
		t.Fatalf("CompensatingControls = %v", d.CompensatingControls)
	}
	wantApproval := policyartifact.Approval{Role: dispositionRecordFixtureApproverRole, Principal: dispositionRecordFixtureApproverID}
	if len(d.Approvals) != 1 || d.Approvals[0] != wantApproval {
		t.Fatalf("Approvals = %+v, want exactly [%+v]", d.Approvals, wantApproval)
	}
	if d.Expiry != "2099-12-31" {
		t.Fatalf("Expiry = %q", d.Expiry)
	}
	if d.Template == nil || d.Template.Identity == "" || d.Template.Digest == "" {
		t.Fatalf("Template = %+v, want a resolved identity/digest", d.Template)
	}
	if !strings.Contains(stdout, "story-alpha-no-conflict") {
		t.Fatalf("stdout = %q, want it to name the written artifact", stdout)
	}
}

// TestCmdDispositionRecord_MultipleRepeatables proves --compensating-control,
// --approver, and --owner each accept more than one occurrence and every
// occurrence lands in the written artifact.
func TestCmdDispositionRecord_MultipleRepeatables(t *testing.T) {
	bin := buildVerdiBinary(t)
	root := writeDispositionRecordStoreRoot(t)
	reportPath := filepath.Join(root, "report.json")
	writeTestFile(t, reportPath, dispositionRecordFixtureReport(t))

	args := []string{
		"--report", reportPath,
		"--row", dispositionRecordFixtureInputID,
		"--target-digest", dispositionRecordFixtureAuthorityDigest,
		"--conclusion", "no-conflict",
		"--compensating-control", "First control.",
		"--compensating-control", "Second control.",
		"--expiry", "2099-12-31",
		"--approver", "policy-owner=principal/github-org/YWxpY2U",
		"--approver", "security-owner=principal/github-org/Ym9i",
		"--id", "multi-repeatable",
		"--title", "Multi repeatable",
		"--owner", "platform-team",
		"--owner", "security-team",
		"--root", root,
	}
	stdout, stderr, code := runDispositionRecordBinary(t, bin, args...)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stdout=%s stderr=%s", code, stdout, stderr)
	}
	raw := readFile(t, filepath.Join(root, ".verdi", "policy", "dispositions", "multi-repeatable.md"))
	d, err := policyartifact.DecodeDisposition(raw)
	if err != nil {
		t.Fatalf("DecodeDisposition: %v", err)
	}
	if len(d.Owners) != 2 {
		t.Fatalf("Owners = %v, want 2", d.Owners)
	}
	if len(d.CompensatingControls) != 2 {
		t.Fatalf("CompensatingControls = %v, want 2", d.CompensatingControls)
	}
	if len(d.Approvals) != 2 {
		t.Fatalf("Approvals = %+v, want 2", d.Approvals)
	}
}

// TestCmdDispositionRecord_Refusals is table-driven over every named
// refusal (Task 3 contract): each is an operational exit (2), names the
// offending operand, and never writes a file.
func TestCmdDispositionRecord_Refusals(t *testing.T) {
	bin := buildVerdiBinary(t)

	// mutateReport returns the fixture bytes with fn applied — used by the
	// non-canonical-report case.
	mutateReport := func(t *testing.T, fn func([]byte) []byte) string {
		t.Helper()
		root := t.TempDir()
		path := filepath.Join(root, "report.json")
		writeTestFile(t, path, fn(dispositionRecordFixtureReport(t)))
		return path
	}

	tests := []struct {
		name       string
		setupRoot  func(t *testing.T) (root, reportPath string)
		mutateArgs func(root, reportPath string) []string
		wantSub    string
	}{
		{
			name: "report unreadable",
			setupRoot: func(t *testing.T) (string, string) {
				return writeDispositionRecordStoreRoot(t), filepath.Join(t.TempDir(), "does-not-exist.json")
			},
			mutateArgs: func(root, reportPath string) []string {
				return dispositionRecordBaseArgs(root, reportPath, "x")
			},
			wantSub: "--report",
		},
		{
			name: "report non-canonical",
			setupRoot: func(t *testing.T) (string, string) {
				root := writeDispositionRecordStoreRoot(t)
				path := mutateReport(t, func(b []byte) []byte { return append(append([]byte(nil), b...), '\n', '\n') })
				return root, path
			},
			mutateArgs: func(root, reportPath string) []string {
				return dispositionRecordBaseArgs(root, reportPath, "x")
			},
			wantSub: "--report",
		},
		{
			name: "row not found",
			setupRoot: func(t *testing.T) (string, string) {
				root := writeDispositionRecordStoreRoot(t)
				path := filepath.Join(root, "report.json")
				writeTestFile(t, path, dispositionRecordFixtureReport(t))
				return root, path
			},
			mutateArgs: func(root, reportPath string) []string {
				args := dispositionRecordBaseArgs(root, reportPath, "x")
				return replaceArgValue(args, "--row", "sha256:"+strings.Repeat("0", 64))
			},
			wantSub: "sha256:" + strings.Repeat("0", 64),
		},
		{
			name: "conclusion outside the closed set",
			setupRoot: func(t *testing.T) (string, string) {
				root := writeDispositionRecordStoreRoot(t)
				path := filepath.Join(root, "report.json")
				writeTestFile(t, path, dispositionRecordFixtureReport(t))
				return root, path
			},
			mutateArgs: func(root, reportPath string) []string {
				args := dispositionRecordBaseArgs(root, reportPath, "x")
				return replaceArgValue(args, "--conclusion", "maybe")
			},
			wantSub: "conclusion",
		},
		{
			name: "no compensating control",
			setupRoot: func(t *testing.T) (string, string) {
				root := writeDispositionRecordStoreRoot(t)
				path := filepath.Join(root, "report.json")
				writeTestFile(t, path, dispositionRecordFixtureReport(t))
				return root, path
			},
			mutateArgs: func(root, reportPath string) []string {
				return removeArg(dispositionRecordBaseArgs(root, reportPath, "x"), "--compensating-control")
			},
			wantSub: "compensating-control",
		},
		{
			name: "no approver",
			setupRoot: func(t *testing.T) (string, string) {
				root := writeDispositionRecordStoreRoot(t)
				path := filepath.Join(root, "report.json")
				writeTestFile(t, path, dispositionRecordFixtureReport(t))
				return root, path
			},
			mutateArgs: func(root, reportPath string) []string {
				return removeArg(dispositionRecordBaseArgs(root, reportPath, "x"), "--approver")
			},
			wantSub: "approver",
		},
		{
			name: "malformed approver: no equals sign",
			setupRoot: func(t *testing.T) (string, string) {
				root := writeDispositionRecordStoreRoot(t)
				path := filepath.Join(root, "report.json")
				writeTestFile(t, path, dispositionRecordFixtureReport(t))
				return root, path
			},
			mutateArgs: func(root, reportPath string) []string {
				args := dispositionRecordBaseArgs(root, reportPath, "x")
				return replaceArgValue(args, "--approver", "policy-owner-no-equals")
			},
			wantSub: "approver",
		},
		{
			name: "malformed approver: invalid principal",
			setupRoot: func(t *testing.T) (string, string) {
				root := writeDispositionRecordStoreRoot(t)
				path := filepath.Join(root, "report.json")
				writeTestFile(t, path, dispositionRecordFixtureReport(t))
				return root, path
			},
			mutateArgs: func(root, reportPath string) []string {
				args := dispositionRecordBaseArgs(root, reportPath, "x")
				return replaceArgValue(args, "--approver", "policy-owner=not-a-principal")
			},
			wantSub: "approver",
		},
		{
			name: "malformed expiry",
			setupRoot: func(t *testing.T) (string, string) {
				root := writeDispositionRecordStoreRoot(t)
				path := filepath.Join(root, "report.json")
				writeTestFile(t, path, dispositionRecordFixtureReport(t))
				return root, path
			},
			mutateArgs: func(root, reportPath string) []string {
				args := dispositionRecordBaseArgs(root, reportPath, "x")
				return replaceArgValue(args, "--expiry", "not-a-date")
			},
			wantSub: "expiry",
		},
		{
			name: "existing artifact at the id refuses to overwrite",
			setupRoot: func(t *testing.T) (string, string) {
				root := writeDispositionRecordStoreRoot(t)
				path := filepath.Join(root, "report.json")
				writeTestFile(t, path, dispositionRecordFixtureReport(t))
				dest := filepath.Join(root, ".verdi", "policy", "dispositions", "already-exists.md")
				writeTestFile(t, dest, []byte("pre-existing content\n"))
				return root, path
			},
			mutateArgs: func(root, reportPath string) []string {
				return dispositionRecordBaseArgs(root, reportPath, "already-exists")
			},
			wantSub: "already-exists",
		},
		{
			name: "operand makes the witness differ from the report: target-digest matches no claim",
			setupRoot: func(t *testing.T) (string, string) {
				root := writeDispositionRecordStoreRoot(t)
				path := filepath.Join(root, "report.json")
				writeTestFile(t, path, dispositionRecordFixtureReport(t))
				return root, path
			},
			mutateArgs: func(root, reportPath string) []string {
				args := dispositionRecordBaseArgs(root, reportPath, "x")
				return replaceArgValue(args, "--target-digest", "sha256:"+strings.Repeat("f", 64))
			},
			wantSub: "target-digest",
		},
		{
			name: "unknown flag",
			setupRoot: func(t *testing.T) (string, string) {
				root := writeDispositionRecordStoreRoot(t)
				path := filepath.Join(root, "report.json")
				writeTestFile(t, path, dispositionRecordFixtureReport(t))
				return root, path
			},
			mutateArgs: func(root, reportPath string) []string {
				return append(dispositionRecordBaseArgs(root, reportPath, "x"), "--bogus-flag", "z")
			},
			wantSub: "bogus-flag",
		},
		{
			name: "missing --report entirely",
			setupRoot: func(t *testing.T) (string, string) {
				return writeDispositionRecordStoreRoot(t), ""
			},
			mutateArgs: func(root, reportPath string) []string {
				args := dispositionRecordBaseArgs(root, "unused.json", "x")
				return removeArg(args, "--report")
			},
			wantSub: "report",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root, reportPath := tc.setupRoot(t)
			destDir := filepath.Join(root, ".verdi", "policy", "dispositions")
			before := snapshotDir(t, destDir)

			args := tc.mutateArgs(root, reportPath)
			_, stderr, code := runDispositionRecordBinary(t, bin, args...)
			if code != 2 {
				t.Fatalf("exit = %d, want 2 (operational); stderr=%s", code, stderr)
			}
			if !strings.Contains(stderr, tc.wantSub) {
				t.Fatalf("stderr = %q, want it to contain %q", stderr, tc.wantSub)
			}
			after := snapshotDir(t, destDir)
			if strings.Join(before, "\x00") != strings.Join(after, "\x00") {
				t.Fatalf("dispositions directory changed despite a refusal:\nbefore=%v\nafter=%v", before, after)
			}
		})
	}
}

// replaceArgValue returns a copy of args with flag's value (the token
// immediately following the LAST occurrence of flag) replaced by value —
// every base-args table case above uses exactly one occurrence per flag,
// except --approver/--owner/--compensating-control, which this helper is
// never used to target.
func replaceArgValue(args []string, flag, value string) []string {
	out := append([]string(nil), args...)
	for i := len(out) - 2; i >= 0; i-- {
		if out[i] == flag {
			out[i+1] = value
			return out
		}
	}
	return out
}

// removeArg returns a copy of args with the first occurrence of flag and
// its following value removed entirely.
func removeArg(args []string, flag string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if args[i] == flag {
			i++ // also skip its value
			continue
		}
		out = append(out, args[i])
	}
	return out
}

// snapshotDir returns the sorted names of every file directly under dir
// (which may not exist, yielding nil) — used to prove a refusal writes
// nothing.
func snapshotDir(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("reading dir %s: %v", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// --- direct unit tests for disposition_record.go's pure helper functions ---

func TestDispositionOrigin(t *testing.T) {
	tests := []struct {
		name string
		row  policyconflict.SemanticEvaluation
		want string
	}{
		{"no primary judgment: human-fallback", policyconflict.SemanticEvaluation{}, string(policyartifact.DispositionHumanFallback)},
		{"primary judgment present: judge-result", policyconflict.SemanticEvaluation{Primary: &policyconflict.JudgmentExchange{}}, string(policyartifact.DispositionJudgeResult)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := dispositionOrigin(tc.row); got != tc.want {
				t.Fatalf("dispositionOrigin() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFindSemanticRow(t *testing.T) {
	report := policyconflict.Report{Semantic: []policyconflict.SemanticEvaluation{
		{InputID: "sha256:aaaa"},
		{InputID: "sha256:bbbb"},
	}}
	t.Run("found", func(t *testing.T) {
		row, ok := findSemanticRow(report, "sha256:bbbb")
		if !ok || row.InputID != "sha256:bbbb" {
			t.Fatalf("findSemanticRow = %+v, %v", row, ok)
		}
	})
	t.Run("not found", func(t *testing.T) {
		_, ok := findSemanticRow(report, "sha256:cccc")
		if ok {
			t.Fatal("findSemanticRow: want not found")
		}
	})
}

func TestTargetDigestMatchesAnyClaim(t *testing.T) {
	claims := []policyartifact.SemanticClaimWitness{
		{ID: "a", AuthorityDigest: "sha256:" + strings.Repeat("1", 64)},
		{ID: "b", AuthorityDigest: "sha256:" + strings.Repeat("2", 64)},
	}
	tests := []struct {
		name   string
		claims []policyartifact.SemanticClaimWitness
		digest string
		want   bool
	}{
		{"matches one claim", claims, "sha256:" + strings.Repeat("2", 64), true},
		{"matches no claim", claims, "sha256:" + strings.Repeat("9", 64), false},
		{"no claims to check against: vacuously true", nil, "sha256:" + strings.Repeat("9", 64), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := targetDigestMatchesAnyClaim(tc.claims, tc.digest); got != tc.want {
				t.Fatalf("targetDigestMatchesAnyClaim() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestReconstructApplicableExemptions(t *testing.T) {
	tests := []struct {
		name       string
		mechanical []policyconflict.MechanicalEvaluation
		want       []dispositionExemption
		wantErr    bool
	}{
		{"no mechanical rows", nil, nil, false},
		{"rows with no exemptions", []policyconflict.MechanicalEvaluation{{ID: "m1"}}, nil, false},
		{
			"one exemption on one row",
			[]policyconflict.MechanicalEvaluation{{ID: "m1", Exemptions: []policyconflict.ExemptionResolution{{ID: "policy-exemption/e1", Digest: "sha256:" + strings.Repeat("1", 64)}}}},
			[]dispositionExemption{{ID: "policy-exemption/e1", Digest: "sha256:" + strings.Repeat("1", 64)}},
			false,
		},
		{
			"same exemption on two rows dedups and sorts by id",
			[]policyconflict.MechanicalEvaluation{
				{ID: "m2", Exemptions: []policyconflict.ExemptionResolution{{ID: "policy-exemption/e2", Digest: "sha256:" + strings.Repeat("2", 64)}}},
				{ID: "m1", Exemptions: []policyconflict.ExemptionResolution{
					{ID: "policy-exemption/e1", Digest: "sha256:" + strings.Repeat("1", 64)},
					{ID: "policy-exemption/e2", Digest: "sha256:" + strings.Repeat("2", 64)},
				}},
			},
			[]dispositionExemption{
				{ID: "policy-exemption/e1", Digest: "sha256:" + strings.Repeat("1", 64)},
				{ID: "policy-exemption/e2", Digest: "sha256:" + strings.Repeat("2", 64)},
			},
			false,
		},
		{
			"same exemption id with two different digests is an inconsistent report",
			[]policyconflict.MechanicalEvaluation{
				{ID: "m1", Exemptions: []policyconflict.ExemptionResolution{{ID: "policy-exemption/e1", Digest: "sha256:" + strings.Repeat("1", 64)}}},
				{ID: "m2", Exemptions: []policyconflict.ExemptionResolution{{ID: "policy-exemption/e1", Digest: "sha256:" + strings.Repeat("2", 64)}}},
			},
			nil,
			true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := reconstructApplicableExemptions(tc.mechanical)
			if tc.wantErr {
				if err == nil {
					t.Fatal("want an error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("reconstructApplicableExemptions() = %+v, want %+v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("reconstructApplicableExemptions()[%d] = %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestParseApprover(t *testing.T) {
	tests := []struct {
		name       string
		in         string
		wantRole   string
		wantPrinID string
		wantErr    bool
	}{
		{"valid", "policy-owner=principal/github-org/YWxpY2U", "policy-owner", "principal/github-org/YWxpY2U", false},
		{"no equals sign", "policy-owner-no-equals", "", "", true},
		{"empty role", "=principal/github-org/YWxpY2U", "", "", true},
		{"empty principal", "policy-owner=", "", "", true},
		{"malformed principal", "policy-owner=not-a-principal", "", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			role, principal, err := parseApprover(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseApprover(%q): want an error, got role=%q principal=%q", tc.in, role, principal)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseApprover(%q): unexpected error: %v", tc.in, err)
			}
			if role != tc.wantRole || principal != tc.wantPrinID {
				t.Fatalf("parseApprover(%q) = %q, %q, want %q, %q", tc.in, role, principal, tc.wantRole, tc.wantPrinID)
			}
		})
	}
}
