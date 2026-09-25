// Test-shard parity guard (SI-266, owner directive 2026-09-24). The Go tests
// run as disjoint make targets — test-cmd, test-cross, test-rest, plus the
// spec-align gate, which alone runs internal/specalign — so that each can be
// its own pull-request CI job and no package runs twice. This file proves,
// from the Makefile's own dry-run output (`make -n`, never a hand-copied
// list), that:
//
//   - the shards partition `go list ./...`: disjoint and complete;
//   - `make test` runs every package exactly once, under -race;
//   - `make verify` executes every package exactly once under -race — any
//     later -race run of a package (the fixture gate) carries the identical
//     flags, so the Go test cache replays it instead of executing it again;
//   - every cross-binary package (ADJ-68) runs under -count=1 wherever the
//     gate runs it.
//
// Reading `make -n` rather than the Makefile text means the guard sees the
// commands make would actually execute, with every variable and filter
// expanded. Its one blind spot is a package list computed by the shell at
// recipe time, which a dry-run prints unexpanded; parseGoTestInvocations
// refuses such a list outright so the guard can never pass over it.
package specalign

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// goTestInvocation is one `go test` command found in a make dry-run: its
// flags, normalized to `-name` or `-name=value` in command-line order, and
// its package arguments exactly as written (patterns or import paths).
type goTestInvocation struct {
	Flags []string
	Pkgs  []string
}

// has reports whether the invocation carries flag, matched exactly after
// normalization ("-race", "-count=1", "-parallel=4", "-v").
func (inv goTestInvocation) has(flag string) bool {
	return slices.Contains(inv.Flags, flag)
}

// flagKey renders the flags in command-line order. Two runs of one package
// share a Go test cache entry only when their flags match, so this is the
// comparison a cache replay depends on.
func (inv goTestInvocation) flagKey() string {
	return strings.Join(inv.Flags, " ")
}

// goTestValueFlags are the `go test` flags this repo's recipes use (or
// plausibly could) that take their value as a separate argument. Anything
// else starting with "-" is a boolean flag or carries its value after "=".
var goTestValueFlags = map[string]bool{
	"-bench": true, "-benchtime": true, "-count": true, "-covermode": true,
	"-coverpkg": true, "-coverprofile": true, "-cpu": true, "-exec": true,
	"-o": true, "-p": true, "-parallel": true, "-run": true, "-skip": true,
	"-tags": true, "-timeout": true,
}

// goTestStartRE finds the start of a `go test` command: "go" not preceded by
// a word or path character (so `cargo test` and `pogo test` do not match),
// then whitespace, then "test" and whitespace.
var goTestStartRE = regexp.MustCompile(`(?:^|[^A-Za-z0-9_./-])go[ \t]+test[ \t]`)

// parseGoTestInvocations extracts every `go test` command from a make
// dry-run transcript. Each command's arguments end at the first unquoted
// shell operator (`;`, `|`, `&`, `)`, `<`, `>`), newline, or backslash; a
// file-descriptor number directly before a redirection (`2>&1`) is dropped.
// Quoted arguments are unquoted, so `-run 'A|B'` keeps its `|`.
//
// A package argument that still contains `$` or a backtick is a list the
// shell computes at recipe time, invisible to a dry-run; it is reported as
// an error, never silently accepted as a package name.
func parseGoTestInvocations(dry string) ([]goTestInvocation, error) {
	var out []goTestInvocation
	for _, loc := range goTestStartRE.FindAllStringIndex(dry, -1) {
		args := splitShellArgs(dry[loc[1]:])
		inv, err := classifyGoTestArgs(args)
		if err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, nil
}

// splitShellArgs tokenizes s up to the first unquoted command terminator.
func splitShellArgs(s string) []string {
	var toks []string
	var cur strings.Builder
	inTok := false
	var quote byte
	flush := func() {
		if inTok {
			toks = append(toks, cur.String())
		}
		cur.Reset()
		inTok = false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == quote {
				quote = 0
				continue
			}
			cur.WriteByte(c)
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
			inTok = true
		case ' ', '\t':
			flush()
		case '>', '<':
			// `2>&1`: the digits before a redirection name a file
			// descriptor, not an argument.
			if inTok && strings.Trim(cur.String(), "0123456789") == "" {
				cur.Reset()
				inTok = false
			}
			flush()
			return toks
		case ';', '|', '&', ')', '\n', '\\':
			flush()
			return toks
		default:
			cur.WriteByte(c)
			inTok = true
		}
	}
	flush()
	return toks
}

