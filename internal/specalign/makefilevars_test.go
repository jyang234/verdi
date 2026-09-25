// Makefile variable guards (SI-266). The shard, parity, and recipe guards
// read the Makefile's first assignment of each gate variable
// (makefileVarFields) and this one file's source. Make itself uses the last
// assignment, applies `+=` and target-specific values, and reads every
// included file. So the variables that define the gate are each assigned
// exactly once, plainly, and the Makefile includes nothing the guards cannot
// read.
package specalign

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// makeAssignment is one assignment to a make variable in the Makefile source.
// Op is the assignment operator ("=", ":=", "::=", ":::=", "+=", "?=", "!="),
// or "define" for a define block, or "undefine". Target is the target list of
// a target-specific assignment, and empty otherwise.
type makeAssignment struct {
	Line   int
	Name   string
	Op     string
	Value  string
	Target string
}

// makeAssignRE matches an assignment, with any override/export/private
// modifiers, once any `targets:` prefix is removed.
var makeAssignRE = regexp.MustCompile(`^(?:(?:override|export|private)\s+)*([A-Za-z0-9_.][A-Za-z0-9_.-]*)\s*(:::=|::=|:=|\+=|\?=|!=|=)\s*(.*)$`)

// makeDefineRE matches the opening line of a define block.
var makeDefineRE = regexp.MustCompile(`^(?:(?:override|export|private)\s+)*define\s+([A-Za-z0-9_.][A-Za-z0-9_.-]*)`)

// makeUndefineRE matches an undefine directive.
var makeUndefineRE = regexp.MustCompile(`^(?:(?:override|export|private)\s+)*undefine\s+([A-Za-z0-9_.][A-Za-z0-9_.-]*)`)

