// E2E shard guard (SI-268, owner directive 2026-09-25, BL-73). The Playwright
// suite runs as three make targets, e2e-1, e2e-2, and e2e-3, each its own
// pull-request CI job with its own fresh store. Shards 1 and 2 name their spec
// files explicitly (E2E_SHARD_1 and E2E_SHARD_2); shard 3 runs every spec file
// under e2e/tests/ that the other two do not name (E2E_SHARD_3, which make
// computes), so a new spec file always runs. Each target passes its list to
// Playwright as VERDI_E2E_SPECS, which e2e/playwright.config.ts turns into an
// exact testMatch. `make e2e` runs the three shards concurrently, as a
// convenience outside the gate.
//
// This file proves, from each target's `make -n` output rather than a
// hand-copied list, that:
//
//   - the three shards partition e2e/tests/*.spec.ts: every spec file runs in
//     exactly one shard, and a listed name that is not a spec file, is not in
//     e2e/tests/, or is listed twice fails, as do an empty shard and the
//     sentinel make yields for an empty remainder;
//   - no file that Playwright would collect lies outside that domain: a nested
//     or *.test.ts file would run in the whole suite and in no shard;
//   - each shard has its own port range and output directory, and installs
//     its setup exactly once, before Playwright runs;
//   - `make e2e` runs exactly the three shard commands, and fails with a
//     per-shard summary when any shard fails;
//   - VERIFY_STEPS lists the three shards and not `e2e`.
//
// What it cannot prove without Node is that playwright.config.ts honors
// VERDI_E2E_SPECS: TestE2EShards_ConfigNamesTheSpecList pins only that the
// config reads the variable. The witness that the selection is exact is each
// shard's own `npx playwright test --list`.
package specalign

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// e2eSpecsEnvVar is the variable each e2e shard target passes its spec file
// list in; e2e/playwright.config.ts reads it.
const e2eSpecsEnvVar = "VERDI_E2E_SPECS"

// e2eSuiteTarget is the whole-suite convenience target. It runs the three
// shards concurrently and is not a gate step.
const e2eSuiteTarget = "e2e"

// e2eShardTargets are the e2e gate steps, in shard order.
var e2eShardTargets = []string{"e2e-1", "e2e-2", "e2e-3"}

// e2ePortSpan is how many ports one shard's harness binds from its
// VERDI_E2E_PORT_BASE: workbench, dex, control, and inspection, base to base+3
// (cmd/e2eharness/ports.go, e2e/ports.ts).
const e2ePortSpan = 4

// e2eShardRun is one Playwright shard command found in a make dry-run.
type e2eShardRun struct {
	PortBase int
	Specs    []string
	Output   string
}

// namedE2ERun is a shard command together with the make target that runs it.
type namedE2ERun struct {
	Target string
	Run    e2eShardRun
}

// e2eShardRunRE matches one shard command exactly as the Makefile's e2e_shard
// template spells it: the port base, the quoted spec list, and the output
// directory.
var e2eShardRunRE = regexp.MustCompile(`VERDI_E2E_PORT_BASE=(\S*) ` + e2eSpecsEnvVar + `='([^']*)' npx playwright test --output=(\S+)`)

// playwrightTestRE finds any Playwright test run in a dry-run transcript,
// whatever its shape.
var playwrightTestRE = regexp.MustCompile(`playwright[ \t]+test\b`)