// classifyGoTestArgs splits tokens into normalized flags and package
// arguments (see goTestInvocation).
func classifyGoTestArgs(args []string) (goTestInvocation, error) {
	var inv goTestInvocation
	for i := 0; i < len(args); i++ {
		tok := args[i]
		if !strings.HasPrefix(tok, "-") {
			if strings.ContainsAny(tok, "$`") {
				return goTestInvocation{}, &shellComputedPkgsError{arg: tok}
			}
			inv.Pkgs = append(inv.Pkgs, tok)
			continue
		}
		name := "-" + strings.TrimLeft(tok, "-")
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			if name[eq+1:] == "true" {
				name = name[:eq]
			}
			inv.Flags = append(inv.Flags, name)
			continue
		}
		if goTestValueFlags[name] && i+1 < len(args) {
			i++
			name += "=" + args[i]
		}
		inv.Flags = append(inv.Flags, name)
	}
	return inv, nil
}

// shellComputedPkgsError names a package argument a dry-run cannot expand.
type shellComputedPkgsError struct{ arg string }

func (e *shellComputedPkgsError) Error() string {
	return "go test package argument " + e.arg + " is computed by the shell at recipe time, so a make dry-run cannot see which packages it names; compute the list in make (a variable or $(shell ...)) so the shard guard can read it"
}

