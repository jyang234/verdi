// spec/strict-lint-gate ac-3 (and strict-lint-target-v2 ac-4): the strict
// lint configuration disables no gated linter and excludes none wholesale;
// every exclusion names a path or a text pattern and carries a reason, and
// the number of exclusions is pinned here, so every added exclusion is a
// visible change to this file.
//
// golangci-lint v2 has three ways to exclude findings in a configuration:
// linters.exclusions.rules (a rule names a path, a text pattern, a source
// pattern, or linters), linters.exclusions.paths (path patterns excluded for
// every linter), and linters.exclusions.presets and linters.exclusions.
// generated (implicit exclusions that name no path or pattern). The checks
// below allow the first two, each named and reasoned, and refuse the other
// two outright: presets are not allowed, and generated must be `disable`
// (ledger SI-311 (3); v2's default, `lax`, is an exclusion the configuration
// never names).
//
// A wholesale exclusion is one that leaves a gated linter nothing to report,
// however its patterns are written (review finding S1-B1). golangci-lint
// 2.5.0 searches a rule's path, an excluded path, and a rule's text with
// unanchored, case-sensitive regular expressions, so the witness measures
// each pattern against what it could exclude: a path pattern against every
// Go file `golangci-lint run ./...` lints for linux/amd64, a text pattern against every
// message each gated linter the rule applies to is known to report (the
// committed baseline's and the strict fixture module's). A path pattern that
// matches every Go file excludes wholesale; so does a text pattern that
// matches every known message of a gated linter the rule applies to; a rule
// naming both excludes wholesale when both match everything, and a rule that
// names neither excludes wholesale outright. A pattern that matches the empty
// string matches everything. A rule's path-except or source does not narrow
// it here: the witness errs toward a visible refusal. A reason is a YAML
// comment directly above the exclusion's list item: golangci-lint v2's rule
// schema has no reason field.
//
// spec/strict-lint-gate-v2 ac-3 widens the witness to //nolint directives in
// Go source, which suppress a finding at its line for whichever configuration
// runs: the same test counts those that name a gated linter apart from these
// exclusions, against a pin of its own, and refuses one that suppresses every
// linter (strictlintdirectives_test.go).
package specalign

import (
	"errors"
	"fmt"
	"go/build"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/lintratchet"
)

// strictLintConfigFile is the strict lint configuration, at the repository
// root (strict-lint-target-v2 dc-6).
const strictLintConfigFile = ".golangci.strict.yml"

// strictLintExclusionCount is the pinned number of exclusions in
// .golangci.strict.yml: rules plus paths. Adding an exclusion means changing
// this number in the same change (ac-3).
const strictLintExclusionCount = 0

// strictGatedLinters is the gated set (strict-lint-target-v2 dc-2), sorted.
func strictGatedLinters() []string {
	return []string{"containedctx", "contextcheck", "errorlint", "gochecknoglobals", "noctx"}
}

// readStrictLintConfig returns the real strict configuration's text.
func readStrictLintConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(verdiRepoRoot, strictLintConfigFile)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(raw)
}

// decodeStrictLintConfig decodes the configuration through internal/artifact's
// loose-decode seam (foreign-schema YAML; see workflow_test.go's package
// comment) and returns its top-level mapping.
func decodeStrictLintConfig(raw string) (map[string]interface{}, error) {
	generic, err := artifact.DecodeYAMLLoose([]byte(raw))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", strictLintConfigFile, err)
	}
	top, ok := asMap(generic)
	if !ok {
		return nil, fmt.Errorf("%s: the document is not a mapping (got %T)", strictLintConfigFile, generic)
	}
	return top, nil
}

