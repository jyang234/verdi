---
id: spec/instruction-conformance
kind: spec
title: "Instruction Conformance"
owners: [platform-team]
class: story
status: draft
story: jira:REPLACE-ME
problem: { text: "An external adversarial review (docs/design/external-assessment/02-surviving-priorities-and-feature-gap-analysis.md, Priority 1) found that a committed agent skill, .claude/skills/commit-to-design/SKILL.md, still instructs an agent to run `verdi board commit` and consume a frozen `board.json` as the CURRENT way to finish a design-branch spec, while docs/architecture-and-journeys.md (section 4, R4-I-9) and this store's own verdi-surfaces/verdi-artifact-contract component specs say that ritual is retired: board editing on a design branch IS spec editing now, with no separate commit-to-design step. This is not a removed-command defect: `board` remains a live, dispatched CLI verb (cmd/verdi/dispatch.go's verbPhase[\"board\"] = 10; cmd/verdi/board.go still wires `verdi board commit` end to end), so a bare verb-existence check would not have caught it. internal/specalign already gates the CLI verb inventory against dispatch.go (verbs_test.go), the MCP tool inventory (mcptools_test.go), and targeted architecture-doc claims via a handEditPhrasings-style tripwire (docsync_test.go) — but nothing walks .claude/skills/ or the repo-root CLAUDE.md, so an agent-facing instruction can drift from the canonical CLI/lifecycle model with no gate ever noticing, and the next agent that reads the stale skill inherits its wrong procedure with full confidence.", anchor: problem }
outcome: { text: "internal/specalign gains a new, purely mechanical check — no semantic or natural-language drift detection, matching this repo's determinism posture — that enumerates every agent-facing instruction file by walking `.claude/skills/*/SKILL.md` (a glob, so a newly added skill is picked up with no code change) plus the repo-root CLAUDE.md, extracts every `verdi <verb>` command reference from each, and validates each extracted verb against dispatch.go's own recognized-verb set by driving the real built binary — the same relationship verbs_test.go already has to dispatch.go, never importing cmd/verdi as a package. A second, independent check tripwires the retired commit-to-design ritual specifically — the case a bare verb-existence check cannot see, since `board` still dispatches — using the same handEditPhrasings idiom docsync_test.go already established, with the same honest disclosure that a substring tripwire is not a semantic proof. Both checks prove their own red direction with a planted, committed fixture that fails loudly and names the offending file and reference, so this gate cannot silently vanish the way this package's own ADJ-47/ADJ-50 history already found and fixed once. Because `spec-align`'s Makefile target is a bare `go test ./internal/specalign/...` with no `-run` filter, the new test file(s) join `make verify` with no Makefile edit. Run against this repo's own committed tree, the gate fails on `.claude/skills/commit-to-design/SKILL.md` as authored today — which is the point: merging this story's build forces that skill to be retired or honestly rewritten before `make verify` can go green again.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "Instruction-file enumeration is derived from the filesystem, never a hardcoded literal list: every `.claude/skills/*/SKILL.md` (glob) plus the single repo-root `CLAUDE.md`. The repo-root CLAUDE.md is a required minimum — its absence is itself a finding, never a silent zero-file vacuous pass — while an absent or empty `.claude/skills/` directory is a legal, honest zero-skills state. Proven by a fixture tree with a varying skill count, including a case where a skill directory is added between two subtests and the enumerated file count changes with no test-code edit.", evidence: [behavioral], anchor: ac-1 }
  - { id: ac-2, text: "Every `verdi <verb>` command reference inside an enumerated instruction file's backtick-delimited span (inline code and fenced code blocks alike) is extracted and validated against dispatch.go's own recognized-verb set, driven by execing the real built verdi binary with the extracted word as its sole argument from an empty, rootless temp directory. A verb dispatch.go does not recognize at all fails, naming the instruction file and the unrecognized verb text; a verb dispatch.go recognizes — fully implemented, phase-gated, or explicitly out-of-scope (`waivers`/`verify-artifact`, which print their own distinct message rather than the top-level unknown-verb usage banner) — passes, since an instruction accurately describing a real-but-unimplemented verb is not stale. This check alone does not catch the motivating SKILL.md defect (`board` is still real and dispatched; see AC-3) — it catches the sibling drift class, a verb removed outright.", evidence: [behavioral], anchor: ac-2 }
  - { id: ac-3, text: "A second, independent check tripwires the retired two-phase commit-to-design ritual specifically, following docsync_test.go's handEditPhrasings idiom: a closed, named set of phrasings that instruct or teach `verdi board commit` / a frozen `board.json` as the CURRENT step of finishing a design-branch spec, with no accompanying retirement or grandfathered disclosure anywhere in the same file, fails the file, naming it and the offending phrase. A file that both names the command and discloses its retired or grandfathered status in the same breath does not trip it. Carries the same honest disclosure docsync_test.go's own header comment already states: a lexical tripwire for the common phrasing, never a semantic guarantee no future paraphrase could describe the same stale procedure and evade it.", evidence: [static], anchor: ac-3 }
  - { id: ac-4, text: "AC-1 through AC-3's checks are proven to actually fire, not merely present: a committed fixture instruction file carries both a `verdi <verb>` reference naming a verb dispatch.go does not recognize and a retired-ritual phrase from AC-3's tripwire set with no disclosure, and driving the real check against that fixture fails with output naming the exact fixture file path and the exact offending verb or phrase — never a bare boolean, and never a `go test -run` pattern that would exit 0 by matching nothing if the underlying test function were renamed or deleted. A second, clean fixture — real verbs, and a `board commit` mention paired with a retirement disclosure — passes with zero findings, proving the checks do not also false-positive on legitimate content.", evidence: [behavioral], anchor: ac-4 }
  - { id: ac-5, text: "Run against this repo's own `.claude/skills/*/SKILL.md` and repo-root `CLAUDE.md` — not a synthetic fixture — AC-1 through AC-3's checks together report zero findings, and `go test ./internal/specalign/...` (equivalently `make spec-align`, and therefore `make verify`) exits clean. Because `.claude/skills/commit-to-design/SKILL.md` teaches `verdi board commit` as the current ritual with no retirement disclosure as authored today, this AC cannot be satisfied by this story's design-time authoring alone: it forces the build phase to apply this story's DC-4 disposition (retire the skill), or the rewrite-to-disclose alternative DC-3 deliberately leaves achievable, before `make verify` can go green on this repo's own tree again.", evidence: [static, behavioral], anchor: ac-5 }
links:
  - { type: implements, ref: "spec/todo-replace-feature-name#ac-1" }
decisions:
  - { id: dc-1, text: "Verb-reference extraction (AC-2) targets the literal `verdi <verb>` invocation shape inside a backtick-delimited span only, never a bare backticked verb name with no `verdi ` prefix. Investigating this repo's two real instruction files found they use different prose shapes: SKILL.md's five verb mentions are all the invocation shape (`verdi board commit ...`, `verdi lint`); CLAUDE.md's own CLI-verbs sentence instead names bare backticked verb words (gate, board, audit, close, gc, waivers, verify-artifact) with no `verdi ` prefix, a shape this rule does not extract. CLAUDE.md therefore passes AC-2 vacuously today — zero references found, not zero references checked-and-clean — which this decision discloses rather than hides. Widening the rule to classify any bare backticked lowercase-hyphenated word as a candidate verb was considered and rejected: a bare backtick span is ambiguous by construction (paths and make targets are backtick spans too), and this repo's determinism posture forbids a heuristic that would need semantic judgment to disambiguate reliably.", anchor: dc-1 }
  - { id: dc-2, text: "An extracted verb is classified known by execing the once-built verdi binary with the word as its sole argument from a fresh, rootless temp directory, and checking whether stderr is exactly dispatch.go's own top-level unknown-verb usage banner. Anything else — a verb-specific usage error, an operational store-root failure, dispatch.go's distinct not-implemented-out-of-v0-scope message, or genuine success — counts as known. This is deliberately a coarser question than verbs_test.go's own assertNotOutOfV0 helper (which detects a different branch, real-vs-not-yet-implemented): it answers does dispatch.go recognize this word at all, the correct question for prose that may accurately describe a recognized-but-out-of-scope verb.", anchor: dc-2 }
  - { id: dc-3, text: "The retired-ritual tripwire (AC-3) fires on the conjunction of (a) presence of a small closed set of phrasings instructing or describing `verdi board commit` / `board.json` as an active current step, and (b) absence, anywhere in the same file, of a small closed set of retirement-disclosure phrasings. A pure presence-only tripwire, mirroring handEditPhrasings' own unconditional shape, was considered and rejected: `verdi board commit` and `board.json` are real, still-dispatched strings this store's own component specs legitimately and correctly repeat while explaining the retirement, so a presence-only rule would make the honest rewrite-to-disclose disposition this story's own brief names as a valid alternative structurally impossible to ever pass. The presence-and-absence pairing is the smallest change that keeps both candidate dispositions of the stale skill — retire outright, or rewrite to disclose grandfathered-only scope — achievable ways to satisfy AC-5.", anchor: dc-3 }
  - { id: dc-4, text: "`.claude/skills/commit-to-design/SKILL.md`'s disposition is RETIRE, not rewrite-to-disclose. Investigated whether any grandfathered v0 artifact still needs the skill's promotion flow — its own stated trigger is a draft spec's dispositions: block with open-question entries and a sibling frozen board.json. A search of this store's actual governing corpus, .verdi/specs/ (active and archive alike), for a dispositions: frontmatter block or a sibling board.json found neither anywhere. The only dispositions:/board.json occurrences left in this repo at all are prose in verdi-evidence-model/verdi-artifact-contract/verdi-surfaces discussing the retired mechanism historically, an already-archived showcase-demo spec's board.json, and VL-014's own negative-path test fixtures — none a live draft needing the skill's promotion flow today. Smallest reversible choice given that finding: retire the file; a rewrite-to-disclose would leave a permanently dead-letter skill in the tree for a use case that does not currently exist, and can be re-authored later if a genuine need ever surfaces. Retirement mechanics themselves are build-phase work, out of this design-only story's own deliverable.", anchor: dc-4 }
constraints:
  - { id: co-1, text: "No network in any test: every check runs entirely against fixture instruction files and the once-built local verdi binary; execing the binary against a fresh, rootless temp directory is local process exec, not network I/O, the same hermetic pattern verbs_test.go's own serve/mcp/audit/align/gc/close/disposition subtests already rely on.", anchor: co-1 }
  - { id: co-2, text: "Mechanical only: this story adds no semantic or natural-language drift detection. AC-3's tripwire is a disclosed, imperfect lexical heuristic, never a claim that no paraphrase could describe the same stale procedure and evade it — closing that residual, if ever warranted, is a judged-sweep-shaped follow-on, not this story.", anchor: co-2 }
  - { id: co-3, text: "Scope is exactly the enumerated gap: .claude/skills/*/SKILL.md plus the repo-root CLAUDE.md. Explicitly out, because each is already covered by a sibling specalign check or is out of today's inventory: README example freshness (internal/showcasealign's TestReadmeExamplesFresh), targeted architecture-doc claims (docsync_test.go), and MCP tool naming (mcptools_test.go). AGENTS.md does not exist anywhere in this repo today; it is not enumerated by this story, and a future story extends the same glob-based enumeration the moment one is added. CLI flag-level validation, as opposed to verb-level, is also out of scope: no canonical, machine-checkable flag inventory exists yet to validate prose flag mentions against, and building one is materially new scope beyond following verbs_test.go's relationship to dispatch.go.", anchor: co-3 }
  - { id: co-4, text: "The board commit CLI verb's own fate — removal, or a feature-style deprecation alias (R4-I-6) — is explicitly out of this story's scope; this story only gates what agent-facing prose teaches about it, never the dispatch table itself. cmd/verdi/board.go and dispatch.go's verbPhase[\"board\"] are untouched by this story. A future story formally deprecating or removing the verb should follow the cli:feature exclusion-from-showcase-coverage precedent (R4-I-54) as its template, rather than inventing a new pattern.", anchor: co-4 }
open_questions:
  - { id: oq-1, text: "instruction-conformance has no accepted parent feature to implement today: the four currently-active feature specs (code-health, disclosure-legibility, public-showcase, scoping-canvas) were each checked against this story's problem/outcome, and none covers agent-instruction conformance. R4-I-56, the most recent invention-ledger entry and also a product of this same external-assessment round, records a comparable case proceeding as a future story with no feature parent named, so a parentless story is not itself unprecedented. The links: block below therefore carries verdi design start's own standard unresolved scaffold placeholder rather than a fabricated edge into an unrelated feature's AC, so verdi lint reports one VL-003 finding against this spec until the owner either designates a real parent feature or ratifies this as a standing exception.", anchor: oq-1 }
---
# Instruction Conformance

## Problem

An external adversarial review
(`docs/design/external-assessment/02-surviving-priorities-and-feature-gap-analysis.md`,
Priority 1) found that a committed agent skill,
`.claude/skills/commit-to-design/SKILL.md`, still instructs an agent to run
`verdi board commit` and consume a frozen `board.json` as the **current** way
to finish a design-branch spec, while `docs/architecture-and-journeys.md`
(section 4, R4-I-9) and this store's own `verdi-surfaces`/
`verdi-artifact-contract` component specs say that ritual is **retired**:
board editing on a design branch *is* spec editing now, with no separate
commit-to-design step.

This is not a removed-command defect. `board` remains a live, dispatched CLI
verb — `cmd/verdi/dispatch.go`'s `verbPhase["board"] = 10`, and
`cmd/verdi/board.go` still wires `verdi board commit` end to end through
`internal/commitdesign.Run`. A bare verb-existence check would not have
caught this: the skill teaches a *procedure* the architecture retired, using
a verb that still runs. `internal/specalign` already gates the CLI verb
inventory against `dispatch.go` (`verbs_test.go`), the MCP tool inventory
(`mcptools_test.go`), and targeted architecture-doc claims via a
`handEditPhrasings`-style tripwire (`docsync_test.go`) — but nothing walks
`.claude/skills/` or the repo-root `CLAUDE.md`. An agent-facing instruction
can drift from the canonical CLI/lifecycle model with no gate ever noticing,
and the next agent that reads the stale skill inherits its wrong procedure
with full confidence, exactly the failure mode the review names: "the
immediate problem is not that agents lack write capability. It is that they
may confidently follow obsolete governance instructions."

## Outcome

`internal/specalign` gains a new, purely mechanical check — no semantic or
natural-language drift detection, matching this repo's determinism posture
(strict decode everywhere, no wall-clock or randomness, no LLM in the gate
path) — built from three parts:

1. **Enumeration** (AC-1) walks `.claude/skills/*/SKILL.md` (a glob) plus the
   repo-root `CLAUDE.md`, so a newly added skill is picked up with no code
   change — the exact drift class a hardcoded file list would silently
   under-enumerate.
2. **Verb validation** (AC-2) extracts every `verdi <verb>` command reference
   from each enumerated file and checks it against `dispatch.go`'s own
   recognized-verb set by driving the real built binary — the same
   relationship `verbs_test.go` already has to `dispatch.go`, never
   importing `cmd/verdi` as a package.
3. **A retired-ritual tripwire** (AC-3) catches what verb validation
   structurally cannot: `board` is still a real verb, so an instruction
   teaching `verdi board commit` as the current ritual passes AC-2 clean.
   The tripwire follows `docsync_test.go`'s `handEditPhrasings` idiom, with
   the same honest disclosure that a substring tripwire is not a semantic
   proof.

Both AC-2 and AC-3 prove their own red direction against a planted, committed
fixture that fails loudly and names the offending file and reference (AC-4)
— so this gate cannot silently vanish the way this package's own ADJ-47/
ADJ-50 history already found and fixed once for `docsync_test.go`. Because
`spec-align`'s Makefile target is a bare `go test ./internal/specalign/...`
with no `-run` filter, the new test file(s) join `make verify` with no
Makefile edit.

Run against this repo's own committed tree (AC-5), the gate fails on
`.claude/skills/commit-to-design/SKILL.md` as authored today — which is the
point: merging this story's build forces that skill to be retired (DC-4) or
honestly rewritten before `make verify` can go green again. The fate of the
`board commit` **verb** itself is untouched (CO-4): this story gates prose,
never the dispatch table.

## AC-1

Instruction-file enumeration is derived from the filesystem, never a
hardcoded literal list: every `.claude/skills/*/SKILL.md` (glob) plus the
single repo-root `CLAUDE.md`. Design intent, per the brief this story
implements: "design the enumeration so new skills are picked up
automatically — a hardcoded file list that silently under-enumerates is the
exact drift class this story kills."

- The repo-root `CLAUDE.md` is a **required minimum** — its absence is
  itself a finding, never a silent zero-file, vacuously-clean run.
- An absent or empty `.claude/skills/` directory is a **legal, honest**
  zero-skills state (this repo may retire its one skill entirely per DC-4,
  and the enumeration must not treat that as an error).
- Proven by a fixture tree with a varying skill count, including a case
  where a skill directory is added between two subtests and the enumerated
  file count changes accordingly with no test-code edit — mirroring
  `TestShowcaseCoverage_EnumerationIsComplete`'s own completeness-proof
  shape in `internal/showcasealign`.

## AC-2

Every `verdi <verb>` command reference inside an enumerated instruction
file's backtick-delimited span (inline code and fenced code blocks alike —
this repo's own two instruction files use inline spans exclusively today,
but a future skill's worked example may reasonably fence a shell block) is
extracted and validated against `dispatch.go`'s own recognized-verb set.

Validation is **behavioral**, driven by execing the real built `verdi`
binary with the extracted word as its sole argument from an empty, rootless
temp directory (DC-2) — mirroring `internal/specalign/helpers_test.go`'s
`runBinary` + package `TestMain` build-once precedent, never `go run` and
never importing `cmd/verdi` as a package (CLAUDE.md's own import boundary:
"Exec its pinned CLIs ... never import its packages" applies by the same
logic to this repo's own binary).

- A verb `dispatch.go` does not recognize at all — including one later
  removed from `verbPhase` — **fails**, naming the instruction file and the
  unrecognized verb text.
- A verb `dispatch.go` recognizes — fully implemented, phase-gated, or
  explicitly out-of-scope (`waivers`/`verify-artifact`, which print their
  own distinct "not implemented (out of v0 scope)" message rather than the
  top-level unknown-verb usage banner) — **passes**: an instruction
  accurately describing a real-but-unimplemented verb is not itself stale.

This check alone does **not** catch the motivating `SKILL.md` defect — see
AC-3.

## AC-3

A second, independent check tripwires the retired two-phase commit-to-design
ritual specifically — the case AC-2 structurally cannot see, because `board`
remains dispatched. Following `docsync_test.go`'s `handEditPhrasings` idiom
exactly (DC-3):

- A closed, named set of phrasings that instruct or teach `verdi board
  commit` / a frozen `board.json` as the **current** step of finishing a
  design-branch spec, with **no** accompanying retirement or grandfathered
  disclosure anywhere in the same file, **fails** the file, naming it and
  the offending phrase.
- A file that both names the command **and** discloses its retired or
  grandfathered status in the same breath — the shape a correctly-scoped
  rewrite of the skill would take — does **not** trip it: the tripwire
  targets teaching the ritual as current, not every mention of it (this
  spec's own Problem/Outcome sections, and `docs/architecture-and-journeys.md`
  itself, must keep discussing `verdi board commit` as *history* without
  ever tripping this check for doing so).

Carries the same honest disclosure `docsync_test.go`'s own header comment
already states: this is a substring/lexical tripwire for the common
phrasing, never a semantic guarantee that no future paraphrase could
describe the same stale procedure in different words (ADJ-50's accepted
residual, inherited verbatim, not re-litigated here).

## AC-4

AC-1 through AC-3's checks are proven to actually **fire**, not merely
present — the "the gate must BITE" requirement this story's own brief
demands, mirroring the Makefile's `lint-showcase`/`showcase-coverage` GUARD
rationale (a check whose red direction is unexercised is itself a silent
pass).

- A committed fixture instruction file carries **both** (a) a `verdi <verb>`
  reference naming a verb `dispatch.go` does not recognize, and (b) a
  retired-ritual phrase from AC-3's tripwire set with no disclosure.
  Driving the real check against that fixture **fails**, with the failure
  output naming the exact fixture file path and the exact offending verb or
  phrase — never a bare boolean, and never a `go test -run` invocation that
  would exit 0 by matching nothing if the underlying test function were
  ever renamed or deleted (the vacuous-pass class this package's own
  ADJ-47/ADJ-50 history already names).
- A second, **clean** fixture — real verbs, and a `board commit` mention
  paired with a retirement disclosure — **passes** with zero findings,
  proving the checks do not also false-positive on legitimate content.

## AC-5

Run against this repo's own `.claude/skills/*/SKILL.md` and repo-root
`CLAUDE.md` — not a synthetic fixture — AC-1 through AC-3's checks together
report zero findings, and `go test ./internal/specalign/...` (equivalently
`make spec-align`, and therefore `make verify`) exits clean.

Because `.claude/skills/commit-to-design/SKILL.md` teaches `verdi board
commit` as the current ritual with no retirement disclosure as authored
today, this AC **cannot** be satisfied by this story's design-time authoring
alone: it forces the build phase to apply DC-4's disposition (retire the
skill), or the rewrite-to-disclose alternative DC-3's tripwire design
deliberately leaves achievable, before `make verify` can go green on this
repo's own tree again.

## DC-1

Verb-reference extraction (AC-2) targets the literal `verdi <verb>`
invocation shape inside a backtick-delimited span only — never a bare
backticked verb name with no `verdi ` prefix.

Investigating both real instruction files found they use two different
prose shapes:

- `SKILL.md`'s five verb mentions are all the invocation shape:
  `` `verdi board commit <board-key> --name <spec-name>` `` (×1),
  `` `verdi board commit` `` (×2), `` `verdi lint` `` (×2) — matching AC-2's
  rule exactly.
- The worktree-root `CLAUDE.md`'s own CLI-verbs sentence instead names bare
  backticked verb words with no `verdi ` prefix at all: `` `gate` ``,
  `` `board` ``, `` `audit` ``, `` `close` ``, `` `gc` ``, `` `waivers` ``,
  `` `verify-artifact` ``.

`CLAUDE.md` therefore passes AC-2 **vacuously** today — zero references
found, not zero references checked-and-found-clean — which this decision
discloses rather than hides.

Widening the rule to classify any bare backticked lowercase-hyphenated word
as a candidate verb was considered and rejected as this decision's
alternative: a bare backtick span is ambiguous by construction
(`` `.verdi/specs/active/` `` and `` `make verify` `` are backtick spans
too, and are not verb references), and this repo's determinism posture
forbids a heuristic that would need semantic judgment to disambiguate
reliably. Reversible: widen the pattern later if a real drift case — a
bare-word instruction actually going stale — demonstrates the gap matters
in practice.

## DC-2

An extracted verb is classified **known** by execing the once-built `verdi`
binary with the extracted word as its sole argument from a fresh, rootless
temp directory, and checking whether stderr is **exactly** `dispatch.go`'s
own top-level unknown-verb usage banner (the literal `usage` const
`dispatch.go` prints when `verb` is neither `"lint"` nor a key in
`verbPhase`).

Anything else — a verb-specific usage error, an operational store-root
failure, `dispatch.go`'s own distinct "not implemented (out of v0 scope)"
message for `waivers`/`verify-artifact`, or genuine success — counts as
known.

This single mechanical rule is deliberately **not**
`verbs_test.go`/`helpers_test.go`'s existing `assertNotOutOfV0` helper: that
helper answers "is this verb real/implemented" (it detects the *different*
"not implemented" branch); this rule answers the coarser "does `dispatch.go`
recognize this word as a verb at all", the correct question for validating
prose that may accurately describe a recognized-but-out-of-scope verb —
`CLAUDE.md`'s own `waivers`/`verify-artifact` sentence is true today and
must not be flagged.

## DC-3

The retired-ritual tripwire (AC-3) fires on the **conjunction** of:

1. presence of a small, closed set of phrasings that instruct or describe
   `verdi board commit` / a frozen `board.json` as an active, current step
   to run; **and**
2. absence, anywhere in the **same file**, of a small, closed set of
   retirement-disclosure phrasings (e.g. "retired", "grandfathered",
   "superseded").

A pure presence-only tripwire — mirroring `handEditPhrasings`' own
unconditional shape — was considered and **rejected**: `verdi board commit`
and `board.json` are real, still-dispatched, legitimately-mentionable
strings. `verdi-surfaces/spec.md`'s own CLI table row and "Superseded"
section say them repeatedly, *correctly*, while explaining the retirement. A
presence-only rule would make the honest, correctly-scoped "rewrite to
disclose grandfathered-only scope" disposition this story's own brief names
as a valid alternative (see DC-4) **structurally impossible to ever pass**,
since any honest disclosure necessarily has to name the retired command to
explain what it is disclosing.

The presence-and-absence pairing is the smallest change that keeps **both**
candidate dispositions of the stale skill — retire outright, or rewrite to
disclose grandfathered-only scope — achievable ways to satisfy AC-5,
whichever the build phase and owner choose.

Same disclosed limit as `handEditPhrasings`/ADJ-50: lexical, not semantic — a
paraphrase of either the instruction phrase or the disclosure phrase can
evade or falsely trip this rule. The common, real cases (today's `SKILL.md`,
and a plausible honest rewrite of it) are what AC-4 proves this rule
against, not every conceivable paraphrase.

## DC-4

`.claude/skills/commit-to-design/SKILL.md`'s disposition is **RETIRE**, not
rewrite-to-disclose.

Investigated whether any grandfathered v0 artifact still needs the skill's
promotion flow. The skill's own stated trigger (its frontmatter
`description:`): "a draft feature spec's `dispositions:` block has
open-question entries and a frozen `board.json` sits beside it."

