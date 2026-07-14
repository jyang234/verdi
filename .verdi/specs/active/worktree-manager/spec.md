---
id: spec/worktree-manager
kind: spec
title: "Worktree Manager"
owners: [platform-team]
class: story
status: draft
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
  - { id: dc-2, text: "ownership mechanism: a per-managed-worktree lockfile at .verdi/data/worktrees/<name>.lock, using the EXACT algorithm this store already ratified for its per-checkout data/writer.lock (internal/mcpserve/lock.go's O_CREATE|O_EXCL {pid,start} JSON, kill(pid,0)-plus-ps -o lstart= liveness cross-check closing the PID-reuse gap, stale-lock takeover) - extracted into a new, shared internal/filelock package (CLAUDE.md: anything used by two or more packages lives in a shared internal/ package) rather than copy-pasted a second time or imported from the semantically-mismatched internal/mcpserve. cmd/verdi/serve.go's existing writer-lock call sites move to internal/filelock unchanged in behavior. The lock is held for the managed worktree's whole lifetime once acquired by its owning serve process (mirroring the per-checkout lock's own lifecycle), not merely during the cut - so gc (a separate process invocation) can test liveness before ever touching a worktree", anchor: "#dc-2" }
  - { id: dc-3, text: "gc's merged-or-deleted signal reuses gitx.IsAncestor (the same primitive the sibling ref-index story's dc-5 already uses for its own merged-branch check) against each directory entry under .verdi/data/worktrees/ cross-referenced with the current refs/heads/design/* listing: merged = the worktree's branch tip is an ancestor of the default-branch tip; deleted = the branch no longer resolves at all. No retention_days grace period applies to managed worktrees (unlike verdi.yaml's derived.retention_days, which the store-layout spec's OTHER gc bullet already reserves for the derived-cache prune, a different mechanism this story does not touch) - a worktree becomes reclaim-eligible the instant its signal fires, the plain reading of feature dc-4's text, which names no time buffer for worktrees specifically; a future revision may add one without breaking this contract, the smallest reversible starting point", anchor: "#dc-3" }
  - { id: dc-4, text: "gc never forces a removal. It checks gitx.StatusDirty (existing function, reused unchanged) before attempting anything, producing a clear disclosed message ('kept: uncommitted changes') rather than parsing git's own refusal text; only a clean, reclaim-eligible worktree reaches git worktree remove, called WITHOUT --force, so git's own dirty-tree safety net is a second, redundant guard rather than the only one. A worktree whose lockfile (dc-2) is currently held by a live process is equally skipped and disclosed ('kept: in use by pid N') rather than removed out from under its owner. No exempts edge is needed against parent dc-4 for this: the lock-liveness skip is TEMPORARY and this-run-only, never a permanent exemption from dc-4's reclamation contract - the exact same merged/deleted worktree remains fully reclaim-eligible on the very next gc invocation once its owner exits and the lock goes stale (unlike the dirty-worktree case, which stays kept until a HUMAN resolves it). Feature dc-4 already establishes that gc is a deliberately-invoked, non-daemon process running on ITS OWN schedule, never instantaneous - a live-lock skip is that same 'runs on its own schedule, not a background process' property applied to a single worktree that happens to be mid-use exactly when this particular invocation ran, not a new kept-forever category dc-4 would need to bless. Every skip and every reclaim is printed, one line per worktree; gc performs no removal a human cannot see in its own output", anchor: "#dc-4" }
  - { id: dc-5, text: "scope line: this story implements ONLY the managed-worktree reclamation slice of verdi gc. verdi-store-layout's Garbage collection section also ratifies derived-cache pruning (data/derived/<ref>/ for refs merged/deleted past derived.retention_days) and layout/tree-hash cache pruning - neither is touched here; cmd/verdi gc's own printed output says so explicitly on every run, so a human is never left inferring full gc coverage from a partial implementation. dispatch.go's gc entry moves from phase 0 (out of v0 scope) to a real, implemented phase, the same flip close-verb already made for its own verb in round 6", anchor: "#dc-5" }
constraints:
  - { id: co-1, text: "inherited verbatim from the feature (co-1): managed worktrees live under the data zone, never committed. EnsureWorktree's every write happens under .verdi/data/worktrees/; nothing it creates is ever git-added or git-committed, and gc's own removals touch only that same subtree", anchor: "#co-1" }
  - { id: co-2, text: "no network in any test (CLAUDE.md): every EnsureWorktree and gc behavior - the cut, the lock contention, the merged/deleted/dirty/locked reclaim decisions - is proven against fixturegit repositories with real local branches and real worktrees on local disk; no live clone or fetch", anchor: "#co-2" }
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

The lock is held for the managed worktree's WHOLE LIFETIME once acquired by
its owning process — mirroring the per-checkout lock's own lifecycle —
rather than only during the cut, so a separate `gc` invocation can test
liveness before ever touching a worktree (ac-3, ac-4), the same way the
existing per-checkout lock lets `verdi mcp` decide whether to proxy or
serve standalone.

## DC-3

`gc`'s merged-or-deleted signal reuses `gitx.IsAncestor` — the same
primitive the sibling `ref-index` story's own dc-5 already uses for its
merged-branch exclusion — against each directory entry under
`.verdi/data/worktrees/`, cross-referenced with the current
`refs/heads/design/*` listing: merged means the worktree's recorded branch
tip is an ancestor of the default branch's tip; deleted means the branch no
longer resolves to anything at all. No `retention_days` grace period
applies to managed worktrees, unlike `verdi.yaml`'s `derived.retention_days`
(which `verdi-store-layout`'s OTHER `gc` bullet reserves for the
derived-cache prune — a different mechanism this story does not touch): a
managed worktree becomes reclaim-eligible the instant its merged-or-deleted
signal fires, the plain reading of feature dc-4's text, which names no time
buffer for worktrees specifically. A future revision may add a grace period
without breaking this contract — the smallest reversible starting point,
not a value derived from data (mirroring `verdi.yaml`'s own documented
posture for its other tunables).

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
out from under its owner — yanking a clean-but-currently-served worktree
out from under a live process would be exactly the kind of surprise a lock
exists to prevent, and it never forces a takeover of a live lock to do so.

**No exempts edge against parent dc-4, closing a decision-conflict finding
raised against an earlier draft:** the lock-liveness skip is TEMPORARY and
this-run-only, never a permanent exemption from feature dc-4's
reclamation contract. The exact same merged-or-deleted worktree stays
fully reclaim-eligible on the very next `gc` invocation, the moment its
owning process exits and the lock goes stale — unlike the dirty-worktree
case (feature dc-4's own, which stays kept until a human resolves the
uncommitted changes, an indefinite hold). Feature dc-4 already establishes
`gc` as a deliberately-invoked process with no background daemon, running
on its own schedule rather than continuously; a live-lock skip is that
exact "runs on its own schedule" property applied to a single worktree that
happens to be mid-use precisely when one particular invocation ran — not a
new permanently-kept category the parent spec would need to bless, only an
ordinary missed cycle.

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
replacing it. `dispatch.go`'s `gc` entry moves from phase 0 ("out of v0
scope") to a real, implemented phase — the same flip `close-verb` already
made for its own verb in round 6 (`"close": 14, // ... flipped from I-23's
phase-0 stub`).

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
