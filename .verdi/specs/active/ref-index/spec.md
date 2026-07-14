---
id: spec/ref-index
kind: spec
title: "Ref Index"
owners: [platform-team]
class: story
status: draft
story: jira:VERDI-18
problem: { text: "spec/workbench-directory ac-2 requires the home directory to list every spec on the default branch and every draft on a design branch, grouped and status-chipped, computed deterministically from git refs — but no code computes this today. verdi serve only ever knows about the one working tree it is bound to. Deciding which refs count, what a design branch with no draft spec looks like, and how status is derived, are all backend seam questions the directory-home page cannot honestly answer for itself.", anchor: problem }
outcome: { text: "an internal package exposes a pure ComputeIndex function that, given only git refs (no checkout switch, ever - feature co-1), returns a deterministic index of every default-branch spec and every design branch's draft, each entry carrying its source (local/remote), its computed status-group (feature dc-2's vocabulary), and - for a design branch with no draft spec - a disclosed entry rather than an omission. directory-home renders this output; it invents none of the computation.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "ComputeIndex enumerates the default branch's specs (walking the tree at the default-branch ref) and, separately, every design branch's draft spec (reading refs/heads/design/* at HEAD via ref-scoped plumbing only - git show <ref>:<path> and for-each-ref, never checkout), returning one entry per spec/draft with no ref read twice and no ref silently skipped", evidence: [static, behavioral], anchor: "#ac-1" }
  - { id: ac-2, text: "local refs/heads/design/* and remote-tracking refs/remotes/origin/design/* both join the enumeration (feature dc-5): an entry that exists as both a local and a remote-tracking ref is a single entry disclosing both sources; an entry that exists only remotely is disclosed as remote-only; the deterministic, refs-only property holds for both sources", evidence: [static, behavioral], anchor: "#ac-2" }
  - { id: ac-3, text: "every entry's status chip is computed from the spec content reachable at that ref (never the working tree) - the default branch's specs chip by their own frontmatter status field, and a design branch's draft chips drafts-in-progress (feature dc-2's vocabulary: drafts in progress / accepted-pending-build / active components / terminal) - grouping is a pure function of the entries, never an incidental ordering", evidence: [static, behavioral], anchor: "#ac-3" }
  - { id: ac-4, text: "a design branch ref that resolves but carries no spec.md reachable at <ref>:.verdi/specs/active/<name>/spec.md (no draft has been authored yet on that branch, or the branch predates a scaffold) yields a disclosed entry - a defined field on that entry's output, never a Go error, a panic, or a silent drop from the returned slice - feeding directory-home's ac-5 notice", evidence: [static, behavioral], anchor: "#ac-4" }
  - { id: ac-5, text: "ComputeIndex never execs git checkout, git switch, or any command that moves HEAD or writes the working tree/index of the directory it is given - proven by an interface-level guarantee (the git runner port ComputeIndex depends on exposes no such method) plus a behavioral test asserting the serving checkout's HEAD/working tree are byte-identical before and after a ComputeIndex run", evidence: [static, behavioral], anchor: "#ac-5" }
links:
  - { type: implements, ref: "spec/workbench-directory#ac-2" }
decisions:
  - { id: dc-1, text: "backend seam only: a new internal/refindex package exposes ComputeIndex(ctx, root, deps) ([]Entry, error) as a pure function of ref state - no HTTP handler, no page, no rendering. directory-home (a separate stub under the same feature) is the only consumer that turns Entry values into markup; ref-index's output type is designed so that story never needs to touch git plumbing itself", anchor: "#dc-1" }
  - { id: dc-2, text: "a consumer-defined git runner port, not the concrete internal/gitx functions directly: internal/refindex declares its own narrow interface (list local design refs, list remote-tracking design refs, resolve the default branch, show a path at a ref, list the default branch's spec directories at that ref) that internal/gitx's existing free functions (LocalBranches, Show, DefaultBranch - gitx/branch.go, gitx/show.go) satisfy via a small adapter; new plumbing this story needs (a for-each-ref query scoped to refs/remotes/origin/design/*, and a tree-listing at a ref) is added to gitx as more of the same shape, not invented ad hoc inside refindex. The port exists so ComputeIndex is testable against a fake with no real git process at all, alongside the hermetic fixturegit exercise (04 §port pattern)", anchor: "#dc-2" }
  - { id: dc-3, text: "the Entry output type carries {Ref (kind/name-shaped local identity), Source (enum: local | remote | both), StatusGroup (feature dc-2's four-value vocabulary), SpecStatus (the raw frontmatter status where a spec was readable, empty otherwise), Disclosed (*disclosure.Disclosure, nil when the entry is ordinary) - reusing internal/disclosure's existing shared shape (disclosure.New/disclosure.Render) for the no-draft-spec and any other degraded case, rather than a bespoke ad hoc string, so directory-home's later disclosed-notice rendering (ac-5's degrade-to-notice requirement) is the same vocabulary every other disclosure in this store already renders in", anchor: "#dc-3" }
  - { id: dc-4, text: "default-branch enumeration walks the default branch's OWN tree at its resolved ref (git ls-tree under .verdi/specs/active/ and .verdi/specs/archive/ at that ref, mirroring internal/index's existing corpus-walk shape but ref-scoped rather than working-tree-scoped) rather than reusing the live corpus index (internal/index), because the live index reads the working tree/checkout the serving process happens to be on - exactly the coupling co-1 forbids for index computation. A future consolidation of the two walkers is left open, not invented here", anchor: "#dc-4" }
constraints:
  - { id: co-1, text: "inherited verbatim from the feature (co-1): managed worktrees live under the data zone, never committed (not this story's concern - see worktree-manager); index computation reads refs and never switches a checkout. ComputeIndex takes the serving checkout's root only to resolve .git and run ref-scoped plumbing against it - it never runs checkout, switch, or any working-tree-mutating command against that root or any other", anchor: "#co-1" }
  - { id: co-2, text: "no network in any test (CLAUDE.md): every ComputeIndex behavior is proven against a fixturegit repository carrying real local and (simulated) remote-tracking design refs, or against the fake git-runner-port double from dc-2 - never a live clone or fetch", anchor: "#co-2" }
---
# Ref Index

## Problem

`spec/workbench-directory` ac-2 requires the home directory to list every
spec on the default branch and every draft on a design branch, grouped and
status-chipped, computed deterministically from git refs — but no code
computes this today. `verdi serve` only ever knows about the one working
tree it is bound to: it has no notion of "every design branch's draft" at
all. Deciding which refs count (local only? remote-tracking too?), what a
design branch with no draft spec looks like, and how status is derived from
ref-reachable content rather than the live working tree, are all backend
seam questions the directory-home page cannot honestly answer for itself —
they belong to a computation the page merely renders.

## Outcome

An internal package exposes a pure `ComputeIndex` function that, given only
git refs — never switching a checkout (feature co-1) — returns a
deterministic index of every default-branch spec and every design branch's
draft. Each entry carries its source (local, remote, or both — feature
dc-5), its computed status group (feature dc-2's vocabulary: drafts in
progress / accepted-pending-build / active components / terminal), and, for
a design branch with no draft spec reachable, a disclosed entry rather than
a silent omission. `directory-home` (a sibling stub under the same feature)
is the only consumer that turns this output into markup; this story invents
no rendering and no HTTP surface.

## AC-1

`ComputeIndex` enumerates the default branch's specs by walking the tree
reachable at the default-branch ref (resolved via `gitx.DefaultBranch`,
falling back honestly per that function's own contract when unconfigured),
and — separately — every design branch's draft spec by reading
`refs/heads/design/*` at each ref's current tip via ref-scoped plumbing
only: `git show <ref>:<path>` (`gitx.Show`) and `for-each-ref`-style listing
(`gitx.LocalBranches`'s shape, scoped to `refs/heads/design`), never
`git checkout` or `git switch`. Every ref is read exactly once; a ref
that fails to resolve at all (a documented git-level error, not "no spec
present") propagates as a real Go error rather than a silently-skipped
entry. Evidence: static (the function signature takes no checkout-mutating
dependency) and behavioral (a fixturegit repo with a default branch and two
design branches proves one entry per branch, no duplicates, no drops).

## AC-2

Local `refs/heads/design/*` and remote-tracking `refs/remotes/origin/design/*`
both join the enumeration, per feature dc-5's resolution of oq-2: an entry
whose branch exists both locally and as a remote-tracking ref is a single
entry disclosing both sources (`Source: both`); an entry that exists only as
`refs/remotes/origin/design/*` (never fetched down to a local branch, or a
teammate's still-open draft) is disclosed remote-only (`Source: remote`).
The deterministic, refs-only property (ac-1) holds identically for both
sources — remote-tracking refs are read exactly the same way local ones
are, through the same git-runner-port methods, never a network call. Only
entries under the `design/` namespace on either side are read; other
remote-tracking refs (build branches, tags) are out of scope for this
index. Evidence: static (the enumeration reads both ref namespaces through
one shared code path) and behavioral (a fixturegit repo with a local-only
design branch, a remote-only design branch, and a branch present as both
proves all three `Source` values, byte-stable across repeated runs).

## AC-3

Every entry's status chip is computed from the spec content reachable at
that ref, never the working tree of the checkout `ComputeIndex` is given.
Default-branch entries chip by their own frontmatter `status:` field (read
via `gitx.Show` at the default-branch ref, decoded through the existing
`internal/artifact` strict-decode seam — never a second, bespoke YAML
parser). A design branch's draft entry always chips `drafts-in-progress`
(feature dc-2's vocabulary: drafts in progress / accepted-pending-build /
active components / terminal) regardless of the draft's own frontmatter
`status:` field — a design branch is definitionally a draft in progress
until its spec MR merges, matching 03 §Lifecycle's two-MR model this store
already ratifies. Grouping (the four-bucket partition `directory-home` will
render) is a pure function mapping each entry's computed group, never an
incidental slice-order artifact — two calls against identical ref state
produce byte-identical groupings. Evidence: static (the four-value
StatusGroup enum, fail-closed on an unrecognized frontmatter status per
CLAUDE.md's "unknown enum values fail closed") and behavioral (a fixturegit
repo mixing an active component, an accepted-pending-build story, and a
design-branch draft proves each entry lands in its ratified group).

## AC-4

A design branch ref that resolves (the branch exists) but carries no
`spec.md` reachable at `<ref>:.verdi/specs/active/<name>/spec.md` — no draft
has been authored yet on that branch (fresh off `design start` before the
first spec commit lands, or an older branch that never got one) — yields a
disclosed entry: a populated `Disclosed *disclosure.Disclosure` field on
that entry (dc-3), constructed through the existing `internal/disclosure`
seam (`disclosure.New`), never a Go error returned from `ComputeIndex`,
never a panic, and never a silent absence from the returned entry slice.
This is the mechanism ac-5 of the parent feature spec (consumed by
`directory-home`, out of this story's scope) turns into a notice rather
than a dead link or an omitted row. Evidence: static (the field exists and
is populated only in this branch of the logic, nil otherwise per dc-3) and
behavioral (a fixturegit repo with a design branch created but never
committed a spec proves one disclosed entry, distinguishable from an
ordinary draft entry, and the run still returns `nil` error).

## AC-5

`ComputeIndex` never execs `git checkout`, `git switch`, or any command that
moves `HEAD` or writes the working tree or index of the directory it is
given — the hard requirement co-1 restates for this story specifically.
Proven two ways: statically, the git-runner-port interface `ComputeIndex`
depends on (dc-2) exposes no method that could perform such a mutation — it
is impossible to call, not merely undocumented; and behaviorally, a test
snapshots the serving checkout's `HEAD` ref and a hash of its working tree
before invoking `ComputeIndex` against a repo carrying multiple design
branches, then asserts both are byte-identical afterward. Evidence: static
(the port's method set, read directly) and behavioral (the before/after
snapshot assertion).

## DC-1

Backend seam only. A new `internal/refindex` package exposes
`ComputeIndex(ctx, root, deps) ([]Entry, error)` as a pure function of ref
state — no HTTP handler, no page template, no rendering logic. The
`directory-home` stub (a sibling story under this same feature) is the only
consumer that turns `Entry` values into the home page's markup; this
story's job is to design `Entry` richly enough that `directory-home` never
needs to reach back into git itself.

## DC-2

A consumer-defined git runner port, not `internal/gitx`'s existing free
functions called directly. `internal/refindex` declares its own narrow
interface — list local design refs, list remote-tracking design refs,
resolve the default branch, read a path's content at a ref, list the
default branch's spec directories at that ref — that a small adapter over
`internal/gitx`'s existing functions (`LocalBranches`, `Show`,
`DefaultBranch` — `gitx/branch.go`, `gitx/show.go`) satisfies. The two
plumbing primitives this story needs that `gitx` does not yet have — a
`for-each-ref`-style query scoped to `refs/remotes/origin/design/*`, and a
tree-listing (`git ls-tree`) at an arbitrary ref — are added to `gitx` as
more of the same shape (a thin wrapper over one `git` invocation, returning
parsed, deterministic output), never invented ad hoc inside `refindex`. The
port exists precisely so `ComputeIndex` is unit-testable against an
in-process fake with no real `git` process at all, in addition to the
hermetic `fixturegit` exercise — the 04 §port pattern this store already
follows everywhere else a real subprocess or network boundary sits behind
an interface.

## DC-3

The `Entry` output type is:

```go
type Entry struct {
    Ref         string // "spec/<name>" - the canonical kind/name identity
    Source      Source // local | remote | both
    StatusGroup string // feature dc-2's four-value vocabulary
    SpecStatus  string // the raw frontmatter status, where a spec was readable; "" otherwise
    Disclosed   *disclosure.Disclosure // non-nil only for a degraded entry (ac-4)
}
```

`Disclosed` reuses `internal/disclosure`'s existing shared shape
(`disclosure.New`, `disclosure.Render`) rather than a bespoke ad hoc
string, for the no-draft-spec case (ac-4) and any other degraded case this
story's implementer discovers — so `directory-home`'s later disclosed-notice
rendering (the parent feature's ac-5) speaks the same vocabulary every
other disclosure in this store already renders in, rather than inventing a
second one.

## DC-4

Default-branch enumeration walks the default branch's own tree at its
resolved ref — a `git ls-tree`-based listing under `.verdi/specs/active/`
and `.verdi/specs/archive/` at that ref, in the same shape
`internal/index`'s existing corpus walk (`internal/index/walk.go`) already
uses, but ref-scoped rather than working-tree-scoped — rather than reusing
the live corpus index directly. The live index reads the working
tree/checkout the serving process happens to be on, which is exactly the
checkout-coupling co-1 forbids for index computation: a directory index
that silently depended on which branch `verdi serve` currently has checked
out would reintroduce the per-draft-port problem one level down. A future
consolidation of the two walkers behind one ref-or-working-tree-parameterized
primitive is a plausible follow-up, left open rather than invented here —
the smallest reversible option is two small walkers, not one prematurely
generalized one.

## CO-1

Inherited verbatim from the feature (co-1): managed worktrees live under
the data zone, never committed (not this story's concern — see
`worktree-manager`); index computation reads refs and never switches a
checkout. `ComputeIndex` takes the serving checkout's root only to resolve
`.git` and run ref-scoped plumbing against it — it never runs `checkout`,
`switch`, or any working-tree-mutating command against that root or any
other repository.

## CO-2

No network in any test (CLAUDE.md). Every `ComputeIndex` behavior is proven
against a `fixturegit` repository carrying real local design branches and
simulated remote-tracking refs (a second bare repo fetched into the fixture,
or `refs/remotes/origin/design/*` refs created directly at known commits —
either is hermetic), or against the fake git-runner-port double from dc-2 —
never a live clone, fetch, or any other network-touching git operation.
