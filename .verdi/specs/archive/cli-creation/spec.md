---
id: spec/cli-creation
kind: spec
title: "CLI Creation"
owners: [platform-team]
class: story
status: closed
story: jira:VERDI-P2-8
problem: { text: "spec/creation-surfaces#ac-3 names the CLI half of the ADJ-65 asymmetry: design start scaffolds every draft with generic TODO placeholders no matter what flags are given, has no way to supply a real problem/outcome statement at all, and has no CLI equivalent of the board's stub-instantiate action — a team working entirely from the command line cannot create a story from a feature's declared stub without opening the board, and can never produce a TODO-free scaffold from the CLI at all. The board's own creation form already requires its statement fields non-empty before it will render a spec at all (spec/creation-form ac-2's required-statement refusal); design start has never made that promise, silently emitting the same TODO markers section-for-section regardless of what the operator actually knows about the work.", anchor: problem }
outcome: { text: "design start grows a --problem/--outcome pair of flags that source a TODO-free scaffold section-for-section, a --defer-statements flag that commits deliberate TODOs anyway but only together with an explicit disclosure line naming them as deferred, and a TTY interview — driven from the exact same placeholder-enumeration descriptors (internal/designscaffold.Fields) the board's own creation form already reuses, one field contract, two front ends — when no creation flags are given at all; a non-interactive invocation given neither the statement flags nor --defer-statements refuses by name rather than silently emitting the old TODO placeholders. --from-stub <feature> <stub> reaches the CLI through the identical shared stub-instantiate core this story extracts out of internal/workbench/boardspecapi.go, so the board action and the CLI path can never drift — proven by an output-equality parity assertion and by the board's own existing tests passing completely unmodified. --owners deliberately stays out of scope, the same posture I-10/X-4 already ratified, disclosed rather than silently reconsidered now that the verb grows other creation flags.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "design start grows a --problem and --outcome pair of flags: given together, they source the scaffold's problem/outcome sections directly, so every section the class template declares renders TODO-free — never the `TODO: replace with the real problem statement before accept` / `TODO: design notes.` placeholders the unflagged path always emitted before this story. --defer-statements is the opposite, explicit choice: it commits the same placeholder TODOs the old default always did, but never silently — the invocation prints a disclosure line naming problem/outcome as deliberately deferred, so a reader of the ritual's own output can see the deferral was chosen, not missed. The two are mutually exclusive with each other, and --problem/--outcome must be given together or not at all — a lone flag refuses by name rather than leaving one section templated and the other not", evidence: [behavioral], anchor: ac-1 }
  - { id: ac-2, text: "given no creation flags at all — no --problem, no --outcome, no --defer-statements — on an attached terminal, design start runs a TTY interview that prompts for exactly the class template's own statement placeholders, enumerated through internal/designscaffold.Fields, the identical descriptor list the board's creation form already validates its own submissions against (spec/creation-form ac-1): one field contract, two front ends, never a second hand-rolled field list to drift from the first. The identical invocation with no creation flags and no attached terminal refuses outright, by name, rather than falling back to the old silent TODO placeholders: statement fields are required content now, exactly as the board form already requires them, and every non-interactive way to skip them is the explicit --defer-statements flag, never an implicit default", evidence: [behavioral], anchor: ac-2 }
  - { id: ac-3, text: "design start --from-stub <feature> <stub> creates a story from a declared feature stub from the CLI for the first time, exactly as the board's own stub-instantiate action already does, because both now call one shared stub-instantiate core extracted out of internal/workbench/boardspecapi.go into its own package rather than a second CLI-side reimplementation drifting from the board's. Given the identical feature and stub, the two surfaces' rendered spec content is asserted equal — the parity proof that closes the ADJ-65 asymmetry at the mechanism, not merely at the surface — and the board's own existing stub-instantiate and creation-form handler tests pass completely unmodified, the proof that extracting the shared core changed no board behavior underneath it", evidence: [behavioral, static], anchor: ac-3 }
  - { id: ac-4, text: "--owners deliberately stays out of design start's flag surface — the same posture I-10/X-4 already ratified (05 §CLI: no magic, no tracker-derived naming, and no CLI-supplied owner override either), disclosed here rather than silently reconsidered now that the verb grows other creation flags: the usage text and the verb's whole flag-parsing source carry no --owners token anywhere", evidence: [static], anchor: ac-4 }
links:
  - { type: implements, ref: "spec/creation-surfaces#ac-3" }
frozen: { at: 2026-07-22, commit: e1cd2d1f957a200804b97a78829482d1ca8b57f9, stub_matched: true }
---
# CLI Creation

## Problem

`spec/creation-surfaces#ac-3` names the CLI half of the ADJ-65 asymmetry.
`design start` scaffolds every draft with generic TODO placeholders no
matter what flags are given, has no way to supply a real problem/outcome
statement at all, and has no CLI equivalent of the board's stub-instantiate
action — a team working entirely from the command line cannot create a
story from a feature's declared stub without opening the board, and can
never produce a TODO-free scaffold from the CLI at all.

The board's own creation form already requires its statement fields
non-empty before it will render a spec at all (`spec/creation-form` ac-2's
required-statement refusal: "a work item with no stated problem or outcome
is not an artifact yet"). `design start` has never made that promise,
silently emitting the same `TODO: design notes.` markers section-for-section
regardless of what the operator actually knows about the work — the exact
inconsistency `spec/creation-surfaces#ac-3`'s own letter calls out.

## Outcome

`design start` grows a `--problem` and `--outcome` pair of flags that
source a TODO-free scaffold section-for-section, a `--defer-statements`
flag that commits deliberate TODOs anyway but only together with an
explicit disclosure line naming them as deferred, and a TTY interview —
driven from the exact same placeholder-enumeration descriptors
(`internal/designscaffold.Fields`) the board's own creation form already
reuses, one field contract, two front ends — when no creation flags are
given at all. A non-interactive invocation given neither the statement
flags nor `--defer-statements` refuses by name rather than silently
emitting the old TODO placeholders.

`--from-stub <feature> <stub>` reaches the CLI through the identical
shared stub-instantiate core this story extracts out of
`internal/workbench/boardspecapi.go`, so the board action and the CLI path
can never drift — proven by an output-equality parity assertion and by the
board's own existing tests passing completely unmodified.

`--owners` deliberately stays out of scope, the same posture I-10/X-4
already ratified, disclosed rather than silently reconsidered now that the
verb grows other creation flags.

## Ac 1

`design start` grows a `--problem` and `--outcome` pair of flags: given
together, they source the scaffold's problem/outcome sections directly, so
every section the class template declares renders TODO-free — never the
`TODO: replace with the real problem statement before accept` /
`TODO: design notes.` placeholders the unflagged path always emitted
before this story.

`--defer-statements` is the opposite, explicit choice: it commits the same
placeholder TODOs the old default always did, but never silently — the
invocation prints a disclosure line naming problem/outcome as deliberately
deferred, so a reader of the ritual's own output (never only the diff) can
see the deferral was chosen, not missed.

The two are mutually exclusive with each other, and `--problem`/`--outcome`
must be given together or not at all — a lone flag refuses by name rather
than leaving one section templated and the other not.

Correction (build-time witness — recorded, not silently narrowed). The
frozen text above's "every section the class template declares renders
TODO-free" is imprecise about WHICH sections, pinned by this story's own
first test against the actual embedded templates
(`internal/designscaffold/templates/story.md`/`feature.md`). The
canonical templates carry the statement content in TWO independent
positions: the frontmatter's `problem: { text: ... }` / `outcome: { text:
... }` attribute — the position VL-020, the evidence-obligation rules,
and `spec/creation-form`'s own already-shipped form all key off — and the
body's separate `## Problem`/`## Outcome` headings, which are a fixed,
always-literal `TODO: design notes.` that no `ScaffoldData` field
controls at all (neither template references `.Problem`/`.Outcome` in
its body). `--problem`/`--outcome` make the FRONTMATTER attribute
TODO-free, exactly the property `TestFeature_RealStatements`/
`TestStory_RealStatements`/`TestRunDesignStart_ProblemOutcomeFlags_
TODOFree` pin; the body headings' own placeholder prose is unaffected —
unchanged from `spec/creation-form`'s own established scope (its ac-3's
"TODO-free in every position whose field was actually filled" already
meant exactly this: a position a FIELD renders into, which the body
headings are not). Changing the templates' body rendering to also
interpolate `.Problem`/`.Outcome` was considered and declined: it would
touch `spec/scaffold-templates` ac-1's own frozen byte-identity contract
(`TestByteForByte`, pinned against the retired string-builder's literal
`TODO: design notes.` body text) for every OTHER caller of `Feature`/
`Story` too, out of this story's scope.

