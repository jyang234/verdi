package lintratchet

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
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

// What each test that needs the pinned golangci-lint cannot show without
// it, as its skip reason says (re-review finding S1-RR3).
const (
	findingsUnshown = "this run cannot show that the strict configuration reports each gated linter's violation"
	captureUnshown  = "this run cannot check that the committed capture testdata/reports/strictfixture.json is still what that golangci-lint reports over the strict fixture module"
	probeUnshown    = "this run cannot show which //nolint shapes in the probe module suppress a gated finding"
)

// nolintProbeDir is the //nolint probe module, relative to the repository
// root: each file but doc.go holds one gochecknoglobals violation under one
// directive shape, and is named suppressed_*.go when the pinned golangci-lint
// is expected to suppress that finding, reported_*.go when it is expected to
// report it. (A comment line of this package must not begin with that
// directive's name, which golangci-lint would read as a bare directive.)
const nolintProbeDir = "internal/lintratchet/testdata/nolintprobe"

// pinnedGolangciLint returns the path of golangci-lint at the Makefile's
// pinned version, and skips the test when that binary is absent (not on
// PATH, or on PATH at another version), with a reason ending in unshown, the
// claim the run therefore cannot show. CI's test jobs do not install it; CI
// job verify does (ledger SI-309), where a skip is recorded as abstain,
// never as a pass (spec/strict-lint-gate dc-3).
func pinnedGolangciLint(t *testing.T, root, unshown string) string {
	t.Helper()
	pin := makefileLintPin(t, root)
	bin, err := exec.LookPath("golangci-lint")
	if err != nil {
		t.Skipf("SKIP (disclosed, not a pass): the Makefile's pinned golangci-lint v%s is absent (%v), so %s", pin, err, unshown)
	}
	out, err := exec.CommandContext(t.Context(), bin, "version").CombinedOutput()
	if err != nil {
		t.Fatalf("%s version: %v\n%s", bin, err, out)
	}
	m := regexp.MustCompile(`version v?(\d+\.\d+\.\d+)`).FindSubmatch(out)
	if m == nil || string(m[1]) != pin {
		t.Skipf("SKIP (disclosed, not a pass): the Makefile's pinned golangci-lint v%s is absent: %s reports %q, so %s", pin, bin, bytes.TrimSpace(out), unshown)
	}
	return bin
}

// strictLintRun returns the command that runs the pinned golangci-lint bin
// as make lint-strict runs it: with the strict configuration at config, for
// linux/amd64, and with --issues-exit-code=0, so a reported finding is data.
// It lints patterns from dir, offline (GOPROXY=off, GOTOOLCHAIN=local), and
// writes its JSON report to report. golangci-lint reports paths relative to
// config's directory.
func strictLintRun(ctx context.Context, bin, dir, config, report string, patterns ...string) *exec.Cmd {
	args := append([]string{"run",
		"--config", config,
		"--issues-exit-code=0",
		// Another golangci-lint holding its lock (a concurrent make lint)
		// would otherwise end this run with exit 3; this flag changes no
		// finding.
		"--allow-parallel-runners",
		"--output.json.path=" + report,
	}, patterns...)
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "GOFLAGS=", "GOWORK=off", "GOPROXY=off", "GOTOOLCHAIN=local")
	return cmd
}

// lintStrictFixture runs the pinned golangci-lint with .golangci.strict.yml,
// for linux/amd64 and with --issues-exit-code=0 as make lint-strict runs it,
// over the committed fixture module at dir, relative to the repository root,
// and returns its findings. It skips the test when the pinned binary is
// absent, saying the run cannot show unshown.
func lintStrictFixture(t *testing.T, dir, unshown string) []Finding {
	t.Helper()
	root := repoRoot(t)
	bin := pinnedGolangciLint(t, root, unshown)

	report := filepath.Join(t.TempDir(), "report.json")
	cmd := strictLintRun(t.Context(), bin, filepath.Join(root, filepath.FromSlash(dir)), filepath.Join(root, ".golangci.strict.yml"), report, "./...")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("golangci-lint run over %s: %v\n%s", dir, err, out)
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
	findings := lintStrictFixture(t, strictFixtureDir, findingsUnshown)

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
	live := lintStrictFixture(t, strictFixtureDir, captureUnshown)
	captured, err := ParseReport(mustRead(t, capturedReport("strictfixture")))
	if err != nil {
		t.Fatalf("parsing the captured strict fixture report: %v", err)
	}
	if !slices.Equal(live, captured) {
		t.Fatalf("the pinned golangci-lint reports over %s:\n%+v\nbut %s holds:\n%+v\nre-capture it with testdata/capture/capture.sh strictfixture", strictFixtureDir, live, capturedReport("strictfixture"), captured)
	}
}