// lookupPath walks a decoded tree down the given mapping keys. It reports
// false when a key is absent or a value on the way is not a mapping.
func lookupPath(tree map[string]interface{}, keys ...string) (interface{}, bool) {
	var cur interface{} = tree
	for _, k := range keys {
		m, ok := asMap(cur)
		if !ok {
			return nil, false
		}
		cur, ok = m[k]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// listItemLines returns the line index of each item of the block sequence
// under the mapping key on line section of lines: the lines that begin with
// "-" at the indentation of the first such line after section, up to the
// first line indented less than that, or at that indentation but not an item.
// Blank and comment lines are skipped. It returns nil when the value under
// section is not a block sequence (a flow sequence, say).
func listItemLines(lines []string, section int) []int {
	var out []int
	indent := -1
	for i := section + 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		ind := len(line) - len(strings.TrimLeft(line, " "))
		isItem := trimmed == "-" || strings.HasPrefix(trimmed, "- ")
		if indent < 0 {
			if !isItem {
				return nil
			}
			indent = ind
		}
		if ind < indent || (ind == indent && !isItem) {
			break
		}
		if ind == indent {
			out = append(out, i)
		}
	}
	return out
}

// commentAbove returns the text of the YAML comment block directly above
// line i of lines, its lines joined and whitespace-collapsed, or "" when the
// line above is not a comment.
func commentAbove(lines []string, i int) string {
	var comment []string
	for j := i - 1; j >= 0; j-- {
		above := strings.TrimSpace(lines[j])
		if !strings.HasPrefix(above, "#") {
			break
		}
		comment = append([]string{strings.TrimSpace(strings.TrimPrefix(above, "#"))}, comment...)
	}
	return strings.Join(strings.Fields(strings.Join(comment, " ")), " ")
}

// sectionLine returns the index of the first line at or after start whose
// trimmed text is exactly key + ":", or -1.
func sectionLine(lines []string, start int, key string) int {
	for i := start; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == key+":" {
			return i
		}
	}
	return -1
}

// exclusionReasonProblems reports each item of the exclusion list under the
// key named what that carries no reason comment, or that cannot be found in
// the source to read one from. n is the decoded list's length.
func exclusionReasonProblems(lines []string, exLine int, what, label string, n int) []string {
	if n == 0 {
		return nil
	}
	section := -1
	if exLine >= 0 {
		section = sectionLine(lines, exLine, what)
	}
	items := listItemLines(lines, section)
	if section < 0 || len(items) != n {
		return []string{fmt.Sprintf("linters.exclusions.%s holds %d item(s), but its block-sequence items could not be read from the source to find their reasons: write each as a `- ` item under a `%s:` line, with its reason in a comment directly above it", what, n, what)}
	}
	var out []string
	for k, at := range items {
		if commentAbove(lines, at) == "" {
			out = append(out, fmt.Sprintf("%s %d (line %d) carries no reason: put a comment directly above it", label, k, at+1))
		}
	}
	return out
}

// strictFixtureReportFile is the committed capture of the strict fixture
// module's golangci-lint report, one finding per gated linter;
// internal/lintratchet's TestStrictFixtureReportIsCurrent holds it equal to
// a live run of the pinned golangci-lint.
const strictFixtureReportFile = "internal/lintratchet/testdata/reports/strictfixture.json"

// strictLintUniverse is what an exclusion's patterns are measured against to
// tell a wholesale exclusion from a narrow one (review finding S1-B1).
type strictLintUniverse struct {
	// goFiles are the Go files `golangci-lint run ./...` lints for
	// linux/amd64, relative to the repository root (the configuration's directory, which
	// golangci-lint matches path patterns relative to) and slash-separated.
	goFiles []string
	// messages are, per gated linter, the distinct messages it is known to
	// report: the committed baseline's and the strict fixture module's.
	messages map[string][]string
}

// strictLintBuildContext is the platform make lint-strict lints for,
// GOOS=linux GOARCH=amd64, with go/build's other defaults (the toolchain's
// release and tool tags).
func strictLintBuildContext() build.Context {
	ctx := build.Default
	ctx.GOOS, ctx.GOARCH = "linux", "amd64"
	return ctx
}

