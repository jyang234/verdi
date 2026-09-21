---
id: spec/ritual-write-scope-v2
kind: spec
title: "Declared write scope for every ritual that touches git"
owners: [platform-team]
class: feature
problem: { text: "The largest and most severe class of shipped product defects is lifecycle rituals branching, committing, staging, or switching in ways the operator did not ask for: six of thirty-two real findings from one day of use (UAT-021, UAT-023, UAT-031, UAT-033, UAT-034, UAT-036) and a seventh adjacent (UAT-019), costing operators their own uncommitted work swept into commits they did not author, by a tool whose premise is that claims are traceable to who made them; no gate check exists over what a verb does to the repository, and the one token witness that exists (internal/gitx's fast-forward test) covers one path (process-audit PA-024).", anchor: problem }
outcome: { text: "Every verb that mutates a repository declares its write scope as data in one registry; a gate witness runs each ritual on fixtures and fails when the observed effects exceed the declaration; the forbidden-token witness covers every verb; and the six findings become regression witnesses, so the whole class turns into a red gate instead of a product finding.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "Every verb that mutates a git repository declares a write scope in one registry with a closed grammar: the refs it may create or move, whether HEAD may switch, the paths it may stage, how it treats index entries it did not stage (refused before any mutation, scoped out of its commit, carried into its commit, or moot because it never commits), whether untracked files may enter a commit, and whether it may push; a static witness fails when a verb that calls gitx has no declaration, and an unknown scope field or value fails closed.", evidence: [static, attestation], anchor: ac-1 }
  - { id: ac-2, text: "A behavioral witness runs each declared ritual against fixturegit repositories seeded with an untracked file, a pre-staged unrelated index entry, and a dirty tracked file; observes refs, index and working tree before and after, and the file list of every commit the ritual creates; and fails when any effect lies outside the declared scope, asserting the declared index-carry state exactly (a refusal exits 2 with no mutation, a scoped commit omits the foreign entry, no commit object exists when none is declared); an effect the sensor cannot attribute is reported as unproven, never as within scope.", evidence: [behavioral, attestation], anchor: ac-2 }
  - { id: ac-3, text: "The forbidden-token witness (reset, restore, clean, stash, --force, update-ref) covers every verb's git command log through one seam in internal/gitx, and spec/readiness-recovery ac-9's recovery witness is one consumer of that seam rather than a second mechanism.", evidence: [static, behavioral, attestation], anchor: ac-3 }
  - { id: ac-4, text: "UAT-036 is pinned for design start and the commit-to-design ritual: their commits never carry a pre-staged entry outside their declared paths, so no ritual in the registry declares that it carries foreign entries; close's existing refusal of a non-empty index is pinned as its own witness (exit 2, no mutation); build start cuts its branch from the resolved default branch and the branch-cutting rituals check name collisions against the tree they cut from (UAT-023, UAT-031); UAT-021, UAT-033 and UAT-034, already fixed at this revision's base, remain covered by their existing tests.", evidence: [behavioral, attestation], anchor: ac-4 }
constraints:
  - { id: co-1, text: "No new git primitive is added; declarations describe what existing rituals do and the witness observes; making a ritual conform is a fix to the ritual, never a widening of its declaration without an owner-visible decision.", anchor: co-1 }
  - { id: co-2, text: "The declaration is data checked by the gate (enforcement rung analyzer or gate assertion), never a comment or a plan sentence an agent remembers.", anchor: co-2 }
  - { id: co-3, text: "No network in any test; every ritual runs against internal/fixturegit repositories; pushes are exercised against a local bare remote only.", anchor: co-3 }
  - { id: co-4, text: "spec/readiness-recovery's recovery semantics are not changed: whichever feature lands first owns the gitx seam and the other consumes it.", anchor: co-4 }
decisions:
  - { id: dc-1, text: "One declaration plus one witness for the class, not six point fixes: the findings share a cause (no check over what a verb does to the repository), and a per-bug fix leaves the next ritual free to repeat it.", anchor: dc-1 }
  - { id: dc-2, text: "Declared scope is compared against observed effects, not only against the command log: a command log proves what gitx ran, a repository state diff proves what changed, and the file list of each commit the ritual created proves what it recorded; the three together catch a mutation made outside gitx and tell a refused commit from a carried one, because a log records what ran and never what refused to run.", anchor: dc-2 }
  - { id: dc-3, text: "The index-carry field is a closed four-valued enum (refused, scoped, carried, no_commit), not a boolean: the five governed rituals occupy four states, and a boolean reads a refusal, a scoped commit and no commit as the same false, so deleting a refusal guard would pass co-1's widening check unseen; where more than one state could describe a ritual, the declared value is the first that holds in the ritual's execution order (refused if a guard refuses a foreign entry before any mutation, no_commit if it never creates a commit, scoped if every commit it creates names its own paths, carried otherwise); carried stays in the grammar so the witness can name the defect state, and ac-4 requires that no shipped ritual declares it.", anchor: dc-3 }
  - { id: dc-4, text: "The registry lives in source as a Go literal in one internal package checked by a gate test, the same shape as the CLI-verb and MCP-tool inventories; it is not a policy payload, because a write scope is fixed product behaviour shipped with the binary and a governance profile an operator adopts must not be able to widen it (co-1), and it is not an amendment to design specs 03 or 04, which say nothing about verb effects, so no ratification precedes the build.", anchor: dc-4 }
  - { id: dc-5, text: "This feature lands the gitx recorder seam: a consumer-defined one-method observer attached through the context every gitx call already receives, so gitx's signatures do not change and no package-level mutable state is added; spec/readiness-recovery ac-9's forbidden-token check is one recorder implementation consuming that seam, and readiness-recovery's wave 3 plan consumes it rather than building a second mechanism, because ac-9 needs strictly less than ac-1 through ac-3 need (a token scan needs no per-verb attribution and no state diff, while the registry witness needs both).", anchor: dc-5 }
stubs:
  - { slug: gitx-recorder-seam, acceptance_criteria: [ac-3] }
  - { slug: write-scope-registry, acceptance_criteria: [ac-1] }
  - { slug: ritual-effect-witness, acceptance_criteria: [ac-2, ac-4] }
links:
  - { type: depends-on, ref: "spec/readiness-recovery" }
  - { type: supersedes, ref: "spec/ritual-write-scope" }
supersession:
  carried: [ac-3, co-1, co-2, co-3, co-4, dc-1]
  amended:
    - { id: ac-1, note: "the boolean index-carry field becomes a four-state value (refused, scoped, carried, no_commit) and fail-closed covers unknown values as well as unknown fields; spike oq-3" }
    - { id: ac-2, note: "the sensor gains the file list of every commit the ritual creates and asserts the declared index-carry state exactly; spike oq-2 and the close correction" }
    - { id: ac-4, note: "UAT-036's pin covers design start and commit-to-design, close's refusal gets its own pin, build start's base and collision check are named as open fixes (UAT-023, UAT-031), and UAT-021/033/034 are recorded as already fixed at the base; spike oq-1" }
    - { id: dc-2, note: "the sensor is three-way: command log, state diff and the created commits' file lists, because the log over-reports a refused commit and under-reports a carried one; spike oq-2" }
  amended_advisory: []
  removed:
    - { id: oq-1, note: "answered by spec/ritual-write-scope-spike (docs/spikes/ritual-write-scope/README.md, oq-1: per-verb inventory, inventory.tsv and inventory-operands.tsv)" }
    - { id: oq-2, note: "answered by spec/ritual-write-scope-spike (oq-2: census of non-gitx exec sites and the log-plus-state-diff sensor ruling), absorbed into dc-2 and ac-2" }
    - { id: oq-3, note: "answered by spec/ritual-write-scope-spike (oq-3: five declarations in one seven-field grammar, declarations.yaml), absorbed into ac-1 and dc-3" }
    - { id: oq-4, note: "answered by spec/ritual-write-scope-spike (oq-4: Go registry plus gate test, no ratification), absorbed into dc-4" }
    - { id: oq-5, note: "answered by spec/ritual-write-scope-spike (oq-5: context-scoped recorder sketch, this feature lands the seam first), absorbed into dc-5" }
  added: [dc-3, dc-4, dc-5]
---
# Declared write scope for every ritual that touches git (v2)

## Problem

Grouped by theme, one class dominates the product findings log: rituals
that touch git state the operator did not ask them to touch. This revision
restates the table at the base the spike measured (main `5f60c76c`), with
each value taken from source guards and a seeded fixture rather than from
the tracker's status column.

| Finding | What the ritual does | Status at `5f60c76c` |
|---|---|---|
| UAT-021 | `design start` branched from HEAD and switched the primary checkout | fixed (resolved default-branch base, disclosed switch) |
| UAT-023 | `build start` cuts the feature branch from HEAD (`gitx.CheckoutNewBranch`, no explicit base) | open |
| UAT-031 | Branch-cutting checks name collisions against the wrong tree | fixed for `design start` and the board; open for `build start` |
| UAT-033 | `design start` committed every untracked file in the checkout | fixed (`gitx.AddPaths`, scoped to the spec directory) |
| UAT-034 | The commit-to-design ritual swept the working tree the same way | fixed (`gitx.AddPaths`, scoped) |
| UAT-036 | Ritual commits carry pre-staged index entries | open for `design start` and commit-to-design; `close` refuses to begin (exit 2, `requireCleanIndex`); `policy adopt` scopes its commit with a pathspec |
| UAT-019 | Adjacent: the commit dialog accepts any message, so a proposal commit can claim acceptance | fixed |

Every other finding class costs confusion or rework. This one costs the
operator's own work.

## Outcome

A ritual's effect on the repository becomes a declared, checked contract.

## What changed from v1

spec/ritual-write-scope was accepted with five open questions and one
spike stub. The spike (spec/ritual-write-scope-spike, answer at
`docs/spikes/ritual-write-scope/README.md` on branch
`agent/process-hardening-spikes`) answered all five and falsified two of
the predecessor's premises:

- `close` is not a UAT-036 ritual. `requireCleanIndex` is the first
  statement of `runClose` and refuses any pre-staged path before any
  mutation. The spike's own first answer put `close` in the UAT-036
  column by reading the shape of the command log; the correction is the
  reason dc-2 now names three sensors and dc-3 exists.
- The seven-field grammar the spike story proposed needed one change: the
  boolean `index_carry_foreign` cannot express the four states the five
  rituals occupy. ac-1 and dc-3 adopt the enum (owner decision, 2026-09-21;
  invention ledger SI-207, mirrored as PLAN.md §7 I-131).

The remaining answers landed as decisions: dc-4 (registry home, no
ratification), dc-5 (seam shape and sequencing against readiness-recovery
wave 3). ac-2 and ac-4 were amended so the witness and the pins match what
the rituals actually do at the base. ac-3 and the four constraints are
carried unchanged.

## Context from the process audit

- PA-024 is the entry. It exists because the findings were clustered rather
  than read one at a time; the entry asks that the product log be
  re-clustered whenever the ledger is revisited.
- The existing token check is `internal/gitx/ffonly_test.go`, a unit test
  over the fast-forward path's calls (reset, --force, update-ref, apply,
  am). gitx's `run` in `exec.go` has no recorder seam today, and gitx has
  two other exec sites (`plumbing.go`, `configvalue.go`); the spike's
  throwaway recorder patched exactly those three.