A search of this store's actual governing corpus, `.verdi/specs/` (`active/`
and `archive/` alike), for a `dispositions:` frontmatter block or a sibling
`board.json` found **neither anywhere**. The only `dispositions:`/
`board.json` occurrences left in this repo at all are:

- prose in `verdi-evidence-model`/`verdi-artifact-contract`/`verdi-surfaces`
  discussing the retired mechanism *historically* — never a live frontmatter
  block;
- `examples/showcase/.verdi/specs/archive/loan-refi-2023/board.json` —
  already **archived** (closed), needing no further disposition work;
- `testdata/violations/VL-014/**` — VL-014's own negative-path test
  fixtures.

Zero live drafts in `specs/active/` anywhere in this repo need the skill's
promotion flow today.

Smallest reversible option given that finding: retire the file. Its
instructions have no current audience and actively mislead — the motivating
defect this whole story exists to fix. A rewrite-to-disclose would leave a
permanently dead-letter skill in the tree for a use case that does not
currently exist. Reversible: nothing prevents re-authoring a
grandfathered-scoped version later if a genuine pre-round-4 artifact ever
surfaces needing it (none is known to exist today).

Retirement mechanics themselves (delete vs. archive the file, any
commit-message/history discipline) are **build-phase work**, out of this
design-only story's own deliverable — this decision records the choice, not
its execution.

