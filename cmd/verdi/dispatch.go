// Command verdi is verb dispatch only (PLAN.md §2 repository layout): it
// recognizes every spec-named verb (05 §CLI) plus the invented `gate` verb
// (PLAN.md I-7) and reports, honestly, whether that verb is implemented yet.
// No verb's semantics live here — that discipline is the point of phase 1.
package main

import (
	"fmt"
	"io"
	"os"
)

// verbPhase records, for each known verb, the PLAN.md phase that implements
// it. A zero phase means the verb is named by the specs (05 §CLI) or the
// invention ledger but is explicitly out of v0 scope (PLAN.md §5: "not
// stubbed — absent") — dispatch still recognizes the name (so the error is
// "not implemented", not "unknown verb"), but there is no phase to cite.
var verbPhase = map[string]int{
	"design":          7,
	"accept":          7,
	"feature":         7,
	"align":           8,
	"sync":            5,
	"serve":           9,
	"mcp":             9,
	"matrix":          6,
	"rollup":          11,
	"close":           0, // out of v0 (PLAN.md §5)
	"waivers":         0, // out of v0 (PLAN.md §5)
	"verify-artifact": 0, // out of v0 (PLAN.md §5)
	"dex":             12,
	"gc":              0, // out of v0 (PLAN.md §5)
	"gate":            8, // I-7, not in 05 §CLI's table
}

const usage = `usage: verdi <verb> [args...]

verbs: lint, design, accept, feature, align, sync, serve, mcp, matrix,
       rollup, close, waivers, verify-artifact, dex, gc, gate`

// run parses args and returns the exit code per the CLAUDE.md contract:
// 0 clean / 1 verdict failure / 2 operational error. Phase 1 has no verdicts
// yet, so every path here is operational: usage (unknown verb, no args) or
// an honest "not implemented" for a recognized verb.
func run(args []string, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return 2
	}

	verb := args[0]
	if verb == "lint" {
		return runLintVerb(args[1:], os.Stdout, stderr)
	}

	phase, known := verbPhase[verb]
	if !known {
		fmt.Fprintln(stderr, usage)
		return 2
	}

	if verb == "sync" {
		return cmdSync(args[1:], os.Stdout, stderr)
	}
	if verb == "matrix" {
		return cmdMatrix(args[1:], os.Stdout, stderr)
	}
	if verb == "dex" {
		return runDexVerb(args[1:], os.Stdout, stderr)
	}
	if verb == "serve" {
		return cmdServe(args[1:], os.Stdout, stderr)
	}
	if verb == "mcp" {
		return cmdMcp(args[1:], os.Stdin, os.Stdout, stderr)
	}

	if phase == 0 {
		fmt.Fprintln(stderr, "not implemented (out of v0 scope)")
		return 2
	}
	fmt.Fprintf(stderr, "not implemented (phase %d)\n", phase)
	return 2
}

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}
