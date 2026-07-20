---
id: spec/gc-reclaim
kind: spec
title: "GC Reclaim"
owners: [platform-team]
class: story
status: draft
story: jira:VERDI-36
problem: { text: "spec/residue-reclamation's own ac-1 needs an implementation: verdi-store-layout's Garbage collection section now licenses, on explicit opt-in, pruning a LOCAL branch and its worktree (if any) when the branch is fully merged into the default branch, its worktree carries no uncommitted changes, and the worktree is not the primary checkout (ledger R4-I-79) — but verdi gc itself only ever reclaims managed worktrees under .verdi/data/worktrees/ (spec/worktree-manager); nothing in the binary acts on the newly-ratified unmanaged slice, and nothing computes which unmanaged branches or worktrees qualify. spec/closure-hygiene's own internal/residue package already computes exactly this survey once, for verdi audit's third report section — every local branch's merge state, every worktree's merge/clean/managed state, disclosed rather than guessed wherever it cannot be resolved — so the eligibility facts this story needs already exist as a single, tested, in-tree computation (internal/residue.Scan); the only thing missing is a consumer that turns eligible rows into a disclosed plan and, on further opt-in, an executed one.", anchor: problem }
outcome: { text: "verdi gc gains --reclaim-unmanaged (prints the plan; touches nothing) and --reclaim-unmanaged --apply (executes it), built as a thin consumer of internal/residue.Scan's own survey rows — eligibility is never re-derived from git independently. Every reclaim-eligible LOCAL branch and its worktree (if any) is named with its tip commit; every kept row is named with one of a closed set of reasons; applying removes the worktree before its branch, each backed by git's own independent refusal as a second guard; a per-item failure is disclosed and the sweep continues; an unresolvable default branch refuses the whole run rather than asserting a plan it cannot compute. verdi gc's existing managed-worktree slice, internal/residue, and every existing verdi audit report section are untouched.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "Given internal/residue.Scan's own survey (never re-derived from git independently), reclaim-eligible is a total, closed predicate over its rows: a row is eligible iff its branch is merged into the default branch (Merged true, MergedUnresolved false), its worktree — where one exists — carries no uncommitted changes (Dirty false, DirtyUnresolved false), it is not managed (verdi gc's existing managed-worktree jurisdiction), it carries a local branch (a detached-HEAD worktree is outside the ratified \"a LOCAL branch and its worktree (if any)\" unit), and it is not the checkout the sweep itself is running from. A row failing any of these is kept and disclosed with exactly one reason drawn from a closed vocabulary — unmerged, dirty, unresolved-state (naming residue's own Reason), detached, managed, or invoking — never a silent omission and never an undocumented second reason string. A branch with no worktree at all (present in residue's merged-branches list, absent from its worktree list) is eligible for branch deletion alone. Witness, as of 2026-07-20 (main/HEAD cda3ec625a276db2cc66aa0603b62c4e8649cd60): close/attest-helper (tip 7495c38a18719cca691b64c80bfa217e2886b3be), close/close-preflight (tip af9848edaac6d57353b86f0f19cdb9ec25931b34), close/disposition-verb (tip d24686552ed25f4e86f169797ce080fc54dfe1c4), and close/home-status-glance (tip 4e3b67a4a46d43f2e1f6ce39c00bb3dea4eba26b) are all closure-hygiene's own superseded-elsewhere witnesses — already redundant, already archived elsewhere — yet every one is UNMERGED, so every one is kept as unmerged: the amendment licenses fully-merged branches only, and being redundant is not the same fact as being merged.", evidence: [static, behavioral], anchor: ac-1 }
  - { id: ac-2, text: "Given --reclaim-unmanaged alone, verdi gc computes and prints the full plan from ac-1's predicate — every eligible item named with its worktree path (if any) and branch, every kept item named with its one reason — and performs no git-mutating call. Given --reclaim-unmanaged --apply, it executes that same plan: per eligible item, its worktree (if any) is removed first (git worktree remove, without --force — git's own refusal on a since-dirtied tree is a second, independent guard beyond the plan's own Dirty check), then its branch is deleted (git branch -d, never -D — git's own refusal on an unmerged or elsewhere-checked-out branch is a second, independent guard beyond the plan's own Merged check); a refusal at either step is disclosed on that item alone and the sweep continues to the next item, including the partial outcome of a worktree successfully removed whose branch delete then failed. Every branch actually deleted prints its pre-delete tip commit. An unresolvable default branch refuses the whole run before computing any plan, dry-run or applied alike, asserting nothing rather than a plan it cannot compute. verdi gc's own exit contract is unchanged: 0 when the run completes, regardless of how many individual items were kept or individually refused; 2 for a whole-run operational failure (an unresolvable default branch, a usage error) — reclamation mints no verdict exit-1.", evidence: [static, behavioral], anchor: ac-2 }
  - { id: ac-3, text: "verdi gc's existing scope-disclosure line grows to name both slices as a closed pair on every invocation: a plain verdi gc run continues to reclaim only managed worktrees and now additionally discloses that unmanaged reclamation is available via --reclaim-unmanaged but was not run this invocation; a --reclaim-unmanaged run (dry-run or applied) discloses that managed-worktree reclamation was not run this invocation, alongside derived-cache and layout/tree-hash-cache pruning, which remain out of scope for either. Every reclaimed and every kept item prints exactly one line, mirroring spec/worktree-manager's own dc-4 one-line-per-worktree idiom — no removal or skip a human running the command cannot see named in its own output. internal/residue (the survey producer) and all three existing verdi audit report sections are byte-for-byte unchanged by this story; this story adds a new consumer package and new verdi gc flags, never new audit behavior.", evidence: [static, behavioral], anchor: ac-3 }
