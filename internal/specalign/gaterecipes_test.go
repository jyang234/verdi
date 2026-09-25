// Makefile-source guards for the gate targets (SI-266). The shard and parity
// guards read `make -n`, which prints the commands make would run with every
// variable expanded, but two things that decide whether the gate can go green
// over a failing test never reach that output:
//
//   - `make verify` itself. Its recipe calls $(MAKE), so even a dry-run
//     executes it; the guards therefore dry-run its steps one at a time and
//     never see verify's own prerequisites or recipe. A prerequisite, or an
//     extra command in the recipe, would run under `make verify` but in no
//     pull-request gate job.
//   - Error ignoring. A `-` recipe prefix, whether written or reached through a
//     leading variable, makes make treat a failed command as a success, and
//     `make -n` strips the prefix from what it prints. So do `.IGNORE`, make
//     flags set from the Makefile (MAKEFLAGS, or -i, -k, -n, -t, -q on a
//     sub-make), and the special targets and shell settings that change how a
//     recipe's failure is seen.
//
// This file reads the Makefile source for both.
package specalign

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// recipeLine is one logical recipe line: the tab-led physical line and every
// backslash continuation after it, joined by "\n", each with its leading
// recipe tab removed. Line is the first physical line's 1-based number.
type recipeLine struct {
	Line int
	Text string
}

// makeRule is one rule line of a Makefile as its source spells it: the
// targets before the colon, the prerequisite text after it (up to any `;` or
// `#`, trimmed), and the recipe. An inline `target: ; command` recipe is the
// first recipe line.
type makeRule struct {
	Line    int
	Targets []string
	Prereqs string
	Recipe  []recipeLine
}

// makeConditionals open or close a conditional block. They do not end a
// rule's recipe: GNU make keeps collecting recipe lines across them.
var makeConditionals = []string{"ifeq", "ifneq", "ifdef", "ifndef", "else", "endif"}

// makeDirectives open a non-rule line that ends a rule's recipe.
var makeDirectives = []string{"include", "-include", "sinclude", "export", "unexport", "override", "private", "vpath", "undefine"}