## CO-1

No network in any test: every check runs entirely against fixture
instruction files and the once-built local `verdi` binary; execing the
binary against a fresh, rootless temp directory (DC-2) is local process
exec, not network I/O — the same hermetic pattern `verbs_test.go`'s own
`serve`/`mcp`/`audit`/`align`/`gc`/`close`/`disposition` subtests already
rely on.

## CO-2

Mechanical only: this story adds no semantic or natural-language drift
detection, matching this repo's determinism posture. AC-3's tripwire is a
disclosed, imperfect lexical heuristic, never a claim that no paraphrase
could describe the same stale procedure and evade it (ADJ-50's residual,
inherited verbatim). Closing that residual, if ever warranted, is explicitly
a judged-sweep-shaped follow-on (mirroring `spec/judged-sweep`'s own
LLM-outside-the-gate-path precedent), not this story.

## CO-3

Scope is exactly the enumerated gap: `.claude/skills/*/SKILL.md` plus the
repo-root `CLAUDE.md`. Explicitly **out**, because each is already covered
by a sibling `specalign` check or is out of today's inventory:

- README example freshness — `internal/showcasealign`'s
  `TestReadmeExamplesFresh`.
- Targeted architecture-doc claims — `docsync_test.go`.
- MCP tool naming — `mcptools_test.go`.

