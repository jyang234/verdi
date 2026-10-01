// spec/strict-lint-gate ac-1 (static half): the strict lint configuration
// enables exactly the five gated linters, errorlint with only its errorf
// check, each under a comment quoting the docs/ground-rules.md sentence it
// enforces, and caps no findings; .golangci.yml, the parity configuration, is
// pinned by its sha256 and unchanged (co-1); lint-strict is a step of make
// verify's VERIFY_STEPS and runs in the static job of merge-gate.yml and
// verify.yml; and `make -n lint-strict` shows golangci-lint run with --config
// .golangci.strict.yml for GOOS=linux GOARCH=amd64, with --issues-exit-code=0,
// over the whole module (./...), its own exit status captured as status=$?
// with nothing masking it, then the baseline check passed that status and the
// committed baseline, .golangci.strict-baseline.json.
package specalign

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// parityLintConfigSHA256 pins .golangci.yml's bytes: the parity configuration
// the strict gate leaves unchanged (strict-lint-target-v2 co-1). A deliberate
// change to it is a change of this pin, made visible here.
const parityLintConfigSHA256 = "2d32fbc1ca4b78b82399d1635eba4afb01d889821c6e5b9392080dc485dbd694"

// strictLintStep is the make target, and VERIFY_STEPS entry, that runs the
// strict gate.
const strictLintStep = "lint-strict"

// strictLintRulePhrases maps each gated linter to a phrase of the
// docs/ground-rules.md sentence it enforces (strict-lint-target-v2 dc-2): the
// sentence its comment quotes must contain the phrase.
func strictLintRulePhrases() map[string]string {
	return map[string]string{
		"containedctx":     "never stored in a struct",
		"contextcheck":     "is the first parameter of anything that blocks or does I/O",
		"noctx":            "is the first parameter of anything that blocks or does I/O",
		"errorlint":        "wrap with `%w`",
		"gochecknoglobals": "No package-level mutable state",
	}
}

// groundRuleSentences splits docs/ground-rules.md into its sentences: each
// list item and paragraph, whitespace-collapsed, cut after every period that
// a space follows. Headings are not sentences.
func groundRuleSentences(md string) []string {
	var units []string
	var cur []string
	flush := func() {
		if len(cur) != 0 {
			units = append(units, strings.Join(strings.Fields(strings.Join(cur, " ")), " "))
			cur = nil
		}
	}
	for _, line := range strings.Split(md, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "" || strings.HasPrefix(trimmed, "#"):
			flush()
		case strings.HasPrefix(line, "- "):
			flush()
			cur = append(cur, strings.TrimPrefix(line, "- "))
		default:
			cur = append(cur, trimmed)
		}
	}
	flush()
	var out []string
	for _, u := range units {
		start := 0
		for i := 0; i+1 < len(u); i++ {
			if u[i] == '.' && u[i+1] == ' ' {
				out = append(out, u[start:i+1])
				start = i + 2
			}
		}
		if start < len(u) {
			out = append(out, u[start:])
		}
	}
	return out
}

// quotedSentence returns the double-quoted text in comment, whitespace-
// collapsed, or "" when the comment quotes nothing.
func quotedSentence(comment string) string {
	first, last := strings.Index(comment, `"`), strings.LastIndex(comment, `"`)
	if first < 0 || last <= first {
		return ""
	}
	return strings.Join(strings.Fields(comment[first+1:last]), " ")
}

// boolSetting returns the boolean at m[key] and whether it is one.
func boolSetting(m map[string]interface{}, key string) (bool, bool) {
	b, ok := m[key].(bool)
	return b, ok
}

