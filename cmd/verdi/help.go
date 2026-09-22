// ac-2 (spec/uat-round-1, closes UAT-004): "help", "--help", and "-h" are
// recognized at top level and after any verb; they print multi-line usage
// and exit 0, and NEVER execute the verb. This file owns the two pieces
// dispatch.go's central intercept needs: the top-level listing
// (topLevelUsage) and the per-verb usage registry (verbUsage), keyed by
// every verbPhase name, "lint" (dispatched before the phase map is ever
// consulted), and — an ac-2 review fix — "help" and "version" themselves,
// so topLevelUsage's own closing sentence holds for every row it lists.
// Neither "help" nor "version" (version.go) is ever added to verbPhase —
// both are top-level dispatch tokens, never phase-numbered verbs, so
// internal/specalign's CLI-verb inventory (a serialized shared registry
// per CLAUDE.md) needs no change for either.
package main

// isHelpToken reports whether tok is one of the three spellings ac-2
// recognizes as a help request, at top level ("verdi help"/"--help"/
// "-h") or immediately after a verb ("verdi <verb> help"/"--help"/"-h").
func isHelpToken(tok string) bool {
	return tok == "help" || tok == "--help" || tok == "-h"
}

// verbHelpRequested reports whether the token immediately following a
// verb asks for that verb's own usage (ac-2: "verdi <verb> --help",
// "-h", "help"). Only the immediate next token is examined — a verb's
// OWN subcommand or flag named identically two levels down (e.g. "verdi
// design start --help") is out of ac-2's scope, which names exactly
// "verdi <verb> --help" as the fixed shape ("today `verdi lint --help`
// runs a full lint and `verdi spec --help` prints only the `spec state`
// form — both must be fixed"), never a deeper subcommand's own help.
func verbHelpRequested(rest []string) bool {
	return len(rest) > 0 && isHelpToken(rest[0])
}

// topLevelUsage is printed by "verdi help", "verdi --help", and "verdi
// -h" (ac-2): one line per verb, each with a short description, so a
// caller can discover the whole surface without reading source. This is
// deliberately fuller than the compact `usage` const (dispatch.go,
// unchanged — still the no-args/unknown-verb operational-error message,
// per co-2's explicit carve-out) — a clean, requested help exit can
// afford to spend more lines than an operational refusal should.
//
// This is a verb-name directory: every left-hand column entry is a
// literal `verdi <name>` token, the same identity role designVerbUsage/
// experimentUsage's own markers already cover elsewhere in this package
// — never display prose about the operating model's classes or lifecycle.
//
// vocab:identity — CLI verb name / usage grammar (identity)
const topLevelUsage = `usage: verdi <verb> [args...]

verbs:
  lint             run every VL-001..VL-020 lint rule over the store
  design           scaffold, mutate, and inspect design-branch specifications
  accept           deprecated: acceptance now follows from landing the reviewed pull request
  feature          deprecated alias for "build"
  build            cut a build branch and begin implementation
  align            check, or freeze, the model/diagram alignment gate
  sync             materialize a derived evidence bundle for the current commit
  serve            run the single writer process (MCP socket + workbench)
  mcp              connect to a running serve, or serve standalone on stdio
  matrix           print the acceptance-criteria matrix for a tracked item
  rollup           publish a rollup to the configured tracker
  close            archive a completed unit once its evidence is complete
  disposition      record a reviewer's decision on a lint or alignment finding
  waivers          not implemented (out of v0 scope)
  verify-artifact  not implemented (out of v0 scope)
  dex              build the static docs/dex site
  gc               reclaim managed worktrees and execution workspaces
  gate             check the current build branch's gate conditions
  board            commit a board card to start a design branch
  audit            audit ADR exemptions, deviations, and closure hygiene
  attest           scaffold an attestation skeleton for a story or feature criterion
  model            validate the store's operating model
  init             scaffold a new verdi store (plain vocabulary by default)
  obligation       author or scaffold an evidence obligation
  waive            create or reaffirm a waiver record
  spec             inspect a spec's effective lifecycle state or render it as a document
  journey          show the guided-lifecycle journey for a tracked item
  context          compile and inspect context-integrity artifacts
  experiment       run a comparative experiment operation
  harness          render or drift-check the verdi skills for Claude Code and Codex
  policy           adopt a starter constitution on a policy/adopt branch
  recover          diagnose an interrupted lifecycle state and offer its safe choices
  version          print the build identification line
  help             print this message

run "verdi <verb> help" (or --help/-h) for that verb's own usage.`