// parseMakeRules returns a Makefile's rules in source order. It reads the
// subset of GNU make's grammar a hand-written Makefile uses:
//
//   - a tab-led line inside a rule's context is a recipe line, and a trailing
//     backslash continues it onto the next physical line; outside a rule's
//     context, a tab-led line is an ordinary makefile line;
//   - blank lines, comment lines, and conditional directives do not end a
//     recipe; any other non-tab line does;
//   - a non-tab line is a rule when a colon that is not part of an assignment
//     operator (`:=`, `::=`, `:::=`) comes before any `=`, within the text
//     before the line's first `;` or `#`. So `X := a:b` and `X = a:b` are
//     assignments, and a target-specific `target: VAR = value` reads as a rule
//     whose prerequisite text is `VAR = value`;
//   - a `define` block is skipped whole.
func parseMakeRules(makefile string) []makeRule {
	lines := strings.Split(makefile, "\n")
	var rules []makeRule
	cur := -1 // index into rules of the rule whose recipe is being read
	for i := 0; i < len(lines); i++ {
		start := i
		line := lines[i]
		if strings.HasPrefix(line, "\t") && cur >= 0 {
			text := strings.TrimPrefix(line, "\t")
			for strings.HasSuffix(lines[i], `\`) && i+1 < len(lines) {
				i++
				text += "\n" + strings.TrimPrefix(lines[i], "\t")
			}
			rules[cur].Recipe = append(rules[cur].Recipe, recipeLine{Line: start + 1, Text: text})
			continue
		}
		text := line
		for strings.HasSuffix(text, `\`) && i+1 < len(lines) {
			i++
			text = strings.TrimRight(strings.TrimSuffix(text, `\`), " \t") + " " + strings.TrimSpace(lines[i])
		}
		trimmed := strings.TrimSpace(text)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		head, inline, hasInline := trimmed, "", false
		if idx := strings.IndexAny(trimmed, ";#"); idx >= 0 {
			head = trimmed[:idx]
			if trimmed[idx] == ';' {
				inline, hasInline = trimmed[idx+1:], true
			}
		}
		fields := strings.Fields(head)
		if len(fields) > 0 {
			if fields[0] == "define" || (len(fields) > 1 && fields[1] == "define" && slices.Contains([]string{"override", "export", "private"}, fields[0])) {
				for i+1 < len(lines) {
					i++
					if strings.TrimSpace(lines[i]) == "endef" {
						break
					}
				}
				cur = -1
				continue
			}
			if slices.Contains(makeConditionals, fields[0]) {
				continue
			}
			if slices.Contains(makeDirectives, fields[0]) {
				cur = -1
				continue
			}
		}

		colon := strings.IndexByte(head, ':')
		eq := strings.IndexByte(head, '=')
		after := ""
		if colon >= 0 {
			after = head[colon+1:]
		}
		if colon < 0 || (eq >= 0 && eq < colon) ||
			strings.HasPrefix(after, "=") || strings.HasPrefix(after, ":=") || strings.HasPrefix(after, "::=") {
			cur = -1 // an assignment or something else that is not a rule
			continue
		}
		rule := makeRule{
			Line:    start + 1,
			Targets: strings.Fields(head[:colon]),
			Prereqs: strings.TrimSpace(strings.TrimPrefix(after, ":")),
		}
		if hasInline {
			rule.Recipe = append(rule.Recipe, recipeLine{Line: start + 1, Text: strings.TrimLeft(inline, " \t")})
		}
		rules = append(rules, rule)
		cur = len(rules) - 1
	}
	return rules
}

// rulesFor returns every rule that names target among its targets.
func rulesFor(rules []makeRule, target string) []makeRule {
	var out []makeRule
	for _, r := range rules {
		if slices.Contains(r.Targets, target) {
			out = append(out, r)
		}
	}
	return out
}

// recipeTexts returns a rule's recipe lines' text, for comparisons.
func recipeTexts(r makeRule) []string {
	out := make([]string, 0, len(r.Recipe))
	for _, l := range r.Recipe {
		out = append(out, l.Text)
	}
	return out
}

// TestGateParity_ParseMakeRules is the Makefile parser's table test: the rule
// and recipe shapes GNU make accepts, and the lines that are not rules.
func TestGateParity_ParseMakeRules(t *testing.T) {
	type rule struct {
		line    int
		targets []string
		prereqs string
		recipe  []string
	}
	cases := []struct {
		name string
		src  string
		want []rule
	}{
		{
			name: "rule with prerequisites and two recipe lines",
			src:  "a: b c\n\tcmd1\n\t@cmd2\n",
			want: []rule{{1, []string{"a"}, "b c", []string{"cmd1", "@cmd2"}}},
		},
		{
			name: "a backslash continuation joins one logical recipe line",
			src:  "a:\n\tx; \\\n\t\ty\n\tz\n",
			want: []rule{{1, []string{"a"}, "", []string{"x; \\\n\ty", "z"}}},
		},
		{
			name: "blank lines, comments, and conditionals do not end a recipe; an assignment does",
			src:  "a:\n\tx\n\n# note\nifeq ($(V),1)\n\t-y\nendif\nB := 1\n\tz\n",
			want: []rule{{1, []string{"a"}, "", []string{"x", "-y"}}},
		},
		{
			name: "inline recipe after a semicolon comes first",
			src:  "a: b ; -cmd\n\tnext\n",
			want: []rule{{1, []string{"a"}, "b", []string{"-cmd", "next"}}},
		},
		{
			name: "double-colon rule, several targets, and a trailing comment",
			src:  "a b:: c # why\n\td\n",
			want: []rule{{1, []string{"a", "b"}, "c", []string{"d"}}},
		},
		{
			name: "a target-specific variable reads as prerequisite text",
			src:  "verify: MAKEFLAGS += -i\n",
			want: []rule{{1, []string{"verify"}, "MAKEFLAGS += -i", nil}},
		},
		{
			name: "assignments are not rules, and a tab line after one is not a recipe",
			src:  "X := a:b\nY = c:d\nZ ?= e\nW ::= f\nV :::= g\n\t-h\n",
			want: nil,
		},
		{
			name: "define blocks and other directives are not rules",
			src:  "define F\nx: y\n\tz\nendef\ninclude other.mk\nexport K := v\n",
			want: nil,
		},
		{
			name: "a tab-led rule line after an assignment is a rule, not a recipe",
			src:  "X := 1\n\ttest-rest: ; -go test\n",
			want: []rule{{2, []string{"test-rest"}, "", []string{"-go test"}}},
		},
		{
			name: "a continued rule line is one rule",
			src:  "a: b \\\n  c\n\td\n",
			want: []rule{{1, []string{"a"}, "b c", []string{"d"}}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseMakeRules(tc.src)
			if len(got) != len(tc.want) {
				t.Fatalf("parseMakeRules() = %+v, want %d rules %+v", got, len(tc.want), tc.want)
			}
			for i, w := range tc.want {
				g := got[i]
				if g.Line != w.line || !slices.Equal(g.Targets, w.targets) || g.Prereqs != w.prereqs || !slices.Equal(recipeTexts(g), w.recipe) {
					t.Errorf("rule %d = {line %d targets %q prereqs %q recipe %q}, want {line %d targets %q prereqs %q recipe %q}",
						i, g.Line, g.Targets, g.Prereqs, recipeTexts(g), w.line, w.targets, w.prereqs, w.recipe)
				}
			}
		})
	}
}

// verifyRecipeLines is `make verify`'s recipe, one physical line each: set up
// the timings file, run every VERIFY_STEPS entry through $(MAKE) in order,
// record each step's time, and exit 1 on the first failed step. Nothing else.
// It is pinned in full because the recipe is the one place `make verify`
// could run a command no gate job runs, or stop failing on a failed step,
// where no dry-run guard can see it. An edit that changes only bookkeeping
// updates these lines in the same commit, under review; a new check belongs
// in VERIFY_STEPS, where the parity guard carries it into merge-gate.yml.
var verifyRecipeLines = []string{
	`@mkdir -p $(dir $(GATE_TIMINGS)); \`,
	`run=$$(date -u +%Y-%m-%dT%H:%M:%SZ); head=$$(git rev-parse --short=12 HEAD 2>/dev/null || echo unknown); \`,
	`total=0; summary=""; \`,
	`for step in $(VERIFY_STEPS); do \`,
	`start=$$(date +%s); \`,
	`if $(MAKE) --no-print-directory $$step; then status=ok; else status=fail; fi; \`,
	`secs=$$(( $$(date +%s) - start )); total=$$(( total + secs )); \`,
	`printf '%s\t%s\t%s\t%s\t%s\n' "$$run" "$$head" "$$step" "$$secs" "$$status" >> $(GATE_TIMINGS); \`,
	`summary="$$summary$$(printf '  %-18s %6ss  %s' "$$step" "$$secs" "$$status")\n"; \`,
	`if [ "$$status" = fail ]; then \`,
	`printf 'verify: step %s FAILED after %ss\n%b' "$$step" "$$secs" "$$summary" >&2; exit 1; \`,
	`fi; \`,
	`done; \`,
	`printf 'verify timings (run %s @ %s, appended to %s):\n%b  %-18s %6ss\n' "$$run" "$$head" "$(GATE_TIMINGS)" "$$summary" total "$$total"; \`,
	`echo "verify OK"`,
}

// collapseSpace joins s's whitespace-separated fields with single spaces, so
// the recipe pin ignores indentation but not a single added or changed token.
func collapseSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// verifyRuleProblems returns every way the Makefile's `verify` rule departs
// from the loop over VERIFY_STEPS: other than exactly one rule naming it, any
// prerequisite (including an order-only one or a target-specific variable),
// or a recipe other than verifyRecipeLines.
func verifyRuleProblems(makefile string) []string {
	rules := rulesFor(parseMakeRules(makefile), "verify")
	var problems []string
	if len(rules) != 1 {
		var at []int
		for _, r := range rules {
			at = append(at, r.Line)
		}
		problems = append(problems, fmt.Sprintf("the Makefile must have exactly one rule naming verify, found %d (lines %v): a second rule adds prerequisites or replaces the recipe", len(rules), at))
		if len(rules) == 0 {
			return problems
		}
	}
	r := rules[0]
	if r.Prereqs != "" {
		problems = append(problems, fmt.Sprintf("line %d: verify has prerequisites %q; make runs them before the VERIFY_STEPS loop, so they run under make verify but in no merge-gate.yml job (and a target-specific variable such as MAKEFLAGS changes how every step runs)", r.Line, r.Prereqs))
	}
	var got []string
	for _, l := range r.Recipe {
		got = append(got, collapseSpace(l.Text))
	}
	want := []string{collapseSpace(strings.Join(verifyRecipeLines, "\n"))}
	if !slices.Equal(got, want) {
		problems = append(problems, fmt.Sprintf("line %d: verify's recipe must be exactly the VERIFY_STEPS loop (verifyRecipeLines); any other command runs under make verify but in no merge-gate.yml job, or changes whether a failed step fails the gate.\n got %d logical line(s): %q\nwant: %q", r.Line, len(got), got, want))
	}
	return problems
}

// TestGateParity_VerifyRunsOnlyItsStepLoop proves `make verify` runs exactly
// VERIFY_STEPS, which the merge-gate parity guard carries into the workflow:
// verify has no prerequisites, and its recipe is the pinned step loop.
func TestGateParity_VerifyRunsOnlyItsStepLoop(t *testing.T) {
	for _, p := range verifyRuleProblems(readMakefile(t)) {
		t.Error(p)
	}
}

// mutateMakefile returns makefile with the first from replaced by to, failing
// the test if from is absent, so a stale mutation cannot pass as a clean one.
func mutateMakefile(t *testing.T, makefile, from, to string) string {
	t.Helper()
	if !strings.Contains(makefile, from) {
		t.Fatalf("mutation target %q is not in the Makefile; update this case", from)
	}
	return strings.Replace(makefile, from, to, 1)
}

// TestGateParity_VerifyRuleProblemsFound is verifyRuleProblems' negative path,
// over mutated copies of the real Makefile.
func TestGateParity_VerifyRuleProblemsFound(t *testing.T) {
	makefile := readMakefile(t)
	const failStep = `"$$summary" >&2; exit 1; \`
	cases := []struct {
		name     string
		from, to string
		want     string
	}{
		{"a prerequisite", "\nverify:\n", "\nverify: build\n", "prerequisites"},
		{"an order-only prerequisite", "\nverify:\n", "\nverify: | e2e\n", "prerequisites"},
		{"a target-specific MAKEFLAGS", "\nverify:\n", "\nverify: MAKEFLAGS += -i\nverify:\n", "exactly one rule"},
		{"a second rule adding a prerequisite", "\nhooks:\n", "\nverify: e2e\n\nhooks:\n", "exactly one rule"},
		{"an inline command", "\nverify:\n", "\nverify: ; go test -race ./...\n", "exactly the VERIFY_STEPS loop"},
		{"an extra recipe line", "\techo \"verify OK\"\n", "\techo \"verify OK\"\n\tgo test -race ./...\n", "exactly the VERIFY_STEPS loop"},
		{"an extra command in the loop line", "\techo \"verify OK\"\n", "\tgo test ./...; echo \"verify OK\"\n", "exactly the VERIFY_STEPS loop"},
		{"a failed step no longer fails the gate", failStep, `"$$summary" >&2; exit 0; \`, "exactly the VERIFY_STEPS loop"},
		{"the rule is gone", "\nverify:\n", "\nverify-all:\n", "found 0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			problems := verifyRuleProblems(mutateMakefile(t, makefile, tc.from, tc.to))
			if !slices.ContainsFunc(problems, func(p string) bool { return strings.Contains(p, tc.want) }) {
				t.Errorf("verifyRuleProblems() = %q, want a problem containing %q", problems, tc.want)
			}
		})
	}
}

// recipePrefixProblem says why make would ignore a recipe line's failure, or
// returns "" when it would not. After expanding a recipe line, make strips a
// leading run of `@`, `+`, `-`, and blanks, and a `-` in that run makes it
// ignore the command's failure. A line that starts with a variable reference is
// expanded here with every value the Makefile gives that variable, including
// target-specific and define values. A reference the Makefile does not resolve
// is reported, because the guard cannot clear it: a function or substitution
// reference, a variable assigned nowhere, or one set by the shell (`!=`).
// $(MAKE) and the automatic variables never expand to a prefix.
func recipePrefixProblem(line string, assigns []makeAssignment, depth int) string {
	rest := strings.TrimLeft(line, "@+ \t")
	switch {
	case strings.HasPrefix(rest, "-"):
		return "starts with a `-` prefix, so make ignores its failure"
	case !strings.HasPrefix(rest, "$") || strings.HasPrefix(rest, "$$"):
		return ""
	case depth >= 4:
		return "starts with variable references nested too deep for the guard to resolve"
	}
	const unproven = "so the guard cannot prove it adds no `-` prefix"
	name, after, ok := leadingReference(rest)
	if !ok {
		ref, _, _ := strings.Cut(rest, " ")
		return fmt.Sprintf("starts with %s, a function or substitution reference, %s", ref, unproven)
	}
	if name == "MAKE" || (len(name) == 1 && strings.Contains("@<^*?+|%", name)) {
		return ""
	}
	values := assignmentsOf(assigns, name)
	if len(values) == 0 {
		return fmt.Sprintf("starts with $(%s), which the Makefile never assigns, %s", name, unproven)
	}
	for _, a := range values {
		if a.Op == "!=" {
			return fmt.Sprintf("starts with $(%s), which line %d sets from the shell, %s", name, a.Line, unproven)
		}
		if why := recipePrefixProblem(a.Value+after, assigns, depth+1); why != "" {
			return fmt.Sprintf("starts with $(%s), which line %d sets to %q, so it %s", name, a.Line, a.Value, why)
		}
	}
	return ""
}

// leadingReference splits the make variable reference off the front of s,
// which starts with a single `$`: `$(NAME)`, `${NAME}`, or the one-character
// `$N`. It reports false for a function call, a substitution reference, or an
// unterminated reference.
func leadingReference(s string) (name, rest string, ok bool) {
	if len(s) < 2 || s[1] == ' ' || s[1] == '\t' || s[1] == '\n' {
		return "", "", false
	}
	closer := map[byte]byte{'(': ')', '{': '}'}[s[1]]
	if closer == 0 {
		return s[1:2], s[2:], true
	}
	end := strings.IndexByte(s, closer)
	if end < 0 {
		return "", "", false
	}
	name = s[2:end]
	if name == "" || strings.ContainsAny(name, " \t,:=$(){}") {
		return "", "", false
	}
	return name, s[end+1:], true
}

// makeRefRE finds a make variable reference by name: $(NAME), ${NAME}, or the
// one-character $N. Run it on text with every `$$` (a literal `$` for the
// shell) removed.
var makeRefRE = regexp.MustCompile(`\$(?:[({]([A-Za-z0-9_.][A-Za-z0-9_.-]*)[)}]|([A-Za-z0-9_]))`)

// referencedValueProblem says why a variable reached from a gate recipe line
// could hide a failure, or returns "". It follows every variable the line
// references, anywhere in it, and every variable their values reference in
// turn. A define value brings newlines, and make runs each line after the
// first as its own recipe line, with its own prefix, wherever the reference
// sits; so every such line is checked like a recipe line
// (recipePrefixProblem). The first line continues the line the reference sits
// on, which recipePrefixProblem already checks when the reference leads it.
// Every value reached is also checked for a sub-make carrying a masking flag
// (makeInvocationProblem).
func referencedValueProblem(text string, assigns []makeAssignment) string {
	seen := map[string]bool{}
	queue := []string{text}
	for len(queue) > 0 {
		cur := strings.ReplaceAll(queue[0], "$$", "")
		queue = queue[1:]
		for _, m := range makeRefRE.FindAllStringSubmatch(cur, -1) {
			name := m[1] + m[2]
			if seen[name] {
				continue
			}
			seen[name] = true
			for _, a := range assignmentsOf(assigns, name) {
				queue = append(queue, a.Value)
				lines := strings.Split(a.Value, "\n")
				for i, line := range lines[1:] {
					if why := recipePrefixProblem(line, assigns, 0); why != "" {
						return fmt.Sprintf("reaches $(%s), set at line %d, whose line %d %q %s", name, a.Line, i+2, line, why)
					}
				}
				if why := makeInvocationProblem(a.Value); why != "" {
					return fmt.Sprintf("reaches $(%s), set at line %d, whose value %s, so the step can pass without running or over a failure", name, a.Line, why)
				}
			}
		}
	}
	return ""
}

// makeInvocationRE finds a make invocation in recipe text: $(MAKE), ${MAKE},
// or a bare `make` command word.
var makeInvocationRE = regexp.MustCompile("(?:^|[\\s;&|(`])(?:\\$\\(MAKE\\)|\\$\\{MAKE\\}|make)[ \\t]")

// makeMaskingLongFlags are the long forms of make's -i, -k, -n, -t, and -q.
var makeMaskingLongFlags = []string{"--ignore-errors", "--keep-going", "--just-print", "--dry-run", "--recon", "--touch", "--question"}

// makeInvocationProblem says why a make invocation in recipe text could pass
// without running its recipes or over their failures, or returns "". It
// refuses -i, -k, -n, -t, and -q, alone or in a short-flag cluster; their long
// forms and any abbreviation GNU make would accept for them; and a make
// variable among the arguments, whose flags the guard cannot read.
func makeInvocationProblem(text string) string {
	text = strings.ReplaceAll(text, "\\\n", " ")
	for _, loc := range makeInvocationRE.FindAllStringIndex(text, -1) {
		for _, tok := range splitShellArgs(text[loc[1]:]) {
			switch {
			case strings.HasPrefix(tok, "$(") || strings.HasPrefix(tok, "${"):
				return "passes make a variable reference whose flags the guard cannot read"
			case strings.HasPrefix(tok, "--"):
				opt, _, _ := strings.Cut(tok, "=")
				for _, long := range makeMaskingLongFlags {
					if len(opt) > 2 && strings.HasPrefix(long, opt) {
						return fmt.Sprintf("runs make with %s (%s)", tok, long)
					}
				}
			case strings.HasPrefix(tok, "-") && strings.ContainsAny(tok[1:], "iknqt"):
				return fmt.Sprintf("runs make with %s, which carries -i, -k, -n, -t, or -q", tok)
			}
		}
	}
	return ""
}

// makeSpecialTargetRE finds, on a non-comment line, a special target that
// changes how make runs recipes or reports their failures: .IGNORE ignores
// failed commands; .ONESHELL runs a recipe in one shell, whose status is only
// its last command's; .POSIX changes how the shell runs each line; .SILENT
// hides the commands that ran.
var makeSpecialTargetRE = regexp.MustCompile(`(^|[\s:])\.(IGNORE|ONESHELL|POSIX|SILENT)([\s:]|$)`)

// makeFlagsVarRE finds make's own flag variables. Set from the Makefile,
// -i, -k, -n, -t, or -q there ignores failures or skips recipes, and `make -n`
// cannot show it.
var makeFlagsVarRE = regexp.MustCompile(`\b(MAKEFLAGS|MFLAGS|GNUMAKEFLAGS)\b`)

// goFlagsVarRE finds GOFLAGS, which adds flags to every go command. Set from
// the Makefile, or in a recipe's environment, it can change what each go test
// runs (-run, -count, -failfast) where the shard guards, which read only a go
// test command's own arguments, cannot see it.
var goFlagsVarRE = regexp.MustCompile(`\bGOFLAGS\b`)

// recipePrefixVarRE finds .RECIPEPREFIX, which changes which lines are
// recipes, so the recipe guards would no longer read the lines make runs.
var recipePrefixVarRE = regexp.MustCompile(`(^|[\s:])\.RECIPEPREFIX([\s:+?!=]|$)`)

// makeRunnerVars decide how recipes and sub-makes run: SHELL and .SHELLFLAGS
// choose the shell that runs every recipe line, and whether a failed command
// fails it; MAKE is the command every $(MAKE) sub-make runs, flags included.
// None may be assigned. An assignment through a computed variable name
// (MK := MAKE, then $(MK) := make -i) is not recognized: that indirection is a
// documented residual.
var makeRunnerVars = []string{"SHELL", ".SHELLFLAGS", "MAKE"}

// errorIgnoringGateRecipes returns every way the Makefile source lets a gate
// target succeed over a failed command. Across the whole file it refuses the
// special targets in makeSpecialTargetRE, .RECIPEPREFIX, any mention of make's
// flag variables or of GOFLAGS, and any assignment of makeRunnerVars. For each
// recipe line of a root target, or of any prerequisite a root pulls in, it
// refuses a `-` prefix, written or reached through a leading variable
// (recipePrefixProblem); a make invocation carrying -i, -k, -n, -t, or -q
// (makeInvocationProblem); and either one reached through a variable the line
// references, including any line of a define value (referencedValueProblem).
// A gate target with no rule, or with neither a recipe nor a prerequisite, is
// reported too: its recipe cannot be read, so it cannot be cleared.
func errorIgnoringGateRecipes(makefile string, roots []string) []string {
	var problems []string
	for i, line := range strings.Split(makefile, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if m := makeSpecialTargetRE.FindStringSubmatch(line); m != nil {
			problems = append(problems, fmt.Sprintf("line %d declares .%s, which changes whether make sees or reports a failed command: %q", i+1, m[2], strings.TrimSpace(line)))
		}
		if m := makeFlagsVarRE.FindString(line); m != "" {
			problems = append(problems, fmt.Sprintf("line %d names %s: make flags set from the Makefile (-i, -k, -n, -t, -q) ignore failures or skip recipes where make -n cannot show it: %q", i+1, m, strings.TrimSpace(line)))
		}
		if goFlagsVarRE.MatchString(line) {
			problems = append(problems, fmt.Sprintf("line %d names GOFLAGS, which changes what every go test runs where the shard guards cannot see it: %q", i+1, strings.TrimSpace(line)))
		}
		if recipePrefixVarRE.MatchString(line) {
			problems = append(problems, fmt.Sprintf("line %d sets .RECIPEPREFIX, which changes which lines are recipes, so the recipe guards cannot read what make runs: %q", i+1, strings.TrimSpace(line)))
		}
	}
	assigns := parseMakeAssignments(makefile)
	for _, name := range makeRunnerVars {
		for _, a := range assignmentsOf(assigns, name) {
			problems = append(problems, fmt.Sprintf("line %d assigns %s, which decides how every recipe or sub-make runs and whether its failure is seen", a.Line, name))
		}
	}

	rules := parseMakeRules(makefile)
	seen := map[string]bool{}
	queue := slices.Clone(roots)
	for len(queue) > 0 {
		target := queue[0]
		queue = queue[1:]
		if seen[target] {
			continue
		}
		seen[target] = true
		targetRules := rulesFor(rules, target)
		if len(targetRules) == 0 {
			problems = append(problems, fmt.Sprintf("gate target %s has no rule in the Makefile source, so its recipe cannot be checked", target))
			continue
		}
		readable := false
		for _, r := range targetRules {
			for _, p := range strings.Fields(r.Prereqs) {
				readable = true
				if p != "|" && !strings.ContainsAny(p, "$=") {
					queue = append(queue, p)
				}
			}
			for _, l := range r.Recipe {
				if strings.TrimSpace(l.Text) != "" {
					readable = true
				}
				first, _, _ := strings.Cut(l.Text, "\n")
				if why := recipePrefixProblem(l.Text, assigns, 0); why != "" {
					problems = append(problems, fmt.Sprintf("line %d: gate target %s's recipe line %q %s", l.Line, target, first, why))
				}
				if why := makeInvocationProblem(l.Text); why != "" {
					problems = append(problems, fmt.Sprintf("line %d: gate target %s's recipe line %q %s, so the step can pass without running or over a failure", l.Line, target, first, why))
				}
				if why := referencedValueProblem(l.Text, assigns); why != "" {
					problems = append(problems, fmt.Sprintf("line %d: gate target %s's recipe line %q %s", l.Line, target, first, why))
				}
			}
		}
		if !readable {
			problems = append(problems, fmt.Sprintf("gate target %s has neither a recipe nor a prerequisite in the Makefile source, so what it runs cannot be checked", target))
		}
	}
	return problems
}

// gateRoots returns the targets whose recipes decide the gate: every
// VERIFY_STEPS entry, the test shards, test, and verify.
func gateRoots(t *testing.T, makefile string) []string {
	t.Helper()
	roots := append(makefileVarFields(t, makefile, "VERIFY_STEPS"), testShardTargets...)
	return append(roots, "test", "verify")
}

// TestGateParity_GateRecipesNeverIgnoreErrors proves no gate target can go
// green over a failed command through the Makefile source, which `make -n`
// cannot show: no `.IGNORE`, `.ONESHELL`, `.POSIX`, `.SILENT`, or
// `.RECIPEPREFIX`, no make flag variables and no GOFLAGS, no assignment of
// SHELL, .SHELLFLAGS, or MAKE, and, on any recipe line of a gate
// target or of a prerequisite it pulls in, no `-` prefix (written, through a
// leading variable, or on any line of a define value the line reaches) and no
// sub-make with -i, -k, -n, -t, or -q (written or in a value the line
// reaches).
func TestGateParity_GateRecipesNeverIgnoreErrors(t *testing.T) {
	makefile := readMakefile(t)
	for _, p := range errorIgnoringGateRecipes(makefile, gateRoots(t, makefile)) {
		t.Error(p)
	}
}

// TestGateParity_ErrorIgnoringRecipesFound is errorIgnoringGateRecipes'
// negative path, over mutated copies of the real Makefile, with the controls
// that must stay clean.
func TestGateParity_ErrorIgnoringRecipesFound(t *testing.T) {
	makefile := readMakefile(t)
	cases := []struct {
		name     string
		from, to string
		want     string // "" means no problem may be reported
	}{
		{"- on a test shard", "\tgo test -race -parallel 4 $(TEST_REST_PKGS)", "\t-go test -race -parallel 4 $(TEST_REST_PKGS)", "test-rest"},
		{"@- on spec-align", "\t@out=\"$$(go test -race -v -count=1", "\t@-out=\"$$(go test -race -v -count=1", "spec-align"},
		{"+ then blank then - on test-cmd", "\tgo test -race -parallel 4 $(TEST_CMD_PKGS)", "\t+ -go test -race -parallel 4 $(TEST_CMD_PKGS)", "test-cmd"},
		{"- on fixture", "\tgo test -race -parallel 4 ./internal/fixturegit/", "\t-go test -race -parallel 4 ./internal/fixturegit/", "fixture"},
		{"- on lint-store's second line", "\t$(LINT_STORE_BIN) lint\n", "\t-$(LINT_STORE_BIN) lint\n", "lint-store"},
		{"- on verify", "\t@mkdir -p $(dir $(GATE_TIMINGS)); \\", "\t-@mkdir -p $(dir $(GATE_TIMINGS)); \\", "verify"},
		{"- on e2e's prerequisite", "\t@if ! command -v node", "\t-@if ! command -v node", "e2e-check-node"},
		{"- in an inline recipe", "\ntest-cmd:\n\tgo test", "\ntest-cmd: ; -go test", "test-cmd"},
		{".IGNORE for every target", "\ntidy:\n", "\n.IGNORE:\n\ntidy:\n", ".IGNORE"},
		{".IGNORE for one target", "\ntidy:\n", "\n.IGNORE: test-rest\n\ntidy:\n", ".IGNORE"},
		{"a gate target's rule is gone", "\nlint-showcase:\n", "\nlint-showcase-x:\n", "lint-showcase"},
		{"MAKEFLAGS appended", "\ntidy:\n", "\nMAKEFLAGS += -i\n\ntidy:\n", "names MAKEFLAGS"},
		{"MAKEFLAGS set", "\ntidy:\n", "\nMAKEFLAGS := -k\n\ntidy:\n", "names MAKEFLAGS"},
		{"MAKEFLAGS exported", "\ntidy:\n", "\nexport MAKEFLAGS = -n\n\ntidy:\n", "names MAKEFLAGS"},
		{"MAKEFLAGS for one target", "\ntest-rest:\n", "\ntest-rest: MAKEFLAGS += -t\ntest-rest:\n", "names MAKEFLAGS"},
		{"GNUMAKEFLAGS set", "\ntidy:\n", "\nGNUMAKEFLAGS := -q\n\ntidy:\n", "names GNUMAKEFLAGS"},
		{".ONESHELL", "\ntidy:\n", "\n.ONESHELL:\n\ntidy:\n", "declares .ONESHELL"},
		{".POSIX", "\ntidy:\n", "\n.POSIX:\n\ntidy:\n", "declares .POSIX"},
		{".SILENT for one target", "\ntidy:\n", "\n.SILENT: test-rest\n\ntidy:\n", "declares .SILENT"},
		{"SHELL replaced", "\ntidy:\n", "\nSHELL := /usr/bin/true\n\ntidy:\n", "assigns SHELL"},
		{".SHELLFLAGS replaced", "\ntidy:\n", "\n.SHELLFLAGS := -c\n\ntidy:\n", "assigns .SHELLFLAGS"},
		{"a variable set to - leads a recipe line", "\ntest-rest:\n\tgo test", "\nIGN := -\ntest-rest:\n\t$(IGN)go test", "starts with $(IGN), which line"},
		{"a target-specific ${IGN} set to - leads", "\ntest-rest:\n\tgo test", "\ntest-rest: IGN := -\ntest-rest:\n\t${IGN}go test", "starts with $(IGN), which line"},
		{"a one-character $I set to - leads", "\ntest-rest:\n\tgo test", "\nI := -\ntest-rest:\n\t$Igo test", "starts with $(I), which line"},
		{"a chain through an empty variable reaches -", "\ntest-rest:\n\tgo test", "\nE :=\nJ := $(E)-\ntest-rest:\n\t$(J)go test", "starts with $(J), which line"},
		{"an unassigned variable leads", "\ntest-rest:\n\tgo test", "\ntest-rest:\n\t$(NOT_SET)go test", "never assigns"},
		{"a function leads", "\ntest-rest:\n\tgo test", "\ntest-rest:\n\t$(if x,-)go test", "function or substitution"},
		{"a shell-set variable leads", "\ntest-rest:\n\tgo test", "\nS != echo -\ntest-rest:\n\t$(S)go test", "from the shell"},
		{"$(MAKE) -i in a gate recipe", "\tgo test -race -parallel 4 $(TEST_CMD_PKGS)", "\t$(MAKE) -i build\n\tgo test -race -parallel 4 $(TEST_CMD_PKGS)", "runs make with -i,"},
		{"bare make with a -sk cluster", "\tgo test -race -parallel 4 $(TEST_CMD_PKGS)", "\tmake -sk build\n\tgo test -race -parallel 4 $(TEST_CMD_PKGS)", "runs make with -sk,"},
		{"${MAKE} --keep-going after a continuation", "\tgo test -race -parallel 4 $(TEST_CMD_PKGS)", "\techo x; \\\n\t${MAKE} --keep-going build\n\tgo test -race -parallel 4 $(TEST_CMD_PKGS)", "(--keep-going)"},
		{"$(MAKE) --ign, an accepted abbreviation", "\tgo test -race -parallel 4 $(TEST_CMD_PKGS)", "\t$(MAKE) --ign build\n\tgo test -race -parallel 4 $(TEST_CMD_PKGS)", "(--ignore-errors)"},
		{"$(MAKE) $(FLAGS)", "\tgo test -race -parallel 4 $(TEST_CMD_PKGS)", "\t$(MAKE) $(FLAGS) build\n\tgo test -race -parallel 4 $(TEST_CMD_PKGS)", "variable reference whose flags"},
		{"a define RUN_REST with a - line leads a recipe line", "\ntest-rest:\n\tgo test -race -parallel 4 $(TEST_REST_PKGS)", "\ndefine RUN_REST\ngo test -race -parallel 4 $(TEST_REST_PKGS)\n-go vet ./...\nendef\ntest-rest:\n\t$(RUN_REST)", "reaches $(RUN_REST)"},
		{"a define RUN_REST with a - line sits mid-line", "\ntest-rest:\n\tgo test -race -parallel 4 $(TEST_REST_PKGS)", "\ndefine RUN_REST\ngo test -race -parallel 4 $(TEST_REST_PKGS)\n-go vet ./...\nendef\ntest-rest:\n\techo run $(RUN_REST)", "reaches $(RUN_REST)"},
		{"a define with a - line reached through another variable", "\ntest-rest:\n\tgo test -race -parallel 4 $(TEST_REST_PKGS)", "\ndefine RUN_REST\ngo test -race -parallel 4 $(TEST_REST_PKGS)\n-go vet ./...\nendef\nRUN = $(RUN_REST)\ntest-rest:\n\t$(RUN)", "reaches $(RUN_REST)"},
		{"a define line led by a variable set to -", "\ntest-rest:\n\tgo test -race -parallel 4 $(TEST_REST_PKGS)", "\nI := -\ndefine RUN_REST\ngo test -race -parallel 4 $(TEST_REST_PKGS)\n$(I)go vet ./...\nendef\ntest-rest:\n\t$(RUN_REST)", "reaches $(RUN_REST)"},
		{"a variable whose value runs $(MAKE) -i", "\ntest-rest:\n\tgo test -race -parallel 4 $(TEST_REST_PKGS)", "\nSUB = $(MAKE) -i\ntest-rest:\n\t$(SUB) fixture\n\tgo test -race -parallel 4 $(TEST_REST_PKGS)", "reaches $(SUB)"},
		{"control: a clean multi-line define leads a recipe line", "\ntest-rest:\n\tgo test -race -parallel 4 $(TEST_REST_PKGS)", "\ndefine RUN_REST\ngo test -race -parallel 4 $(TEST_REST_PKGS)\n@go vet ./...\nendef\ntest-rest:\n\t$(RUN_REST)", ""},
		{"GOFLAGS exported", "\ntidy:\n", "\nexport GOFLAGS := -run=^$$\n\ntidy:\n", "names GOFLAGS"},
		{"GOFLAGS in a recipe's environment", "\tgo test -race -parallel 4 $(TEST_CMD_PKGS)", "\tGOFLAGS=-count=1 go test -race -parallel 4 $(TEST_CMD_PKGS)", "names GOFLAGS"},
		{"MAKE reassigned", "\ntidy:\n", "\nMAKE := make -i\n\ntidy:\n", "assigns MAKE"},
		{"MAKE reassigned for one target", "\ntest-rest:\n", "\ntest-rest: MAKE = make -k\ntest-rest:\n", "assigns MAKE"},
		{".RECIPEPREFIX set", "\ntidy:\n", "\n.RECIPEPREFIX = >\n\ntidy:\n", "sets .RECIPEPREFIX"},
		{"control: GOFLAGS and .RECIPEPREFIX named in a comment", "\ntidy:\n", "\n# never set GOFLAGS or .RECIPEPREFIX here\ntidy:\n", ""},
		{"control: - on a target outside the gate", "\tgo mod tidy", "\t-go mod tidy", ""},
		{"control: - opening a continuation line is shell text, not a prefix", "\tstatus=$$?; \\\n\tif [ \"$$status\" -ne 0 ]", "\tstatus=$$?; \\\n\t-true; if [ \"$$status\" -ne 0 ]", ""},
		{"control: .IGNORE named in a comment", "\ntidy:\n", "\n# never declare .IGNORE: here\ntidy:\n", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mutated := mutateMakefile(t, makefile, tc.from, tc.to)
			problems := errorIgnoringGateRecipes(mutated, gateRoots(t, mutated))
			if tc.want == "" {
				if len(problems) != 0 {
					t.Errorf("errorIgnoringGateRecipes() = %q, want none", problems)
				}
				return
			}
			if !slices.ContainsFunc(problems, func(p string) bool { return strings.Contains(p, tc.want) }) {
				t.Errorf("errorIgnoringGateRecipes() = %q, want a problem naming %q", problems, tc.want)
			}
		})
	}
}