// strictLintConfigShapeProblems returns every way raw, a strict
// configuration's source, departs from ac-1's shape, measured against the
// ground rules' sentences.
func strictLintConfigShapeProblems(raw string, sentences []string) []string {
	top, err := decodeStrictLintConfig(raw)
	if err != nil {
		return []string{err.Error()}
	}
	var out []string
	if keys := sortedKeys(top); !slices.Equal(keys, []string{"issues", "linters", "version"}) {
		out = append(out, fmt.Sprintf("%s must declare exactly the top-level keys [issues linters version], got %v: run, output, formatters, and severity each change what the gate reports", strictLintConfigFile, keys))
	}
	if v, _ := asStringVal(top["version"]); v != "2" {
		out = append(out, fmt.Sprintf("version is %q, want \"2\" (golangci-lint v2)", v))
	}

	linters, _ := asMap(top["linters"])
	if extra := keysOutside(sortedKeys(linters), []string{"default", "enable", "exclusions", "settings"}); len(extra) != 0 {
		out = append(out, fmt.Sprintf("linters declares %v, outside [default enable exclusions settings]", extra))
	}
	if d, _ := asStringVal(linters["default"]); d != "none" {
		out = append(out, fmt.Sprintf("linters.default is %q, want \"none\": any other default enables linters beyond the gated set", d))
	}
	enabled := asStringSlice(linters["enable"])
	if sorted := slices.Sorted(slices.Values(enabled)); !slices.Equal(sorted, strictGatedLinters()) {
		out = append(out, fmt.Sprintf("linters.enable is %v, want exactly the gated linters %v (strict-lint-target-v2 dc-2)", enabled, strictGatedLinters()))
	}

	settings, _ := asMap(linters["settings"])
	if keys := sortedKeys(settings); !slices.Equal(keys, []string{"errorlint"}) {
		out = append(out, fmt.Sprintf("linters.settings configures %v, want exactly [errorlint]", keys))
	}
	errorlint, _ := asMap(settings["errorlint"])
	if keys := sortedKeys(errorlint); !slices.Equal(keys, []string{"asserts", "comparison", "errorf"}) {
		out = append(out, fmt.Sprintf("linters.settings.errorlint sets %v, want exactly [asserts comparison errorf]", keys))
	}
	for key, want := range map[string]bool{"errorf": true, "asserts": false, "comparison": false} {
		if got, ok := boolSetting(errorlint, key); !ok || got != want {
			out = append(out, fmt.Sprintf("linters.settings.errorlint.%s is %v, want %v: errorlint runs with only its errorf check", key, errorlint[key], want))
		}
	}

	issues, _ := asMap(top["issues"])
	if keys := sortedKeys(issues); !slices.Equal(keys, []string{"max-issues-per-linter", "max-same-issues", "uniq-by-line"}) {
		out = append(out, fmt.Sprintf("issues sets %v, want exactly [max-issues-per-linter max-same-issues uniq-by-line]", keys))
	}
	for _, key := range []string{"max-issues-per-linter", "max-same-issues"} {
		if n, ok := issues[key].(int); !ok || n != 0 {
			out = append(out, fmt.Sprintf("issues.%s is %v, want 0: a cap hides a new finding whose text matches existing ones", key, issues[key]))
		}
	}
	if uniq, ok := boolSetting(issues, "uniq-by-line"); !ok || uniq {
		out = append(out, fmt.Sprintf("issues.uniq-by-line is %v, want false: true merges a second finding on an already-flagged line into the first (ledger SI-311 (2))", issues["uniq-by-line"]))
	}

	lines := strings.Split(raw, "\n")
	items := listItemLines(lines, sectionLine(lines, 0, "enable"))
	phrases := strictLintRulePhrases()
	for _, linter := range strictGatedLinters() {
		at := slices.IndexFunc(items, func(i int) bool { return strings.TrimSpace(lines[i]) == "- "+linter })
		if at < 0 {
			out = append(out, fmt.Sprintf("no `- %s` item under linters.enable to read its rule comment from", linter))
			continue
		}
		line := items[at]
		quote := quotedSentence(commentAbove(lines, line))
		switch {
		case quote == "":
			out = append(out, fmt.Sprintf("%s (line %d) is not under a comment quoting the docs/ground-rules.md sentence it enforces", linter, line+1))
		case !slices.Contains(sentences, quote):
			out = append(out, fmt.Sprintf("%s (line %d) quotes %q, which is not a sentence of docs/ground-rules.md", linter, line+1, quote))
		case !strings.Contains(quote, phrases[linter]):
			out = append(out, fmt.Sprintf("%s (line %d) quotes %q, but the sentence it enforces says %q", linter, line+1, quote, phrases[linter]))
		}
	}
	return out
}

// strictWiring is what the strict target's wiring is read from.
type strictWiring struct {
	parity      []byte              // .golangci.yml's bytes
	verifySteps []string            // the Makefile's VERIFY_STEPS
	staticRuns  map[string][]string // each workflow's static-job run commands
	dryRun      string              // `make -n lint-strict`
}

// golangciRunRE finds each golangci-lint run invocation in a dry run, with
// whatever sits before it on its line.
var golangciRunRE = regexp.MustCompile(`(?m)^.*golangci-lint run\b.*$`)