## Ac 2

Given no creation flags at all — no `--problem`, no `--outcome`, no
`--defer-statements` — on an attached terminal, `design start` runs a TTY
interview that prompts for exactly the class template's own statement
placeholders, enumerated through `internal/designscaffold.Fields`, the
identical descriptor list the board's creation form already validates its
own submissions against (`spec/creation-form` ac-1): one field contract,
two front ends, never a second hand-rolled field list to drift from the
first.

The identical invocation with no creation flags and no attached terminal
refuses outright, by name, rather than falling back to the old silent TODO
placeholders: statement fields are required content now, exactly as the
board form already requires them, and every non-interactive way to skip
them is the explicit `--defer-statements` flag, never an implicit default.

## Ac 3

`design start --from-stub <feature> <stub>` creates a story from a
declared feature stub from the CLI for the first time, exactly as the
board's own stub-instantiate action already does, because both now call
one shared stub-instantiate core extracted out of
`internal/workbench/boardspecapi.go` into its own package rather than a
second CLI-side reimplementation drifting from the board's.

Given the identical feature and stub, the two surfaces' rendered spec
content is asserted equal — the parity proof that closes the ADJ-65
asymmetry at the mechanism, not merely at the surface — and the board's
own existing stub-instantiate and creation-form handler tests pass
completely unmodified, the proof that extracting the shared core changed
no board behavior underneath it.

