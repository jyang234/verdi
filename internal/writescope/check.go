package writescope

import (
	"fmt"
	"sort"
	"strings"
)

// Facts is what a static analysis of the module's source observed: the
// exported API of internal/gitx, the functions outside it that write under
// a git directory, every function the module declares, and every verb its
// inventories define with the mutating functions that verb reaches.
type Facts struct {
	// GitxPackage is gitx's import path with the module path trimmed.
	GitxPackage   string
	GitxExports   []string
	GitDirWriters []Writer
	Functions     map[string]bool
	// Verbs maps every verb the CLI-verb, MCP-tool, and workbench
	// inventories define to the mutating functions it reaches (none for a
	// verb that reaches none).
	Verbs map[Verb][]Hit
}

// Writer is a function outside gitx that writes under a git directory, and
// the position of that write.
type Writer struct {
	Func string
	At   string
}

// Hit is one mutating function a verb reaches, with one call path to it.
type Hit struct {
	Func string
	Path []string
}

// Check returns every way the registry, its awaiting-fix list, and the
// classification fail to describe facts, sorted; none means the static
// witness holds. It reports: an invalid registry or classification; an
// exported gitx function or a git-directory writer that is not classified;
// a classified name that no longer exists; a verb that reaches a mutating
// function with no declaration naming it; and a declaration naming a verb
// that no inventory defines or that reaches no mutating function.
func Check(decls []Declaration, awaiting []AwaitingFix, classes []Classified, facts Facts) []string {
	if facts.GitxPackage == "" || len(facts.GitxExports) == 0 || len(facts.Verbs) == 0 {
		return []string{"no facts: the analysis observed no gitx export or no verb, so nothing can be proven"}
	}
	var out []string
	if err := ValidateRegistry(decls, awaiting); err != nil {
		out = append(out, err.Error())
	}
	if err := ValidateClassification(classes); err != nil {
		out = append(out, err.Error())
	}
	out = append(out, checkClassification(classes, facts)...)
	out = append(out, checkCoverage(decls, facts)...)
	sort.Strings(out)
	return out
}

func checkClassification(classes []Classified, facts Facts) []string {
	var out []string
	classified := map[string]bool{}
	for _, c := range classes {
		classified[c.Func] = true
	}
	exports := map[string]bool{}
	for _, e := range facts.GitxExports {
		exports[e] = true
		if !classified[e] {
			out = append(out, fmt.Sprintf("unclassified exported gitx function %s: classify it mutating or read-only", e))
		}
	}
	for _, w := range facts.GitDirWriters {
		if !classified[w.Func] {
			out = append(out, fmt.Sprintf("unclassified git-directory writer %s (writes at %s): classify it mutating or read-only", w.Func, w.At))
		}
	}
	for _, c := range classes {
		exists := facts.Functions[c.Func]
		if inPackage(c.Func, facts.GitxPackage) {
			exists = exports[c.Func]
		}
		if !exists {
			out = append(out, fmt.Sprintf("classified name %s no longer exists", c.Func))
		}
	}
	return out
}

func checkCoverage(decls []Declaration, facts Facts) []string {
	var out []string
	owner := map[Verb]string{}
	for _, d := range decls {
		for _, v := range d.Verbs {
			if _, ok := owner[v]; !ok {
				owner[v] = d.Ritual
			}
		}
	}
	for v, hits := range facts.Verbs {
		if len(hits) == 0 || owner[v] != "" {
			continue
		}
		var reached []string
		for _, h := range hits {
			reached = append(reached, h.Func)
		}
		out = append(out, fmt.Sprintf("verb %s reaches %s (%s) and no declaration names it",
			v, strings.Join(reached, ", "), strings.Join(hits[0].Path, " -> ")))
	}
	for _, d := range decls {
		for _, v := range d.Verbs {
			hits, ok := facts.Verbs[v]
			switch {
			case !ok:
				out = append(out, fmt.Sprintf("declaration %s names verb %s, which no inventory defines", d.Ritual, v))
			case len(hits) == 0:
				out = append(out, fmt.Sprintf("declaration %s names verb %s, which reaches no mutating function", d.Ritual, v))
			}
		}
	}
	return out
}

// inPackage reports whether a function's full name belongs to pkg.
func inPackage(name, pkg string) bool {
	name = strings.TrimPrefix(strings.TrimPrefix(name, "("), "*")
	return strings.HasPrefix(name, pkg+".")
}
