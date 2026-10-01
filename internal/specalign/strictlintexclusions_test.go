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
// A pattern that matches the empty string matches every path or message
// (golangci-lint searches with an unanchored regular expression), so it
// excludes wholesale; so does a rule that names neither a path nor a text
// pattern. A reason is a YAML comment directly above the exclusion's list
// item: golangci-lint v2's rule schema has no reason field.
//
// Out of this witness's scope, by the obligation's own scope
// (.golangci.strict.yml): //nolint directives in Go source, which suppress
// a finding at its line for whichever configuration runs.
package specalign

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
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

// wholesalePattern reports why pattern excludes wholesale or cannot be read,
// or "" when it names a narrower path or text.
func wholesalePattern(pattern string) string {
	if strings.TrimSpace(pattern) == "" {
		return "is empty"
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Sprintf("is not a regular expression (%v)", err)
	}
	if re.MatchString("") {
		return "matches the empty string, so it matches every path or message: a wholesale exclusion"
	}
	return ""
}

// strictLintExclusionProblems returns every way raw, a strict configuration's
// source, departs from ac-3: a gated linter disabled or not enabled, an
// implicit exclusion (presets, or generated other than `disable`), an
// exclusion that names no path or text pattern, names one that matches
// everything, or carries no reason, and an exclusion count other than
// pinned.
func strictLintExclusionProblems(raw string, pinned int) []string {
	top, err := decodeStrictLintConfig(raw)
	if err != nil {
		return []string{err.Error()}
	}
	var out []string
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
		if extra := keysOutside(sortedKeys(rule), []string{"linters", "path", "path-except", "source", "text"}); len(extra) != 0 {
			out = append(out, fmt.Sprintf("exclusion rule %d declares %v, which golangci-lint v2's rule schema does not have", i, extra))
		}
		pathPattern, hasPath := asStringVal(rule["path"])
		textPattern, hasText := asStringVal(rule["text"])
		if !hasPath && !hasText {
			out = append(out, fmt.Sprintf("exclusion rule %d names neither a path nor a text pattern, so it excludes wholesale", i))
		}
		if hasPath {
			if why := wholesalePattern(pathPattern); why != "" {
				out = append(out, fmt.Sprintf("exclusion rule %d's path %q %s", i, pathPattern, why))
			}
		}
		if hasText {
			if why := wholesalePattern(textPattern); why != "" {
				out = append(out, fmt.Sprintf("exclusion rule %d's text %q %s", i, textPattern, why))
			}
		}
	}
	paths, _ := asSlice(exMap["paths"])
	for i, p := range paths {
		count++
		pattern, _ := asStringVal(p)
		if why := wholesalePattern(pattern); why != "" {
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
// and that the witness bites: every exclusion in .golangci.strict.yml names a
// path or a text pattern and carries a reason, no gated linter is disabled or
// excluded wholesale, and the exclusion count equals strictLintExclusionCount;
// each falsifier shape, applied to a fixture, is reported.
func TestStrictLintExclusionsCounted(t *testing.T) {
	t.Run("the committed configuration", func(t *testing.T) {
		for _, p := range strictLintExclusionProblems(readStrictLintConfig(t), strictLintExclusionCount) {
			t.Error(p)
		}
	})

	mutate := func(t *testing.T, from, to string) string {
		return mutateSource(t, "the exclusion fixture", strictExclusionFixture, from, to)
	}
	const (
		ruleReason = "      # Generated protobuf code is never edited by hand.\n"
		pathReason = "      # A vendored copy is linted upstream.\n"
	)
	cases := []struct {
		name   string
		config func(t *testing.T) string
		pinned int
		want   string // a substring of one problem; "" wants none
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
			problems := strictLintExclusionProblems(tc.config(t), tc.pinned)
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