// parseMakeAssignments returns every variable assignment in a Makefile's
// source, in order: plain and modified assignments, target-specific ones,
// define blocks, and undefine directives. It tracks rule context the way
// parseMakeRules does: a tab-led line inside a rule is a recipe line and is
// skipped, while a tab-led line outside one is an ordinary makefile line.
func parseMakeAssignments(makefile string) []makeAssignment {
	lines := strings.Split(makefile, "\n")
	var out []makeAssignment
	inRule := false
	for i := 0; i < len(lines); i++ {
		start := i
		if strings.HasPrefix(lines[i], "\t") && inRule {
			for strings.HasSuffix(lines[i], `\`) && i+1 < len(lines) {
				i++
			}
			continue
		}
		text := lines[i]
		for strings.HasSuffix(text, `\`) && i+1 < len(lines) {
			i++
			text = strings.TrimRight(strings.TrimSuffix(text, `\`), " \t") + " " + strings.TrimSpace(lines[i])
		}
		trimmed := strings.TrimSpace(text)
		if idx := strings.IndexByte(trimmed, '#'); idx >= 0 {
			trimmed = strings.TrimSpace(trimmed[:idx])
		}
		if trimmed == "" {
			continue
		}
		if m := makeDefineRE.FindStringSubmatch(trimmed); m != nil {
			var body []string
			for i+1 < len(lines) {
				i++
				if strings.TrimSpace(lines[i]) == "endef" {
					break
				}
				body = append(body, lines[i])
			}
			out = append(out, makeAssignment{Line: start + 1, Name: m[1], Op: "define", Value: strings.Join(body, "\n")})
			inRule = false
			continue
		}
		if m := makeUndefineRE.FindStringSubmatch(trimmed); m != nil {
			out = append(out, makeAssignment{Line: start + 1, Name: m[1], Op: "undefine"})
			inRule = false
			continue
		}
		fields := strings.Fields(trimmed)
		if slices.Contains(makeConditionals, fields[0]) {
			continue
		}

		target, body := "", trimmed
		colon := strings.IndexByte(trimmed, ':')
		eq := strings.IndexByte(trimmed, '=')
		if colon >= 0 && (eq < 0 || colon < eq) {
			after := trimmed[colon+1:]
			if !strings.HasPrefix(after, "=") && !strings.HasPrefix(after, ":=") && !strings.HasPrefix(after, "::=") {
				target, body = strings.TrimSpace(trimmed[:colon]), strings.TrimSpace(strings.TrimPrefix(after, ":"))
			}
		}
		if m := makeAssignRE.FindStringSubmatch(body); m != nil {
			out = append(out, makeAssignment{Line: start + 1, Name: m[1], Op: m[2], Value: m[3], Target: target})
			inRule = false
			continue
		}
		// A rule line starts a recipe context; any other line ends one.
		inRule = target != ""
	}
	return out
}

// assignmentsOf returns the assignments to name.
func assignmentsOf(assigns []makeAssignment, name string) []makeAssignment {
	var out []makeAssignment
	for _, a := range assigns {
		if a.Name == name {
			out = append(out, a)
		}
	}
	return out
}

// TestGateParity_ParseMakeAssignments is the assignment reader's table test.
func TestGateParity_ParseMakeAssignments(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []makeAssignment
	}{
		{
			name: "every operator, with modifiers and a trailing comment",
			src:  "A := 1\nB = x:y\nC ?= 3 # why\nD += 4\nE != echo 5\nF ::= 6\noverride G := 7\nexport H = 8\n",
			want: []makeAssignment{
				{Line: 1, Name: "A", Op: ":=", Value: "1"},
				{Line: 2, Name: "B", Op: "=", Value: "x:y"},
				{Line: 3, Name: "C", Op: "?=", Value: "3"},
				{Line: 4, Name: "D", Op: "+=", Value: "4"},
				{Line: 5, Name: "E", Op: "!=", Value: "echo 5"},
				{Line: 6, Name: "F", Op: "::=", Value: "6"},
				{Line: 7, Name: "G", Op: ":=", Value: "7"},
				{Line: 8, Name: "H", Op: "=", Value: "8"},
			},
		},
		{
			name: "target-specific assignment, define, and undefine",
			src:  "t u: V += -i\ndefine W\n-\nendef\nundefine X\n",
			want: []makeAssignment{
				{Line: 1, Name: "V", Op: "+=", Value: "-i", Target: "t u"},
				{Line: 2, Name: "W", Op: "define", Value: "-"},
				{Line: 5, Name: "X", Op: "undefine"},
			},
		},
		{
			name: "recipe lines are not assignments, but a tab line outside a rule is",
			src:  "r:\n\tA=1 go test\n\n\tB := 2\nC := 3\n\tD := 4\n",
			want: []makeAssignment{
				{Line: 5, Name: "C", Op: ":=", Value: "3"},
				{Line: 6, Name: "D", Op: ":=", Value: "4"},
			},
		},
		{
			name: "rules, a bare export, and comments are not assignments",
			src:  ".PHONY: a b\na: b\nexport K\n# L := 1\n",
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseMakeAssignments(tc.src)
			if !slices.Equal(got, tc.want) {
				t.Errorf("parseMakeAssignments() =\n %+v\nwant\n %+v", got, tc.want)
			}
		})
	}
}

// gateVariables are the variables whose values define the gate: the steps
// `make verify` runs and the package sets of the test shards.
var gateVariables = []string{"VERIFY_STEPS", "CROSS_BINARY_PKGS", "TEST_CMD_PKGS", "SPEC_ALIGN_PKGS", "TEST_REST_PKGS"}

// makeIncludeRE finds a directive that reads another makefile, or an eval
// that defines makefile text at run time; the source guards read neither.
var makeIncludeRE = regexp.MustCompile(`^\s*(?:-include|sinclude|include)\s|\$[({]eval\s`)

// makefileSourceProblems returns every way the Makefile's source could make
// the gate differ from what the guards read. Each gate variable is assigned
// exactly once, with `=` or `:=` (never `+=`, `?=`, `!=`, define, undefine,
// or a target-specific value), and no line includes another makefile or
// evals makefile text.
func makefileSourceProblems(makefile string) []string {
	var problems []string
	assigns := parseMakeAssignments(makefile)
	for _, name := range gateVariables {
		got := assignmentsOf(assigns, name)
		if len(got) != 1 {
			var at []int
			for _, a := range got {
				at = append(at, a.Line)
			}
			problems = append(problems, fmt.Sprintf("%s is assigned %d times (lines %v), want exactly once: the guards read the first assignment while make uses the last, or appends", name, len(got), at))
			continue
		}
		a := got[0]
		if a.Target != "" {
			problems = append(problems, fmt.Sprintf("line %d: %s is assigned only for target %s; a gate variable takes one global value", a.Line, name, a.Target))
		} else if a.Op != "=" && a.Op != ":=" {
			problems = append(problems, fmt.Sprintf("line %d: %s is assigned with %q; a gate variable takes a plain `=` or `:=` so the value the guards read is the value make uses", a.Line, name, a.Op))
		}
	}
	for i, line := range strings.Split(makefile, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if makeIncludeRE.MatchString(line) {
			problems = append(problems, fmt.Sprintf("line %d reads makefile text the guards cannot see: %q", i+1, strings.TrimSpace(line)))
		}
	}
	return problems
}

// TestGateParity_GateVariablesAssignedOnceAndNothingIncluded proves the gate
// variables the guards read are the values make uses, and that the Makefile
// is the only makefile text there is.
func TestGateParity_GateVariablesAssignedOnceAndNothingIncluded(t *testing.T) {
	for _, p := range makefileSourceProblems(readMakefile(t)) {
		t.Error(p)
	}
}

// TestGateParity_MakefileSourceProblemsFound is makefileSourceProblems'
// negative path, over mutated copies of the real Makefile, with controls.
func TestGateParity_MakefileSourceProblemsFound(t *testing.T) {
	makefile := readMakefile(t)
	const tidy = "\ntidy:\n"
	cases := []struct {
		name     string
		from, to string
		want     string // "" means no problem may be reported
	}{
		{"VERIFY_STEPS appended to", tidy, "\nVERIFY_STEPS += lint-extra\n" + tidy, "VERIFY_STEPS is assigned 2 times"},
		{"VERIFY_STEPS reassigned later", tidy, "\nVERIFY_STEPS := build\n" + tidy, "VERIFY_STEPS is assigned 2 times"},
		{"CROSS_BINARY_PKGS made conditional", "CROSS_BINARY_PKGS :=", "CROSS_BINARY_PKGS ?=", `CROSS_BINARY_PKGS is assigned with "?="`},
		{"TEST_CMD_PKGS overridden", tidy, "\noverride TEST_CMD_PKGS := ./internal/corpus\n" + tidy, "TEST_CMD_PKGS is assigned 2 times"},
		{"TEST_REST_PKGS target-specific", "\ntest-rest:\n", "\ntest-rest: TEST_REST_PKGS = ./internal/corpus\ntest-rest:\n", "TEST_REST_PKGS is assigned 2 times"},
		{"SPEC_ALIGN_PKGS undefined", tidy, "\nundefine SPEC_ALIGN_PKGS\n" + tidy, "SPEC_ALIGN_PKGS is assigned 2 times"},
		{"VERIFY_STEPS from the shell", "VERIFY_STEPS :=", "VERIFY_STEPS !=", `VERIFY_STEPS is assigned with "!="`},
		{"VERIFY_STEPS gone", "VERIFY_STEPS :=", "VERIFY_STEP :=", "VERIFY_STEPS is assigned 0 times"},
		{"include", tidy, "\ninclude local.mk\n" + tidy, "include local.mk"},
		{"-include", tidy, "\n-include local.mk\n" + tidy, "-include local.mk"},
		{"sinclude", tidy, "\nsinclude local.mk\n" + tidy, "sinclude local.mk"},
		{"eval", tidy, "\n$(eval VERIFY_STEPS := build)\n" + tidy, "$(eval VERIFY_STEPS := build)"},
		{"control: a bare export", tidy, "\nexport VERIFY_STEPS\n" + tidy, ""},
		{"control: a comment naming include and VERIFY_STEPS", tidy, "\n# include VERIFY_STEPS += x\n" + tidy, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			problems := makefileSourceProblems(mutateMakefile(t, makefile, tc.from, tc.to))
			if tc.want == "" {
				if len(problems) != 0 {
					t.Errorf("makefileSourceProblems() = %q, want none", problems)
				}
				return
			}
			if !slices.ContainsFunc(problems, func(p string) bool { return strings.Contains(p, tc.want) }) {
				t.Errorf("makefileSourceProblems() = %q, want a problem containing %q", problems, tc.want)
			}
		})
	}
}