// moduleGoFiles returns, sorted, the Go files under root that `./...`
// visits and golangci-lint lints for linux/amd64: it skips directories named
// testdata, vendor, or node_modules, or beginning with "." or "_"
// (guideClaimCorpusSkipDir), directories holding their own go.mod (another
// module), files beginning with "." or "_", and files whose build
// constraints (a _GOOS or _GOARCH name suffix, a //go:build line) exclude
// them for linux/amd64 (review finding S1-RR1: a file golangci-lint never
// lints cannot keep a path pattern from excluding wholesale).
func moduleGoFiles(root string) ([]string, error) {
	ctx := strictLintBuildContext()
	var files []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if p == root {
				return nil
			}
			if guideClaimCorpusSkipDir(name) {
				return filepath.SkipDir
			}
			switch _, err := os.Stat(filepath.Join(p, "go.mod")); {
			case err == nil:
				return filepath.SkipDir
			case !errors.Is(err, fs.ErrNotExist):
				return err
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			return nil
		}
		switch match, err := ctx.MatchFile(filepath.Dir(p), name); {
		case err != nil:
			return fmt.Errorf("reading %s's build constraints: %w", p, err)
		case !match:
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	return files, err
}

// messagesByLinter returns each linter's distinct messages among keys,
// sorted.
func messagesByLinter(keys []lintratchet.Key) map[string][]string {
	seen := map[string]map[string]bool{}
	for _, k := range keys {
		if seen[k.Linter] == nil {
			seen[k.Linter] = map[string]bool{}
		}
		seen[k.Linter][k.Message] = true
	}
	out := map[string][]string{}
	for linter, msgs := range seen {
		out[linter] = slices.Sorted(maps.Keys(msgs))
	}
	return out
}

// realStrictLintUniverse measures the universe on this checkout: the
// module's Go files, and each gated linter's messages in the committed
// baseline and the strict fixture's captured report.
func realStrictLintUniverse(t *testing.T) strictLintUniverse {
	t.Helper()
	files, err := moduleGoFiles(verdiRepoRoot)
	if err != nil {
		t.Fatalf("listing the module's Go files: %v", err)
	}
	baseline, err := lintratchet.ParseBaseline(readRepoFile(t, strictLintBaselineFile))
	if err != nil {
		t.Fatalf("%s: %v", strictLintBaselineFile, err)
	}
	fixture, err := lintratchet.ParseReport(readRepoFile(t, strictFixtureReportFile))
	if err != nil {
		t.Fatalf("%s: %v", strictFixtureReportFile, err)
	}
	keys := slices.Collect(maps.Keys(baseline))
	for _, f := range fixture {
		keys = append(keys, f.Key)
	}
	return strictLintUniverse{goFiles: files, messages: messagesByLinter(keys)}
}

// universeProblems reports what the universe cannot measure: no Go file to
// measure a path pattern against, or a gated linter with no known message to
// measure a text pattern against. Either would let a wholesale exclusion
// pass unmeasured.
func (u strictLintUniverse) universeProblems() []string {
	var out []string
	if len(u.goFiles) == 0 {
		out = append(out, "no Go file of the module is known, so no path pattern can be measured for a wholesale exclusion")
	}
	for _, l := range strictGatedLinters() {
		if len(u.messages[l]) == 0 {
			out = append(out, fmt.Sprintf("no %s message is known (neither %s nor %s holds one), so no text pattern can be measured against it for a wholesale exclusion", l, strictLintBaselineFile, strictFixtureReportFile))
		}
	}
	return out
}

// everyPath reports how re, a path pattern, matches every Go file
// golangci-lint lints, or "" when it leaves one out.
func (u strictLintUniverse) everyPath(re *regexp.Regexp) string {
	if re.MatchString("") {
		return "matches the empty string, so it matches every path"
	}
	if slices.ContainsFunc(u.goFiles, func(f string) bool { return !re.MatchString(f) }) {
		return ""
	}
	return fmt.Sprintf("matches every one of the module's %d Go files golangci-lint lints for linux/amd64", len(u.goFiles))
}

// everyMessage reports how re, the text pattern of a rule naming linters
// (every gated linter when it names none, as golangci-lint reads a rule
// without linters), matches every known message of a gated linter the rule
// applies to, or "" when it leaves one of each out.
func (u strictLintUniverse) everyMessage(re *regexp.Regexp, linters []string) string {
	if re.MatchString("") {
		return "matches the empty string, so it matches every message"
	}
	for _, l := range strictGatedLinters() {
		msgs := u.messages[l]
		if (len(linters) != 0 && !slices.Contains(linters, l)) || len(msgs) == 0 {
			continue
		}
		if !slices.ContainsFunc(msgs, func(m string) bool { return !re.MatchString(m) }) {
			return fmt.Sprintf("matches every %s message the witness knows (%d, from the committed baseline and the strict fixture)", l, len(msgs))
		}
	}
	return ""
}

// compilePattern compiles an exclusion's path or text pattern, or reports
// why it cannot be read as one: empty, or not a regular expression.
func compilePattern(pattern string) (*regexp.Regexp, string) {
	if strings.TrimSpace(pattern) == "" {
		return nil, "is empty"
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Sprintf("is not a regular expression (%v)", err)
	}
	return re, ""
}

// exclusionRuleProblems reports how rule i departs from ac-3: a key outside
// golangci-lint v2's rule schema, no path or text pattern, a pattern that
// cannot be read, or patterns that together exclude a gated linter
// wholesale (every named path and text pattern matching everything).
func exclusionRuleProblems(i int, rule map[string]interface{}, u strictLintUniverse) []string {
	var out []string
	if extra := keysOutside(sortedKeys(rule), []string{"linters", "path", "path-except", "source", "text"}); len(extra) != 0 {
		out = append(out, fmt.Sprintf("exclusion rule %d declares %v, which golangci-lint v2's rule schema does not have", i, extra))
	}
	pathPattern, hasPath := asStringVal(rule["path"])
	textPattern, hasText := asStringVal(rule["text"])
	if !hasPath && !hasText {
		return append(out, fmt.Sprintf("exclusion rule %d names neither a path nor a text pattern, so it excludes wholesale", i))
	}
	named, everything := 0, []string(nil)
	if hasPath {
		named++
		if re, why := compilePattern(pathPattern); why != "" {
			out = append(out, fmt.Sprintf("exclusion rule %d's path %q %s", i, pathPattern, why))
		} else if all := u.everyPath(re); all != "" {
			everything = append(everything, fmt.Sprintf("its path %q %s", pathPattern, all))
		}
	}
	if hasText {
		named++
		if re, why := compilePattern(textPattern); why != "" {
			out = append(out, fmt.Sprintf("exclusion rule %d's text %q %s", i, textPattern, why))
		} else if all := u.everyMessage(re, asStringSlice(rule["linters"])); all != "" {
			everything = append(everything, fmt.Sprintf("its text %q %s", textPattern, all))
		}
	}
	if len(everything) == named {
		out = append(out, fmt.Sprintf("exclusion rule %d excludes wholesale: %s, and it names no narrower pattern", i, strings.Join(everything, ", and ")))
	}
	return out
}

// strictLintExclusionProblems returns every way raw, a strict configuration's
// source, departs from ac-3, measured against u: a gated linter disabled or
// not enabled, an implicit exclusion (presets, or generated other than
// `disable`), an exclusion that names no path or text pattern, names one that
// cannot be read, excludes a gated linter wholesale, or carries no reason, an
// exclusion count other than pinned, and a universe that cannot measure an
// exclusion.
func strictLintExclusionProblems(raw string, pinned int, u strictLintUniverse) []string {
	top, err := decodeStrictLintConfig(raw)
	if err != nil {
		return []string{err.Error()}
	}
	out := u.universeProblems()
	if v, ok := lookupPath(top, "linters", "disable"); ok && v != nil {
		out = append(out, fmt.Sprintf("linters.disable is set (%v): the strict configuration disables no linter", v))
	}
	enabled := asStringSlice(lookupOrNil(top, "linters", "enable"))
	for _, l := range strictGatedLinters() {
		if !slices.Contains(enabled, l) {
			out = append(out, fmt.Sprintf("gated linter %s is not in linters.enable %v", l, enabled))
		}
	}

	exclusions, ok := lookupPath(top, "linters", "exclusions")
	if !ok {
		return append(out, "linters.exclusions is absent: exclusions.generated must be set to disable, or golangci-lint v2 excludes generated files implicitly (SI-311 (3))")
	}
	exMap, ok := asMap(exclusions)
	if !ok {
		return append(out, fmt.Sprintf("linters.exclusions is not a mapping (got %T)", exclusions))
	}
	allowed := []string{"generated", "paths", "rules"}
	if extra := keysOutside(sortedKeys(exMap), allowed); len(extra) != 0 {
		out = append(out, fmt.Sprintf("linters.exclusions declares %v, outside %v: presets and other implicit exclusions name no path or pattern", extra, allowed))
	}
	if g, _ := asStringVal(exMap["generated"]); g != "disable" {
		out = append(out, fmt.Sprintf("linters.exclusions.generated is %q, want \"disable\": any other mode excludes generated files without naming them (SI-311 (3))", g))
	}

	count := 0
	rules, _ := asSlice(exMap["rules"])
	for i, r := range rules {
		count++
		rule, ok := asMap(r)
		if !ok {
			out = append(out, fmt.Sprintf("exclusion rule %d is not a mapping", i))
			continue
		}
		out = append(out, exclusionRuleProblems(i, rule, u)...)
	}
	paths, _ := asSlice(exMap["paths"])
	for i, p := range paths {
		count++
		pattern, _ := asStringVal(p)
		re, why := compilePattern(pattern)
		if why == "" {
			if all := u.everyPath(re); all != "" {
				why = all + ": a wholesale exclusion"
			}
		}
		if why != "" {
			out = append(out, fmt.Sprintf("excluded path %d %q %s", i, pattern, why))
		}
	}

	lines := strings.Split(raw, "\n")
	exLine := sectionLine(lines, 0, "exclusions")
	out = append(out, exclusionReasonProblems(lines, exLine, "rules", "exclusion rule", len(rules))...)
	out = append(out, exclusionReasonProblems(lines, exLine, "paths", "excluded path", len(paths))...)

	if count != pinned {
		out = append(out, fmt.Sprintf("%s holds %d exclusion(s), but strictLintExclusionCount pins %d: change the pin in the same change, so every added exclusion is visible", strictLintConfigFile, count, pinned))
	}
	return out
}

// lookupOrNil is lookupPath returning nil when the path is absent.
func lookupOrNil(tree map[string]interface{}, keys ...string) interface{} {
	v, _ := lookupPath(tree, keys...)
	return v
}

// strictExclusionFixture is a strict configuration with one reasoned rule and
// one reasoned path, the base each falsifier case below mutates.
const strictExclusionFixture = `version: "2"
linters:
  default: none
  enable:
    - containedctx
    - contextcheck
    - noctx
    - errorlint
    - gochecknoglobals
  exclusions:
    generated: disable
    rules:
      # Generated protobuf code is never edited by hand.
      - path: \.pb\.go$
        linters:
          - gochecknoglobals
    paths:
      # A vendored copy is linted upstream.
      - ^third_party/
issues:
  max-issues-per-linter: 0
  max-same-issues: 0
  uniq-by-line: false
`

// TestStrictLintExclusionsCounted proves ac-3 on the committed configuration
// and source, and that the witness bites: every exclusion in
// .golangci.strict.yml names a path or a text pattern and carries a reason,
// no gated linter is disabled or excluded wholesale, and the exclusion count
// equals strictLintExclusionCount; every //nolint directive in the module's
// linux/amd64 lint set that names a gated linter carries a reason, none
// suppresses every linter, and their count equals strictLintDirectiveCount
// (spec/strict-lint-gate-v2 ac-3); each falsifier shape, applied to a
// fixture, is reported.
func TestStrictLintExclusionsCounted(t *testing.T) {
	t.Run("source directives", testStrictLintSourceDirectives)
	universe := realStrictLintUniverse(t)
	t.Run("the committed configuration", func(t *testing.T) {
		for _, p := range strictLintExclusionProblems(readStrictLintConfig(t), strictLintExclusionCount, universe) {
			t.Error(p)
		}
	})

	mutate := func(t *testing.T, from, to string) string {
		return mutateSource(t, "the exclusion fixture", strictExclusionFixture, from, to)
	}
	const (
		ruleReason  = "      # Generated protobuf code is never edited by hand.\n"
		pathReason  = "      # A vendored copy is linted upstream.\n"
		fixtureRule = "      - path: \\.pb\\.go$\n        linters:\n          - gochecknoglobals\n"
	)
	cases := []struct {
		name     string
		config   func(t *testing.T) string
		pinned   int
		universe func(u strictLintUniverse) strictLintUniverse // nil measures against the real one
		want     string                                        // a substring of one problem; "" wants none
	}{
		{name: "the fixture, reasoned and counted", config: func(*testing.T) string { return strictExclusionFixture }, pinned: 2},
		{name: "a count other than the pin", config: func(*testing.T) string { return strictExclusionFixture }, pinned: 1, want: "holds 2 exclusion(s), but strictLintExclusionCount pins 1"},
		{name: "a rule without a reason", config: func(t *testing.T) string { return mutate(t, ruleReason, "") }, pinned: 2, want: "exclusion rule 0 (line 13) carries no reason"},
		{name: "a path without a reason", config: func(t *testing.T) string { return mutate(t, pathReason, "") }, pinned: 2, want: "excluded path 0 (line 18) carries no reason"},
		{name: "a rule naming only linters", config: func(t *testing.T) string {
			return mutate(t, "      - path: \\.pb\\.go$\n        linters:", "      - linters:")
		}, pinned: 2, want: "names neither a path nor a text pattern"},
		{name: "a rule whose path matches everything", config: func(t *testing.T) string { return mutate(t, `path: \.pb\.go$`, `path: .*`) }, pinned: 2, want: `path ".*" matches the empty string`},
		{name: "a rule whose text matches everything", config: func(t *testing.T) string { return mutate(t, `path: \.pb\.go$`, `text: "(?:)"`) }, pinned: 2, want: `text "(?:)" matches the empty string`},
		{name: "an excluded path matching everything", config: func(t *testing.T) string { return mutate(t, "      - ^third_party/\n", "      - \"\"\n") }, pinned: 2, want: `excluded path 0 "" is empty`},
		// Review finding S1-B1: wholesale exclusions whose patterns do not
		// match the empty string, measured against the module's Go files and
		// each gated linter's known messages. The first four are the
		// reviewer's.
		{name: "a text pattern matching every noctx message", config: func(t *testing.T) string {
			return mutate(t, fixtureRule, "      - text: \".\"\n        linters:\n          - noctx\n")
		}, pinned: 2, want: `its text "." matches every noctx message`},
		{name: "a path pattern matching every Go file, for one linter", config: func(t *testing.T) string { return mutate(t, `path: \.pb\.go$`, `path: "."`) }, pinned: 2, want: `its path "." matches every one of the module's`},
		{name: "a path pattern matching every Go file, for every linter", config: func(t *testing.T) string {
			return mutate(t, fixtureRule, "      - path: \\.go$\n")
		}, pinned: 2, want: `its path "\\.go$" matches every one of the module's`},
		{name: "a text pattern matching every gochecknoglobals message", config: func(t *testing.T) string {
			return mutate(t, `path: \.pb\.go$`, `text: "is a global variable"`)
		}, pinned: 2, want: `its text "is a global variable" matches every gochecknoglobals message`},
		{name: "a text pattern matching every message of one of the linters a rule names none of", config: func(t *testing.T) string {
			return mutate(t, fixtureRule, "      - text: \"found a struct\"\n")
		}, pinned: 2, want: `its text "found a struct" matches every containedctx message`},
		{name: "a path and a text pattern, both matching everything", config: func(t *testing.T) string {
			return mutate(t, `path: \.pb\.go$`, "path: \\.go$\n        text: \"is a global variable\"")
		}, pinned: 2, want: `excludes wholesale: its path "\\.go$" matches every one of the module's`},
		{name: "an excluded path matching every Go file", config: func(t *testing.T) string { return mutate(t, "      - ^third_party/\n", "      - \\.go$\n") }, pinned: 2, want: `excluded path 0 "\\.go$" matches every one of the module's`},
		// Review finding S1-RR1 (the re-reviewer's mutant OWN2): a path
		// pattern matching every file golangci-lint lints for linux/amd64,
		// but not a darwin-only file it never lints, excludes wholesale all
		// the same.
		{name: "a path pattern matching every linux/amd64 file but not a darwin-only one", config: func(t *testing.T) string {
			return mutate(t, `path: \.pb\.go$`, `path: '(?:[^n]|[^i]n|[^w]in|[^r]win|[^a]rwin|[^d]arwin|[^_]darwin)\.go$'`)
		}, pinned: 2, want: `excludes wholesale: its path "(?:[^n]|[^i]n|[^w]in|[^r]win|[^a]rwin|[^d]arwin|[^_]darwin)\\.go$" matches every one of the module's`},
		// A narrow pattern beside one that matches everything is not
		// wholesale: the rule still leaves the linter findings to report.
		{name: "a narrow path beside a text matching everything", config: func(t *testing.T) string {
			return mutate(t, `path: \.pb\.go$`, "path: ^internal/\n        text: \".\"")
		}, pinned: 2},
		{name: "a narrow text beside a path matching everything", config: func(t *testing.T) string {
			return mutate(t, `path: \.pb\.go$`, "path: \\.go$\n        text: \"^counter is\"")
		}, pinned: 2},
		{name: "a text pattern matching every message of a linter the rule does not name", config: func(t *testing.T) string {
			return mutate(t, `path: \.pb\.go$`, `text: "must not be called"`)
		}, pinned: 2},
		// A universe that cannot measure an exclusion fails rather than
		// passing it unmeasured.
		{name: "no Go file to measure a path against", config: func(*testing.T) string { return strictExclusionFixture }, pinned: 2, universe: func(u strictLintUniverse) strictLintUniverse {
			u.goFiles = nil
			return u
		}, want: "no Go file of the module is known"},
		{name: "no known message of a gated linter", config: func(*testing.T) string { return strictExclusionFixture }, pinned: 2, universe: func(u strictLintUniverse) strictLintUniverse {
			u.messages = maps.Clone(u.messages)
			delete(u.messages, "errorlint")
			return u
		}, want: "no errorlint message is known"},
		{name: "a rule key outside the schema", config: func(t *testing.T) string {
			return mutate(t, "        linters:\n", "        reason: x\n        linters:\n")
		}, pinned: 2, want: "declares [reason]"},
		{name: "a preset", config: func(t *testing.T) string {
			return mutate(t, "    generated: disable\n", "    generated: disable\n    presets:\n      - comments\n")
		}, pinned: 2, want: "declares [presets]"},
		{name: "generated files excluded implicitly", config: func(t *testing.T) string { return mutate(t, "generated: disable", "generated: lax") }, pinned: 2, want: `generated is "lax", want "disable"`},
		{name: "no exclusions section", config: func(t *testing.T) string {
			return mutate(t, strictExclusionFixture[strings.Index(strictExclusionFixture, "  exclusions:\n"):strings.Index(strictExclusionFixture, "issues:\n")], "")
		}, pinned: 0, want: "linters.exclusions is absent"},
		{name: "a gated linter disabled", config: func(t *testing.T) string {
			return mutate(t, "  exclusions:\n", "  disable:\n    - noctx\n  exclusions:\n")
		}, pinned: 2, want: "linters.disable is set"},
		{name: "a gated linter not enabled", config: func(t *testing.T) string { return mutate(t, "    - errorlint\n", "") }, pinned: 2, want: "gated linter errorlint is not in linters.enable"},
		{name: "a second rule without a reason", config: func(t *testing.T) string {
			return mutate(t, "    paths:\n", "      - text: \"is a global variable$\"\n        path: _test\\.go$\n    paths:\n")
		}, pinned: 3, want: "exclusion rule 1 (line 17) carries no reason"},
		{name: "a rule list whose items cannot be read", config: func(t *testing.T) string {
			return mutate(t, ruleReason+"      - path: \\.pb\\.go$\n        linters:\n          - gochecknoglobals\n", "      [{path: x_test\\.go$}]\n")
		}, pinned: 2, want: "linters.exclusions.rules holds 1 item(s), but its block-sequence items could not be read"},
		{name: "not YAML", config: func(*testing.T) string { return "linters: [" }, pinned: 0, want: strictLintConfigFile},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := universe
			if tc.universe != nil {
				u = tc.universe(u)
			}
			problems := strictLintExclusionProblems(tc.config(t), tc.pinned, u)
			if tc.want == "" {
				if len(problems) != 0 {
					t.Fatalf("problems = %q, want none", problems)
				}
				return
			}
			if !slices.ContainsFunc(problems, func(p string) bool { return strings.Contains(p, tc.want) }) {
				t.Fatalf("problems = %q, want one containing %q", problems, tc.want)
			}
		})
	}
}