links:
  - { type: implements, ref: "spec/residue-reclamation#ac-1" }
decisions:
  - { id: dc-1, text: "Surface: a new internal/reclaim package (single concern: the sweep, CLAUDE.md's one-package-one-concern rule), consuming *residue.Result directly; cmd/verdi/gc.go stays thin wiring, gaining --reclaim-unmanaged and --apply flags plus the resolved default branch it already computes today (lint.ResolveDefaultBranch, unchanged). internal/reclaim calls zero gitx ELIGIBILITY primitives itself — no IsAncestor, no StatusDirty — mirroring the same one-computation-consumed-downstream precedent internal/residue.Result.Flagged() itself already established relative to internal/decisionsweep (whose own computed results cmd/verdi/audit.go reads rather than re-deriving): the audit's report and gc's plan are ONE COMPUTATION, internal/residue.Scan, read by two different verbs, never computed twice. The two verdi gc modes are mutually exclusive per invocation, never combined in one run: bare verdi gc (unchanged) runs only the existing managed-worktree slice; verdi gc --reclaim-unmanaged (with or without --apply) runs only the new unmanaged slice — this story's own resolution of a mechanical detail neither the ratification amendment nor spec/residue-reclamation states, chosen because running a MUTATING managed pass silently alongside a DRY-RUN-by-default unmanaged one in one invocation would make the run's mutating-or-not character depend on which flag a reader noticed, exactly what ac-3's disclosure contract exists to foreclose. Dry-run and --apply share the identical eligibility computation (ac-1's predicate, computed exactly once per run); --apply differs only in continuing past it into the mutating calls ac-2 describes. A dry-run plan and a later, separate --apply invocation are two separate process runs with no transactional guarantee between them — git state can change in between, same as any scan-then-act tool; ac-2's own second-guard requirement exists precisely because of this gap, not despite it.", anchor: dc-1 }
  - { id: dc-2, text: "The predicate runs over two row shapes internal/residue.Result already exposes, never a third re-derived one. Worktree rows: every entry in Result.Worktrees (residue.Scan's own contract already excludes the primary checkout from this slice; relied upon here, never independently re-verified). A row is eligible iff, in order: MergedUnresolved false AND DirtyUnresolved false (else kept: unresolved-state, naming the row's own Reason); Merged true (else kept: unmerged); Dirty false (else kept: dirty); Branch not empty (else kept: detached); Managed false (else kept: managed); Path not equal to the invoking checkout's own root (else kept: invoking) — the ordering is significant, mirroring internal/wtmanager.decideReclaim's own total, ordered switch, so a row with multiple simultaneously-true exclusion facts still gets exactly one disclosed reason, deterministically. Branch-only rows: every name in Result.MergedBranches with no matching Worktrees[].Branch (MergedBranches is pre-filtered to merged branches by residue.Scan itself, so unmerged never reaches this shape); eligible unless the name equals the invoking checkout's own current branch (else kept: invoking) — the one check this shape needs, since a bare branch has no worktree to be managed, detached, dirty, or unresolved. The invoking checkout's identity is resolved once by the caller from facts every verdi verb already computes: its root (store.FindRoot(\".\"), the same call cmd/verdi/gc.go already makes today) against worktree rows' Path, and its current branch (gitx.CurrentBranch, an existing primitive, empty for a detached invoking HEAD) against branch-only rows' names. LEDGER R4-I-80 (genuine ambiguity, resolved here — PLAN-V1.md's own table needs this entry appended by whoever owns that file; this story's authoring session is fenced to this worktree and does not edit it directly): the design names \"not the primary checkout\" as its own condition alongside \"not the invoking checkout\" — free for worktree rows (residue.Scan's Worktrees never contains primary) but NOT independently free for a branch-only row, since Result exposes no fact identifying which merged branch, if any, the PRIMARY checkout currently has checked out. Re-deriving it independently would be exactly the re-derivation this story's own architecture (dc-1) forbids; extending internal/residue.Result to expose it would mean editing an already-closed sibling story's frozen deliverable, a bigger call than this story's own scope. Smallest reversible resolution: the predicate does not pre-classify this one case. When the invoking checkout IS the primary, the invoking-checkout check already catches it for free (both checks collapse to the same comparison); when it is not, DeleteMergedBranch's own git branch -d refusal (ac-2's second guard — git refuses to delete a branch checked out ANYWHERE, primary included) catches it at apply time instead, disclosed as an ordinary per-item failure rather than a plan-time kept-and-disclosed row. Safe (git itself is the backstop), but a dry-run plan can, in this one narrow case, list such a row as eligible when a later --apply would in fact refuse it — disclosed here as a known, narrow limitation, not a silent gap; ac-2's own fixture set proves the refusal fires exactly this way.", anchor: dc-2 }
  - { id: dc-3, text: "One new gitx primitive, DeleteMergedBranch(ctx, dir, name) (string, error), returning the branch's own pre-delete tip commit alongside git branch -d's ordinary success/refusal — deliberately -d, never -D: git's own merged-check is an independent second signal beyond the plan's own Merged fact (dc-2), and a force-delete would erase that guard entirely. LEDGER R4-I-81 (genuine ambiguity, resolved here — PLAN-V1.md needs this entry too, for the reason dc-2 names): gitx already has a DeleteBranch(ctx, dir, name) error primitive (branch.go) that wraps git branch -d identically, today used by exactly one call site (close.go's CheckoutNewBranch-unwind, verified by grep — no second caller exists). A second, independent primitive doing the identical git call would be the copy-paste CLAUDE.md forbids (\"anything used by two or more packages lives in a shared internal/ package; never copy-paste across packages\"). DeleteMergedBranch is not that: its distinguishing, load-bearing contract is the returned tip commit (ac-2's recovery-affordance requirement) that DeleteBranch's existing signature has no room for and that cannot be read back after a successful -d deletes the ref — so it must be resolved before the delete, not after. The smallest reversible shape is composition, not duplication: DeleteMergedBranch resolves the tip via the existing gitx.RevParse, then calls the existing gitx.DeleteBranch for the delete itself, wrapping only the ordering and the return value; the underlying git branch -d call exists exactly once in the package, in DeleteBranch, exactly as it does today.", anchor: dc-3 }
  - { id: dc-4, text: "Report lines mirror spec/worktree-manager's own Result/Line() shape (gc.go's existing gcOne per-worktree report), one line per reclaim UNIT — a branch and its worktree together where one exists, matching the ratified amendment's own \"a LOCAL branch and its worktree (if any)\" wording — never two separate lines for one unit: reclaimed (worktree+branch) names the worktree path, the branch, and the branch's tip commit; reclaimed (branch-only) names the branch and its tip commit; kept names the unit and exactly one reason from dc-2's closed vocabulary; a partial outcome (--apply only: a worktree successfully removed whose branch delete then failed) is its own disclosed line, distinct from both full success and a row kept before anything was touched, naming the branch's own residual presence explicitly rather than folding it into a generic failure bucket. The updated scope-disclosure line (ac-3) is dc-1's own mutual-exclusivity resolution made observable: printed on every invocation, naming which of the two slices ran and which did not, plus the still-unimplemented derived-cache/layout-cache bullets (spec/residue-reclamation co-1's own scope, unaffected).", anchor: dc-4 }
