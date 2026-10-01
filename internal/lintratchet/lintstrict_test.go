package lintratchet

import (
	"bytes"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
)

// strictFixtureDir is the committed fixture module, relative to the
// repository root: one file per gated linter, named after it, holding that
// linter's one violation, and clean.go, holding none.
const strictFixtureDir = "internal/lintratchet/testdata/strictfixture"

// repoRoot returns the repository root: go test runs a package's tests in
// the package's own directory, two levels below it.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolving the repository root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("%s is not the repository root: %v", root, err)
	}
	return root
}

// makefileLintPin returns the Makefile's GOLANGCI_LINT_VERSION, the one pin
// both lint targets run (strict-lint-target-v2 co-2).
func makefileLintPin(t *testing.T, root string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		t.Fatalf("reading the Makefile: %v", err)
	}
	m := regexp.MustCompile(`(?m)^GOLANGCI_LINT_VERSION[ \t]*\??=[ \t]*v?(\S+)[ \t]*$`).FindSubmatch(raw)
	if m == nil {
		t.Fatal("the Makefile assigns no GOLANGCI_LINT_VERSION")
	}
	return string(m[1])
}

// pinnedGolangciLint returns the path of golangci-lint at the Makefile's
// pinned version, and skips the test with the reason when that binary is
// absent: not on PATH, or on PATH at another version. CI's test jobs do not
// install it; CI job verify does (ledger SI-309), where a skip is recorded as
// abstain, never as a pass (spec/strict-lint-gate dc-3).
func pinnedGolangciLint(t *testing.T, root string) string {
	t.Helper()
	pin := makefileLintPin(t, root)
	bin, err := exec.LookPath("golangci-lint")
	if err != nil {
		t.Skipf("SKIP (disclosed, not a pass): the Makefile's pinned golangci-lint v%s is absent (%v), so this run cannot show that the strict configuration reports each gated linter's violation", pin, err)
	}
	out, err := exec.CommandContext(t.Context(), bin, "version").CombinedOutput()
	if err != nil {
		t.Fatalf("%s version: %v\n%s", bin, err, out)
	}
	m := regexp.MustCompile(`version v?(\d+\.\d+\.\d+)`).FindSubmatch(out)
	if m == nil || string(m[1]) != pin {
		t.Skipf("SKIP (disclosed, not a pass): the Makefile's pinned golangci-lint v%s is absent: %s reports %q, so this run cannot show that the strict configuration reports each gated linter's violation", pin, bin, bytes.TrimSpace(out))
	}
	return bin
}

// lintStrictFixture runs the pinned golangci-lint with .golangci.strict.yml,
// for linux/amd64 and with --issues-exit-code=0 as make lint-strict runs it,
// over the committed fixture module, and returns its findings. It skips the
// test with the reason when the pinned binary is absent.
func lintStrictFixture(t *testing.T) []Finding {
	t.Helper()
	root := repoRoot(t)
	bin := pinnedGolangciLint(t, root)

	report := filepath.Join(t.TempDir(), "report.json")
	cmd := exec.CommandContext(t.Context(), bin, "run",
		"--config", filepath.Join(root, ".golangci.strict.yml"),
		"--issues-exit-code=0",
		// Another golangci-lint holding its lock (a concurrent make lint)
		// would otherwise end this run with exit 3; this flag changes no
		// finding.
		"--allow-parallel-runners",
		"--output.json.path="+report,
		"./...")
	cmd.Dir = filepath.Join(root, filepath.FromSlash(strictFixtureDir))
	cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "GOFLAGS=", "GOWORK=off", "GOPROXY=off", "GOTOOLCHAIN=local")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("golangci-lint run over %s: %v\n%s", strictFixtureDir, err, out)
	}
	data, err := os.ReadFile(report)
	if err != nil {
		t.Fatalf("reading the report: %v", err)
	}
	findings, err := ParseReport(data)
	if err != nil {
		t.Fatalf("ParseReport: %v", err)
	}
	return findings
}

// TestLintStrict_ReportsGroundRuleFindings runs the pinned golangci-lint with
// .golangci.strict.yml, for linux/amd64 and with --issues-exit-code=0 as make
// lint-strict runs it, over the committed fixture module, and proves the
// strict configuration reports exactly one finding per gated linter, each in
// that linter's own file, and none in the clean file (spec/strict-lint-gate
// ac-1). A gated linter's violation going unreported, a finding in the clean
// file, or any extra finding fails it.
func TestLintStrict_ReportsGroundRuleFindings(t *testing.T) {
	findings := lintStrictFixture(t)

	gated := []string{"containedctx", "contextcheck", "errorlint", "gochecknoglobals", "noctx"}
	byLinter := map[string][]Finding{}
	for _, f := range findings {
		if path.Dir(f.File) != strictFixtureDir {
			t.Errorf("finding outside the fixture module: %+v", f)
		}
		if path.Base(f.File) == "clean.go" {
			t.Errorf("finding in the clean file: %+v", f)
		}
		if !slices.Contains(gated, f.Key.Linter) {
			t.Errorf("finding from %s, which is not a gated linter: %+v", f.Key.Linter, f)
		}
		byLinter[f.Key.Linter] = append(byLinter[f.Key.Linter], f)
	}
	for _, linter := range gated {
		got := byLinter[linter]
		if len(got) != 1 {
			t.Errorf("%s reported %d findings, want exactly 1: %+v", linter, len(got), got)
			continue
		}
		if want := linter + ".go"; path.Base(got[0].File) != want {
			t.Errorf("%s's finding is in %s, want its own fixture file %s: %+v", linter, got[0].File, want, got[0])
		}
	}
	if len(findings) != len(gated) {
		t.Errorf("the strict configuration reported %d findings over the fixture module, want exactly %d (one per gated linter): %+v", len(findings), len(gated), findings)
	}
}

// TestStrictFixtureReportIsCurrent proves the committed capture of the strict
// fixture module's report, testdata/reports/strictfixture.json, is what the
// pinned golangci-lint reports over that module today: the same findings,
// keys and positions alike. internal/specalign's
// TestStrictLintExclusionsCounted reads each gated linter's message from that
// capture to tell a wholesale text exclusion from a narrow one (review finding
// S1-B1), so a stale capture would measure exclusions against messages the
// linter no longer prints. capture.sh strictfixture re-captures it.
func TestStrictFixtureReportIsCurrent(t *testing.T) {
	live := lintStrictFixture(t)
	captured, err := ParseReport(mustRead(t, capturedReport("strictfixture")))
	if err != nil {
		t.Fatalf("parsing the captured strict fixture report: %v", err)
	}
	if !slices.Equal(live, captured) {
		t.Fatalf("the pinned golangci-lint reports over %s:\n%+v\nbut %s holds:\n%+v\nre-capture it with testdata/capture/capture.sh strictfixture", strictFixtureDir, live, capturedReport("strictfixture"), captured)
	}
}