// TestGateShards_ParseGoTestInvocations is the dry-run parser's own
// table-driven test, happy and negative paths, over the recipe shapes this
// Makefile uses.
func TestGateShards_ParseGoTestInvocations(t *testing.T) {
	cases := []struct {
		name    string
		dry     string
		want    []goTestInvocation
		wantErr bool
	}{
		{
			name: "plain shard command",
			dry:  "go test -race -parallel 4 ./cmd/verdi\n",
			want: []goTestInvocation{{Flags: []string{"-race", "-parallel=4"}, Pkgs: []string{"./cmd/verdi"}}},
		},
		{
			name: "captured in a shell variable with 2>&1",
			dry:  `out="$(go test -race -v -count=1 -parallel 4 ./internal/specalign/... 2>&1)"; \` + "\n\tstatus=$?; \\\n",
			want: []goTestInvocation{{Flags: []string{"-race", "-v", "-count=1", "-parallel=4"}, Pkgs: []string{"./internal/specalign/..."}}},
		},
		{
			name: "quoted -run value keeps its pipe; flags after the package",
			dry:  `out="$(go test -count=1 ./internal/showcasealign/ -run 'TestA|TestB' -v 2>&1)"; \` + "\n",
			want: []goTestInvocation{{Flags: []string{"-count=1", "-run=TestA|TestB", "-v"}, Pkgs: []string{"./internal/showcasealign/"}}},
		},
		{
			name: "two commands, separate-value and equals forms normalized alike",
			dry:  "go test -count 1 --race ./a/...\ngo test -race=true -p=2 ./b ./c\n",
			want: []goTestInvocation{
				{Flags: []string{"-count=1", "-race"}, Pkgs: []string{"./a/..."}},
				{Flags: []string{"-race", "-p=2"}, Pkgs: []string{"./b", "./c"}},
			},
		},
		{
			name: "no go test at all",
			dry:  "go build ./...\ngo vet ./...\ncargo test --all\n",
			want: nil,
		},
		{
			name:    "shell-computed package list is refused",
			dry:     "go test -race $(go list ./...)\n",
			wantErr: true,
		},
		{
			name:    "shell variable package list is refused",
			dry:     "go test -race $$pkgs\n",
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseGoTestInvocations(tc.dry)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseGoTestInvocations() = %+v, want an error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseGoTestInvocations() error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("parseGoTestInvocations() = %+v, want %+v", got, tc.want)
			}
			for i := range got {
				if !slices.Equal(got[i].Flags, tc.want[i].Flags) || !slices.Equal(got[i].Pkgs, tc.want[i].Pkgs) {
					t.Errorf("invocation %d = %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// readMakefile returns the real Makefile's text.
func readMakefile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(verdiRepoRoot, "Makefile")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(raw)
}

// makefileVarFields returns the whitespace-separated value of a simple
// `NAME := ...` (or `=`/`?=`) assignment, failing the test if it is absent.
func makefileVarFields(t *testing.T, makefile, name string) []string {
	t.Helper()
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(name) + `[ \t]*[:?]?=[ \t]*(.*)$`)
	m := re.FindStringSubmatch(makefile)
	if m == nil {
		t.Fatalf("Makefile: no %s assignment found", name)
	}
	return strings.Fields(m[1])
}

// makefilePrereqs returns the prerequisites of an explicit `target:` rule,
// failing the test if the rule is absent.
func makefilePrereqs(t *testing.T, makefile, target string) []string {
	t.Helper()
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(target) + `:([^=\n][^\n]*)?$`)
	m := re.FindStringSubmatch(makefile)
	if m == nil {
		t.Fatalf("Makefile: no %q rule found", target)
	}
	rest := m[1]
	if semi := strings.IndexByte(rest, ';'); semi >= 0 {
		rest = rest[:semi]
	}
	return strings.Fields(rest)
}

// testShardTargets is `make test`'s composition: the three shards plus the
// spec-align gate, which is the only target that runs internal/specalign.
var testShardTargets = []string{"test-cmd", "test-cross", "test-rest", "spec-align"}

// expandedVerifySteps returns VERIFY_STEPS with `test` (if listed) replaced
// by its prerequisites — the make targets `make verify` actually runs.
func expandedVerifySteps(t *testing.T, makefile string) []string {
	t.Helper()
	var out []string
	for _, step := range makefileVarFields(t, makefile, "VERIFY_STEPS") {
		if step != "test" {
			out = append(out, step)
			continue
		}
		prereqs := makefilePrereqs(t, makefile, "test")
		if len(prereqs) == 0 {
			t.Fatalf("Makefile: VERIFY_STEPS lists `test`, but `test` has no shard prerequisites to expand to")
		}
		out = append(out, prereqs...)
	}
	return out
}

// hermeticMakeEnv is the environment for a nested `make -n`: this test may
// itself be running under make (make spec-align, make verify), whose
// MAKEFLAGS/MAKELEVEL would otherwise leak into the child — a parent -i, -k,
// or jobserver descriptor must not change what the dry-run prints.
func hermeticMakeEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		switch name {
		case "MAKEFLAGS", "MFLAGS", "MAKELEVEL", "MAKEOVERRIDES", "GNUMAKEFLAGS":
			continue
		}
		env = append(env, kv)
	}
	return env
}

// makeDryRun returns `make -n` output for target in the repo root.
func makeDryRun(t *testing.T, target string) string {
	t.Helper()
	cmd := exec.Command("make", "-n", "-s", "--no-print-directory", target)
	cmd.Dir = verdiRepoRoot
	cmd.Env = hermeticMakeEnv()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("make -n %s: %v\nstderr:\n%s", target, err, stderr.String())
	}
	return stdout.String()
}

// targetGoTests dry-runs target and parses its `go test` commands.
func targetGoTests(t *testing.T, target string) []goTestInvocation {
	t.Helper()
	invs, err := parseGoTestInvocations(makeDryRun(t, target))
	if err != nil {
		t.Fatalf("make -n %s: %v", target, err)
	}
	return invs
}

// goList expands package patterns to sorted import paths via `go list`.
func goList(t *testing.T, patterns ...string) []string {
	t.Helper()
	if len(patterns) == 0 {
		t.Fatalf("goList called with no patterns (a go test with no package arguments tests the current directory — never a shard)")
	}
	cmd := exec.Command("go", append([]string{"list"}, patterns...)...)
	cmd.Dir = verdiRepoRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go list %v: %v\nstderr:\n%s", patterns, err, stderr.String())
	}
	pkgs := strings.Fields(stdout.String())
	slices.Sort(pkgs)
	return pkgs
}

// crossBinaryImportPaths expands the Makefile's CROSS_BINARY_PKGS.
func crossBinaryImportPaths(t *testing.T, makefile string) []string {
	t.Helper()
	patterns := makefileVarFields(t, makefile, "CROSS_BINARY_PKGS")
	if len(patterns) == 0 {
		t.Fatalf("Makefile: CROSS_BINARY_PKGS is empty")
	}
	return goList(t, patterns...)
}

// TestGateShards_PartitionGoList proves the shards partition `go list ./...`
// (contract item 4b): `make test` is exactly test-cmd, test-cross, test-rest,
// and spec-align; each runs its packages under -race; no package is in two of
// them; every package is in one; and each holds what the gate says it holds —
// test-cmd is cmd/verdi, spec-align is internal/specalign, test-cross is the
// rest of CROSS_BINARY_PKGS, and test-rest is everything else. It then proves
// `make test` as a whole runs every package exactly once.
func TestGateShards_PartitionGoList(t *testing.T) {
	makefile := readMakefile(t)
	if got := makefilePrereqs(t, makefile, "test"); !slices.Equal(got, testShardTargets) {
		t.Fatalf("Makefile: `test` must be exactly the shards %v, got prerequisites %v", testShardTargets, got)
	}

	all := goList(t, "./...")
	owner := map[string]string{}
	byShard := map[string][]string{}
	for _, shard := range testShardTargets {
		invs := targetGoTests(t, shard)
		if len(invs) == 0 {
			t.Errorf("make %s runs no `go test` command", shard)
		}
		for _, inv := range invs {
			if !inv.has("-race") {
				t.Errorf("make %s runs `go test %s %s` without -race — every package runs under -race exactly once", shard, inv.flagKey(), strings.Join(inv.Pkgs, " "))
			}
			for _, pkg := range goList(t, inv.Pkgs...) {
				if prev, dup := owner[pkg]; dup {
					t.Errorf("package %s runs in both %s and %s — the shards must be disjoint", pkg, prev, shard)
					continue
				}
				owner[pkg] = shard
				byShard[shard] = append(byShard[shard], pkg)
			}
		}
	}
	for _, pkg := range all {
		if _, ok := owner[pkg]; !ok {
			t.Errorf("package %s (in go list ./...) runs in no shard — the shards must be complete", pkg)
		}
	}
	for pkg, shard := range owner {
		if !slices.Contains(all, pkg) {
			t.Errorf("shard %s runs %s, which is not in go list ./...", shard, pkg)
		}
	}

	specAlign := goList(t, "./internal/specalign/...")
	var crossWithoutSpecAlign []string
	for _, pkg := range crossBinaryImportPaths(t, makefile) {
		if !slices.Contains(specAlign, pkg) {
			crossWithoutSpecAlign = append(crossWithoutSpecAlign, pkg)
		}
	}
	wantShard := map[string][]string{
		"test-cmd":   goList(t, "./cmd/verdi"),
		"test-cross": crossWithoutSpecAlign,
		"spec-align": specAlign,
	}
	for shard, want := range wantShard {
		got := slices.Clone(byShard[shard])
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Errorf("make %s runs %v, want %v", shard, got, want)
		}
	}

	runs := map[string]int{}
	for _, inv := range targetGoTests(t, "test") {
		for _, pkg := range goList(t, inv.Pkgs...) {
			runs[pkg]++
		}
	}
	for _, pkg := range all {
		if runs[pkg] != 1 {
			t.Errorf("make test runs %s %d times, want exactly once", pkg, runs[pkg])
		}
	}
}

// TestGateShards_SpecAlignGateKeepsItsShape pins the spec-align gate's own
// command: specalign runs fresh (-count=1), under -race, verbosely (-v, so
// the recipe can reprint every `--- SKIP:`), as one named target.
func TestGateShards_SpecAlignGateKeepsItsShape(t *testing.T) {
	invs := targetGoTests(t, "spec-align")
	if len(invs) != 1 {
		t.Fatalf("make spec-align runs %d go test commands, want exactly 1: %+v", len(invs), invs)
	}
	for _, flag := range []string{"-race", "-count=1", "-v"} {
		if !invs[0].has(flag) {
			t.Errorf("make spec-align's go test lacks %s (flags %v)", flag, invs[0].Flags)
		}
	}
}

// verifyRaceRun is one package's run under -race in one `make verify` step.
type verifyRaceRun struct {
	step  string // the expanded VERIFY_STEPS entry that runs it
	pkg   string // the import path
	flags string // the go test flags in command-line order (flagKey)
	fresh bool   // -count=1: never replayed from the test cache
}

// verifyRaceRuns dry-runs each expanded VERIFY_STEPS entry in the order
// `make verify` runs them and returns every package each one runs under
// -race, in that order.
func verifyRaceRuns(t *testing.T, makefile string) []verifyRaceRun {
	t.Helper()
	steps := expandedVerifySteps(t, makefile)
	if slices.Contains(steps, "test") {
		t.Fatalf("expanded VERIFY_STEPS still contains `test`: %v", steps)
	}
	var runs []verifyRaceRun
	for _, step := range steps {
		for _, inv := range targetGoTests(t, step) {
			if !inv.has("-race") {
				continue
			}
			for _, pkg := range goList(t, inv.Pkgs...) {
				runs = append(runs, verifyRaceRun{step: step, pkg: pkg, flags: inv.flagKey(), fresh: inv.has("-count=1")})
			}
		}
	}
	return runs
}

// verifyCacheReplays maps each `make verify` step that re-runs, under -race,
// a package an earlier step already ran to those earlier steps, sorted. In
// `make verify` such a re-run is a cache replay
// (TestGateShards_VerifyExecutesEachPackageOnceUnderRace), but only because
// both steps share one machine's test cache: the pull-request gate must keep
// them in one job, in this order
// (TestMergeGateParity_CacheReplaysRunAfterTheirExecutorInOneJob).
func verifyCacheReplays(t *testing.T, makefile string) map[string][]string {
	t.Helper()
	executor := map[string]string{}
	replays := map[string][]string{}
	for _, run := range verifyRaceRuns(t, makefile) {
		first, seen := executor[run.pkg]
		if !seen {
			executor[run.pkg] = run.step
			continue
		}
		if first != run.step && !slices.Contains(replays[run.step], first) {
			replays[run.step] = append(replays[run.step], first)
			slices.Sort(replays[run.step])
		}
	}
	return replays
}

// TestGateShards_VerifyExecutesEachPackageOnceUnderRace proves contract item
// 1 for `make verify`: walking VERIFY_STEPS in order, every package's first
// -race run executes it and any later -race run of the same package is a
// cache replay — the identical flags, never -count=1 — so it is not executed
// a second time. The fixture gate re-runs fixturegit, corpus, and
// svcfixcanned after test-rest; this is what keeps that a replay.
func TestGateShards_VerifyExecutesEachPackageOnceUnderRace(t *testing.T) {
	first := map[string]verifyRaceRun{}
	for _, run := range verifyRaceRuns(t, readMakefile(t)) {
		prev, seen := first[run.pkg]
		if !seen {
			first[run.pkg] = run
			continue
		}
		if run.flags != prev.flags || run.fresh {
			t.Errorf("make verify executes %s under -race twice: in %s (flags %q) and again in %s (flags %q) — a later run must carry the identical flags and no -count=1 so the test cache replays it", run.pkg, prev.step, prev.flags, run.step, run.flags)
		}
	}
	for _, pkg := range goList(t, "./...") {
		if _, ok := first[pkg]; !ok {
			t.Errorf("make verify never runs %s under -race", pkg)
		}
	}
}

// TestGateCacheHonesty_CrossBinaryPkgsRunFresh is the ADJ-68 freshness
// guard for the sharded gate: every `go test` command in `make test` or any
// `make verify` step that touches a package in CROSS_BINARY_PKGS must carry
// -count=1, whichever shard or named gate it lives in. (The companion
// TestGateCacheHonesty_CrossBinaryPkgsListInSync proves the list itself is
// complete.)
func TestGateCacheHonesty_CrossBinaryPkgsRunFresh(t *testing.T) {
	makefile := readMakefile(t)
	cross := crossBinaryImportPaths(t, makefile)
	targets := append([]string{"test"}, expandedVerifySteps(t, makefile)...)
	ranFresh := map[string]bool{}
	for _, target := range targets {
		for _, inv := range targetGoTests(t, target) {
			for _, pkg := range goList(t, inv.Pkgs...) {
				if !slices.Contains(cross, pkg) {
					continue
				}
				if !inv.has("-count=1") {
					t.Errorf("make %s runs cross-binary package %s without -count=1 (flags %v) — its test cache cannot see cmd/verdi's sources, so it may replay a stale PASS (ADJ-68)", target, pkg, inv.Flags)
					continue
				}
				ranFresh[pkg] = true
			}
		}
	}
	for _, pkg := range cross {
		if !ranFresh[pkg] {
			t.Errorf("cross-binary package %s never runs in make test or make verify", pkg)
		}
	}
}