constraints:
  - { id: co-1, text: "No network in any test (CLAUDE.md). Every eligibility, execution-ordering, second-guard-refusal, and disclosure-format claim is proven against fixturegit repositories with real local branches, real close/<name> archival commits where relevant, and real git worktrees materialized on local disk — mirroring spec/closure-hygiene's own co-1 and spec/worktree-manager's own co-2 precedent exactly.", anchor: co-1 }
  - { id: co-2, text: "internal/residue (the survey producer spec/closure-hygiene shipped), all three existing verdi audit report sections, and internal/wtmanager's existing managed-worktree GC logic are byte-for-byte unchanged by this story. New code is confined to internal/reclaim (a new package), one new additive gitx primitive (DeleteMergedBranch, dc-3), and cmd/verdi/gc.go's own flag dispatch plus its gcScopeDisclosure string (ac-3) — never a rewrite of anything spec/closure-hygiene or spec/worktree-manager already delivered.", anchor: co-2 }
---

# GC Reclaim

## Problem

`spec/residue-reclamation`'s own AC-1 needs an implementation.

`verdi-store-layout`'s Garbage collection section now licenses, on
explicit opt-in, pruning a LOCAL branch and its worktree (if any) when the
branch is fully merged into the default branch, its worktree carries no
uncommitted changes, and the worktree is not the primary checkout (ledger
R4-I-79) — but `verdi gc` itself only ever reclaims managed worktrees under
`.verdi/data/worktrees/` (`spec/worktree-manager`); nothing in the binary
acts on the newly-ratified unmanaged slice, and nothing computes which
unmanaged branches or worktrees even qualify.