// strictRunStatusRE matches a golangci-lint run followed directly by the
// capture of its own exit status: its arguments, then `; status=$?;`, with
// no `|` or `&` between, so no `|| true`, pipe, or `&&` stands between
// golangci-lint and the status the baseline check is passed.
var strictRunStatusRE = regexp.MustCompile(`golangci-lint run [^;|&\n]*; status=\$\?;`)

// lintratchetCheckRE finds each baseline-check invocation in a dry run, with
// the rest of its line.
var lintratchetCheckRE = regexp.MustCompile(`(?m)lintratchet check\b.*$`)

// strictLintBaselineFile is the committed baseline, at the repository root
// (spec/strict-lint-gate ac-2).
const strictLintBaselineFile = ".golangci.strict-baseline.json"

// strictCheckExitArg is what the baseline check's -lint-exit must be passed:
// the shell variable the recipe captures golangci-lint's own exit status in.
const strictCheckExitArg = `"$status"`

// flagValue returns the token after flag in fields, without a trailing `;`,
// and whether flag is there.
func flagValue(fields []string, flag string) (string, bool) {
	at := slices.Index(fields, flag)
	if at < 0 || at+1 >= len(fields) {
		return "", false
	}
	return strings.TrimSuffix(fields[at+1], ";"), true
}

// strictRunProblems returns every way one golangci-lint run line r of the
// lint-strict dry run departs from ac-1 and dc-1: a missing explicit
// configuration, platform, or --issues-exit-code=0; a package pattern other
// than the whole module (./...); and golangci-lint's own exit status not
// captured as status=$? right after it, or masked with || (review finding
// S1-B3).
func strictRunProblems(r string) []string {
	var out []string
	line := strings.TrimSpace(r)
	for _, want := range []string{"GOOS=linux GOARCH=amd64 golangci-lint run", "--config .golangci.strict.yml", "--issues-exit-code=0"} {
		if !strings.Contains(r, want) {
			out = append(out, fmt.Sprintf("make -n lint-strict runs %q, which lacks %q", line, want))
		}
	}
	args := r[strings.Index(r, "golangci-lint run")+len("golangci-lint run"):]
	if cut := strings.IndexAny(args, ";|&"); cut >= 0 {
		args = args[:cut]
	}
	if !slices.Contains(strings.Fields(args), "./...") {
		out = append(out, fmt.Sprintf("make -n lint-strict runs %q, which does not lint the whole module (./...): the baseline check would pass a run that never analyzed the rest", line))
	}
	if !strictRunStatusRE.MatchString(r) {
		out = append(out, fmt.Sprintf("make -n lint-strict runs %q, which does not capture golangci-lint's own exit status as status=$? right after it: the check must see that status to exit 2 on a failed run (dc-1)", line))
	}
	if strings.Contains(r, "||") {
		out = append(out, fmt.Sprintf("make -n lint-strict runs %q, which masks golangci-lint's exit status with ||", line))
	}
	return out
}

// strictCheckProblems returns every way one baseline-check line of the
// lint-strict dry run departs from the wiring: -lint-exit not passed the
// captured status (review finding S1-B3), or -baseline not the committed
// baseline, whose absence at the merge base would turn the growth comparison
// into a bootstrap disclosure (review finding S1-A2).
func strictCheckProblems(line string) []string {
	var out []string
	fields := strings.Fields(line)
	for _, f := range []struct{ flag, want, why string }{
		{"-lint-exit", strictCheckExitArg, "golangci-lint's own exit status, captured right after the run"},
		{"-baseline", strictLintBaselineFile, "the committed baseline, which the growth comparison reads at the merge base"},
	} {
		got, ok := flagValue(fields, f.flag)
		switch {
		case !ok:
			out = append(out, fmt.Sprintf("make -n lint-strict's baseline check %q passes no %s, want %q: %s", strings.TrimSpace(line), f.flag, f.want, f.why))
		case got != f.want:
			out = append(out, fmt.Sprintf("make -n lint-strict's baseline check passes %s %q, want %q: %s", f.flag, got, f.want, f.why))
		}
	}
	return out
}

