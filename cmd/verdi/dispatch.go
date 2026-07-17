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
	"feature":         7, // R4-I-6: deprecation alias for "build"
	"build":           7,
	"align":           8,
	"sync":            5,
	"serve":           9,
	"mcp":             9,
	"matrix":          6,
	"rollup":          11,
	"close":           14, // round 6, spec/close-verb — flipped from I-23's phase-0 stub
	"waivers":         0,  // out of v0 (PLAN.md §5)
	"verify-artifact": 0,  // out of v0 (PLAN.md §5)
	"dex":             12,
	"gc":              15, // round 6, spec/worktree-manager — flipped from I-23's phase-0 stub (managed-worktree reclamation slice only, dc-5)
	"gate":            8,  // I-7, not in 05 §CLI's table
	"board":           10, // I-20, not in 05 §CLI's table (like "gate")
	"audit":           13, // R4-I-10, V1-P5 — beyond v0's numbered phases; a real, implemented verb, never "out of scope" (phase 0)
	"attest":          16, // legibility-ergonomics round, spec/attest-helper dc-1 — ratified new verb (task 3.R); scaffolds an attestation skeleton for a (story, AC) pair
	"disposition":     16, // round 6, spec/disposition-verb (spec/closure-ergonomics ac-3) — new verb, ratified into 05 §CLI in the same change
	"model":           17, // extensibility phase 1, spec/model-schema ac-3 (ledger L-M1) — new verb, ratified into 05 §CLI in the same change; `verdi model check` validates .verdi/model.yaml (or the embedded canonical default) fail-closed
}

const usage = `usage: verdi <verb> [args...]

verbs: lint, design, accept, feature, build, align, sync, serve, mcp, matrix,
       rollup, close, disposition, waivers, verify-artifact, dex, gc, gate,
       board, audit, attest, model`

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
	if verb == "rollup" {
		return cmdRollup(args[1:], os.Stdout, stderr)
	}
	if verb == "dex" {
		return runDexVerb(args[1:], os.Stdout, stderr)
	}
	if verb == "design" {
		return runDesignVerb(args[1:], os.Stdout, stderr)
	}
	if verb == "accept" {
		return cmdAccept(args[1:], os.Stdout, stderr)
	}
	if verb == "feature" {
		return runFeatureVerb(args[1:], os.Stdout, stderr)
	}
	if verb == "build" {
		return runBuildVerb(args[1:], os.Stdout, stderr)
	}
	if verb == "serve" {
		return cmdServe(args[1:], os.Stdout, stderr)
	}
	if verb == "mcp" {
		return cmdMcp(args[1:], os.Stdin, os.Stdout, stderr)
	}
	if verb == "align" {
		return cmdAlign(args[1:], os.Stdout, stderr)
	}
	if verb == "gate" {
		return cmdGate(args[1:], os.Stdout, stderr)
	}
	if verb == "board" {
		return runBoardVerb(args[1:], os.Stdout, stderr)
	}
	if verb == "audit" {
		return cmdAudit(args[1:], os.Stdout, stderr)
	}
	if verb == "close" {
		return cmdClose(args[1:], os.Stdout, stderr)
	}
	if verb == "gc" {
		return cmdGc(args[1:], os.Stdout, stderr)
	}
	if verb == "attest" {
		return cmdAttest(args[1:], os.Stdout, stderr)
	}
	if verb == "disposition" {
		return cmdDisposition(args[1:], os.Stdout, stderr)
	}
	if verb == "model" {
		return runModelVerb(args[1:], os.Stdout, stderr)
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