- spec/readiness-recovery ac-9 requires that no recovery run's git command
  log contains the forbidden tokens; that witness is wave 3 of that feature
  and unbuilt at authoring, and no wave 3 plan exists yet. co-4 and dc-5
  govern the shared seam.
- PA-015 and spec/self-governance: the registry is product behaviour in
  source, not governance data an operator adopts (dc-4); self-governance
  may cite it as an analyzer-rung witness but does not own it.

## ac-1

The registry is the contract; the static witness makes a missing
declaration impossible to ship, and a closed grammar makes an unknown field
or value fail closed like every other artifact here. The index-carry slot
is four-valued (dc-3).

## ac-2

The seeded fixture is the whole point: an untracked file, an unrelated
staged entry, and a dirty tracked file are exactly what the findings swept
up. The witness asserts a different thing per declared index-carry state,
which is why a boolean could not have written it. Unattributable effects
are unproven, never within scope (three-valued honesty).

## ac-3

One seam, two consumers.

## ac-4

The findings become pins, so the class stays closed. Two rituals owe the
UAT-036 fix; `close` owes the opposite pin, that its refusal keeps working,
because under ac-1 a declared `carried` for `close` would have been a
permission that let a future deletion of the guard pass the witness.
`build start` owes the UAT-021 shape of fix (explicit resolved base) and
its half of UAT-031.

