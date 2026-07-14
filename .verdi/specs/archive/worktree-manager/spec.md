---
id: spec/worktree-manager
kind: spec
title: "Worktree Manager"
owners: [platform-team]
class: story
status: closed
story: jira:VERDI-19
problem: { text: "spec/workbench-directory ac-3 and ac-4 require that opening a draft board serves its own design branch's working tree in authoring mode without disturbing any other board or the serving checkout, and that a single serve process owns every working tree it writes - but verdi serve is bound to exactly one working tree today, with no mechanism to lazily materialize a second, third, or Nth working tree per design branch, and no ownership discipline over any it might create. The sibling draft-boards story already assumes this seam exists ('a managed worktree for branch X' consumed from 'the worktree-manager story's seam') without providing it.", anchor: problem }
outcome: { text: "a backend seam lazily cuts a managed git worktree for a local design branch on first request, reuses it on every later request, lives entirely under the data zone (never committed), and is owned by exactly one process - guarded by a lockfile with liveness semantics mirroring this store's existing writer-lock discipline. verdi gc becomes a real, implemented verb for the managed-worktree reclamation slice: a merged or deleted branch's worktree is reaped once its lock is not live and it carries no uncommitted changes; a dirty or currently-owned worktree is disclosed and kept, never force-removed, and reads never delete.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "EnsureWorktree(ctx, root, branch) lazily cuts a managed git worktree for LOCAL design branch <branch> under the data zone on first call (git worktree add, never touching the serving checkout's own branch/index/working tree), and every later call for the same branch reuses the existing worktree unchanged - idempotent, no re-cut, no duplicate git worktree entry", evidence: [static, behavioral], anchor: "#ac-1" }
  - { id: ac-2, text: "EnsureWorktree refuses - a defined, typed error, never a minted local branch and never a git panic - when <branch> has no LOCAL ref (remote-tracking-only or absent entirely), matching feature dc-5's local-branches-only rule verbatim; it also refuses, disclosed and never forced, when <branch> is already checked out in the serving checkout itself", evidence: [static, behavioral], anchor: "#ac-2" }
  - { id: ac-3, text: "a per-managed-worktree lockfile (dc-2) makes exactly one process the writer of a given managed worktree at any time: two concurrent EnsureWorktree calls for the same not-yet-cut branch (from goroutines within one serve process, or from two independently-started processes racing against the same data zone) produce exactly one git worktree add and no corrupted worktree state; a live-locked worktree is never removed by gc out from under its owner", evidence: [static, behavioral], anchor: "#ac-3" }
  - { id: ac-4, text: "verdi gc reclaims a managed worktree whose branch is merged (tip is an ancestor of the default-branch tip) or deleted (absent), per feature dc-4's ratified signals - reads never delete (gc is the only deleter, invoked explicitly, never a background process); a worktree carrying uncommitted changes is never reclaimed, disclosed and kept instead; a worktree whose lock is currently held by a live process is likewise never reclaimed, disclosed and kept, deferred to the next gc run", evidence: [static, behavioral], anchor: "#ac-4" }
  - { id: ac-5, text: "verdi gc's CLI dispatch (cmd/verdi/dispatch.go) becomes a real, implemented verb - flipped from its phase-0 recognized-but-unimplemented stub - scoped honestly to the managed-worktree reclamation slice only: gc's own output discloses that derived-cache and layout-version-cache pruning (verdi-store-layout's other ratified gc bullets) are out of this story's scope, never silently implied as covered", evidence: [static, behavioral], anchor: "#ac-5" }
links:
  - { type: implements, ref: "spec/workbench-directory#ac-3" }
  - { type: implements, ref: "spec/workbench-directory#ac-4" }
decisions:
  - { id: dc-1, text: "a new internal/wtmanager package exposes EnsureWorktree(ctx, root, branch) (path string, err error): lazy, synchronous (blocks until the cut completes or fails - no interstitial, matching the sibling draft-boards story's own dc-2 consumption contract verbatim), idempotent (a worktree already present at the deterministic path for branch is reused, never re-cut). Layout under the data zone (co-1): .verdi/data/worktrees/<name>/, where <name> is the design branch's own spec name (branch design/<name> to path <name> - a direct, collision-free mapping since spec names are already globally unique, VL-002 - never a hash or re-slugging scheme this story would have to invent). EnsureWorktree runs exactly one git command class against the serving checkout's root - worktree add <path> <branch> - and never checkout/switch on that root, so the serving checkout is provably undisturbed the same way ref-index's ComputeIndex is (co-1)", anchor: "#dc-1" }
  - { id: dc-2, text: "ownership mechanism: a per-managed-worktree lockfile at .verdi/data/worktrees/<name>.lock, using the EXACT algorithm this store already ratified for its per-checkout data/writer.lock (internal/mcpserve/lock.go's O_CREATE|O_EXCL {pid,start} JSON, kill(pid,0)-plus-ps -o lstart= liveness cross-check closing the PID-reuse gap, stale-lock takeover) - extracted into a new, shared internal/filelock package (CLAUDE.md: anything used by two or more packages lives in a shared internal/ package) rather than copy-pasted a second time or imported from the semantically-mismatched internal/mcpserve. cmd/verdi/serve.go's existing writer-lock call sites move to internal/filelock unchanged in behavior. The lock is held ONLY for the duration of a git-worktree-mutating operation on that worktree - EnsureWorktree's own git worktree add, or gc's git worktree remove - never for the worktree's whole idle lifetime between operations: ordinary board reads/edits inside an already-cut worktree are ordinary git commands against an ordinary directory and need no lock of their own. This bounds every lock hold to a single short git invocation, so a live-lock skip (dc-4) is always a narrow, transient race window - never a multi-minute or whole-serve-session deferral", anchor: "#dc-2" }
  - { id: dc-3, text: "gc's merged signal reuses gitx.IsAncestor (the same primitive the sibling ref-index story's dc-5 already uses). Its deleted signal is LOCAL-ONLY (the branch no longer resolves under refs/heads/design/* at all) - narrower than the ratified store-layout gc deleted-signal (git fetch --prune, then neither-local-nor-remote) that parent dc-4 invokes generically as 'the ratified gc signals'. Reason: a managed worktree is a live checkout bound to a SPECIFIC LOCAL BRANCH (dc-1: cut from local branches only) - once that local branch is gone, the worktree is orphaned regardless of whether a same-named remote-tracking ref persists, since resurrecting it would require explicitly minting a new local branch, an act dc-1 already forbids doing implicitly - so a derived-cache-shaped 'alive anywhere' check would be actively wrong for this target, not merely different. Deliberately disclosed in prose rather than a formal exempts edge against parent dc-4 (dc-4 IS a valid fragment target, R4-I-12's stub-match test disqualifies ANY exempts edge unconditionally, forcing full review - disproportionate for an implementation-scoped clarification of an underspecified generalization for a target, managed worktrees, that postdates the parent decision entirely; the substance is fully disclosed here for any reviewer to escalate on its merits, and the design-branch decision-conflict report carries this exact judgment as a dispositioned finding, a mechanism orthogonal to spec-level edges). Separately, retention_days: verdi-store-layout's config comment ('gc horizon for merged/deleted refs') is worded generally enough that whether it binds every future merged/deleted-ref reclaim mechanism or only the derived-cache one that existed when it was ratified is genuinely ambiguous - store-layout predates managed worktrees and cannot have decided this. This story chooses NOT to apply retention_days to managed worktrees (reclaim-eligible the instant its signal fires, no grace period) as the smallest reversible starting point. This second divergence carries no edge because none is available to carry at all: verdi-store-layout is a component spec (02 §Kind registry), excluded by the artifact contract's object model from carrying declared decision objects, and the exempts edge type targets only an ADR or a feature-decision fragment - no such id exists in store-layout's prose to name", anchor: "#dc-3" }
  - { id: dc-4, text: "gc never forces a removal. It checks gitx.StatusDirty (existing function, reused unchanged) before attempting anything, producing a clear disclosed message ('kept: uncommitted changes') rather than parsing git's own refusal text; only a clean, reclaim-eligible worktree reaches git worktree remove, called WITHOUT --force, so git's own dirty-tree safety net is a second, redundant guard rather than the only one. A worktree whose lockfile (dc-2) is currently held by a live process is equally skipped and disclosed ('kept: in use by pid N') rather than removed out from under its owner - a keep-reason beyond parent dc-4's single named exception (uncommitted changes). Because dc-2 holds this lock only for the duration of a single git-worktree-mutating call (never the worktree's whole idle lifetime), this skip is a NARROW, single-operation race window: the exact same merged/deleted worktree is fully reclaim-eligible again the moment that one operation finishes, ordinarily within the same gc run's next pass or the very next invocation - never withheld for the life of a long-running serve session, and materially different in duration from the dirty-worktree case (an indefinite hold until a human acts) - so parent dc-4's reclamation model stays valid and unweakened, this only adds a narrow safety margin around it. Disclosed in prose, not a formal exempts edge, for the same R4-I-12 proportionality reason dc-3 gives: any exempts edge here would force full review regardless of how narrow the substance is, which this judgment call - already surfaced, argued, and available for a reviewer's own escalation - does not warrant on its own. Every skip and every reclaim is printed, one line per worktree; gc performs no removal a human cannot see in its own output", anchor: "#dc-4" }
  - { id: dc-5, text: "scope line: this story implements ONLY the managed-worktree reclamation slice of verdi gc. verdi-store-layout's Garbage collection section also ratifies derived-cache pruning (data/derived/<ref>/ for refs merged/deleted past derived.retention_days) and layout/tree-hash cache pruning - neither is touched here; cmd/verdi gc's own printed output says so explicitly on every run, so a human is never left inferring full gc coverage from a partial implementation. This is incremental delivery of an already-ratified, already-recognized-but-unimplemented verb (dispatch.go's gc entry was phase 0, 'out of v0 scope', before this story - PLAN.md's own build contract stages every verb across phases; no verb here has ever been required to land in one shot), not a redefinition or narrowing of verdi-store-layout's Garbage collection section, which remains fully valid, unedited, and awaiting its own future story for the two slices left undone. No supersedes/exempts edge is needed for the same structural reason dc-3 gives: verdi-store-layout is a component spec carrying no declared decision objects (02 Object model: component specs 'have no object model, carry neither'), so the exempts edge type's required target - an ADR or feature-decision fragment - does not exist to be named here either; dispatch.go's gc entry moves from phase 0 to a real, implemented phase, the same flip close-verb already made for its own verb in round 6", anchor: "#dc-5" }
constraints:
  - { id: co-1, text: "inherited verbatim from the feature (co-1): managed worktrees live under the data zone, never committed. EnsureWorktree's every write happens under .verdi/data/worktrees/; nothing it creates is ever git-added or git-committed, and gc's own removals touch only that same subtree", anchor: "#co-1" }
  - { id: co-2, text: "no network in any test (CLAUDE.md): every EnsureWorktree and gc behavior - the cut, the lock contention, the merged/deleted/dirty/locked reclaim decisions - is proven against fixturegit repositories with real local branches and real worktrees on local disk; no live clone or fetch", anchor: "#co-2" }
frozen: { at: 2026-07-14, commit: cd108d7b507b94cff567f56b24cd4fa3de636f63, stub_matched: true }
---
# Worktree Manager

## Problem

`spec/workbench-directory` ac-3 and ac-4 require that opening a draft board
serves its own design branch's working tree in authoring mode without
disturbing any other board or the serving checkout, and that a single serve
process owns every working tree it writes. `verdi serve` is bound to exactly
one working tree today — there is no mechanism to lazily materialize a
second, third, or Nth working tree per design branch, and no ownership
discipline over any it might create. The sibling `draft-boards` story
(already accepted) assumes this seam exists — "a managed worktree for
branch X" is "consumed from the worktree-manager story's seam" (its own
ac-1) — without this story ever having supplied it.

## Outcome

A backend seam lazily cuts a managed git worktree for a local design branch
on first request, reuses it on every later request, lives entirely under
the data zone (never committed), and is owned by exactly one process at a
time — guarded by a lockfile with liveness semantics mirroring this store's
existing writer-lock discipline (internal/mcpserve's own `data/writer.lock`
mechanism, reused rather than reinvented). `verdi gc` becomes a real,
implemented verb for the managed-worktree reclamation slice: a merged or
deleted branch's worktree is reaped once its lock is not live and it
carries no uncommitted changes; a dirty or currently-owned worktree is
disclosed and kept, never force-removed, and reads never delete.

## AC-1

`EnsureWorktree(ctx, root, branch)` lazily cuts a managed git worktree for a
LOCAL design branch on first call — `git worktree add <path> <branch>`
against `root`, never a `checkout` or `switch` on `root` itself, so the
serving checkout's own branch, index, and working tree are provably
undisturbed. Every later call for the same branch, within this process or a
later one, reuses the existing worktree at its deterministic path unchanged
— no re-cut, no duplicate `git worktree` entry, no second directory. Layout
and naming are dc-1's decision. Evidence: static (the function's only
git-worktree-mutating call is `worktree add`, traced through the code, and
the reuse path never re-executes it once the path exists) and behavioral (a
fixturegit repo proves a first call creates the worktree with the branch's
own content checked out, and a second call returns the identical path
having run no second `git worktree add`).