// strictWiringProblems returns every way the wiring departs from ac-1: a
// changed parity configuration, lint-strict outside VERIFY_STEPS or either
// workflow's static job, a recipe whose golangci-lint run lacks the explicit
// configuration, the platform, or --issues-exit-code=0, lints less than the
// whole module, or does not pass its own exit status to the baseline check,
// and a recipe that does not run the baseline check after it over the
// committed baseline.
func strictWiringProblems(w strictWiring) []string {
	var out []string
	sum := sha256.Sum256(w.parity)
	if got := hex.EncodeToString(sum[:]); got != parityLintConfigSHA256 {
		out = append(out, fmt.Sprintf(".golangci.yml's sha256 is %s, want the pinned %s: the parity configuration is not modified (strict-lint-target-v2 co-1)", got, parityLintConfigSHA256))
	}
	if !slices.Contains(w.verifySteps, strictLintStep) {
		out = append(out, fmt.Sprintf("VERIFY_STEPS %v does not run %s", w.verifySteps, strictLintStep))
	}
	for _, file := range slices.Sorted(maps.Keys(w.staticRuns)) {
		if n := countOf(w.staticRuns[file], "make "+strictLintStep); n != 1 {
			out = append(out, fmt.Sprintf("%s's %s job runs `make %s` %d times, want exactly once", file, mergeGateLintJob, strictLintStep, n))
		}
	}
	runs := golangciRunRE.FindAllString(w.dryRun, -1)
	if len(runs) == 0 {
		out = append(out, "make -n lint-strict runs no golangci-lint run")
	}
	for _, r := range runs {
		out = append(out, strictRunProblems(r)...)
	}
	lintAt := strings.Index(w.dryRun, "golangci-lint run")
	checkAt := strings.Index(w.dryRun, "lintratchet check ")
	if checkAt < 0 || checkAt < lintAt {
		out = append(out, "make -n lint-strict does not run the baseline check (lintratchet check -lint-exit ...) after golangci-lint")
	}
	for _, line := range lintratchetCheckRE.FindAllString(w.dryRun, -1) {
		out = append(out, strictCheckProblems(line)...)
	}
	return out
}

// countOf counts s in list.
func countOf(list []string, s string) int {
	n := 0
	for _, x := range list {
		if x == s {
			n++
		}
	}
	return n
}

// readRepoFile returns a repository file's bytes.
func readRepoFile(t *testing.T, rel string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(verdiRepoRoot, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("reading %s: %v", rel, err)
	}
	return raw
}

// realStrictWiring reads the wiring from this checkout.
func realStrictWiring(t *testing.T) strictWiring {
	t.Helper()
	w := strictWiring{
		parity:      readRepoFile(t, ".golangci.yml"),
		verifySteps: makefileVarFields(t, readMakefile(t), "VERIFY_STEPS"),
		staticRuns:  map[string][]string{},
		dryRun:      makeDryRun(t, strictLintStep),
	}
	for _, file := range []string{"merge-gate.yml", "verify.yml"} {
		doc := decodeWorkflow(t, workflowPath(verdiRepoRoot, file))
		job, ok := doc.Jobs[mergeGateLintJob]
		if !ok {
			t.Fatalf("%s has no %q job", file, mergeGateLintJob)
		}
		w.staticRuns[file] = runCommands(job.Steps)
	}
	return w
}