## Ac 4

`--owners` deliberately stays out of `design start`'s flag surface — the
same posture I-10/X-4 already ratified (05 §CLI: no magic, no
tracker-derived naming, and no CLI-supplied owner override either),
disclosed here rather than silently reconsidered now that the verb grows
other creation flags: the usage text and the verb's whole flag-parsing
source carry no `--owners` token anywhere.

## Note

Disclosed build-time decisions, recorded per the provenance discipline
(none reverses the frozen ACs above; each fills a gap the design wave
that pressure-tested `spec/creation-surfaces` deliberately deferred to
"story spec time," per design doc §7):

**Single-line interview answers.** The TTY interview
(`internal/designinterview`) collects each statement field as ONE line
of scripted or real terminal input, re-prompting on a blank answer. The
board form's own statement fields are a free-form textarea; no CLI
line-based multi-line-input convention exists anywhere in this module to
build on, and inventing one was out of this story's scope. Multi-line
statement authoring stays a design-branch hand-edit of `spec.md` itself,
or the board form.

**`--from-stub`'s exit-code convention.** Every refusal on the
`--from-stub` path (unknown slug, wrong class/status, an already-existing
branch, an unreadable feature spec) exits 2, matching `design start`'s
own established local convention: every OTHER refusal in this verb — an
invalid name, an already-existing spec directory, a malformed story ref —
is exit 2 too (never exit 1). This keeps the verb internally consistent
rather than introducing its first exit-1 business verdict on one path
alone; the board's own HTTP action expresses the identical refusals as
400 Bad Request, its own transport's equivalent.

**`<feature>` is a bare spec name.** `design start --from-stub <feature>
<stub>` takes `<feature>` as a bare, kebab-case spec name (e.g.
`creation-surfaces`), never a `spec/`-prefixed ref — matching this verb's
own `--name` convention for the spec being CREATED, and the board's own
bare `{name}` URL path segment for the wall a stub-instantiate action
runs against. The smallest reversible choice consistent with both
existing siblings, over inventing a third addressing form.
