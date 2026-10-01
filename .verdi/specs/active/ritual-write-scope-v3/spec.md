---
id: spec/ritual-write-scope-v3
kind: spec
title: "Declared write scope for every ritual that touches git"
owners: [platform-team]
class: feature
problem: { text: "The largest and most severe class of shipped product defects is lifecycle rituals branching, committing, staging, or switching in ways the operator did not ask for: six of thirty-two real findings from one day of use (UAT-021, UAT-023, UAT-031, UAT-033, UAT-034, UAT-036) and a seventh adjacent (UAT-019), costing operators their own uncommitted work swept into commits they did not author, by a tool whose premise is that claims are traceable to who made them; no gate check exists over what a verb does to the repository, and the one token witness that exists (internal/gitx's fast-forward test) covers one path (process-audit PA-024).", anchor: problem }
outcome: { text: "Every verb that mutates a repository declares its write scope as data in one registry; a gate witness runs each ritual on fixtures and fails when the observed effects exceed the declaration; the forbidden-token witness covers every verb; and the six findings become regression witnesses, so the whole class turns into a red gate instead of a product finding.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "Every verb that mutates a git repository declares a write scope in one registry with a closed grammar: the refs it may create, move, or delete, whether HEAD may switch, the linked worktrees it may add or remove, the paths it may stage, how it treats index entries it did not stage (refused before any mutation, scoped out of its commit, carried into its commit, or moot because it never commits), whether untracked files may enter a commit, and whether it may push; every exported internal/gitx function is classified as mutating or read-only, a static witness fails when an unclassified function exists or when a verb that reaches a mutating one has no declaration, and an unknown scope field or value fails closed.", evidence: [static, attestation], anchor: ac-1 }
  - { id: ac-2, text: "A behavioral witness runs each declared ritual, along each publication path it supports, against fixturegit repositories in two seeded states: one with an untracked file, a pre-staged unrelated index entry, and a dirty tracked file, and one with a clean index and the same unrelated untracked and dirty tracked work, so a ritual that refuses a foreign index entry is also observed running to completion; it observes refs, HEAD, the index, the working tree and the repository's linked worktrees before and after, and the file list of every commit the ritual creates; it fails when any effect lies outside the declared scope, when a ritual expected to complete does not, and it asserts the declared index-carry state exactly (a refusal exits 2 with no mutation, a scoped commit omits the foreign entry, no commit object exists when none is declared); an effect the sensor cannot attribute is reported as unproven, never as within scope.", evidence: [behavioral, attestation], anchor: ac-2 }
  - { id: ac-3, text: "The forbidden-token witness (reset, restore, clean, stash, --force, update-ref) covers every verb's git command log through one seam in internal/gitx, and spec/readiness-recovery ac-9's recovery witness is one consumer of that seam rather than a second mechanism.", evidence: [static, behavioral, attestation], anchor: ac-3 }
  - { id: ac-4, text: "UAT-036 is pinned for design start (including its --supersedes path), the commit-to-design ritual, accept diagram, and constitution propose: their commits never carry a pre-staged entry outside their declared paths; the board's Commit and push, which commits and pushes the whole working tree at the operator's explicit request, is the one ritual that declares carried, and no other ritual declares it; close's existing refusal of a non-empty index is pinned as its own witness (exit 2, no mutation); build start and constitution propose cut their branches from the resolved default branch, and the branch-cutting rituals check name collisions against the tree they cut from (UAT-023, UAT-031); UAT-021, UAT-033 and UAT-034, already fixed at this revision's base, remain covered by their existing tests.", evidence: [behavioral, attestation], anchor: ac-4 }
constraints:
  - { id: co-1, text: "No new git primitive is added; declarations describe what existing rituals do and the witness observes; making a ritual conform is a fix to the ritual, never a widening of its declaration without an owner-visible decision.", anchor: co-1 }
  - { id: co-2, text: "The declaration is data checked by the gate (enforcement rung analyzer or gate assertion), never a comment or a plan sentence an agent remembers.", anchor: co-2 }
  - { id: co-3, text: "No network in any test; every ritual runs against internal/fixturegit repositories; pushes are exercised against a local bare remote only.", anchor: co-3 }
  - { id: co-4, text: "spec/readiness-recovery's recovery semantics are not changed: whichever feature lands first owns the gitx seam and the other consumes it.", anchor: co-4 }
decisions:
  - { id: dc-1, text: "One declaration plus one witness for the class, not six point fixes: the findings share a cause (no check over what a verb does to the repository), and a per-bug fix leaves the next ritual free to repeat it.", anchor: dc-1 }
  - { id: dc-2, text: "Declared scope is compared against observed effects, not only against the command log: a command log proves what gitx ran, a repository state diff proves what changed, and the file list of each commit the ritual created proves what it recorded; the three together catch a mutation made outside gitx and tell a refused commit from a carried one, because a log records what ran and never what refused to run.", anchor: dc-2 }
  - { id: dc-3, text: "The index-carry field is a closed four-valued enum (refused, scoped, carried, no_commit), not a boolean: the five governed rituals occupy four states, and a boolean reads a refusal, a scoped commit and no commit as the same false, so deleting a refusal guard would pass co-1's widening check unseen; where more than one state could describe a ritual, the declared value is the first that holds in the ritual's execution order (refused if a guard refuses a foreign entry before any mutation, no_commit if it never creates a commit, scoped if every commit it creates records only its declared paths, whether by a pathspec or by a scratch index, carried otherwise); carried stays in the grammar so the witness can name the defect state, and ac-4 names the one shipped ritual that may declare it (the board's Commit and push, dc-11); no other ritual may.", anchor: dc-3 }
  - { id: dc-4, text: "The registry lives in source as a Go literal in one internal package checked by a gate test, the same shape as the CLI-verb and MCP-tool inventories; it is not a policy payload, because a write scope is fixed product behaviour shipped with the binary and a governance profile an operator adopts must not be able to widen it (co-1), and it is not an amendment to design specs 03 or 04, which say nothing about verb effects, so no ratification precedes the build.", anchor: dc-4 }
  - { id: dc-5, text: "This feature consumes the gitx recorder seam that readiness-recovery's wave 3 landed first (ledger SI-219, under co-4): gitx.Observer and gitx.WithObserver, a consumer-defined one-method observer attached through the context every gitx call already receives, called at every exec site in internal/gitx. What this feature adds is threading an observer through every verb and the universal forbidden-token witness; readiness-recovery's recovery check stays one consumer of the same seam. Every git execution goes through internal/gitx, so the command log is complete: the three read-only git calls made outside it at this revision's base move behind read-only gitx functions, which mutate nothing and so are not new git primitives (co-1).", anchor: dc-5 }
  - { id: dc-6, text: "A verb is every entry point that can mutate a repository: a CLI verb, a workbench action, or an MCP tool, because several rituals are reachable only off the command line (the board's stub, create, revise, Commit and push, and switch actions; spec import through the workbench and MCP; managed worktrees behind the board's branch routes). A verb mutates a git repository when it changes a ref, HEAD, the index, a linked worktree, or a remote; a verb that only writes files in the working tree does not, and neither does a verb whose only git writes are unreferenced objects.", anchor: dc-6 }
  - { id: dc-7, text: "The grammar gains ref deletion and linked worktrees because real rituals do both (close's failure unwind, gc, reclaim, and recover delete refs; managed worktrees, execution workspaces, the impact review, gc, and reclaim add or remove linked worktrees), and a witness that fails on every such ritual with no way to declare it would force either silent exemptions or a widened reading. The fields are read this way: a commit on the checked-out branch moves that branch; untracked files inside a declared stage path belong to that path, and untracked-may-enter governs only files outside the declared paths; the upstream configuration written by a push belongs to may-push; a worktree's administrative entry under the repository's git directory belongs to that worktree's add or remove, including the execution-workspace reconciler's direct writes; and effects inside a linked worktree the ritual itself added belong to that worktree.", anchor: dc-7 }
  - { id: dc-8, text: "The rituals that create a design branch without switching to it (design start --from-stub, the board's stub, create and revise actions, and spec import) do so today with a create-only git update-ref, the one forbidden token any verb emits. They conform by creating the branch with git branch at the commit, which is also create-only and fails when the branch exists (owner decision 2026-09-30). Changing the command inside the existing gitx function is a fix to the ritual, which co-1 prescribes, not a new git primitive, and ac-3's token list stays whole.", anchor: dc-8 }
  - { id: dc-9, text: "The forbidden tokens are one shared list consumed by both witnesses. ac-3's six tokens are a floor: the list recovery already enforces also carries -f (ledger SI-224), and the universal witness uses the same list, moved out of internal/recovery into a shared package without changing recovery's semantics (co-4). This feature's witness runs at test time over every verb's command log; the runtime refusal stays recovery's own.", anchor: dc-9 }
  - { id: dc-10, text: "A verb runs its own context, so an observer attached in a test reaches only the verb's in-process core, while the testing rules require CLI behavior to be proven by driving the built binary. The binary therefore records its git command log to the file named by one test-only environment variable, VERDI_GITLOG, which generalizes recovery's VERDI_RECOVERY_GITLOG; when it is unset nothing is recorded.", anchor: dc-10 }
  - { id: dc-11, text: "The board's Commit and push is the one declared carried ritual (owner decision 2026-09-30). Unlike the rituals UAT-036 names, it does not sweep the operator's work into a commit the operator did not author: its purpose is to commit and push the operator's whole working tree, and spec/wall-changes shows every uncommitted change before it runs (ledger SI-299). The other rituals found carrying foreign entries (accept diagram, design start --supersedes, and constitution propose) are fixed to scoped commits instead, and constitution propose also stops cutting from HEAD (owner decision 2026-09-30).", anchor: dc-11 }
stubs:
  - { slug: gitx-recorder-seam, acceptance_criteria: [ac-3] }
  - { slug: write-scope-registry, acceptance_criteria: [ac-1] }
  - { slug: ritual-effect-witness, acceptance_criteria: [ac-2, ac-4] }
links:
  - { type: depends-on, ref: "spec/readiness-recovery-v2" }
  - { type: supersedes, ref: "spec/ritual-write-scope-v2" }
supersession:
  carried: [ac-3, co-1, co-2, co-3, co-4, dc-1, dc-2, dc-4]
  amended:
    - { id: ac-1, note: "the grammar gains ref deletion and linked worktrees, every exported gitx function is classified mutating or read-only, and the static witness fires on reaching a mutating one rather than on any gitx call; census of 2026-09-30 (14 entry groups beyond the spike's five); dc-6, dc-7" }
    - { id: ac-2, note: "the sensor also observes HEAD and the repository's linked worktrees; dc-7" }
    - { id: ac-4, note: "UAT-036's pin extends to design start --supersedes, accept diagram, and constitution propose; the board's Commit and push is the one declared carried ritual; constitution propose joins build start in cutting from the resolved default branch; owner decisions 2026-09-30; dc-11" }
    - { id: dc-3, note: "the enum is unchanged; its last clause names the board's Commit and push as the one ritual that may declare carried, instead of saying none may (owner decision 2026-09-30; dc-11)" }
    - { id: dc-5, note: "the seam already exists (readiness-recovery wave 3, ledger SI-219); this feature consumes it" }
  amended_advisory: []
  removed: []
  added: [dc-6, dc-7, dc-8, dc-9, dc-10, dc-11]
---

# Declared write scope for every ritual that touches git (v3)

## Problem

Grouped by theme, one class dominates the product findings log: rituals
that touch git state the operator did not ask them to touch. This revision
restates the table at main `a303a71f`, with each value taken from source
and from seeded fixture runs (census of 2026-09-30).

| Finding | What the ritual does | Status at `a303a71f` |
|---|---|---|
| UAT-021 | `design start` branched from HEAD and switched the primary checkout | fixed (resolved default-branch base, disclosed switch) |
| UAT-023 | `build start` cuts the feature branch from HEAD (`gitx.CheckoutNewBranch`, no explicit base) | open; `constitution propose` cuts from HEAD the same way |
| UAT-031 | Branch-cutting checks name collisions against the wrong tree | fixed for `design start`, `--supersedes`, stub instantiation and the board; open for `build start` |
| UAT-033 | `design start` committed every untracked file in the checkout | fixed (`gitx.AddPaths`, scoped to the spec directory) |
| UAT-034 | The commit-to-design ritual swept the working tree the same way | fixed (`gitx.AddPaths`, scoped) |
| UAT-036 | Ritual commits carry pre-staged index entries | open for `design start`, commit-to-design, `design start --supersedes`, `accept diagram` and `constitution propose`; `close` refuses to begin (exit 2); `policy adopt` and stub instantiation are scoped |
| UAT-019 | Adjacent: the commit dialog accepts any message, so a proposal commit can claim acceptance | fixed as a dialog aid |

Every other finding class costs confusion or rework. This one costs the
operator's own work.

## Outcome

A ritual's effect on the repository becomes a declared, checked contract.

## What changed from v2

v2 was accepted with its three stories unwritten. Before writing them, a
census of every path that mutates a repository (2026-09-30, at main
`a303a71f`, from a call graph of `cmd/verdi` confirmed by source reads and
seeded runs) found what v2 could not have seen from the spike's five
rituals:

- **Nineteen entry groups, not five.** Twenty of gitx's 74 exported
  functions mutate. Fourteen entry groups reach them beyond the spike's
  five, several only from the workbench or MCP (dc-6).
- **Effects the grammar could not say.** Close's failure unwind, `gc`,
  reclaim and `recover` delete refs; five rituals add or remove linked
  worktrees; the execution-workspace reconciler writes worktree
  administrative entries directly (dc-7).
- **The seam already exists.** Readiness-recovery's wave 3 landed
  `gitx.Observer` first (SI-219), so dc-5 now consumes it.
- **update-ref is the one forbidden token any verb emits**, from the
  create-only design-branch creation (dc-8).
- **More carried rituals.** `design start --supersedes`, `accept diagram`,
  `constitution propose` and the board's Commit and push all commit what is
  pre-staged (dc-11).
- **The two values v2 left unmeasured are measured.** Close's CI publish
  path moves no existing ref: it creates `close/<name>` from HEAD, exactly
  as `--force-local` does (in-process run with fakes). Commit-to-design has
  a CLI verb after all (`verdi board commit`): run from a seeded clone, it
  carried the foreign staged entry, and it advanced the checked-out branch,
  where the spike had declared no ref move.

The owner decided on 2026-09-30: revise the feature before writing its
stories; fix design-branch creation rather than exempt update-ref; keep the
board's Commit and push as the one declared carried ritual; and fix the
other carried rituals. v2's body cited ledger SI-207 for the index-carry
enum; the row is SI-216 (renumbered on 2026-09-21).

## Context from the process audit

- PA-024 is the entry. It exists because the findings were clustered rather
  than read one at a time; the entry asks that the product log be
  re-clustered whenever the ledger is revisited.
- The first token check was `internal/gitx/ffonly_test.go`, a unit test
  over the fast-forward path's calls. `recover` now enforces its own token
  list at run time over the seam (exit 2 on a forbidden token).
- PA-015 and spec/self-governance: the registry is product behaviour in
  source, not governance data an operator adopts (dc-4); self-governance
  may cite it as an analyzer-rung witness but does not own it.

## ac-1

The registry is the contract; the static witness makes a missing
declaration impossible to ship, and a closed grammar makes an unknown field
or value fail closed like every other artifact here. The index-carry slot
is four-valued (dc-3, ledger SI-216). The witness fires on reaching a
mutating gitx function, not on any gitx call: 29 of 31 CLI dispatchers call
gitx somewhere, most only to read, and classifying gitx's exported
functions once is cheaper and more exact than declaring every reader.

## ac-2

The seeded fixture is the whole point: an untracked file, an unrelated
staged entry, and a dirty tracked file are exactly what the findings swept
up. That fixture alone is vacuous for a refusing ritual: `close` exits
before its branch cut, staging and commit, so a later out-of-scope
`git add` on its success path would never be observed. The second state
(clean index, unrelated unstaged and untracked work) makes every ritual run
to completion under the same three sensors, and the witness fails if it
does not. The observed state now includes HEAD and the linked-worktree
list, because a worktree's administrative entry changes neither refs nor
the index. Unattributable effects are unproven, never within scope.

## ac-3

One seam, two consumers. The token list is shared (dc-9); the one token a
verb emits today goes away by fixing its ritual (dc-8).

## ac-4

The findings become pins, so the class stays closed. Five rituals owe the
UAT-036 fix; `close` owes the opposite pin, that its refusal keeps
working. `build start` and `constitution propose` owe the UAT-021 shape of
fix (an explicit resolved base), and `build start` its half of UAT-031.
The board's Commit and push is declared, not exempted: the witness still
checks it against its declaration (dc-11).

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
never reached) and under-reported `design start`. Every ritual also writes
its artifacts with ordinary file I/O before it calls git, which no command
log can explain.