`spec/closure-hygiene`'s own `internal/residue` package already computes
exactly this survey once, for `verdi audit`'s third report section — every
local branch's merge state, every worktree's merge/clean/managed state,
disclosed rather than guessed wherever it cannot be resolved. The
eligibility facts this story needs already exist as a single, tested,
in-tree computation (`internal/residue.Scan`); the only thing missing is a
consumer that turns eligible rows into a disclosed plan and, on further
opt-in, an executed one.

## Outcome

`verdi gc` gains `--reclaim-unmanaged` (prints the plan; touches nothing)
and `--reclaim-unmanaged --apply` (executes it), built as a thin consumer
of `internal/residue.Scan`'s own survey rows — eligibility is never
re-derived from git independently.

Every reclaim-eligible LOCAL branch and its worktree (if any) is named
with its tip commit; every kept row is named with one of a closed set of
reasons; applying removes the worktree before its branch, each backed by
git's own independent refusal as a second guard; a per-item failure is
disclosed and the sweep continues; an unresolvable default branch refuses
the whole run rather than asserting a plan it cannot compute.

`verdi gc`'s existing managed-worktree slice, `internal/residue`, and
every existing `verdi audit` report section are untouched.

## AC-1

Given `internal/residue.Scan`'s own survey (never re-derived from git
independently), reclaim-eligible is a total, closed predicate over its
rows: a row is eligible iff its branch is merged into the default branch
(`Merged` true, `MergedUnresolved` false), its worktree — where one
exists — carries no uncommitted changes (`Dirty` false, `DirtyUnresolved`
false), it is not managed (`verdi gc`'s existing managed-worktree
jurisdiction), it carries a local branch (a detached-HEAD worktree is
outside the ratified "a LOCAL branch and its worktree (if any)" unit), and
it is not the checkout the sweep itself is running from.

