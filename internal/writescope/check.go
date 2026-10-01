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
	// PreDispatch is what the code the binary runs for every verb, outside
	// any verb's arm, reaches (ledger SI-314 (3)): main, the dispatcher
	// outside its arms, init functions, and package-level variable
	// initializers that run code. PreDispatchRoots counts that code's
	// roots; zero means it was not analyzed, which proves nothing.
	PreDispatch      []Hit
	PreDispatchRoots int
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
// function with no declaration naming it; a declaration naming a verb that
// no inventory defines or that reaches no mutating function; an
// awaiting-fix list that disagrees with what reaches gitx's whole-index
// commit (checkAwaiting); and pre-dispatch code that reaches a mutating
// function, which runs for every verb, so no declaration can own it
// (ledger SI-314 (3)).
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
	out = append(out, checkAwaiting(decls, awaiting, facts)...)
	out = append(out, checkPreDispatch(facts)...)
	sort.Strings(out)
	return out
}

// checkAwaiting holds the awaiting-fix list to the facts (story dc-3). A
// scoped declaration states the fix its ritual is owed; while one of its
// verbs still reaches gitx's whole-index commit (CreateCommit, which
// records every pre-staged entry), the ritual does not meet it, so the
// declaration must sit in the list. And every listed ritual must still
// reach that commit: an entry whose ritual no longer does is stale.
func checkAwaiting(decls []Declaration, awaiting []AwaitingFix, facts Facts) []string {
	whole := facts.GitxPackage + ".CreateCommit"
	reaching := func(d Declaration) (Verb, bool) {
		for _, v := range d.Verbs {
			for _, h := range facts.Verbs[v] {
				if h.Func == whole {
					return v, true
				}
			}
		}
		return Verb{}, false
	}
	listed := map[string]bool{}
	for _, a := range awaiting {
		listed[a.Ritual] = true
	}
	var out []string
	byRitual := map[string]Declaration{}
	for _, d := range decls {
		byRitual[d.Ritual] = d
		if d.IndexCarry != CarryScoped || listed[d.Ritual] {
			continue
		}
		if v, ok := reaching(d); ok {
			out = append(out, fmt.Sprintf("declaration %s is scoped, but %s reaches %s, a commit of the whole index, and %s sits in no awaiting-fix entry (spec/write-scope-registry dc-3)",
				d.Ritual, v, whole, d.Ritual))
		}
	}
	rituals := make([]string, 0, len(listed))
	for r := range listed {
		rituals = append(rituals, r)
	}
	sort.Strings(rituals)
	for _, r := range rituals {
		d, ok := byRitual[r]
		if !ok {
			continue // ValidateRegistry reports an unknown ritual
		}
		if _, ok := reaching(d); !ok {
			out = append(out, fmt.Sprintf("awaiting fix in %s is stale: no verb of %s reaches %s, the whole-index commit its fix replaces", r, r, whole))
		}
	}
	return out
}

func checkPreDispatch(facts Facts) []string {
	if facts.PreDispatchRoots == 0 {
		return []string{"no pre-dispatch facts: the code every verb runs outside its arm was not analyzed, so nothing proves it mutates nothing"}
	}
	var out []string
	for _, h := range facts.PreDispatch {
		out = append(out, fmt.Sprintf("pre-dispatch code reaches %s (%s): it runs for every verb, so no declaration can own it (ledger SI-314 (3))",
			h.Func, strings.Join(h.Path, " -> ")))
	}
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