// verbUsage is ac-2's per-verb usage registry: dispatch.go's central
// intercept prints verbUsage[verb] — never the verb's own code — for
// "verdi <verb> --help"/"-h"/"help", one line per subverb/form. Every
// verbPhase key has an entry, plus "lint" (dispatched before the phase
// map is ever consulted). Entries with an existing, already-ratified
// usage constant (design, obligation, waive, experiment, spec) reuse it
// directly rather than duplicating its text — the SAME constant its own
// usage-error call site prints, so the two can never drift apart (fix
// round 1, F5: a hand-duplicated "spec" literal here once drifted from
// specVerbUsage's own text); the
// rest are fresh literals grounded in each verb's own real argument-shape
// check, cited in the comment beside anything non-obvious.
var verbUsage = map[string]string{
	// "help" and "version" are NOT verbPhase keys (they're top-level
	// dispatch tokens, dispatch.go), so TestVerbUsageRegistry_CoversEveryVerb
	// does not require these two — they're here so topLevelUsage's own
	// closing sentence ("run \"verdi <verb> help\" ... for that verb's own
	// usage") is true for EVERY row it lists, "help" and "version"
	// included (ac-2 review fix): dispatch.go's run() now checks
	// verbHelpRequested(args[1:]) for both before acting on them.
	"help":    "usage: verdi help\n       verdi --help\n       verdi -h",
	"version": "usage: verdi version\n       verdi --version",
	"lint":    "usage: verdi lint",
	"design":  designVerbUsage,
	"accept":  "usage: verdi accept <spec-ref|diagram-ref>",
	"feature": "usage: verdi feature start <story-spec | story-ref>", // vocab:identity — CLI usage/flag grammar (identity)
	"build":   "usage: verdi build start <story-spec | story-ref>",   // vocab:identity — CLI usage/flag grammar (identity)
	"align":   "usage: verdi align [--freeze] [--wait[=seconds]]\n       verdi align --diagram-sweep <diagram-ref>",
	// vocab:identity — CLI usage/flag grammar (--story flag name, identity)
	"sync":  "usage: verdi sync [--or-regen]\n       verdi sync --produce [--force-local]\n       verdi sync --produce-runtime [--force-local] [--story <ref>] [--ac <id>] [--verdict pass|fail|abstain] [--witness <text>]",
	"serve": "usage: verdi serve [--http ADDR] [--context-request <path|->]",
	"mcp":   "usage: verdi mcp",
	// vocab:identity — CLI usage/flag grammar (identity)
	"matrix": "usage: verdi matrix [--preview] --json <story-or-feature-ref>\n       verdi matrix <story-or-feature-ref> [--preview]",
	"rollup": "usage: verdi rollup <jira:STORY-KEY | spec/name> --publish [--force-local]",
	// vocab:identity — CLI invocation grammar ("verdi close", identity)
	"close": "usage: verdi close <jira:STORY-KEY | spec/name> [--force-local]\n       verdi close --preflight <jira:STORY-KEY | spec/name> [--force-local]\n       verdi close --prepare <jira:STORY-KEY | spec/name> [--force-local]",
	// A clean usage line, not dispositionUsage/dispositionRecordUsage
	// reused verbatim (ac-2 review fix): both those constants carry their
	// own "disposition: "/"disposition record: " ERROR-prefix (their real
	// job is prefixing a diagnostic line, not standing alone as help text)
	// — hand-written here the same way "context" above is, so this entry
	// reads as a clean "usage: verdi disposition ..." like every other
	// row, never a stray "disposition: usage: ...".
	"disposition": "usage: verdi disposition --rationale <text> [--amend] [--] <spec-ref> <finding-id> <fixed|accepted-deviation> (-- ends option parsing)\n" +
		"       verdi disposition record --report PATH --row INPUT_ID --target REF --target-digest DIGEST --conclusion no-conflict|conflict --compensating-control TEXT [--compensating-control TEXT ...] --expiry DATE --approver ROLE=PRINCIPAL_ID [--approver ROLE=PRINCIPAL_ID ...] --id NAME --title TEXT --owner TEXT [--owner TEXT ...] [--root DIR]",
	"waivers":         "verdi waivers: not implemented (out of v0 scope)",
	"verify-artifact": "verdi verify-artifact: not implemented (out of v0 scope)",
	"dex":             "usage: verdi dex build -o <dir>",
	"gc":              "usage: verdi gc [--reclaim-unmanaged [--apply]]",
	"gate":            "usage: verdi gate",
	// vocab:identity — CLI usage/flag grammar (--story-ref flag name, identity)
	"board":  "usage: verdi board commit <board-key> --name <spec-name> [--story-ref <scheme:key>]",
	"audit":  "usage: verdi audit",
	"attest": "usage: verdi attest <spec-ref> <ac-id>",
	"model":  "usage: verdi model check",
	// "usage: " + initUsageText (init.go) — review round 1, Minor 2: this
	// row and every hand-printed init refusal now cite the SAME constant,
	// so they cannot silently drift apart (TestHelp_InitUsageMatchesConstant
	// pins the same equality from the other direction, at the built-binary
	// level, in help_test.go).
	"init":       "usage: " + initUsageText,
	"obligation": obligationVerbUsage,
	"waive":      waiveUsage,
	"spec":       specVerbUsage,
	// vocab:identity — CLI usage/flag grammar (identity)
	"journey": "usage: verdi journey [--json] <feature-or-story-ref>",
	"context": "usage: verdi context compile --request <path|-> [--out <path>]\n" +
		"       verdi context conflict --request <path|-> [--out <path>]\n" +
		"       verdi context constitution <inspect|propose|validate|impact-review|submit-preparation> --request <path|-> [--out <path>]\n" +
		"       verdi context contract\n" +
		"       verdi context execution --request <path|-> [--out <path>]\n" +
		"       verdi context mcp --request <path|->\n" +
		"       verdi context owner <decode|encode> --operation OP\n" +
		"       verdi context project [--root DIR]\n" +
		"       verdi context receipt verify --request <path|-> [--out <path>]\n" +
		"       verdi context resolve --request -",
	"experiment": experimentUsage,
	"harness":    harnessUsage,
	"policy":     policyUsage,
	"recover":    recoverUsage,
}

// verbUsageOrFallback returns the registered usage for verb, or a bare
// "usage: verdi <verb>" line if the registry somehow lacks an entry — a
// defensive fallback so a future verb added to verbPhase without a
// matching verbUsage entry still gets SOME help text (never an empty
// print) rather than silently proving nothing.
func verbUsageOrFallback(verb string) string {
	if u, ok := verbUsage[verb]; ok {
		return u
	}
	return "usage: verdi " + verb
}