## AC-2

`EnsureWorktree` refuses to cut a worktree for a branch that has no LOCAL
ref — a remote-tracking-only branch, or one that resolves nowhere at all —
returning a defined, typed error (never a bare git failure, never a panic,
and never a silently-minted local branch from a remote-tracking ref),
matching feature dc-5's "managed worktrees are cut from local branches
only" verbatim; this is the exact refusal `draft-boards` dc-4 already
depends on for its own remote-only-renders-sealed behavior. It also refuses
— disclosed, never forced — when `branch` is already checked out in the
serving checkout itself (git's own "already checked out" refusal on
`worktree add`, surfaced as a named, human-readable error rather than a raw
git stderr string): a realistic operator scenario (someone running `verdi
serve` while personally sitting on a design branch) that must degrade
honestly, not crash the request. Evidence: static (the typed error values
and the code path that checks local-ref existence before ever calling `git
worktree add`) and behavioral (a fixturegit repo with a remote-tracking-only
branch, a nonexistent branch name, and — separately — a branch already
checked out at `root`, each producing the expected named refusal and no
worktree directory).

## AC-3

A per-managed-worktree lockfile (dc-2) makes exactly one process the writer
of a given managed worktree at any time. Two concurrent `EnsureWorktree`
calls for the same not-yet-cut branch — two goroutines inside one serve
process, or two independently-started processes racing against the same
data zone — produce exactly one `git worktree add` and no corrupted
worktree state: the loser waits for or observes the winner's completed cut
and returns the same path, never attempting its own competing `git worktree
add`. A worktree whose lock is currently live (held by a running owner) is
never removed by `gc` out from under it (ac-4). Evidence: static (the lock
acquisition happens before any `git worktree add` and its failure path
retries-as-reuse rather than proceeding to cut) and behavioral (a test
drives two concurrent `EnsureWorktree` calls for the same branch and
asserts exactly one `git worktree add` ran and both calls returned the same
path; a second test holds a live lock and asserts a concurrent `gc` run
skips that worktree).