// TestModuleGoFiles covers the path universe: the Go files `./...` visits,
// skipping what the go tool skips (testdata, vendor, "." and "_" names,
// another module) and every file that is not Go; a root that cannot be
// walked is an error, never an empty universe.
func TestModuleGoFiles(t *testing.T) {
	root := t.TempDir()
	const pkg = "package p\n"
	for f, content := range map[string]string{
		"a.go": pkg, "b_test.go": pkg, "doc.md": "", "pkg/z.go": pkg, "pkg/_x.go": pkg, "pkg/.y.go": pkg,
		".hidden/x.go": pkg, "_scratch/x.go": pkg, "testdata/x.go": pkg, "pkg/testdata/x.go": pkg,
		"vendor/x.go": pkg, "node_modules/x.go": pkg, "nested/go.mod": "module nested\n", "nested/x.go": pkg,
		// Build constraints, for linux/amd64 (review finding S1-RR1): file
		// name suffixes and //go:build lines.
		"pkg/r_linux.go": pkg, "pkg/r_darwin.go": pkg, "pkg/s_windows_test.go": pkg, "pkg/s_arm64.go": pkg,
		"pkg/tagged_unix.go":        "//go:build unix\n\n" + pkg,
		"pkg/tagged_darwin_only.go": "//go:build darwin\n\n" + pkg,
		"pkg/tagged_not_linux.go":   "//go:build !linux\n\n" + pkg,
	} {
		p := filepath.Join(root, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		name    string
		root    string
		want    []string
		wantErr bool
	}{
		{name: "a module tree", root: root, want: []string{"a.go", "b_test.go", "pkg/r_linux.go", "pkg/tagged_unix.go", "pkg/z.go"}},
		{name: "a root that does not exist", root: filepath.Join(root, "absent"), wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := moduleGoFiles(tc.root)
			if (err != nil) != tc.wantErr {
				t.Fatalf("moduleGoFiles(%s) error = %v, want error %v", tc.root, err, tc.wantErr)
			}
			if !tc.wantErr && !slices.Equal(got, tc.want) {
				t.Fatalf("moduleGoFiles(%s) = %q, want %q", tc.root, got, tc.want)
			}
		})
	}
}

// TestMessagesByLinter covers the message universe: each linter's distinct
// messages, sorted, and nothing for a linter no key names.
func TestMessagesByLinter(t *testing.T) {
	cases := []struct {
		name string
		keys []lintratchet.Key
		want map[string][]string
	}{
		{name: "no keys", keys: nil, want: map[string][]string{}},
		{name: "repeats and two linters", keys: []lintratchet.Key{
			{Linter: "noctx", Message: "b", Package: "p"},
			{Linter: "noctx", Message: "a", Package: "q"},
			{Linter: "noctx", Message: "b", Package: "r", Source: "s"},
			{Linter: "errorlint", Message: "c"},
		}, want: map[string][]string{"noctx": {"a", "b"}, "errorlint": {"c"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := messagesByLinter(tc.keys)
			if !maps.EqualFunc(got, tc.want, slices.Equal[[]string]) {
				t.Fatalf("messagesByLinter = %q, want %q", got, tc.want)
			}
		})
	}
}