A row failing any of these is kept and disclosed with exactly one reason
drawn from a closed vocabulary — **unmerged, dirty, unresolved-state**
(naming residue's own `Reason`), **detached, managed, invoking** — never a
silent omission and never an undocumented second reason string. A branch
with no worktree at all (present in residue's merged-branches list, absent
from its worktree list) is eligible for branch deletion alone.

Witness, as of 2026-07-20 (main/HEAD
`cda3ec625a276db2cc66aa0603b62c4e8649cd60`, this repository's own `verdi
audit`): `close/attest-helper` (tip `7495c38a18719cca691b64c80bfa217e2886b3be`),
`close/close-preflight` (tip `af9848edaac6d57353b86f0f19cdb9ec25931b34`),
`close/disposition-verb` (tip `d24686552ed25f4e86f169797ce080fc54dfe1c4`),
and `close/home-status-glance` (tip `4e3b67a4a46d43f2e1f6ce39c00bb3dea4eba26b`)
are all closure-hygiene's own `superseded-elsewhere` witnesses — already
redundant, already archived elsewhere — yet every one is **unmerged**, so
every one is kept as unmerged: the amendment licenses fully-merged
branches only, and being redundant is not the same fact as being merged.
The same live audit also names several genuinely eligible-shaped pairs
(e.g. `close/model-digest` and its `model-digest-build` worktree,
`close/scaffold-templates` and its `scaffold-templates-build` worktree —
each merged, clean, unmanaged), a live worktree with no branch at all
(`w6-exit`, detached), and, distinctly, this very story's own worktree
(`verdi-wt/residue-reclamation`, on `design/residue-reclamation` —
trivially merged and clean, since nothing has yet been committed on it
beyond its branch point) — which the invoking-checkout exclusion, not any
other reason, is what keeps it out of its own plan.

Evidence: static (the predicate is a pure function of `*residue.Result`
plus the invoking checkout's own root and current branch, with no silent
seventh path — a table test enumerates every one of the six ordered
exclusion checks plus the eligible case, and a compile-time check that the
kept-reason type is a closed enum) and behavioral — a fixturegit repository
combining, in one survey, an eligible worktree+branch pair, an eligible
branch-only row, an unmerged row (mirroring the four live `close/<name>`
witnesses above), a dirty row, a row with both `MergedUnresolved` and a
populated `Reason`, a detached-HEAD row, a managed-worktree row, and a row
at the invoking checkout's own path/branch asserts every single row is
named exactly once with its correct eligible-or-kept-and-reason
classification, with no row silently dropped from the report.

## AC-2

Given `--reclaim-unmanaged` alone, `verdi gc` computes and prints the full
plan from AC-1's predicate — every eligible item named with its worktree
path (if any) and branch, every kept item named with its one reason — and
performs no git-mutating call.

Given `--reclaim-unmanaged --apply`, it executes that same plan: per
eligible item, its worktree (if any) is removed first (`git worktree
remove`, without `--force` — git's own refusal on a since-dirtied tree is
a second, independent guard beyond the plan's own `Dirty` check), then its
branch is deleted (`git branch -d`, never `-D` — git's own refusal on an
unmerged or elsewhere-checked-out branch is a second, independent guard
beyond the plan's own `Merged` check). A refusal at either step is
disclosed on that item alone and the sweep continues to the next item,
including the partial outcome of a worktree successfully removed whose
branch delete then failed. Every branch actually deleted prints its
pre-delete tip commit.

An unresolvable default branch refuses the whole run before computing any
plan, dry-run or applied alike, asserting nothing rather than a plan it
cannot compute — a stricter posture than `internal/wtmanager`'s own
managed-worktree GC, which treats an unresolved default branch as "nothing
eligible" rather than a refusal, because that path only ever touches
worktrees it created itself under `.verdi/data/worktrees/`, while this
mode reaches worktrees and branches it did not create, at higher cost if
its plan is wrong.

`verdi gc`'s own exit contract is unchanged: 0 when the run completes,
regardless of how many individual items were kept or individually
refused; 2 for a whole-run operational failure (an unresolvable default
branch, a usage error) — reclamation mints no verdict exit-1.

Evidence: static (the execution ordering — worktree before branch,
disclosure before or independent of any mutation, the exit-code map — is
a fixed, inspectable sequence with no conditional reordering) and
behavioral — fixturegit cases proving: dry-run performs zero git-mutating
calls (asserted directly against the fixture's own git state before and
after); `--apply` on a clean eligible pair removes both, in order, and
prints the branch's tip commit; a worktree dirtied AFTER the scan but
BEFORE `--apply` runs is kept via `git worktree remove`'s own refusal
(the first second-guard witness), disclosed, sweep continuing to the next
item; a branch-only row equal to the (non-primary) invoking checkout's own
branch is kept via the invoking check, while a second branch-only row
constructed to be checked out at a NON-invoking primary-shaped worktree is
instead caught by `git branch -d`'s own refusal at apply time (the second
second-guard witness, proving DC-2/ledger R4-I-80's resolution); a
worktree whose removal succeeds but whose paired branch delete is then
forced to fail asserts the partial-outcome line, not a generic failure;
and an empty default branch ref refuses the whole run, dry-run and
`--apply` alike, with no plan printed and no mutating call attempted.

## AC-3

`verdi gc`'s existing scope-disclosure line grows to name both slices as a
closed pair on every invocation: a plain `verdi gc` run continues to
reclaim only managed worktrees and now additionally discloses that
unmanaged reclamation is available via `--reclaim-unmanaged` but was not
run this invocation; a `--reclaim-unmanaged` run (dry-run or applied)
discloses that managed-worktree reclamation was not run this invocation,
alongside derived-cache and layout/tree-hash-cache pruning, which remain
out of scope for either.

Every reclaimed and every kept item prints exactly one line, mirroring
`spec/worktree-manager`'s own DC-4 one-line-per-worktree idiom — no
removal or skip a human running the command cannot see named in its own
output.

`internal/residue` (the survey producer) and all three existing `verdi
audit` report sections are byte-for-byte unchanged by this story; this
story adds a new consumer package and new `verdi gc` flags, never new
`audit` behavior.

Evidence: static (the scope-disclosure string and the per-item line
templates are literal, inspectable constants, and a diff of
`internal/residue`'s and `cmd/verdi/audit.go`'s own report-section code
against this story's own merge base is empty) and behavioral — a
built-binary test asserts a plain `verdi gc` run's own output still
contains its pre-existing managed-worktree behavior plus the new
"available, not run" disclosure naming `--reclaim-unmanaged`; a
`--reclaim-unmanaged` run's output contains the mirrored "managed
reclamation not run" disclosure plus the pre-existing derived/cache
disclosure; and a fixture exercising every AC-1 exclusion reason at once
asserts each renders as its own single line, matching a golden transcript.

## DC-1

Surface: a new `internal/reclaim` package (single concern: the sweep,
CLAUDE.md's one-package-one-concern rule), consuming `*residue.Result`
directly; `cmd/verdi/gc.go` stays thin wiring, gaining `--reclaim-unmanaged`
and `--apply` flags plus the resolved default branch it already computes
today (`lint.ResolveDefaultBranch`, unchanged).

`internal/reclaim` calls zero gitx **eligibility** primitives itself — no
`IsAncestor`, no `StatusDirty` — mirroring the same one-computation-
consumed-downstream precedent `internal/residue.Result.Flagged()` itself
already established relative to `internal/decisionsweep` (whose own
computed results `cmd/verdi/audit.go` reads rather than re-deriving): the
audit's report and gc's plan are ONE COMPUTATION, `internal/residue.Scan`,
read by two different verbs, never computed twice.

The two `verdi gc` modes are mutually exclusive per invocation, never
combined in one run: bare `verdi gc` (unchanged) runs only the existing
managed-worktree slice; `verdi gc --reclaim-unmanaged` (with or without
`--apply`) runs only the new unmanaged slice — this story's own resolution
of a mechanical detail neither the ratification amendment nor
`spec/residue-reclamation` states, chosen because running a MUTATING
managed pass silently alongside a DRY-RUN-by-default unmanaged one in one
invocation would make the run's own mutating-or-not character depend on
which flag a reader noticed, exactly what AC-3's disclosure contract
exists to foreclose.

Dry-run and `--apply` share the identical eligibility computation (AC-1's
predicate, computed exactly once per run); `--apply` differs only in
continuing past it into the mutating calls AC-2 describes. A dry-run plan
and a later, separate `--apply` invocation are two separate process runs
with no transactional guarantee between them — git state can change in
between, same as any scan-then-act tool; AC-2's own second-guard
requirement exists precisely because of this gap, not despite it.

## DC-2

The predicate runs over two row shapes `internal/residue.Result` already
exposes, never a third re-derived one.

**Worktree rows** — every entry in `Result.Worktrees` (`residue.Scan`'s
own contract already excludes the primary checkout from this slice;
relied upon here, never independently re-verified — the anti-hairball
point applies to that guarantee too). A row is eligible iff, in order:
`MergedUnresolved` false AND `DirtyUnresolved` false (else kept:
unresolved-state, naming the row's own `Reason`); `Merged` true (else
kept: unmerged); `Dirty` false (else kept: dirty); `Branch` not empty
(else kept: detached); `Managed` false (else kept: managed); `Path` not
equal to the invoking checkout's own root (else kept: invoking). The
ordering is significant — mirroring `internal/wtmanager.decideReclaim`'s
own total, ordered switch — so a row with multiple simultaneously-true
exclusion facts (e.g. both dirty and detached) still gets exactly one
disclosed reason, deterministically, never an arbitrary or combinatorial
one.

**Branch-only rows** — every name in `Result.MergedBranches` with no
matching `Worktrees[].Branch` (a branch merged into the default branch
with no worktree of its own, per AC-1's own "eligible for branch deletion
alone" clause; `MergedBranches` is pre-filtered to merged branches by
`residue.Scan` itself, so unmerged never reaches this shape at all).
Eligible unless the name equals the invoking checkout's own current
branch (else kept: invoking) — the one check this shape needs, since a
bare branch has no worktree to be managed, detached, dirty, or unresolved.

The invoking checkout's own identity — needed by both shapes — is
resolved once by the caller from facts every `verdi` verb already
computes, never a new eligibility re-derivation: its root
(`store.FindRoot(".")`, the same call `cmd/verdi/gc.go` already makes
today) against worktree rows' `Path`, and its current branch
(`gitx.CurrentBranch`, an existing primitive, empty for a detached
invoking HEAD, in which case no branch-only row can ever match it)
against branch-only rows' names.

**Ledger R4-I-80 (genuine ambiguity, resolved here; `PLAN-V1.md`'s own
table needs this entry appended by whoever owns that file — this story's
authoring session is fenced to this worktree and does not edit it
directly).** The eligibility design this story implements names "not the
primary checkout" as its own condition, independent of "not the invoking
checkout" — correct for worktree rows, where it is structurally free
(`residue.Scan`'s `Worktrees` never contains primary). It is NOT
independently free for a branch-only row: `Result` exposes no fact
identifying which merged branch, if any, the PRIMARY checkout currently
has checked out (only worktree rows carry a `Branch` field, and primary is
never one), so a branch-only row that happens to be primary's own current
branch is indistinguishable, from survey data alone, from an ordinary
orphaned merged branch.

Re-deriving primary's branch independently (a fresh `gitx.WorktreeList` or
a `gitx.CurrentBranch` call against primary's own root, from inside
eligibility) would be exactly the re-derivation this story's own
architecture (DC-1) forbids, and extending `internal/residue.Result` to
also expose it would mean editing an already-closed sibling story's frozen
deliverable — a bigger call than this story's own scope.

Smallest reversible resolution: the predicate does not attempt to
pre-classify this one case. When the invoking checkout IS the primary, the
invoking-checkout check above already catches it for free (both checks
collapse to the same comparison); when it is not, `DeleteMergedBranch`'s
own `git branch -d` refusal (AC-2's second guard — git refuses to delete a
branch checked out ANYWHERE, primary included, not only an unmerged one)
catches it at apply time instead, disclosed as an ordinary per-item
failure rather than a plan-time kept-and-disclosed row. This is safe (git
itself, not this story's own logic, is the backstop that makes the
destructive call refuse) but means a dry-run plan can, in this one narrow
case, list such a row as eligible when a later `--apply` would in fact
refuse it — disclosed here as a known, narrow limitation, not a silent
gap. AC-2's own fixture set proves the refusal fires exactly this way.

## DC-3

One new gitx primitive, `DeleteMergedBranch(ctx, dir, name) (string,
error)`, returning the branch's own pre-delete tip commit alongside `git
branch -d`'s ordinary success/refusal — deliberately `-d`, never `-D`:
git's own merged-check is an independent second signal beyond the plan's
own `Merged` fact (DC-2), and a force-delete would erase that guard
entirely.

**Ledger R4-I-81 (genuine ambiguity, resolved here; `PLAN-V1.md` needs
this entry too, for the reason DC-2 names).** `gitx` already has a
`DeleteBranch(ctx, dir, name) error` primitive (`branch.go`) that wraps
`git branch -d` identically, today used by exactly one call site
(`close.go`'s `CheckoutNewBranch`-unwind, verified by grep — no second
caller exists). A second, independent primitive doing the identical git
call would be the copy-paste CLAUDE.md forbids ("anything used by two or
more packages lives in a shared `internal/` package; never copy-paste
across packages").

`DeleteMergedBranch` is not that: its distinguishing, load-bearing
contract is the returned tip commit (AC-2's recovery-affordance
requirement) that `DeleteBranch`'s existing signature has no room for and
that cannot be read back after a successful `-d` deletes the ref — so it
must be resolved before the delete, not after. The smallest reversible
shape is composition, not duplication: `DeleteMergedBranch` resolves the
tip via the existing `gitx.RevParse`, then calls the existing
`gitx.DeleteBranch` for the delete itself, wrapping only the ordering and
the return value; the underlying `git branch -d` call exists exactly once
in the package, in `DeleteBranch`, exactly as it does today.

## DC-4

Report lines mirror `spec/worktree-manager`'s own `Result`/`Line()` shape
(`gc.go`'s existing `gcOne` per-worktree report), one line per reclaim
UNIT — a branch and its worktree together where one exists, matching the
ratified amendment's own "a LOCAL branch and its worktree (if any)"
wording — never two separate lines for one unit:

- **reclaimed, worktree+branch**: names the worktree path, the branch,
  and the branch's tip commit.
- **reclaimed, branch-only**: names the branch and its tip commit.
- **kept**: names the unit and exactly one reason from DC-2's closed
  vocabulary (unmerged, dirty, unresolved-state — with residue's own
  `Reason` text, detached, managed, invoking).
- **partial** (`--apply` only): a worktree successfully removed whose
  branch delete then failed is its own disclosed outcome, distinct from
  both full success and a row kept before anything was touched — the
  branch's own residual presence is named explicitly, not folded into a
  generic "failed" bucket.

The updated scope-disclosure line (AC-3) is DC-1's own mutual-exclusivity
resolution made observable: printed on every invocation, naming which of
the two slices ran and which did not, plus the still-unimplemented
derived-cache/layout-cache bullets (`spec/residue-reclamation` CO-1's own
scope, unaffected).

## CO-1

No network in any test (CLAUDE.md). Every eligibility, execution-ordering,
second-guard-refusal, and disclosure-format claim is proven against
`fixturegit` repositories with real local branches, real `close/<name>`
archival commits where relevant, and real git worktrees materialized on
local disk — mirroring `spec/closure-hygiene`'s own CO-1 and
`spec/worktree-manager`'s own CO-2 precedent exactly.

## CO-2

`internal/residue` (the survey producer `spec/closure-hygiene` shipped),
all three existing `verdi audit` report sections, and
`internal/wtmanager`'s existing managed-worktree GC logic are
byte-for-byte unchanged by this story. New code is confined to
`internal/reclaim` (a new package), one new additive gitx primitive
(`DeleteMergedBranch`, DC-3), and `cmd/verdi/gc.go`'s own flag dispatch
plus its `gcScopeDisclosure` string (AC-3) — never a rewrite of anything
`spec/closure-hygiene` or `spec/worktree-manager` already delivered.