## co-1

Describe, observe, fix the ritual; never widen the declaration to make the
gate green without an owner-visible decision. Narrowing a declaration to
close a defect is the owner-visible decision ac-4 already records.

## co-2

Data, not prose.

## co-3

Hermetic, as always.

## co-4

The seam is shared; recovery semantics are not this feature's to change.

## dc-1

Class remedy, not point fixes.

## dc-2

Command log, effect diff and the commit's own file list together. The
spike showed the log and the state diff fail in opposite directions: the
log over-reported `close` (a pathspec-less commit that a guard means is
never reached) and under-reported `design start` (the same shape, where
the sweep happens). Every ritual also writes its artifacts with ordinary
file I/O before it ever calls git, which no command log can explain.

## dc-3

Grammar by construction, from five real rituals: `design start` carried,
`build start` no_commit, `close` refused, commit-to-design carried,
`policy adopt` scoped. The precedence rule answers the case the spike left
undefined, a ritual that both guards and scopes. The old boolean is
recoverable as `carried`, so nothing derivable is duplicated.

## dc-4

Home of the declaration. The policy payload grammar admits the shape, but
adopting it would make how far a ritual may mutate a repository a property
of the operator's chosen profile, which is backwards from co-1. Design
specs 00 through 05 contain no sentence about verb git-effects, so there is
nothing to amend.

## dc-5

Sequencing against wave 3. readiness-recovery ac-9 is a forbidden-token
scan over one run's log; the registry witness needs a per-verb,
attributable log paired with a state diff. A seam built to the smaller
need would have to be widened; a seam built to the larger need serves the
smaller one as a single recorder implementation. Every gitx function
already takes a context, so attaching the recorder there changes no
signature, and a package-level hook would be a race hazard under
`go test -race` across parallel subtests.

## Known unmeasured at authoring

The spike left two values disclosed rather than guessed; the feature build
measures them before the registry ships, and neither is a design question:

- `close`'s plain, CI-gated publish path (`PublishRollup`) was not
  exercised, so its `refs_move` value is unmeasured; only `--force-local`
  was observed.
- commit-to-design's `carried` is derived from source (no guard, a
  pathspec-less commit) and not reproduced dynamically, because the ritual
  has no CLI verb to drive from a seeded clone; `design start`'s identical
  source pair was reproduced.

Two adjacent facts the build inherits: `internal/policyadopt`'s own
package tests exercise no git call, so `policy adopt`'s footprint is
covered only through `cmd/verdi` subprocess tests; and `may_push` is false
for all five rituals, with the workbench's board push action the first
place a true value would appear.