`AGENTS.md` does not exist anywhere in this repo today (checked); it is not
enumerated by this story, and a future story extends AC-1's same glob-based
enumeration the moment one is added — no logic change anticipated, only the
glob growing a second pattern.

CLI **flag**-level validation, as opposed to verb-level, is also out of
scope: `dispatch.go`'s `verbPhase` is a ready-made canonical verb inventory
(`verbs_test.go`'s own precedent); no equivalent canonical, machine-checkable
flag inventory exists yet to validate prose flag mentions against, and
building one is materially new scope beyond "follow `verbs_test.go`'s
relationship to `dispatch.go`."

## CO-4

The `board commit` CLI **verb**'s own fate — removal, or a `feature`-style
deprecation alias (R4-I-6) — is explicitly out of this story's scope: this
story only gates what agent-facing **prose** teaches about it, never the
dispatch table itself. `cmd/verdi/board.go` and `dispatch.go`'s
`verbPhase["board"]` are untouched by this story.

A future story, if the owner ever decides to formally deprecate or remove
the verb, should follow the `cli:feature` exclusion-from-showcase-coverage
precedent (R4-I-54, `internal/showcasealign/coverage_test.go`'s `cliVerbs`/
`featureVerbExcluded`) as its template for how a real-but-non-canonical/
aliased verb is handled inside an inventory-shaped gate, rather than
inventing a new pattern.

## OQ-1

`instruction-conformance` has no accepted parent feature to `implements`
today. The four currently-active feature specs — `code-health`,
`disclosure-legibility`, `public-showcase`, `scoping-canvas` — were each
checked against this story's problem/outcome:

- `code-health`'s own `co-3` closes its scope to "the pressure-test survivor
  list only — nothing else enters."
- `disclosure-legibility`, `public-showcase`, and `scoping-canvas` are each
  unrelated by subject (disclosure vocabulary, the showcase corpus/README,
  and the wall/stub editor, respectively).

R4-I-56 — the most recent invention-ledger entry, and also a product of this
same external-assessment round — records a comparable case (`verdi
init`/`doctor`) proceeding as "a future story" with no feature parent named,
so a parentless story is not itself unprecedented.

The `links:` block above therefore carries `verdi design start`'s own
standard unresolved scaffold placeholder (`spec/todo-replace-feature-name#ac-1`,
`cmd/verdi/design.go`) rather than a fabricated edge into an unrelated
feature's AC — never resolving this ambiguity silently, and never from what
a similarly-shaped artifact happens to do. `verdi lint` therefore reports
one `VL-003` finding ("does not resolve") against this spec until the owner
either designates a real parent feature or ratifies this as a standing
exception. Recorded as R4-I-58 in `PLAN-V1.md` §7.