// TestLintStrict_NolintProbe runs the pinned golangci-lint with
// .golangci.strict.yml, as make lint-strict runs it, over the //nolint probe
// module, and proves which directive shapes suppress a gated finding there
// (spec/strict-lint-gate-v2 ac-3, dc-4): a bare //nolint, the same after a
// space, //nolint:all, //nolint:unused,all, and //nolint:gochecknoglobals
// each suppress the gochecknoglobals finding beside them, so the witness's
// refusal of the first four follows real suppression; //nolint:unused and a
// mid-comment mention suppress nothing gated. Every finding must be
// gochecknoglobals' in a reported_*.go file, exactly one in each, and none in
// a suppressed_*.go file.
func TestLintStrict_NolintProbe(t *testing.T) {
	findings := lintStrictFixture(t, nolintProbeDir, probeUnshown)

	entries, err := os.ReadDir(filepath.Join(repoRoot(t), filepath.FromSlash(nolintProbeDir)))
	if err != nil {
		t.Fatalf("reading the probe module: %v", err)
	}
	want := map[string]int{} // findings per file
	for _, e := range entries {
		name := e.Name()
		switch {
		case e.IsDir() || !strings.HasSuffix(name, ".go") || name == "doc.go":
		case strings.HasPrefix(name, "reported_"):
			want[name] = 1
		case strings.HasPrefix(name, "suppressed_"):
			want[name] = 0
		default:
			t.Fatalf("probe file %s is named neither suppressed_*.go nor reported_*.go", name)
		}
	}
	got := map[string]int{}
	for _, f := range findings {
		if path.Dir(f.File) != nolintProbeDir || f.Key.Linter != "gochecknoglobals" {
			t.Errorf("finding outside the probe's gochecknoglobals findings: %+v", f)
			continue
		}
		got[path.Base(f.File)]++
	}
	for name, n := range want {
		if got[name] != n {
			t.Errorf("%s: golangci-lint reported %d gochecknoglobals finding(s), want %d", name, got[name], n)
		}
	}
	for name := range got {
		if _, ok := want[name]; !ok {
			t.Errorf("%s: a finding in a file that is not a probe file", name)
		}
	}
}

// TestPinnedLintSkipReasons re-runs, in this test binary and with no
// golangci-lint on PATH, each test that needs the pinned golangci-lint, and
// proves each skips naming its own claim the run then cannot show, never
// another's (re-review finding S1-RR3).
func TestPinnedLintSkipReasons(t *testing.T) {
	const (
		findingsReason = "so this run cannot show that the strict configuration reports each gated linter's violation"
		captureReason  = "so this run cannot check that the committed capture testdata/reports/strictfixture.json is still what that golangci-lint reports over the strict fixture module"
		probeReason    = "so this run cannot show which //nolint shapes in the probe module suppress a gated finding"
	)
	noLint := t.TempDir()
	cases := []struct {
		test, want string
		notWant    []string
	}{
		{test: "TestLintStrict_ReportsGroundRuleFindings", want: findingsReason, notWant: []string{captureReason, probeReason}},
		{test: "TestStrictFixtureReportIsCurrent", want: captureReason, notWant: []string{findingsReason, probeReason}},
		{test: "TestLintStrict_NolintProbe", want: probeReason, notWant: []string{findingsReason, captureReason}},
	}
	for _, tc := range cases {
		t.Run(tc.test, func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^"+tc.test+"$", "-test.v", "-test.count=1")
			cmd.Env = append(os.Environ(), "PATH="+noLint)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("re-running %s: %v\n%s", tc.test, err, out)
			}
			got := string(out)
			if !strings.Contains(got, "--- SKIP: "+tc.test) {
				t.Fatalf("%s did not skip without golangci-lint on PATH:\n%s", tc.test, got)
			}
			if !strings.Contains(got, tc.want) || slices.ContainsFunc(tc.notWant, func(r string) bool { return strings.Contains(got, r) }) {
				t.Fatalf("%s's skip reason does not say %q alone:\n%s", tc.test, tc.want, got)
			}
		})
	}
}