// e2eSpecNameRE is a spec file name a shard may list: a *.spec.ts file name
// of letters, digits, dots, dashes, and underscores, with no directory. The
// restricted alphabet keeps the name a literal in Playwright's testMatch.
var e2eSpecNameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*\.spec\.ts$`)

// playwrightTestFileRE is Playwright's default testMatch,
// `**/*.@(spec|test).?(c|m)[jt]s?(x)`, as a regexp over a file's base name.
var playwrightTestFileRE = regexp.MustCompile(`\.(spec|test)\.(c|m)?[jt]sx?$`)

// parseE2EShardRuns returns every shard command in a make dry-run transcript,
// in order. Every Playwright test run in the transcript must have the shard
// command's shape: one that does not (no spec list, say, which runs the whole
// suite) is an error, as is a spec list the shell computes at recipe time, a
// port base that is not an integer, or a spec list that is not a shard's.
func parseE2EShardRuns(dry string) ([]e2eShardRun, error) {
	locs := e2eShardRunRE.FindAllStringSubmatchIndex(dry, -1)
	for _, p := range playwrightTestRE.FindAllStringIndex(dry, -1) {
		if !slices.ContainsFunc(locs, func(loc []int) bool { return loc[0] <= p[0] && p[1] <= loc[1] }) {
			return nil, fmt.Errorf("a Playwright test run does not have the shard command's shape (VERDI_E2E_PORT_BASE=<n> %s='<spec files>' npx playwright test --output=<dir>), so which spec files it runs cannot be read: %q", e2eSpecsEnvVar, lineAround(dry, p[0]))
		}
	}
	runs := make([]e2eShardRun, 0, len(locs))
	for _, loc := range locs {
		portText, specs, output := dry[loc[2]:loc[3]], dry[loc[4]:loc[5]], dry[loc[6]:loc[7]]
		if strings.ContainsAny(specs, "$`") {
			return nil, fmt.Errorf("the spec list %q is computed by the shell at recipe time, so a make dry-run cannot see which files it names; compute it in make", specs)
		}
		port, err := strconv.Atoi(portText)
		if err != nil {
			return nil, fmt.Errorf("the port base %q is not an integer", portText)
		}
		runs = append(runs, e2eShardRun{PortBase: port, Specs: strings.Fields(specs), Output: output})
	}
	return runs, nil
}

// lineAround returns the line of s that holds offset i, for error messages.
func lineAround(s string, i int) string {
	start := strings.LastIndexByte(s[:i], '\n') + 1
	end := len(s)
	if n := strings.IndexByte(s[i:], '\n'); n >= 0 {
		end = i + n
	}
	return strings.TrimSpace(s[start:end])
}

// e2eSpecDomain splits files, every file under e2e/tests/ as a slash-separated
// path relative to it, into the shard domain, which is the spec files directly
// under e2e/tests/, sorted, and a problem for each other file that
// Playwright's default testMatch would collect: the whole suite would run it,
// and no shard could select it. An empty domain is a problem too.
func e2eSpecDomain(files []string) (specs, problems []string) {
	for _, f := range files {
		switch {
		case !strings.Contains(f, "/") && e2eSpecNameRE.MatchString(f):
			specs = append(specs, f)
		case playwrightTestFileRE.MatchString(path.Base(f)):
			problems = append(problems, fmt.Sprintf("e2e/tests/%s is a test file Playwright collects when the suite runs whole, but it is not a *.spec.ts file (named only by letters, digits, dots, dashes, and underscores) directly under e2e/tests/, so no e2e shard can select it", f))
		}
	}
	slices.Sort(specs)
	if len(specs) == 0 {
		problems = append(problems, "e2e/tests/ holds no *.spec.ts file, so every e2e shard would pass over nothing")
	}
	return specs, problems
}

// e2eShardProblems returns every way runs, the shard commands in shard order,
// fail to partition specs, the shard domain: a shard that lists no file; a
// listed name that is not a spec file name (make's empty-remainder sentinel
// among them) or is not in the domain; a file listed twice or in two shards; a
// file in no shard; port ranges that overlap or leave the valid range; and two
// shards sharing an output directory.
func e2eShardProblems(runs []namedE2ERun, specs []string) []string {
	var problems []string
	owner := map[string]string{}
	outputs := map[string]string{}
	for i, r := range runs {
		if len(r.Run.Specs) == 0 {
			problems = append(problems, fmt.Sprintf("%s lists no spec file", r.Target))
		}
		for _, name := range r.Run.Specs {
			switch prev, dup := owner[name]; {
			case !e2eSpecNameRE.MatchString(name):
				problems = append(problems, fmt.Sprintf("%s lists %q, which is not a spec file name (a *.spec.ts file directly under e2e/tests/)", r.Target, name))
			case !slices.Contains(specs, name):
				problems = append(problems, fmt.Sprintf("%s lists %s, which is not in e2e/tests/", r.Target, name))
			case dup && prev == r.Target:
				problems = append(problems, fmt.Sprintf("%s lists %s twice", r.Target, name))
			case dup:
				problems = append(problems, fmt.Sprintf("%s runs in both %s and %s; each spec file runs in exactly one e2e shard", name, prev, r.Target))
			default:
				owner[name] = r.Target
			}
		}
		lo, hi := r.Run.PortBase, r.Run.PortBase+e2ePortSpan-1
		if lo < 1 || hi > 65535 {
			problems = append(problems, fmt.Sprintf("%s's ports %d-%d leave the range 1-65535", r.Target, lo, hi))
		}
		for _, other := range runs[:i] {
			if olo, ohi := other.Run.PortBase, other.Run.PortBase+e2ePortSpan-1; lo <= ohi && olo <= hi {
				problems = append(problems, fmt.Sprintf("%s's ports %d-%d overlap %s's %d-%d, so the two shards collide when they run concurrently", r.Target, lo, hi, other.Target, olo, ohi))
			}
		}
		if prev, dup := outputs[r.Run.Output]; dup {
			problems = append(problems, fmt.Sprintf("%s and %s share the output directory %s, which Playwright empties when a run starts", prev, r.Target, r.Run.Output))
		}
		outputs[r.Run.Output] = r.Target
	}
	for _, name := range specs {
		if _, ok := owner[name]; !ok {
			problems = append(problems, fmt.Sprintf("%s runs in no e2e shard", name))
		}
	}
	return problems
}

// e2eSetupCommands are the e2e shards' shared setup, in order.
var e2eSetupCommands = []string{"npm install", "npx playwright install --with-deps chromium"}

// e2eSetupProblem says why a dry-run transcript does not install the e2e
// setup exactly once, before its first Playwright test run, or returns "".
func e2eSetupProblem(dry string) string {
	first := len(dry)
	if p := playwrightTestRE.FindStringIndex(dry); p != nil {
		first = p[0]
	}
	for _, cmd := range e2eSetupCommands {
		if n := strings.Count(dry, cmd); n != 1 {
			return fmt.Sprintf("runs %q %d times, want exactly once: the shards share one setup per make invocation, and two installs must never run in e2e/ at once", cmd, n)
		}
		if strings.Index(dry, cmd) > first {
			return fmt.Sprintf("runs %q after Playwright starts", cmd)
		}
	}
	return ""
}

// e2eTestFiles returns every file under e2e/tests/, as slash-separated paths
// relative to it.
func e2eTestFiles(t *testing.T) []string {
	t.Helper()
	root := filepath.Join(verdiRepoRoot, "e2e", "tests")
	var files []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	return files
}

// targetE2ERuns dry-runs target and parses its shard commands, and reports
// any problem with its setup.
func targetE2ERuns(t *testing.T, target string) []e2eShardRun {
	t.Helper()
	dry := makeDryRun(t, target)
	runs, err := parseE2EShardRuns(dry)
	if err != nil {
		t.Fatalf("make -n %s: %v", target, err)
	}
	if p := e2eSetupProblem(dry); p != "" {
		t.Errorf("make -n %s %s", target, p)
	}
	return runs
}

// TestE2EShards_PartitionSpecFiles proves the three e2e shards partition
// e2e/tests/*.spec.ts, each with its own ports and output directory and one
// setup; that `make e2e` runs exactly those three shard commands; and that
// VERIFY_STEPS runs the shards and not the convenience target.
func TestE2EShards_PartitionSpecFiles(t *testing.T) {
	steps := makefileVarFields(t, readMakefile(t), "VERIFY_STEPS")
	for _, target := range e2eShardTargets {
		if !slices.Contains(steps, target) {
			t.Errorf("VERIFY_STEPS %v does not run the e2e shard %s", steps, target)
		}
	}
	if slices.Contains(steps, e2eSuiteTarget) {
		t.Errorf("VERIFY_STEPS %v runs %s, the whole-suite convenience target; the shards already run every spec file once, so it would run the suite a second time", steps, e2eSuiteTarget)
	}

	specs, problems := e2eSpecDomain(e2eTestFiles(t))
	for _, p := range problems {
		t.Error(p)
	}

	var shards []namedE2ERun
	for _, target := range e2eShardTargets {
		runs := targetE2ERuns(t, target)
		if len(runs) != 1 {
			t.Fatalf("make -n %s runs %d Playwright shard commands, want exactly 1: %+v", target, len(runs), runs)
		}
		shards = append(shards, namedE2ERun{Target: target, Run: runs[0]})
	}
	for _, p := range e2eShardProblems(shards, specs) {
		t.Error(p)
	}

	suite := targetE2ERuns(t, e2eSuiteTarget)
	if len(suite) != len(shards) {
		t.Fatalf("make -n %s runs %d Playwright shard commands, want the %d shards' own: %+v", e2eSuiteTarget, len(suite), len(shards), suite)
	}
	for i, run := range suite {
		want := shards[i].Run
		if run.PortBase != want.PortBase || run.Output != want.Output || !slices.Equal(run.Specs, want.Specs) {
			t.Errorf("make %s's shard command %d is %+v, want %s's own %+v", e2eSuiteTarget, i+1, run, shards[i].Target, want)
		}
	}
}

// TestE2EShards_ParseShardRuns is parseE2EShardRuns' happy and negative
// paths over dry-run transcripts of the shapes the Makefile prints.
func TestE2EShards_ParseShardRuns(t *testing.T) {
	const setup = "cd e2e && npm install && npx playwright install --with-deps chromium\n"
	cases := []struct {
		name    string
		dry     string
		want    []e2eShardRun
		wantErr string
	}{
		{
			name: "one shard target",
			dry:  setup + "cd e2e && VERDI_E2E_PORT_BASE=21000 VERDI_E2E_SPECS='10-a.spec.ts 11-b.spec.ts' npx playwright test --output=test-results/e2e-1\n",
			want: []e2eShardRun{{PortBase: 21000, Specs: []string{"10-a.spec.ts", "11-b.spec.ts"}, Output: "test-results/e2e-1"}},
		},
		{
			name: "the concurrent suite's three commands, in order",
			dry: setup + "cd e2e && tmp=$(mktemp -d) && \\\n" +
				"{ shard 1 env VERDI_E2E_PORT_BASE=21000 VERDI_E2E_SPECS='10-a.spec.ts' npx playwright test --output=test-results/e2e-1 & " +
				"shard 2 env VERDI_E2E_PORT_BASE=22000 VERDI_E2E_SPECS='20-b.spec.ts' npx playwright test --output=test-results/e2e-2 & " +
				"shard 3 env VERDI_E2E_PORT_BASE=23000 VERDI_E2E_SPECS='00-c.spec.ts 30-d.spec.ts' npx playwright test --output=test-results/e2e-3 & wait; }\n",
			want: []e2eShardRun{
				{PortBase: 21000, Specs: []string{"10-a.spec.ts"}, Output: "test-results/e2e-1"},
				{PortBase: 22000, Specs: []string{"20-b.spec.ts"}, Output: "test-results/e2e-2"},
				{PortBase: 23000, Specs: []string{"00-c.spec.ts", "30-d.spec.ts"}, Output: "test-results/e2e-3"},
			},
		},
		{
			name: "setup alone runs no shard",
			dry:  setup,
			want: []e2eShardRun{},
		},
		{
			name: "an empty spec list parses, for the partition check to refuse",
			dry:  "VERDI_E2E_PORT_BASE=21000 VERDI_E2E_SPECS='' npx playwright test --output=test-results/e2e-1\n",
			want: []e2eShardRun{{PortBase: 21000, Specs: []string{}, Output: "test-results/e2e-1"}},
		},
		{
			name:    "a whole-suite run without a spec list",
			dry:     setup + "cd e2e && npx playwright test\n",
			wantErr: "does not have the shard command's shape",
		},
		{
			name:    "a second, unlisted run beside a shard command",
			dry:     "VERDI_E2E_PORT_BASE=21000 VERDI_E2E_SPECS='10-a.spec.ts' npx playwright test --output=test-results/e2e-1 && npx playwright test tests/\n",
			wantErr: "npx playwright test tests/",
		},
		{
			name:    "a spec list the shell computes",
			dry:     "VERDI_E2E_PORT_BASE=21000 VERDI_E2E_SPECS='$(ls tests)' npx playwright test --output=test-results/e2e-1\n",
			wantErr: "computed by the shell",
		},
		{
			name:    "a port base that is not an integer",
			dry:     "VERDI_E2E_PORT_BASE=$BASE VERDI_E2E_SPECS='10-a.spec.ts' npx playwright test --output=test-results/e2e-1\n",
			wantErr: `port base "$BASE" is not an integer`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseE2EShardRuns(tc.dry)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("parseE2EShardRuns() = %+v, %v; want an error containing %q", got, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseE2EShardRuns() error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("parseE2EShardRuns() = %+v, want %+v", got, tc.want)
			}
			for i := range got {
				if got[i].PortBase != tc.want[i].PortBase || got[i].Output != tc.want[i].Output || !slices.Equal(got[i].Specs, tc.want[i].Specs) {
					t.Errorf("run %d = %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestE2EShards_SpecDomain is e2eSpecDomain's happy and negative paths.
func TestE2EShards_SpecDomain(t *testing.T) {
	cases := []struct {
		name      string
		files     []string
		wantSpecs []string
		wantProb  []string // a substring of each problem, in order
	}{
		{
			name:      "spec files and helpers directly under tests/",
			files:     []string{"10-b.spec.ts", "fixtures.ts", "00-a.spec.ts", "helpers.ts", "README.md"},
			wantSpecs: []string{"00-a.spec.ts", "10-b.spec.ts"},
		},
		{
			name:      "a nested spec file would run whole but in no shard",
			files:     []string{"00-a.spec.ts", "sub/01-b.spec.ts"},
			wantSpecs: []string{"00-a.spec.ts"},
			wantProb:  []string{"e2e/tests/sub/01-b.spec.ts is a test file Playwright collects"},
		},
		{
			name:      "a *.test.ts file would run whole but in no shard",
			files:     []string{"00-a.spec.ts", "01-b.test.ts", "02-c.spec.js"},
			wantSpecs: []string{"00-a.spec.ts"},
			wantProb:  []string{"e2e/tests/01-b.test.ts", "e2e/tests/02-c.spec.js"},
		},
		{
			name:      "a spec file name outside the literal alphabet",
			files:     []string{"00-a.spec.ts", "01 b.spec.ts"},
			wantSpecs: []string{"00-a.spec.ts"},
			wantProb:  []string{"e2e/tests/01 b.spec.ts"},
		},
		{
			name:     "no spec file at all",
			files:    []string{"fixtures.ts"},
			wantProb: []string{"holds no *.spec.ts file"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			specs, problems := e2eSpecDomain(tc.files)
			if !slices.Equal(specs, tc.wantSpecs) {
				t.Errorf("specs = %v, want %v", specs, tc.wantSpecs)
			}
			if len(problems) != len(tc.wantProb) {
				t.Fatalf("problems = %q, want %d containing %q", problems, len(tc.wantProb), tc.wantProb)
			}
			for i, want := range tc.wantProb {
				if !strings.Contains(problems[i], want) {
					t.Errorf("problem %d = %q, want it to contain %q", i, problems[i], want)
				}
			}
		})
	}
}

// TestE2EShards_ShardProblems is e2eShardProblems' happy and negative paths,
// over a three-file domain and three shards that start as a partition.
func TestE2EShards_ShardProblems(t *testing.T) {
	specs := []string{"00-a.spec.ts", "10-b.spec.ts", "20-c.spec.ts"}
	partition := func() []namedE2ERun {
		return []namedE2ERun{
			{Target: "e2e-1", Run: e2eShardRun{PortBase: 21000, Specs: []string{"10-b.spec.ts"}, Output: "test-results/e2e-1"}},
			{Target: "e2e-2", Run: e2eShardRun{PortBase: 22000, Specs: []string{"20-c.spec.ts"}, Output: "test-results/e2e-2"}},
			{Target: "e2e-3", Run: e2eShardRun{PortBase: 23000, Specs: []string{"00-a.spec.ts"}, Output: "test-results/e2e-3"}},
		}
	}
	// addSpec returns a mutation that appends name to shard i's list.
	addSpec := func(i int, name string) func([]namedE2ERun) []namedE2ERun {
		return func(r []namedE2ERun) []namedE2ERun {
			r[i].Run.Specs = append(r[i].Run.Specs, name)
			return r
		}
	}
	cases := []struct {
		name   string
		mutate func(runs []namedE2ERun) []namedE2ERun
		want   []string // a substring of each problem, in order; nil wants none
	}{
		{name: "a partition", mutate: func(r []namedE2ERun) []namedE2ERun { return r }},
		{
			name:   "a file listed in two shards",
			mutate: addSpec(1, "10-b.spec.ts"),
			want:   []string{"10-b.spec.ts runs in both e2e-1 and e2e-2"},
		},
		{
			name:   "a file listed twice in one shard",
			mutate: addSpec(0, "10-b.spec.ts"),
			want:   []string{"e2e-1 lists 10-b.spec.ts twice"},
		},
		{
			name:   "a listed file that does not exist",
			mutate: addSpec(0, "11-gone.spec.ts"),
			want:   []string{"e2e-1 lists 11-gone.spec.ts, which is not in e2e/tests/"},
		},
		{
			name:   "a listed path that is not a spec file",
			mutate: addSpec(0, "helpers.ts"),
			want:   []string{`e2e-1 lists "helpers.ts", which is not a spec file name`},
		},
		{
			name:   "a listed spec file in a subdirectory",
			mutate: addSpec(0, "tests/10-b.spec.ts"),
			want:   []string{`e2e-1 lists "tests/10-b.spec.ts", which is not a spec file name`},
		},
		{
			name: "the empty-remainder sentinel",
			mutate: func(r []namedE2ERun) []namedE2ERun {
				r[2].Run.Specs = []string{"e2e-shard-3-list-failed"}
				return r
			},
			want: []string{`e2e-3 lists "e2e-shard-3-list-failed", which is not a spec file name`, "00-a.spec.ts runs in no e2e shard"},
		},
		{
			name:   "a file in no shard",
			mutate: func(r []namedE2ERun) []namedE2ERun { r[1].Run.Specs = nil; return r },
			want:   []string{"e2e-2 lists no spec file", "20-c.spec.ts runs in no e2e shard"},
		},
		{
			name:   "two shards on one port base",
			mutate: func(r []namedE2ERun) []namedE2ERun { r[2].Run.PortBase = 21000; return r },
			want:   []string{"e2e-3's ports 21000-21003 overlap e2e-1's 21000-21003"},
		},
		{
			name:   "overlapping port ranges",
			mutate: func(r []namedE2ERun) []namedE2ERun { r[1].Run.PortBase = 21003; return r },
			want:   []string{"e2e-2's ports 21003-21006 overlap e2e-1's 21000-21003"},
		},
		{
			name:   "ports past the valid range",
			mutate: func(r []namedE2ERun) []namedE2ERun { r[0].Run.PortBase = 65533; return r },
			want:   []string{"e2e-1's ports 65533-65536 leave the range"},
		},
		{
			name:   "a shared output directory",
			mutate: func(r []namedE2ERun) []namedE2ERun { r[2].Run.Output = "test-results/e2e-1"; return r },
			want:   []string{"e2e-1 and e2e-3 share the output directory test-results/e2e-1"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			problems := e2eShardProblems(tc.mutate(partition()), specs)
			if len(problems) != len(tc.want) {
				t.Fatalf("problems = %q, want %d containing %q", problems, len(tc.want), tc.want)
			}
			for i, want := range tc.want {
				if !strings.Contains(problems[i], want) {
					t.Errorf("problem %d = %q, want it to contain %q", i, problems[i], want)
				}
			}
		})
	}
}

// TestE2EShards_SetupProblem is e2eSetupProblem's happy and negative paths.
func TestE2EShards_SetupProblem(t *testing.T) {
	const (
		setup = "cd e2e && npm install && npx playwright install --with-deps chromium\n"
		run   = "cd e2e && VERDI_E2E_PORT_BASE=21000 VERDI_E2E_SPECS='10-a.spec.ts' npx playwright test --output=test-results/e2e-1\n"
	)
	cases := []struct {
		name string
		dry  string
		want string // "" wants no problem
	}{
		{name: "setup once, then the shard", dry: setup + run},
		{name: "no setup", dry: run, want: `runs "npm install" 0 times`},
		{name: "setup twice", dry: setup + setup + run, want: `runs "npm install" 2 times`},
		{name: "no browser install", dry: "cd e2e && npm install\n" + run, want: `runs "npx playwright install --with-deps chromium" 0 times`},
		{name: "setup after the shard", dry: run + setup, want: `runs "npm install" after Playwright starts`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := e2eSetupProblem(tc.dry)
			if tc.want == "" {
				if got != "" {
					t.Errorf("e2eSetupProblem() = %q, want none", got)
				}
				return
			}
			if !strings.Contains(got, tc.want) {
				t.Errorf("e2eSetupProblem() = %q, want it to contain %q", got, tc.want)
			}
		})
	}
}

// fakeE2ETools are stand-ins for node, npm, and npx, so `make e2e` runs
// hermetically: no install, no browser, no network. The fake Playwright run
// names its shard (read from --output) and fails when that shard is listed in
// FAKE_FAILING_SHARDS.
var fakeE2ETools = map[string]string{
	"node": "#!/bin/sh\nexit 0\n",
	"npm":  "#!/bin/sh\nexit 0\n",
	"npx": `#!/bin/sh
case "$1 $2" in
"playwright install") exit 0 ;;
"playwright test") ;;
*) echo "fake npx: unexpected: $*" >&2; exit 97 ;;
esac
shard=unknown
for arg in "$@"; do
	case "$arg" in --output=test-results/e2e-*) shard=${arg#--output=test-results/e2e-} ;; esac
done
for f in $FAKE_FAILING_SHARDS; do
	if [ "$f" = "$shard" ]; then echo "fake playwright: shard $shard failed"; exit 1; fi
done
echo "fake playwright: shard $shard passed"
`,
}

// TestE2EShards_SuiteFailsWhenAnyShardFails runs the real `make e2e` recipe
// over fake node, npm, and npx, and proves it fails when any shard fails,
// passes only when all three pass, prefixes each shard's output with its
// name, and ends with a summary line per shard.
func TestE2EShards_SuiteFailsWhenAnyShardFails(t *testing.T) {
	bin := t.TempDir()
	for name, body := range fakeE2ETools {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatalf("writing fake %s: %v", name, err)
		}
	}
	cases := []struct {
		name    string
		failing string
		wantOK  bool
	}{
		{name: "every shard passes", failing: "", wantOK: true},
		{name: "the middle shard fails", failing: "2", wantOK: false},
		{name: "the last shard fails", failing: "3", wantOK: false},
		{name: "every shard fails", failing: "1 2 3", wantOK: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command("make", "--no-print-directory", e2eSuiteTarget)
			cmd.Dir = verdiRepoRoot
			cmd.Env = append(hermeticMakeEnv(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "FAKE_FAILING_SHARDS="+tc.failing)
			raw, err := cmd.CombinedOutput()
			out := string(raw)
			if ok := err == nil; ok != tc.wantOK {
				t.Fatalf("make %s succeeded = %v, want %v (error %v); output:\n%s", e2eSuiteTarget, ok, tc.wantOK, err, out)
			}
			for i, target := range e2eShardTargets {
				shard := strconv.Itoa(i + 1)
				failed := slices.Contains(strings.Fields(tc.failing), shard)
				line, status := "passed", "0"
				if failed {
					line, status = "failed", "1"
				}
				if want := fmt.Sprintf("[%s] fake playwright: shard %s %s", target, shard, line); !strings.Contains(out, want) {
					t.Errorf("output lacks %q, the shard's own line under its name; output:\n%s", want, out)
				}
				if re := regexp.MustCompile(`(?m)^  ` + target + `\s+exit ` + status + `\s+\d+s$`); !re.MatchString(out) {
					t.Errorf("output lacks the summary line for %s with exit %s; output:\n%s", target, status, out)
				}
			}
		})
	}
}

// TestE2EShards_ConfigNamesTheSpecList pins that e2e/playwright.config.ts
// reads VERDI_E2E_SPECS, the variable the shard targets pass. It does not
// prove the config honors it (that needs Node; each shard's `--list` run is
// the witness), but a rename on one side alone fails here.
func TestE2EShards_ConfigNamesTheSpecList(t *testing.T) {
	cfg := filepath.Join(verdiRepoRoot, "e2e", "playwright.config.ts")
	raw, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatalf("reading %s: %v", cfg, err)
	}
	if !strings.Contains(string(raw), `"`+e2eSpecsEnvVar+`"`) {
		t.Errorf("%s does not name %q, the variable the e2e shard targets pass their spec files in; without it every shard runs the whole suite", cfg, e2eSpecsEnvVar)
	}
}