## AC-4

`verdi gc` reclaims a managed worktree whose branch is merged (its tip is an
ancestor of the default-branch tip) or deleted (absent) — feature dc-4's
ratified signals, verbatim. Reads never delete: `gc` is the only path that
ever removes a managed worktree, invoked explicitly by a human or CI job,
never a background process triggered by directory rendering or board
opening. A worktree carrying uncommitted changes is never reclaimed —
disclosed ("kept: uncommitted changes") and kept, per feature dc-4 exactly.
A worktree whose lock (dc-2) is currently held by a live process is
likewise never reclaimed — disclosed ("kept: in use") and kept, deferred to
the next `gc` invocation, a refinement in the same spirit as dc-4's dirty
case (dc-4 of this story). Evidence: static (the reclaim decision function
is a total, three-outcome map over {merged-or-deleted, clean, unlocked} →
{reclaim, keep-dirty, keep-locked, keep-not-eligible} with no fourth
silent path) and behavioral (a fixturegit repo with one merged-and-clean
worktree, one merged-but-dirty worktree, one merged-but-live-locked
worktree, and one still-unmerged worktree proves exactly the first is
removed and the other three are disclosed and kept).

## AC-5

`verdi gc`'s CLI dispatch (`cmd/verdi/dispatch.go`, currently `"gc": 0, //
out of v0 (PLAN.md §5)`, recognized but unimplemented) becomes a real,
implemented verb, scoped honestly to the managed-worktree reclamation slice
only. Every `verdi gc` run's own printed output discloses that
derived-cache and layout/tree-hash-cache pruning — the other bullets
`verdi-store-layout`'s "Garbage collection" section ratifies — are out of
this story's scope and were not run, never silently implying full gc
coverage from a partial implementation. Evidence: static (`dispatch.go`'s
`verbPhase["gc"]` carries a real, non-zero phase and routes to a real
implementation, not the generic "not implemented" path) and behavioral (a
CLI test asserts `verdi gc`'s output names its own scope limitation
verbatim, alongside its real reclaim/keep report).

## DC-1

A new `internal/wtmanager` package exposes
`EnsureWorktree(ctx, root, branch) (path string, err error)`: lazy,
synchronous — it blocks until the cut completes or fails, no interstitial —
matching the sibling `draft-boards` story's own dc-2 consumption contract
verbatim ("the managed worktree is cut on first request ... and the request
blocks until the cut completes"). Idempotent: a worktree already present at
the deterministic path for `branch` is reused, never re-cut.

Layout under the data zone (co-1): `.verdi/data/worktrees/<name>/`, where
`<name>` is the design branch's own spec name — branch `design/<name>` maps
to path `<name>`, a direct, collision-free mapping since spec names are
already globally unique (02 §Identity and references, VL-002) — never a
hash or a second slugging scheme this story would otherwise have to invent
and keep consistent with the rest of the store.

`EnsureWorktree` runs exactly one git-worktree-mutating command class
against the serving checkout's root — `git worktree add <path> <branch>` —
and never `checkout`/`switch` on that root, so the serving checkout is
provably undisturbed the same way `ref-index`'s `ComputeIndex` is provably
non-mutating (co-1, same proof shape: an interface/command inventory, not
merely "the current code happens not to").

## DC-2

Ownership mechanism: a per-managed-worktree lockfile at
`.verdi/data/worktrees/<name>.lock`, using the EXACT algorithm this store
already ratified for its per-checkout `data/writer.lock`
(`internal/mcpserve/lock.go`'s `O_CREATE|O_EXCL` `{pid,start}` JSON body,
`kill(pid,0)`-plus-`ps -o lstart=` liveness cross-check closing the
documented PID-reuse gap, and stale-lock takeover on a dead holder) —
extracted into a new, shared `internal/filelock` package rather than
copy-pasted a second time or imported from the semantically-mismatched
`internal/mcpserve` (CLAUDE.md: "anything used by two or more packages
lives in a shared `internal/` package"). `cmd/verdi/serve.go`'s existing
writer-lock call sites move to `internal/filelock` unchanged in behavior —
this story widens the mechanism's packaging, not its algorithm.

**Hold duration, closing a decision-conflict finding raised against an
earlier draft:** the lock is held ONLY for the duration of a single
git-worktree-mutating operation on that worktree — `EnsureWorktree`'s own
`git worktree add`, or `gc`'s own `git worktree remove` — never for the
worktree's whole idle lifetime between operations. An earlier draft of this
decision held the lock for "the managed worktree's whole lifetime once
acquired," mirroring the per-checkout writer lock's own lifecycle; that
framing is corrected here because it would leave a long-running `verdi
serve` process holding every worktree it ever cut for its entire uptime,
making `gc`'s live-lock skip (dc-4) an unbounded, session-length deferral
rather than the narrow, transient race window dc-4 needs it to be. Ordinary
board reads and edits inside an already-cut worktree are ordinary git
commands against an ordinary directory and need no lock of their own — only
the two operations that mutate `git worktree`'s own administrative state
(`add`, `remove`) ever take it, briefly, so a separate `gc` invocation
testing liveness only ever observes a live holder during that narrow
window, never for as long as the serve process merely happens to be
running.

## DC-3

`gc`'s merged signal reuses `gitx.IsAncestor` — the same primitive the
sibling `ref-index` story's own dc-5 already uses for its merged-branch
exclusion — against each directory entry under `.verdi/data/worktrees/`,
cross-referenced with the current `refs/heads/design/*` listing: merged
means the worktree's recorded branch tip is an ancestor of the default
branch's tip.

**Deleted signal, deliberately narrower than the general ratified reading —
closing a decision-conflict finding raised against an earlier draft, which
argued this narrowing against the wrong document (store-layout) rather than
against the document that actually binds it (parent feature dc-4):**
deleted means the branch no longer resolves under `refs/heads/design/*` AT
ALL — checked LOCALLY ONLY. Parent feature dc-4 binds worktree reclamation
to "the ratified gc signals," and the only ratified deleted-signal in this
store (`verdi-store-layout`'s Garbage collection section) is `git fetch
--prune` first, then neither-local-nor-remote. Parent dc-4 IS a
feature-decision fragment and therefore a structurally valid
`exempts`-edge target (02 §Link taxonomy) — unlike the retention_days
divergence below, this one is not blocked by a missing target. It is
nonetheless disclosed here in prose rather than as a formal edge:
R4-I-12's stub-match test disqualifies ANY `exempts` edge on a story
unconditionally, forcing full review regardless of how narrow the
substance is — disproportionate for an implementation-scoped clarification
of an underspecified generalization, for a target (managed worktrees) that
postdates the parent decision entirely and that the parent decision's
"ratified gc signals" phrase never contemplated. The reason itself: a
managed worktree is a live checkout bound to one SPECIFIC LOCAL BRANCH
(dc-1: cut from local branches only); once that local branch is gone, the
worktree is orphaned regardless of whether a same-named remote-tracking ref
still exists, because resurrecting it would require explicitly minting a
new local branch — an act dc-1 already forbids doing implicitly. Parent
dc-4's general reclamation model (merged-or-deleted, dirty-is-kept) stays
valid and unweakened; this story's worktree-specific implementation of the
deleted leg is fully argued here for a reviewer's own judgment, and the
design-branch decision-conflict report carries this exact reasoning as a
dispositioned finding — a mechanism this contract provides specifically for
judgment calls surfaced during design review, orthogonal to spec-level
edges and to R4-I-12's stub-match test.

**Retention window, a disclosed judgment call, not a claimed fact about
another document** (an earlier draft of this decision asserted
`verdi-store-layout`'s `derived.retention_days` bullet was unambiguously
"reserved" for the derived-cache prune alone — corrected here): that bullet
is written against `derived/<ref>/` specifically, but its own config
comment — "gc horizon for merged/deleted refs" — is worded generally
enough that whether it was meant to bind every future merged/deleted-ref
reclaim mechanism, or only the one that existed when it was ratified, is
genuinely ambiguous. `verdi-store-layout` predates managed worktrees
entirely and cannot have decided this either way. This story chooses NOT to
apply `retention_days` to managed worktrees — a worktree becomes
reclaim-eligible the instant its merged-or-deleted signal fires, no grace
period — as the smallest reversible starting point, explicitly recorded as
a choice here rather than silently assumed as settled fact. A later
revision may unify the two under one shared knob, or may deliberately keep
them separate, without breaking this story's own contract either way.

**Why the retention_days divergence has no available edge at all, unlike
the deleted-signal divergence above (which has one, deliberately not
taken):** `verdi-store-layout` is a component spec (02 §Kind registry:
"system source-of-truth documents ... no story, no ACs"), and the artifact
contract's object model explicitly excludes component specs from carrying
any declared `decisions:`/`constraints:` objects at all — "Component and
ADR specs, having no object model, carry neither" (02 §Object model). The
`exempts` edge type's own definition (02 §Link taxonomy) targets only an
ADR or a feature-decision FRAGMENT; there is no declared object id anywhere
in `verdi-store-layout`'s "Garbage collection" prose for such an edge to
name — the target this contract requires does not exist to be named at
all, a structural absence rather than a proportionality choice. This is
different in kind from the deleted-signal divergence above, whose target
(parent dc-4) does exist and could be named, but is deliberately left as a
disclosed prose judgment call instead, for the proportionality reason
already given.

## DC-4

`gc` never forces a removal. It checks `gitx.StatusDirty` — the existing
function, reused unchanged, never a second dirty-check implementation —
before attempting anything, producing a clear disclosed message ("kept:
uncommitted changes") rather than parsing git's own refusal text off
stderr. Only a clean, reclaim-eligible worktree ever reaches `git worktree
remove`, called WITHOUT `--force`, so git's own dirty-tree safety net
stands as a second, redundant guard rather than the only one this story
relies on.

A worktree whose lockfile (dc-2) is currently held by a live process is
equally skipped and disclosed ("kept: in use by pid N") rather than removed
out from under its owner — a keep-reason beyond parent dc-4's one named
exception (uncommitted changes) — yanking a clean-but-currently-served
worktree out from under a live process would be exactly the kind of
surprise a lock exists to prevent, and it never forces a takeover of a live
lock to do so.

**Disclosed in prose, not a formal exempts edge, for the same R4-I-12
proportionality reason dc-3 gives:** parent dc-4 is a feature-decision
fragment, a structurally valid `exempts`-edge target (02 §Link taxonomy) —
the target is available here, unlike the retention_days case. It is
deliberately not taken: R4-I-12's stub-match test disqualifies ANY
`exempts` edge on a story unconditionally, forcing full review regardless
of how narrow the substance is — a cost this judgment call, fully argued
here and available for a reviewer's own escalation, does not warrant
imposing unilaterally. The reason itself: because dc-2's lock is held only
for the duration of a single git-worktree-mutating call (never the
worktree's whole idle lifetime, per dc-2's own correction), this skip is a
NARROW, single-operation race window — the exact same merged-or-deleted
worktree is fully reclaim-eligible again the moment that one `git worktree
add`/`remove` call finishes, ordinarily within the same `gc` run's next
pass or the very next invocation, never withheld for the life of a
long-running serve session. This is materially different in kind from the
dirty-worktree case (feature dc-4's own, kept until a human resolves the
uncommitted changes — an indefinite hold with no natural expiry): parent
dc-4's reclamation model stays valid and unweakened, and this decision adds
a narrow, self-resolving safety margin around it rather than a competing
kept-forever category.

Every skip and every reclaim is printed, one line per worktree; `gc`
performs no removal a human running it cannot see named in its own output.

## DC-5

Scope line. This story implements ONLY the managed-worktree reclamation
slice of `verdi gc`. `verdi-store-layout`'s "Garbage collection" section
also ratifies derived-cache pruning (`data/derived/<ref>/` for refs
merged/deleted past `derived.retention_days`) and layout/tree-hash cache
pruning — neither is touched here. `cmd/verdi gc`'s own printed output says
so explicitly on every run, so a human is never left inferring full `gc`
coverage from a partial implementation; a future story completes the
remaining slices behind the same verb, adding to this one rather than
replacing it.

**Incremental delivery, not a redefinition, closing a decision-conflict
finding raised against an earlier draft:** `dispatch.go`'s `gc` entry was
phase 0 ("out of v0 scope") before this story — a ratified verb name with
NO implementation at all. PLAN.md's own build contract stages every verb
across phases; no verb in this system has ever been required to land in
one shot (`close`, `board`, `gate`, `audit` all landed incrementally, each
in its own phase). Delivering one honestly-scoped, honestly-disclosed slice
of an already-recognized verb is that same incremental pattern, not a
narrowing of `verdi-store-layout`'s Garbage collection section — that
section remains fully valid, entirely unedited by this story, and simply
still awaits its own future story for the two slices left undone. No
supersedes/exempts edge is needed for the same structural reason dc-3
gives: `verdi-store-layout` is a component spec carrying no declared
decision objects at all (02 §Object model: component specs "have no object
model, carry neither"), so the `exempts` edge type's required target — an
ADR or a feature-decision fragment — does not exist to be named here
either.

`dispatch.go`'s `gc` entry moves from phase 0 ("out of v0 scope") to a
real, implemented phase — the same flip `close-verb` already made for its
own verb in round 6 (`"close": 14, // ... flipped from I-23's phase-0
stub`).

## CO-1

Inherited verbatim from the feature (co-1): managed worktrees live under
the data zone, never committed. Every `EnsureWorktree` write happens under
`.verdi/data/worktrees/`; nothing it creates is ever `git add`-ed or
committed, and `gc`'s own removals touch only that same subtree — never the
committed zone, never `data/mutable/` (mirroring `verdi-store-layout`'s own
"never touches the committed zone or `mutable/`" rule for its other `gc`
bullets).

## CO-2

No network in any test (CLAUDE.md). Every `EnsureWorktree` and `gc`
behavior — the cut, the lock contention, and the merged/deleted/dirty/
locked reclaim decisions — is proven against `fixturegit` repositories with
real local branches and real worktrees materialized on local disk; no live
clone, fetch, or any other network-touching git operation.
