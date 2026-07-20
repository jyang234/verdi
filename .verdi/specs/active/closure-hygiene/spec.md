---
id: spec/closure-hygiene
kind: spec
title: "Closure Hygiene"
owners: [platform-team]
class: story
status: draft
story: jira:REPLACE-ME
problem: { text: "verdi audit (R4-I-10) checks exactly two things — ADR exemption thresholds and per-story spec-stale deviation counts (internal/decisionsweep) — and nothing else. It has no visibility into whether a spec's declared status still matches git reality, or into the closure-ritual branches (close/<name>) that verdi close (spec/close-verb dc-3) cuts and then stops at, leaving the human to push and merge. On this repository's own main today: close/showcase-corpus-renovation's tip already moved spec/showcase-corpus-renovation to archive/, but the branch never merged, so the spec still reads accepted-pending-build in specs/active/ — nothing detects that the ritual ran and never landed. Four more close/<name> branches (attest-helper, close-preflight, disposition-verb, home-status-glance) sit unmerged though archive/<name> already exists on main for each — dead leftovers indistinguishable from a genuinely stranded ritual without checking the branch's own tip tree. spec/code-health is accepted-pending-build though every stub story it declared is already closed and merged — stub reconciliation would likely pass, but nothing surfaces that. And workspace-wide, as of 2026-07-19, 153 of 169 non-default local branches are fully merged into main and never deleted, across 30 registered git worktrees, most sitting on long-archived work — verdi gc (spec/worktree-manager) reclaims managed worktrees under .verdi/data/worktrees/ only, so none of this is even visible, let alone reclaimable.", anchor: problem }
outcome: { text: "verdi audit gains a third report section, additive to its existing exemption and spec-stale audits, that names every git-reality-versus-spec-status contradiction, every stranded close/<name> branch, and a read-only survey of merged-but-undeleted branches and worktrees — each finding a concrete witness (branch, tip commit, the exact contradiction), never a guess where git state cannot decide. A stranded closure ritual or a stub-complete unclosed feature flags the run (exit 1, an actionable defect); superseded-elsewhere branches and the merged-residue survey are reported but never flip the exit code, since they are ordinary git housekeeping, not defects. No reclamation of any kind is performed — this story implements spec/assurance-integrity's ac-2 only.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "verdi audit reports every active-zone spec whose declared status contradicts git reality, in two named patterns: (a) a stranded closure ritual — an active-zone spec status: accepted-pending-build named <name> for which a close/<name> branch exists, is unmerged into the default branch, and whose tip tree already contains archive/<name> (witness: spec/showcase-corpus-renovation, close/showcase-corpus-renovation); (b) a stub-complete unclosed feature — a class: feature spec status: accepted-pending-build whose every declared stubs[] slug is realized by a closed, merged story, yet the feature itself has not closed (witness: spec/code-health and its forge-transport/shared-homes/fail-loud/file-topics stubs). status: superseded specs are explicitly NOT checked (dc-2): remaining in specs/active/ under status: superseded is correct, permanent behavior (02 §Kind registry), never a finding. Pattern (a) findings flag the run (exit 1); where a default branch cannot be resolved, nothing is asserted for either pattern", evidence: [static, behavioral], anchor: ac-1 }
  - { id: ac-2, text: "verdi audit reports every close/<name> local branch unmerged into the default branch, classified by whether its own tip tree's archive/<name> is already present at the audited ref: ritual-incomplete (absent — cross-references ac-1 pattern (a), flags the run, exit 1) or superseded-elsewhere (present — the branch is redundant leftover, reported only, never flags; witness: close/attest-helper, close/close-preflight, close/disposition-verb, close/home-status-glance, each already archived on main through a different commit history)", evidence: [static, behavioral], anchor: ac-2 }
  - { id: ac-3, text: "verdi audit reports a read-only survey — never flagging, never mutating — of (a) every local branch whose tip is an ancestor of the default branch tip, counted and named, and (b) every git worktree registered against the repository (git worktree list, not limited to managed worktrees under .verdi/data/worktrees/), excluding the primary checkout, each named with its branch (or, for a detached HEAD, its commit) and whether that branch/commit is merged, clean, and managed or unmanaged — disclosed rather than guessed when a worktree's branch state cannot be resolved (e.g. detached HEAD merge status)", evidence: [static, behavioral], anchor: ac-3 }
links:
  - { type: implements, ref: "spec/assurance-integrity#ac-2" }
decisions:
  - { id: dc-1, text: "surface: a third section on the existing verdi audit verb (== Closure hygiene audit ==, alongside == Exemption audit == and == Spec-stale audit ==), not a new verb — audit already owns \"detect and report, never mutate\" (R4-I-10), and a new internal/residue package (single responsibility, CLAUDE.md: one package = one concern) implements the scan, consumed by cmd/verdi/audit.go directly. internal/decisionsweep's own scope stays exactly 03 §Exemption audit plus spec-stale (its doc.go's own words) — this story adds a sibling package and a sibling call in cmd/verdi/audit.go's runAudit, never folding new logic into decisionsweep", anchor: dc-1 }
  - { id: dc-2, text: "status: superseded is explicitly OUT of scope for ac-1/ac-2 — not a disclosed-unproven case, a deliberate non-target, correcting an initial framing of this residue. 02 §Kind registry's own table lists a feature/story spec's terminal statuses as closed(archive) | superseded — two PARALLEL outcomes, only one of which archives; internal/store.ArchiveMove's sole call site in this codebase is close.go's accepted-pending-build to closed flip, never the accepted-pending-build to superseded flip cmd/verdi/accept.go performs, and internal/artifact/spec.go's validateComponent doc comment states it explicitly for the sibling component class: \"superseded ... stay in specs/active/ rather than moving/freezing\" — the same posture applies to feature/story class by the Kind registry table's own parallel-branch reading. spec/disclosure-seam sitting in specs/active/ with status: superseded on this repository's own main, right now, is this reading's live witness, not a counterexample to explain away", anchor: dc-2 }
  - { id: dc-3, text: "exit-code posture: ac-1's pattern (a) and ac-2's ritual-incomplete classification flag the run (exit 1) — both name a spec whose active-zone status is actually wrong about what happened, an operator-actionable defect, matching audit's existing exit-1 conditions (a newly-filed ADR conflict, a spec-stale story). ac-1's pattern (b), ac-2's superseded-elsewhere classification, and the whole of ac-3's survey never flag: a stub-complete-but-unclosed feature and a redundant closed-elsewhere branch are suggestive, not provably wrong (closure eligibility needs the real closure-gate fold, which this story does not run — reclamation is out of the parent feature (spec/assurance-integrity co-2; ledger R4-I-66) and happens only \"once licensed\", not asserted as fact here), and merged-but-undeleted branches/worktrees are ordinary git housekeeping this story exists to make VISIBLE, not to declare a failure state over", anchor: dc-3 }
  - { id: dc-4, text: "worktree enumeration: a new gitx.WorktreeList(ctx, dir) primitive (git worktree list --porcelain, parsed) — no such primitive exists today (gitx currently offers only WorktreeAdd/WorktreeRemove, verified by grep), and git's own worktree metadata already sees every worktree ever registered against this repository regardless of where it physically lives, so no sibling-directory convention (verdi-wt/ or otherwise) is hardcoded anywhere. Managed-vs-unmanaged classification reuses wtmanager's own definition of the managed root — internal/wtmanager.worktreesRoot (naming.go) is exported as WorktreesRoot (a small additive change, CLAUDE.md: anything used by two or more packages lives in a shared internal/ package) rather than a second hardcoded .verdi/data/worktrees/ literal. The primary checkout is excluded via git worktree list --porcelain's own first-entry-is-primary ordering, cross-checked by that entry's .git being a directory rather than a linked-worktree .git file (this story's own worktree, verified during authoring: a plain file) — two independent signals, not one assumed convention", anchor: dc-4 }
  - { id: dc-5, text: "reclamation (out of the parent feature, spec/assurance-integrity co-2) is NOT built by this story and is not licensed yet. verdi-store-layout's Garbage collection section (a ratified, status: active component spec) reads in full: \"verdi gc (local hygiene verb, also runnable in CI images): prunes derived/<ref>/ for refs merged or deleted more than derived.retention_days ago ... ; prunes cache entries whose layout version or tree hash no longer matches; never touches the committed zone or mutable/.\" Every clause is scoped inside .verdi/data/ (derived/, cache) or explicitly disclaims scope inside the committed zone/mutable/ — nothing in it reaches outside the checkout at all. spec/worktree-manager's own managed-worktree extension stayed inside that same boundary (.verdi/data/worktrees/, its own co-1) and argued compatibility explicitly (its dc-3/dc-5). Unmanaged verdi-wt/ worktrees are sibling directories OUTSIDE this git repository entirely, and local branches are git-ref state, never a store path — categorically outside verdi-store-layout's own purpose statement (01 §Purpose: \"a directory tree in the monorepo plus a per-checkout working area\"). This is a gap, not a license: unlike a mere implementation-detail ambiguity (the invention-ledger's usual smallest-reversible-option territory), reclaiming branches and worktrees is destructive (unlike disposable derived/cache) and reaches outside the store's own documented boundary — this story records, per the dispatching task's own instruction, that reclamation needs a ratification-flow amendment to verdi-store-layout's Garbage collection section before it can be built, rather than silently expanding ratified scope through a story-level decision alone (R4-I-66). Candidate amendment language, offered for the owner's convenience, not self-ratified here: a fourth bullet reading \"optionally, on explicit opt-in, prunes a LOCAL branch and its worktree (if any) when the branch is fully merged into the default branch, its worktree carries no uncommitted changes, and the worktree is not the primary checkout — reads never delete without that opt-in; every run names verbatim what it did and did not touch.\" The full conservative shape such a future story would build, recorded here so the analysis is not redone: reclaim-eligible requires ALL of (i) branch fully merged into the default branch (gitx.IsAncestor, the same primitive dc-4/wtmanager already use), (ii) worktree clean (gitx.StatusDirty) where a worktree exists at all, (iii) not the primary checkout (dc-4's two-signal exclusion), and (iv) explicit opt-in (a flag or an interactive confirmation — the future story's own design decision); dry-run-by-default is worth that story's serious consideration; every run prints, verbatim, what was reclaimed and what was kept and why, mirroring spec/worktree-manager's own dc-4 one-line-per-worktree idiom and verdi gc's existing dc-5 scope-disclosure line; a dirty or currently-primary worktree is disclosed and kept, never forced; nothing unmerged is ever touched, full stop", anchor: dc-5 }
constraints:
  - { id: co-1, text: "no network in any test (CLAUDE.md): every status-contradiction, stranded-branch, and merged-residue check is proven against fixturegit repositories with real local branches, real close/<name> archival commits, and real git worktrees materialized on local disk — mirroring spec/worktree-manager's own co-2 precedent exactly", anchor: co-1 }
  - { id: co-2, text: "additive only: verdi audit's existing == Exemption audit == and == Spec-stale audit == sections, their own logic (internal/decisionsweep, untouched), and their existing exit-code contributions are byte-for-byte unchanged by this story — the new == Closure hygiene audit == section is a third, independent pass appended to the same run, never a rewrite of the first two", anchor: co-2 }
---

# Closure Hygiene

## Problem

`verdi audit` (R4-I-10) checks exactly two things — ADR exemption
thresholds and per-story spec-stale deviation counts, both computed by
`internal/decisionsweep` — and nothing else. It has no visibility into
whether a spec's declared status still matches git reality, and no
visibility into the closure-ritual branches `verdi close` (spec/close-verb
dc-3) cuts and then stops at, leaving the human to push and open the MR.

On this repository's own main, today:

`close/showcase-corpus-renovation`'s tip already moved
`spec/showcase-corpus-renovation` to `archive/` — but the branch never
merged, so the spec still reads `accepted-pending-build` in
`specs/active/`. Nothing detects that the ritual ran and never landed.

Four more `close/<name>` branches — `attest-helper`, `close-preflight`,
`disposition-verb`, `home-status-glance` — sit unmerged though
`archive/<name>` already exists on main for every one of them: dead
leftovers, structurally indistinguishable from a genuinely stranded ritual
without checking the branch's own tip tree against the archive that is
already, separately, on main.

`spec/code-health` is `accepted-pending-build` though every stub story it
declared at scaffold time is already closed and merged
(`forge-transport`, `shared-homes`, `fail-loud`, `file-topics`). Stub
reconciliation would likely pass; nothing surfaces that this feature may be
ready to close.

And workspace-wide, as of 2026-07-19, 153 of 169 non-default local branches
are fully merged into main and were never deleted, spread across 30
registered git worktrees — most sitting on design/feature/close branch
trios for specs long since archived.
`verdi gc` (spec/worktree-manager) reclaims managed worktrees under
`.verdi/data/worktrees/` only, so none of this is even visible, let alone
reclaimable.

## Outcome

`verdi audit` gains a third report section, additive to its existing
exemption and spec-stale audits, that names every git-reality-versus-
spec-status contradiction, every stranded `close/<name>` branch, and a
read-only survey of merged-but-undeleted branches and worktrees — each
finding a concrete witness (branch, tip commit, the exact contradiction),
never a guess where git state cannot decide.

A stranded closure ritual or a stub-complete unclosed feature flags the run
(exit 1, an actionable defect); superseded-elsewhere branches and the
merged-residue survey are reported but never flip the exit code, since
they are ordinary git housekeeping, not defects.

No reclamation of any kind is performed by this story. It implements
`spec/assurance-integrity`'s AC-2 only; reclamation is deliberately out of
scope (that feature's CO-2) — see DC-5.

## AC-1

`verdi audit` reports every active-zone spec whose declared status
contradicts git reality, in two named patterns:

**(a) Stranded closure ritual.** An active-zone spec `status:
accepted-pending-build` named `<name>`, for which a `close/<name>` branch
exists, is unmerged into the default branch, and whose tip tree already
contains `archive/<name>`. Witness: `spec/showcase-corpus-renovation` and
`close/showcase-corpus-renovation` (tip `24214fd`, "close: archive
spec/showcase-corpus-renovation (jira:VERDI-22)").

**(b) Stub-complete unclosed feature.** A `class: feature` spec `status:
accepted-pending-build` whose every declared `stubs[]` slug is realized by
a closed, merged story, yet the feature itself has not closed. Witness:
`spec/code-health` and its `forge-transport`/`shared-homes`/`fail-loud`/
`file-topics` stubs, every one archived and merged on main today.

`status: superseded` specs are explicitly NOT checked by either pattern
(DC-2): remaining in `specs/active/` under `status: superseded` is correct,
permanent behavior, never a finding.

Pattern (a) findings flag the run (exit 1) — a spec's active-zone status is
provably wrong about what git already shows happened. Pattern (b) findings
do not flag (DC-3). Where a default branch cannot be resolved, nothing is
asserted for either pattern.

Evidence: static (the two-pattern detection is a total function over
{status, matching close/<name> tip contents, stub-realization set} with no
silent third path) and behavioral — a RED-direction fixturegit repository
constructing exactly pattern (a) (an active-zone `accepted-pending-build`
spec plus a `close/<name>` branch whose tip already archives it,
unmerged) asserts the exact witness line appears and the run exits 1; a
second RED-direction fixture constructs pattern (b) (a feature with every
stub closed-and-merged, itself still `accepted-pending-build`) and asserts
its own witness line, without flagging exit 1 (DC-3); a GREEN-direction
fixture — every active-zone spec's status consistent with git reality,
including one `status: superseded` spec — asserts neither pattern fires
and the section reports clean.

## AC-2

`verdi audit` reports every `close/<name>` local branch unmerged into the
default branch, classified by whether its own tip tree's `archive/<name>`
is already present at the audited ref:

- **ritual-incomplete** (absent) — cross-references AC-1 pattern (a);
  flags the run, exit 1.
- **superseded-elsewhere** (present) — the branch is redundant leftover;
  reported only, never flags. Witness: `close/attest-helper`,
  `close/close-preflight`, `close/disposition-verb`,
  `close/home-status-glance`, each already archived on main through a
  different commit history than its own `close/<name>` branch.

Evidence: static (classification is a total two-outcome function over
{branch merged?, archive/<name> present at the audited ref?}, restricted
to unmerged `close/<name>` branches, no silent third path) and behavioral
— a RED-direction fixturegit repository with one unmerged `close/<name>`
branch whose tip does NOT yet carry the archive move (ritual-incomplete)
and a second unmerged `close/<name>` branch whose archive move is ALSO,
separately, already on the default branch (superseded-elsewhere) asserts
both witness lines appear, with only the first contributing to exit 1; a
GREEN-direction fixture with no unmerged `close/<name>` branches at all
asserts the section reports clean.

## AC-3

`verdi audit` reports a read-only survey — never flagging, never
mutating — of:

**(a) Merged branches.** Every local branch whose tip is an ancestor of
the default branch tip (`gitx.IsAncestor`), counted and named. Witness:
153 of this repository's own 169 non-default local branches, as of 2026-07-19.

**(b) Worktrees.** Every git worktree registered against the repository
(`git worktree list`, not limited to managed worktrees under
`.verdi/data/worktrees/`), excluding the primary checkout, each named with
its branch (or, for a detached HEAD, its commit) and whether that
branch/commit is merged, whether the worktree is clean, and whether it is
managed or unmanaged (DC-4). Witnesses drawn from this repository's own
`verdi-wt/` orchestration worktrees, as of 2026-07-19: the worktree
directory `feature-close` (branch `close/operating-model`, merged) is
reclaim-candidate-shaped, as are `wave-a` (branch
`fix/final-wave-kernel-guards`) and `wave-b` (branch
`fix/final-wave-prose-witness`), whose PRs #167 and #166 both merged into
`main` on 2026-07-19; the worktree directories `assess` (branch
`feature-close`, unmerged) and `wave-c` (branch
`fix/final-wave-ritual-guards`, tip `102f392`, unmerged) are named but
never flagged, since their branches are not merged; `w6-exit` (detached
HEAD at `1d6359c`, a commit that IS an ancestor of the default branch, no
branch name at all) is disclosed with its commit-level merge state and no
branch to report, rather than guessed at or silently skipped.

Where a worktree's branch state cannot be resolved (for instance, a
detached HEAD's merge state, which is checked at the commit level, not
guessed as a branch-level property it does not have), that is disclosed,
never asserted either way.

Evidence: static (the survey performs zero git-mutating calls — an
inventory, verified by an exhaustive command-surface check of the new
code path) and behavioral — a fixturegit repository with a mix of merged
and unmerged local branches, a managed worktree, an unmanaged worktree on
a merged branch, an unmanaged worktree on an unmerged branch, and a
detached-HEAD worktree asserts every one is named with its correct
classification and that the run's exit code is unaffected by any of them.

## DC-1

Surface: a third section on the existing `verdi audit` verb (`== Closure
hygiene audit ==`, alongside `== Exemption audit ==` and `== Spec-stale
audit ==`), not a new verb — `audit` already owns "detect and report,
never mutate" (R4-I-10), and a new `internal/residue` package (single
responsibility, CLAUDE.md: one package = one concern) implements the scan,
consumed by `cmd/verdi/audit.go` directly.

`internal/decisionsweep`'s own scope stays exactly 03 §Exemption audit
plus spec-stale (its own `doc.go`'s words) — this story adds a sibling
package and a sibling call in `cmd/verdi/audit.go`'s `runAudit`, never
folding new logic into `decisionsweep`.

## DC-2

`status: superseded` is explicitly OUT of scope for AC-1/AC-2 — not a
disclosed-unproven case, a deliberate non-target, correcting an initial
framing of this residue.

02 §Kind registry's own table lists a feature/story spec's terminal
statuses as `closed(archive) | superseded` — two PARALLEL outcomes, only
one of which archives. `internal/store.ArchiveMove`'s sole call site in
this codebase is `close.go`'s `accepted-pending-build` → `closed` flip,
never the `accepted-pending-build` → `superseded` flip `cmd/verdi/accept.go`
performs, and `internal/artifact/spec.go`'s `validateComponent` doc comment
states it explicitly for the sibling component class: "superseded ... stay
in `specs/active/` rather than moving/freezing" — the same posture applies
to the feature/story class by the Kind registry table's own
parallel-branch reading, not by extension or analogy alone.

`spec/disclosure-seam` sitting in `specs/active/` with `status: superseded`
on this repository's own main, right now, is this reading's live witness,
not a counterexample to explain away.

## DC-3

Exit-code posture: AC-1's pattern (a) and AC-2's ritual-incomplete
classification flag the run (exit 1) — both name a spec whose active-zone
status is actually wrong about what happened, an operator-actionable
defect, matching `audit`'s existing exit-1 conditions (a newly-filed ADR
conflict, a spec-stale story).

AC-1's pattern (b), AC-2's superseded-elsewhere classification, and the
whole of AC-3's survey never flag: a stub-complete-but-unclosed feature and
a redundant closed-elsewhere branch are suggestive, not provably wrong —
closure eligibility needs the real closure-gate fold, which this story
does not run (reclamation is out of the parent feature —
`spec/assurance-integrity`'s CO-2 — and licensed only once R4-I-66's
ratification question resolves, not asserted as fact here) — and
merged-but-undeleted
branches/worktrees are ordinary git housekeeping this story exists to make
VISIBLE, not to declare a failure state over.

## DC-4

Worktree enumeration: a new `gitx.WorktreeList(ctx, dir)` primitive
(`git worktree list --porcelain`, parsed) — no such primitive exists today
(`gitx` currently offers only `WorktreeAdd`/`WorktreeRemove`, verified by
grep), and git's own worktree metadata already sees every worktree ever
registered against this repository regardless of where it physically
lives, so no sibling-directory convention (`verdi-wt/` or otherwise) is
hardcoded anywhere.

Managed-vs-unmanaged classification reuses `wtmanager`'s own definition of
the managed root — `internal/wtmanager.worktreesRoot` (`naming.go`) is
exported as `WorktreesRoot` (a small additive change; CLAUDE.md: anything
used by two or more packages lives in a shared `internal/` package) rather
than a second hardcoded `.verdi/data/worktrees/` literal.

The primary checkout is excluded via `git worktree list --porcelain`'s own
first-entry-is-primary ordering, cross-checked by that entry's `.git` being
a directory rather than a linked-worktree `.git` file (this story's own
worktree, verified during authoring: a plain file) — two independent
signals, not one assumed convention.

## DC-5

Reclamation (out of the parent feature, `spec/assurance-integrity`'s CO-2)
is NOT built by this story and is not licensed yet.

`verdi-store-layout`'s Garbage collection section (a ratified, `status:
active` component spec) reads in full:

> `verdi gc` (local hygiene verb, also runnable in CI images):
> - prunes `derived/<ref>/` for refs merged or deleted more than
>   `derived.retention_days` ago ...;
> - prunes cache entries whose layout version or tree hash no longer
>   matches;
> - never touches the committed zone or `mutable/`.

Every clause is scoped inside `.verdi/data/` (`derived/`, cache) or
explicitly disclaims scope inside the committed zone/`mutable/` — nothing
in it reaches outside the checkout at all. `spec/worktree-manager`'s own
managed-worktree extension stayed inside that same boundary
(`.verdi/data/worktrees/`, its own co-1) and argued compatibility
explicitly (its DC-3/DC-5).

Unmanaged `verdi-wt/` worktrees are sibling directories OUTSIDE this git
repository entirely, and local branches are git-ref state, never a store
path — categorically outside `verdi-store-layout`'s own purpose statement
(01 §Purpose: "a directory tree in the monorepo plus a per-checkout
working area").

This is a gap, not a license. Unlike a mere implementation-detail
ambiguity (the invention-ledger's usual smallest-reversible-option
territory), reclaiming branches and worktrees is destructive (unlike
disposable derived/cache) and reaches outside the store's own documented
boundary. This story records, per the dispatching task's own instruction,
that reclamation needs a **ratification-flow amendment** to
`verdi-store-layout`'s Garbage collection section before it can be built,
rather than silently expanding ratified scope through a story-level
decision alone (**R4-I-66**).

**Candidate amendment language**, offered for the owner's convenience, not
self-ratified here:

> optionally, on explicit opt-in, prunes a LOCAL branch and its worktree
> (if any) when the branch is fully merged into the default branch, its
> worktree carries no uncommitted changes, and the worktree is not the
> primary checkout — reads never delete without that opt-in; every run
> names verbatim what it did and did not touch.

The full conservative shape such a future story would build, recorded
here so the analysis is not redone: reclaim-eligible requires ALL of
(i) branch fully merged into the default branch (`gitx.IsAncestor`, the
same primitive DC-4/`wtmanager` already use), (ii) worktree clean
(`gitx.StatusDirty`) where a worktree exists at all, (iii) not the primary
checkout (DC-4's two-signal exclusion), and (iv) explicit opt-in (a flag
or an interactive confirmation — the future story's own design decision);
dry-run-by-default is worth that story's serious consideration; every run
prints, verbatim, what was reclaimed and what was kept and why, mirroring
`spec/worktree-manager`'s own DC-4 one-line-per-worktree idiom and `verdi
gc`'s existing DC-5 scope-disclosure line; a dirty or currently-primary
worktree is disclosed and kept, never forced; nothing unmerged is ever
touched, full stop.

## CO-1

No network in any test (CLAUDE.md). Every status-contradiction,
stranded-branch, and merged-residue check is proven against `fixturegit`
repositories with real local branches, real `close/<name>` archival
commits, and real git worktrees materialized on local disk — mirroring
`spec/worktree-manager`'s own co-2 precedent exactly.

## CO-2

Additive only: `verdi audit`'s existing `== Exemption audit ==` and
`== Spec-stale audit ==` sections, their own logic (`internal/decisionsweep`,
untouched), and their existing exit-code contributions are byte-for-byte
unchanged by this story — the new `== Closure hygiene audit ==` section is
a third, independent pass appended to the same run, never a rewrite of the
first two.