// TestStrictLintTargetIsWired proves ac-1's static claim on this checkout,
// and that each falsifier is reported: a linter added, dropped, or
// misconfigured; a comment quoting no ground-rules sentence, or the wrong
// one; a nonzero cap; a changed .golangci.yml; lint-strict missing from
// VERIFY_STEPS or from either workflow's static job; a recipe lacking the
// explicit configuration or the platform; and a recipe linting less than the
// whole module, masking or dropping golangci-lint's exit status, or checking
// another baseline (review findings S1-A2 and S1-B3).
func TestStrictLintTargetIsWired(t *testing.T) {
	sentences := groundRuleSentences(string(readRepoFile(t, "docs/ground-rules.md")))
	config := readStrictLintConfig(t)
	wiring := realStrictWiring(t)

	t.Run("the committed configuration and wiring", func(t *testing.T) {
		for _, p := range strictLintConfigShapeProblems(config, sentences) {
			t.Error(p)
		}
		for _, p := range strictWiringProblems(wiring) {
			t.Error(p)
		}
	})

	const (
		contextQuote = `"` + "`context.Context`" + ` is the first parameter of anything that blocks or does I/O; never stored in a struct."`
		globalsQuote = `"No package-level mutable state: a package-level variable is only a sentinel error, a compiled regular expression, an embedded file, or a blank (` + "`_`" + `) compile-time check."`
	)
	configCases := []struct {
		name, from, to, want string
	}{
		{"a linter added", "    - gochecknoglobals\n", "    - gochecknoglobals\n    - dupl\n", "want exactly the gated linters"},
		{"a linter dropped", "    - noctx\n", "", "want exactly the gated linters"},
		{"linters enabled by default", "default: none", "default: standard", `linters.default is "standard"`},
		{"errorlint's type-assertion check on", "asserts: false", "asserts: true", "linters.settings.errorlint.asserts is true, want false"},
		{"errorlint's errorf check off", "errorf: true", "errorf: false", "linters.settings.errorlint.errorf is false, want true"},
		{"another errorlint setting", "      comparison: false\n", "      comparison: false\n      errorf-multi: false\n", "want exactly [asserts comparison errorf]"},
		{"another linter's settings", "    errorlint:\n", "    gochecknoglobals: {}\n    errorlint:\n", "want exactly [errorlint]"},
		{"a comment quoting no ground-rules sentence", globalsQuote, `"No package-level state."`, `quotes "No package-level state."`},
		{"a comment quoting the wrong sentence", "    # docs/ground-rules.md: " + globalsQuote + "\n", "    # docs/ground-rules.md: " + contextQuote + "\n", "gochecknoglobals (line"},
		{"a linter under no comment", "    # docs/ground-rules.md: " + contextQuote + "\n    - containedctx\n", "    - containedctx\n", "containedctx (line"},
		{"a capped linter", "max-issues-per-linter: 0", "max-issues-per-linter: 50", "issues.max-issues-per-linter is 50, want 0"},
		{"capped repeats", "max-same-issues: 0", "max-same-issues: 3", "issues.max-same-issues is 3, want 0"},
		{"one finding per line", "uniq-by-line: false", "uniq-by-line: true", "issues.uniq-by-line is true, want false"},
		{"test files skipped", "version: \"2\"\n", "version: \"2\"\nrun:\n  tests: false\n", "exactly the top-level keys"},
	}
	for _, tc := range configCases {
		t.Run("config: "+tc.name, func(t *testing.T) {
			problems := strictLintConfigShapeProblems(mutateSource(t, strictLintConfigFile, config, tc.from, tc.to), sentences)
			if !slices.ContainsFunc(problems, func(p string) bool { return strings.Contains(p, tc.want) }) {
				t.Fatalf("problems = %q, want one containing %q", problems, tc.want)
			}
		})
	}

	wiringCases := []struct {
		name   string
		mutate func(t *testing.T, w strictWiring) strictWiring
		want   string
	}{
		{"a changed .golangci.yml", func(_ *testing.T, w strictWiring) strictWiring {
			w.parity = append(slices.Clone(w.parity), '\n')
			return w
		}, ".golangci.yml's sha256 is"},
		{"lint-strict missing from VERIFY_STEPS", func(_ *testing.T, w strictWiring) strictWiring {
			w.verifySteps = slices.DeleteFunc(slices.Clone(w.verifySteps), func(s string) bool { return s == strictLintStep })
			return w
		}, "does not run lint-strict"},
		{"lint-strict missing from merge-gate.yml's static job", func(_ *testing.T, w strictWiring) strictWiring {
			w.staticRuns = maps.Clone(w.staticRuns)
			w.staticRuns["merge-gate.yml"] = slices.DeleteFunc(slices.Clone(w.staticRuns["merge-gate.yml"]), func(s string) bool { return s == "make lint-strict" })
			return w
		}, "merge-gate.yml's static job runs `make lint-strict` 0 times"},
		{"lint-strict missing from verify.yml's static job", func(_ *testing.T, w strictWiring) strictWiring {
			w.staticRuns = maps.Clone(w.staticRuns)
			w.staticRuns["verify.yml"] = slices.DeleteFunc(slices.Clone(w.staticRuns["verify.yml"]), func(s string) bool { return s == "make lint-strict" })
			return w
		}, "verify.yml's static job runs `make lint-strict` 0 times"},
		{"the recipe lacks the explicit configuration", func(t *testing.T, w strictWiring) strictWiring {
			w.dryRun = mutateSource(t, "the lint-strict dry run", w.dryRun, "--config .golangci.strict.yml", "")
			return w
		}, `lacks "--config .golangci.strict.yml"`},
		{"the recipe lacks the platform", func(t *testing.T, w strictWiring) strictWiring {
			w.dryRun = mutateSource(t, "the lint-strict dry run", w.dryRun, "GOOS=linux GOARCH=amd64 golangci-lint run", "golangci-lint run")
			return w
		}, `lacks "GOOS=linux GOARCH=amd64 golangci-lint run"`},
		{"the recipe treats findings as failures", func(t *testing.T, w strictWiring) strictWiring {
			w.dryRun = mutateSource(t, "the lint-strict dry run", w.dryRun, "--issues-exit-code=0", "")
			return w
		}, `lacks "--issues-exit-code=0"`},
		{"the recipe runs no baseline check", func(t *testing.T, w strictWiring) strictWiring {
			w.dryRun = mutateSource(t, "the lint-strict dry run", w.dryRun, "lintratchet check -lint-exit", "lintratchet version")
			return w
		}, "does not run the baseline check"},
		// Review finding S1-A2: a renamed baseline is absent at the merge
		// base, so the growth comparison would pass under the bootstrap
		// disclosure.
		{"the check reads another baseline", func(t *testing.T, w strictWiring) strictWiring {
			w.dryRun = mutateSource(t, "the lint-strict dry run", w.dryRun, "-baseline .golangci.strict-baseline.json;", "-baseline .golangci.strict-baseline.v2.json;")
			return w
		}, `passes -baseline ".golangci.strict-baseline.v2.json", want ".golangci.strict-baseline.json"`},
		{"the check reads a baseline whose name extends the committed one", func(t *testing.T, w strictWiring) strictWiring {
			w.dryRun = mutateSource(t, "the lint-strict dry run", w.dryRun, "-baseline .golangci.strict-baseline.json;", "-baseline .golangci.strict-baseline.json.new;")
			return w
		}, `passes -baseline ".golangci.strict-baseline.json.new", want ".golangci.strict-baseline.json"`},
		// Review finding S1-B3, the reviewer's mutant B1: the check never
		// sees golangci-lint's own exit status.
		{"the check is passed a constant exit status (mutant B1)", func(t *testing.T, w strictWiring) strictWiring {
			w.dryRun = mutateSource(t, "the lint-strict dry run", w.dryRun, `-lint-exit "$status"`, "-lint-exit 0")
			return w
		}, `passes -lint-exit "0", want "\"$status\""`},
		// Review finding S1-B3, the reviewer's mutant B3: part of the module
		// is linted and golangci-lint's exit status is masked.
		{"the run lints part of the module and masks its exit (mutant B3)", func(t *testing.T, w strictWiring) strictWiring {
			w.dryRun = mutateSource(t, "the lint-strict dry run", w.dryRun, "./...; status=$?;", "./cmd/... || true; status=$?;")
			return w
		}, "does not lint the whole module (./...)"},
		{"the run's exit status is masked with ||", func(t *testing.T, w strictWiring) strictWiring {
			w.dryRun = mutateSource(t, "the lint-strict dry run", w.dryRun, "./...; status=$?;", "./... || true; status=$?;")
			return w
		}, "masks golangci-lint's exit status with ||"},
		{"the run's exit status is not captured", func(t *testing.T, w strictWiring) strictWiring {
			w.dryRun = mutateSource(t, "the lint-strict dry run", w.dryRun, "./...; status=$?;", "./...; status=0;")
			return w
		}, "does not capture golangci-lint's own exit status as status=$? right after it"},
		{"the run lints only part of the module", func(t *testing.T, w strictWiring) strictWiring {
			w.dryRun = mutateSource(t, "the lint-strict dry run", w.dryRun, "./...; status=$?;", "./internal/...; status=$?;")
			return w
		}, "does not lint the whole module (./...)"},
	}
	for _, tc := range wiringCases {
		t.Run("wiring: "+tc.name, func(t *testing.T) {
			problems := strictWiringProblems(tc.mutate(t, wiring))
			if !slices.ContainsFunc(problems, func(p string) bool { return strings.Contains(p, tc.want) }) {
				t.Fatalf("problems = %q, want one containing %q", problems, tc.want)
			}
		})
	}
}

// TestGroundRuleSentences covers splitting the rules file into sentences.
func TestGroundRuleSentences(t *testing.T) {
	md := "# Title\n\nA paragraph. Its second\nsentence.\n\n## Section\n\n- One rule: wrapped\n  over lines; still one.\n- Two. Three\n"
	want := []string{"A paragraph.", "Its second sentence.", "One rule: wrapped over lines; still one.", "Two.", "Three"}
	if got := groundRuleSentences(md); !slices.Equal(got, want) {
		t.Fatalf("groundRuleSentences = %q, want %q", got, want)
	}
}
