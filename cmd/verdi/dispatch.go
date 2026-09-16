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
	"init":            18, // extensibility phase 2, spec/init-wizard (ledger L-N5, R4-I-56) — new verb; bare = the non-interactive scaffold wrapper, --wizard = the guide Part II configuring interview
	"obligation":      19, // extensibility phase 2, spec/obligation-seam ac-5 (spec/creation-surfaces#ac-4, ledger L-N8) — new verb, ratified into 05 §CLI in the same change; `verdi obligation author` is the design-branch, pre-freeze authoring/regeneration surface for an evidence obligation
	"waive":           20, // extensibility phase 2, spec/verb-surfaces ac-1/ac-2 (spec/creation-surfaces#ac-5, ledger L-N9, guide 8.4) — new verb, ratified into 05 §CLI in the same change; `verdi waive` creates or (--reaffirm) extends a waiver record over the waivers/ kind
	"spec":            21, // merge-signaled spec acceptance, Task 5 — new verb; `verdi spec state SPEC_REF` is the read-only surface over internal/specstate's Git-derived effective-state projection, never a lifecycle mutation
	"journey":         22, // GLG v3 AC-1 (guided-lifecycle-governance-v3, journey-projection delivery unit) — first GLG runtime verb; `verdi journey <feature-or-story-ref>` is a read-only projection over internal/journey, never a lifecycle mutation
	"context":         23, // Context Integrity Wave-3 (context-compiler and policy-conflict authority designs, ledger SI-78..SI-122) — `context compile` and `context conflict` are read-only inspection surfaces, mutating nothing but their own explicit --out destination and conflict's existing immutable cache
	"experiment":      24, // Comparative Spike Experiments Wave 5B — bounded CLI adapter over internal/experimentapp
}

// vocab:identity — CLI verb names (identity)
const usage = `usage: verdi <verb> [args...]

verbs: lint, design, accept, feature, build, align, sync, serve, mcp, matrix,
       rollup, close, disposition, waivers, verify-artifact, dex, gc, gate,
       board, audit, attest, model, init, obligation, waive, spec, journey,
       context, experiment`

// run parses args and returns the exit code per the CLAUDE.md contract:
// 0 clean / 1 verdict failure / 2 operational error. Phase 1 has no verdicts
// yet, so every path here is operational: usage (unknown verb, no args) or
// an honest "not implemented" for a recognized verb.
//
// spec/uat-round-1 ac-1/ac-2 add two families of top-level, never-phase-
// numbered tokens, both checked before the unknown-verb/phase lookup so
// neither is ever added to verbPhase (internal/specalign's CLI-verb
// inventory, a serialized shared registry per CLAUDE.md, stays untouched):
// "help"/"--help"/"-h" print topLevelUsage (help.go) and exit 0; "version"/
// "--version" print buildinfo.Line() (version.go) and exit 0 — UNLESS the
// very next token is itself one of the three help spellings, in which
// case each prints its OWN verbUsage row ("help"/"version") instead,
// exactly like every other verb below (an ac-2 review fix: topLevelUsage's
// closing sentence, "run `verdi <verb> help` ... for that verb's own
// usage," must hold for every row it lists, "help" and "version"
// included, not just the 29 phase-numbered ones). Right after, the
// per-verb help intercept (ac-2) fires whenever the token immediately
// following a KNOWN verb is one of those same three spellings: it prints
// that verb's own registered usage (help.go's verbUsage) and returns
// before the verb's real implementation is ever called — the fix for
// today's "`verdi lint --help` runs a full lint" and "`verdi spec --help`
// prints only the `spec state` form" (via the usage-error exit/stream,
// stderr+exit 2, rather than a clean stdout+exit 0 help response).
// Unknown verb and no-args behavior are untouched (co-2): both still fall
// through to the existing `usage` banner on stderr, exit 2.
func run(args []string, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return 2
	}

	verb := args[0]
	if isHelpToken(verb) {
		// ac-2 review fix: "verdi help help"/"--help"/"-h" — and the
		// three-spelling cross products this same check also reaches,
		// e.g. "verdi --help help" — ask for the "help" ROW's own usage,
		// not a second dump of topLevelUsage; makes topLevelUsage's
		// closing sentence true for the "help" row too.
		if verbHelpRequested(args[1:]) {
			fmt.Fprintln(os.Stdout, verbUsageOrFallback("help"))
			return 0
		}
		fmt.Fprintln(os.Stdout, topLevelUsage)
		return 0
	}
	if verb == "version" || verb == "--version" {
		// ac-2 review fix: "verdi version --help"/"-h"/"help" asks for
		// the "version" row's own usage, not the version line itself —
		// same closing-sentence guarantee as above, for the "version" row.
		if verbHelpRequested(args[1:]) {
			fmt.Fprintln(os.Stdout, verbUsageOrFallback("version"))
			return 0
		}
		return cmdVersion(os.Stdout)
	}

	if verb == "lint" {
		if verbHelpRequested(args[1:]) {
			fmt.Fprintln(os.Stdout, verbUsageOrFallback(verb))
			return 0
		}
		return runLintVerb(args[1:], os.Stdout, stderr)
	}

	phase, known := verbPhase[verb]
	if !known {
		fmt.Fprintln(stderr, usage)
		return 2
	}

	if verbHelpRequested(args[1:]) {
		fmt.Fprintln(os.Stdout, verbUsageOrFallback(verb))
		return 0
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
	if verb == "obligation" {
		return runObligationVerb(args[1:], os.Stdout, stderr)
	}
	if verb == "init" {
		return cmdInit(args[1:], os.Stdout, stderr)
	}
	if verb == "waive" {
		return cmdWaive(args[1:], os.Stdout, stderr)
	}
	if verb == "spec" {
		return runSpecVerb(args[1:], os.Stdout, stderr)
	}
	if verb == "journey" {
		return cmdJourney(args[1:], os.Stdout, stderr)
	}
	if verb == "context" {
		return cmdContext(args[1:], os.Stdin, os.Stdout, stderr)
	}
	if verb == "experiment" {
		return cmdExperiment(args[1:], os.Stdin, os.Stdout, stderr)
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