## dc-3

Grammar by construction, from five real rituals. Amended in v3 only in its
last clause: `carried` was to be declared by no shipped ritual, and now
exactly one declares it (dc-11). The precedence rule
answers the case the spike left undefined, a ritual that both guards and
scopes. `scoped` is defined by the commit's recorded delta, not by the
presence of a `--` pathspec: stub instantiation builds its commit from a
scratch tree and never touches the caller's index, and that is scoped too.

## dc-4

Home of the declaration. Design specs 00 through 05 contain no sentence
about verb git-effects, so there is nothing to ratify.

## dc-5

The seam landed with readiness-recovery (`internal/gitx/observer.go`),
counted against every exec site by a structural test. Three read-only git
calls outside gitx (the local actor's top-level lookup, the draft
identity's, and the disclosure cache key's reads) do not pass through it
today.

## dc-6

The census's entry groups, by surface: CLI verbs (`accept`, `board`,
`build`/`feature`, `close`, `context`, `design`, `experiment`, `gc`,
`policy`, `recover`, `serve`); workbench actions (stub, create, revise,
Commit and push, switch, commit-to-design, spec import, the branch routes'
managed worktrees); MCP tools (`import_apply`,
`constitution_impact_review`, `experiment`). Verbs such as `sync`,
`harness`, `journey`, `lint` and `init` reach no git mutation.

## dc-7

Each reading keeps a field's meaning closed while covering what real
rituals do; none widens what an existing ritual may do.

## dc-8

`git branch <name> <commit>` refuses an existing branch exactly as the
three-argument update-ref with a zero old value does, and design branch
names already satisfy git's branch-name rules.

## dc-9

Runtime refusal in every verb would trip on operator-supplied text, such
as a commit message that is exactly `reset`; the test-time witness drives
fixed inputs.

## dc-10

`VERDI_RECOVERY_GITLOG` was a plan ruling with no ledger row; the general
hook replaces it.

## dc-11

Scoping the board's commit would reopen spec/wall-changes, whose change
popover is built on the commit taking everything; refusing foreign entries
would make the button fail on the working state it exists to publish.
