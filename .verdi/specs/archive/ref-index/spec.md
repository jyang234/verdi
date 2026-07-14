---
id: spec/ref-index
kind: spec
title: "Ref Index"
owners: [platform-team]
class: story
status: closed
story: jira:VERDI-18
problem: { text: "spec/workbench-directory ac-2 requires the home directory to list every spec on the default branch and every draft on a design branch, grouped and status-chipped, computed deterministically from git refs — but no code computes this today. verdi serve only ever knows about the one working tree it is bound to. Deciding which refs count, what a design branch with no draft spec looks like, and how status is derived, are all backend seam questions the directory-home page cannot honestly answer for itself.", anchor: problem }
outcome: { text: "an internal package exposes a pure ComputeIndex function that, given only git refs (no checkout switch, ever - feature co-1), returns a deterministic index of every default-branch spec and every design branch's draft, each entry carrying its source (local/remote), its computed status-group (feature dc-2's vocabulary), and - for a design branch with no draft spec - a disclosed entry rather than an omission. directory-home renders this output; it invents none of the computation.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "ComputeIndex enumerates the default branch's specs (walking the tree at the default-branch ref) and, separately, every UNMERGED design branch's draft spec (reading refs/heads/design/* at HEAD via ref-scoped plumbing only - git show <ref>:<path> and for-each-ref, never checkout; a design branch already merged into the default branch is excluded per dc-5, its spec already present as a default-branch entry), returning one entry per spec/draft with no ref read twice, no ref silently skipped, and no spec double-counted across the two walks", evidence: [static, behavioral], anchor: "#ac-1" }
  - { id: ac-2, text: "local refs/heads/design/* and remote-tracking refs/remotes/origin/design/* both join the enumeration (feature dc-5): an entry that exists as both a local and a remote-tracking ref is a single entry disclosing both sources; an entry that exists only remotely is disclosed as remote-only; the deterministic, refs-only property holds for both sources", evidence: [static, behavioral], anchor: "#ac-2" }
  - { id: ac-3, text: "every entry's status chip is computed from the spec content reachable at that ref (never the working tree) - the default branch's specs chip by their own frontmatter status field, and a design branch's draft chips drafts-in-progress (feature dc-2's vocabulary: drafts in progress / accepted-pending-build / active components / terminal) UNCONDITIONALLY - every design-branch entry, ordinary or degraded (ac-4), is drafts-in-progress by definition of being on a design branch, never derived from readable content - grouping is a pure function of the entries, never an incidental ordering", evidence: [static, behavioral], anchor: "#ac-3" }
  - { id: ac-4, text: "a design branch ref that resolves but carries no spec.md reachable at <ref>:.verdi/specs/active/<name>/spec.md (no draft has been authored yet on that branch, or the branch predates a scaffold) yields a disclosed entry - a defined field on that entry's output, never a Go error, a panic, or a silent drop from the returned slice - feeding directory-home's ac-5 notice", evidence: [static, behavioral], anchor: "#ac-4" }
  - { id: ac-5, text: "ComputeIndex never execs git checkout, git switch, or any command that moves HEAD or writes the working tree/index of the directory it is given - proven by an interface-level guarantee (the git runner port ComputeIndex depends on exposes no such method) plus a behavioral test asserting the serving checkout's HEAD/working tree are byte-identical before and after a ComputeIndex run", evidence: [static, behavioral], anchor: "#ac-5" }
links:
  - { type: implements, ref: "spec/workbench-directory#ac-2" }
decisions:
  - { id: dc-1, text: "backend seam only: a new internal/refindex package exposes ComputeIndex(ctx, root, deps) ([]Entry, error) as a pure function of ref state - no HTTP handler, no page, no rendering, and no forge/network call of any kind. directory-home (a separate stub under the same feature) is the only consumer that turns Entry values into markup; ref-index's output type is designed so that story never needs to touch git plumbing itself. Explicit scope boundary, closing a decision-conflict finding against this draft: parent dc-5's forge-sourced in-review chip is, by dc-5's own words, 'a second, non-ref source' layered on top of the refs-computed directory - it is not part of ac-2's 'computed deterministically from git refs' claim this story implements, and Entry (dc-3) deliberately carries no forge/MR field. Composing that second source onto ComputeIndex's output is directory-home's job (it implements both ac-2 and ac-5); this is a disclosed scope line, not an oversight, and no exempts edge is needed because nothing here contradicts dc-5 - it only implements the ref-only half dc-5 itself distinguishes", anchor: "#dc-1" }
  - { id: dc-2, text: "a consumer-defined git runner port, not the concrete internal/gitx functions directly: internal/refindex declares its own narrow interface (list local design refs, list remote-tracking design refs, resolve the default branch, show a path at a ref, list the default branch's spec directories at that ref, test whether one commit is an ancestor of another - dc-5's merged-branch check, reusing gitx.IsAncestor's existing primitive) that internal/gitx's existing free functions (LocalBranches, Show, DefaultBranch - gitx/branch.go, gitx/show.go) satisfy via a small adapter; new plumbing this story needs (a for-each-ref query scoped to refs/remotes/origin/design/*, and a tree-listing at a ref) is added to gitx as more of the same shape, not invented ad hoc inside refindex. The port exists so ComputeIndex is testable against a fake with no real git process at all, alongside the hermetic fixturegit exercise (04 §port pattern). The remote name is hardcoded to 'origin', matching this codebase's existing single-remote convention verbatim - gitx.DefaultBranch already resolves refs/remotes/origin/HEAD, gitx.Push already pushes to 'origin', and gitx.HasRemote's callers already treat 'origin' as the one configured remote (gitx/branch.go, gitx/worktree.go) - so this is not a new narrowing this story invents; it is consistent with every other remote-naming assumption already load-bearing in this store. A repo with a differently-named remote is already unsupported by those existing functions, not newly unsupported by this one. Parent dc-5 never mandates multi-remote support - it says only that local and remote-tracking design refs 'alike' join the enumeration, silent on remote naming - so hardcoding origin fills a gap dc-5 leaves open using this store's one existing convention, rather than narrowing an explicit dc-5 promise; no exempts edge is warranted because nothing in dc-5's text is being excused from, only an unstated detail being filled in the same way the rest of the codebase already fills it", anchor: "#dc-2" }
  - { id: dc-3, text: "the Entry output type carries {Ref (kind/name-shaped local identity), Source (enum: default | local | remote | both), StatusGroup (feature dc-2's four-value vocabulary), SpecStatus (the raw frontmatter status where a spec was readable, empty otherwise), Disclosed (*disclosure.Disclosure, nil when the entry is ordinary)}. Source is a CLOSED, always-populated four-value enum covering every source ComputeIndex reads (dc-4 adds a third kind of entry - a default-branch spec, neither a local nor a remote-tracking design ref - so Source gains a fourth value, default, rather than leaving those entries un-sourced against dc-3's original three-value design-branch-only framing). Source itself IS the mechanism that satisfies parent dc-5's 'each entry discloses its source' and 'a remote-only branch renders sealed with its remoteness disclosed': Source is a plain, always-populated field on every entry (never omitted, never defaulted away), so a remote-only entry's remoteness, or a default-branch entry's default-ness, is disclosed simply by directory-home rendering its Source value - no disclosure.Disclosure wrapper is needed or created for that ordinary case. Disclosed is reserved narrowly for a DEGRADED entry whose content could not be read at all (ac-4's no-draft-spec case, and any future such case) - a materially different situation (absent content, not merely a sourcing fact) from an ordinary remote-only or default-branch entry (present content, just sourced from a particular place). This closes two decision-conflict findings against this draft's earlier wording: Source's ambiguity against Disclosed, and Source's original three-value vocabulary having no value for dc-4's default-branch entries", anchor: "#dc-3" }
  - { id: dc-4, text: "default-branch enumeration walks the default branch's OWN tree at its resolved ref (git ls-tree under .verdi/specs/active/ and .verdi/specs/archive/ at that ref, mirroring internal/index's existing corpus-walk shape but ref-scoped rather than working-tree-scoped) rather than reusing the live corpus index (internal/index), because the live index reads the working tree/checkout the serving process happens to be on - exactly the coupling co-1 forbids for index computation. A future consolidation of the two walkers is left open, not invented here. Scope note, closing a decision-conflict finding: this is not an extension of parent dc-5's source taxonomy needing dc-5's own blessing - dc-5 resolves oq-2 (whether remote-tracking design refs join the enumeration), a strictly narrower question than ac-2's own base text, which already requires 'every spec on the default branch' independent of dc-5. dc-4 realizes that pre-existing ac-2 requirement; Source=default (dc-3) discloses it by source exactly as dc-5's disclosed-by-source rule asks, and parent dc-2's accepted-pending-build/active/terminal groups can only ever be populated from default-branch content, so dc-4 is required scaffolding for dc-2, not a competing enumeration", anchor: "#dc-4" }
  - { id: dc-5, text: "a design branch already merged into the default branch (its tip is an ancestor of the default-branch tip, tested with the same gitx.IsAncestor primitive feature dc-4 prescribes for gc's merged signal) is EXCLUDED from the design-branch enumeration entirely, for local and remote-tracking design refs alike - its spec is already reachable, correctly grouped, as a default-branch entry (dc-4), so re-enumerating the same spec a second time as a draft would violate dc-2's one-spec-one-status premise and fabricate a duplicate entry from a source (Source: both is reserved for genuinely un-merged local+remote design refs, never for a merged-but-not-yet-gc'd leftover). This is a merged/unmerged filter, independent of dc-5's (parent feature's) local/remote axis, applied identically on both sides. NARROW CLAIM (not by-construction agreement with gc): this exclusion test is refs-only (co-1) and structurally blind to managed-worktree dirtiness, which parent dc-4 also gates gc's reclaim on ('a worktree with uncommitted changes is never reclaimed but disclosed and kept') - a filesystem fact only worktree-manager (ac-3/ac-4), which actually manages worktrees, can see or disclose. This story's exclusion answers only dc-4's directory-listing half honestly (no duplicate draft entry for an already-merged spec) using the one signal co-1 permits it to read; the dirty-worktree DISCLOSURE half of dc-4 is entirely worktree-manager's obligation, realized through its own gc-reporting mechanism, not through this story's Entry type or through re-including the branch here. No exempts edge is needed against parent dc-4: this decision narrows nothing dc-4 promises, it satisfies a different slice of it than worktree-manager satisfies, and the two compose at directory-home. No exempts edge is needed against parent dc-5 either: dc-5's 'local design refs and remote-tracking design refs alike join the enumeration, each entry disclosed by source' resolves oq-2, a question about WHICH REF NAMESPACES are read (both, not local-only) - both namespaces are still read here, including for a merged ref (the ancestry check itself reads it); dc-5 is not a promise that every ref that resolves also yields a retained, undeduplicated entry regardless of merge state, a question dc-5's text never reaches and this story's dc-2/one-status premise answers instead", anchor: "#dc-5" }
constraints:
  - { id: co-1, text: "inherited verbatim from the feature (co-1): managed worktrees live under the data zone, never committed (not this story's concern - see worktree-manager); index computation reads refs and never switches a checkout. ComputeIndex takes the serving checkout's root only to resolve .git and run ref-scoped plumbing against it - it never runs checkout, switch, or any working-tree-mutating command against that root or any other", anchor: "#co-1" }
  - { id: co-2, text: "no network in any test (CLAUDE.md): every ComputeIndex behavior is proven against a fixturegit repository carrying real local and (simulated) remote-tracking design refs, or against the fake git-runner-port double from dc-2 - never a live clone or fetch", anchor: "#co-2" }
frozen: { at: 2026-07-14, commit: 7e425b6ed982b44605c29bef0b0580565e8a9cbc, stub_matched: true }
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
and — separately — every UNMERGED design branch's draft spec (dc-5) by
reading `refs/heads/design/*` at each ref's current tip via ref-scoped
plumbing only: `git show <ref>:<path>` (`gitx.Show`) and `for-each-ref`-style
listing (`gitx.LocalBranches`'s shape, scoped to `refs/heads/design`), never
`git checkout` or `git switch`. Every ref is read exactly once; a ref
that fails to resolve at all (a documented git-level error, not "no spec
present") propagates as a real Go error rather than a silently-skipped
entry. A design branch already merged (dc-5) contributes no entry from this
walk at all — its spec is already counted once, from the default-branch
walk — so no spec is ever double-counted across the two walks. Evidence:
static (the function signature takes no checkout-mutating dependency) and
behavioral (a fixturegit repo with a default branch, two unmerged design
branches, and one ALREADY-MERGED-but-not-yet-deleted design branch proves
one entry per unmerged branch, ONE entry — not two — for the merged spec,
no duplicates, no drops).

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
already ratifies. This holds unconditionally, closing a decision-conflict
finding raised against an earlier draft: `StatusGroup` for a design-branch
entry is never derived from that branch's spec content at all (readable or
not), so ac-4's degraded, no-draft-spec entry is not a case of "no status to
derive a group from" — it is simply another design-branch entry, grouped
`drafts-in-progress` by the same unconditional rule as every other one, one
level upstream of where `SpecStatus` (which IS content-derived, and is
correctly empty for a degraded entry) ever enters the computation. Grouping
(the four-bucket partition `directory-home` will
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
state — no HTTP handler, no page template, no rendering logic, and no
forge/network call of any kind. The `directory-home` stub (a sibling story
under this same feature) is the only consumer that turns `Entry` values
into the home page's markup; this story's job is to design `Entry` richly
enough that `directory-home` never needs to reach back into git itself.

**Explicit scope boundary** (closing a decision-conflict finding raised
against an earlier draft of this spec): parent feature dc-5 also requires
"an entry whose branch has an open MR is chipped in-review from the forge
port" — but dc-5's own words call this "a second, non-ref source", clearly
distinguished from the refs-only computation ac-2 (and this story) claims.
`Entry` (dc-3) deliberately carries no forge/MR field; composing the
forge-sourced in-review chip onto `ComputeIndex`'s ref-computed output is
`directory-home`'s job — that stub implements both ac-2 and ac-5 and is
positioned to layer a second, degradable source on top. This is a disclosed
scope line, not an oversight, and it needs no `exempts` edge against dc-5:
nothing here contradicts dc-5, it implements exactly the ref-only half dc-5
itself calls out as distinct from the forge-sourced half.

## DC-2

A consumer-defined git runner port, not `internal/gitx`'s existing free
functions called directly. `internal/refindex` declares its own narrow
interface — list local design refs, list remote-tracking design refs,
resolve the default branch, read a path's content at a ref, list the
default branch's spec directories at that ref, and test whether one commit
is an ancestor of another (dc-5's merged-branch check) — that a small
adapter over `internal/gitx`'s existing functions (`LocalBranches`, `Show`,
`DefaultBranch`, `IsAncestor` — `gitx/branch.go`, `gitx/show.go`,
`gitx/ancestry.go`) satisfies. The two plumbing primitives this story needs
that `gitx` does not yet have — a `for-each-ref`-style query scoped to
`refs/remotes/origin/design/*`, and a tree-listing (`git ls-tree`) at an
arbitrary ref — are added to `gitx` as
more of the same shape (a thin wrapper over one `git` invocation, returning
parsed, deterministic output), never invented ad hoc inside `refindex`. The
port exists precisely so `ComputeIndex` is unit-testable against an
in-process fake with no real `git` process at all, in addition to the
hermetic `fixturegit` exercise — the 04 §port pattern this store already
follows everywhere else a real subprocess or network boundary sits behind
an interface.

The remote name is hardcoded to `origin`, matching this codebase's existing
single-remote convention verbatim rather than inventing a new narrowing:
`gitx.DefaultBranch` already resolves `refs/remotes/origin/HEAD` unconditionally,
`gitx.Push` already pushes to `origin` unconditionally, and every existing
`gitx.HasRemote` call site already treats `origin` as the one configured
remote (`gitx/branch.go`, `gitx/worktree.go`). A repository with a
differently-named remote is already unsupported by those existing
functions today; this story's remote-tracking read adds no NEW unsupported
case, it is consistent with what the rest of this store already assumes.
Widening to a configurable remote name, if ever needed, is a single later
decision that touches all of these call sites together, not a
`refindex`-local invention.

Closing a decision-conflict finding raised against an earlier draft:
parent dc-5 never mandates multi-remote support — it says only that local
and remote-tracking design refs "alike" join the enumeration, entirely
silent on remote naming. Hardcoding `origin` fills a gap dc-5 leaves open,
using this store's one existing convention; it narrows no explicit dc-5
promise, so no `exempts` edge is warranted — there is nothing in dc-5's
text being excused from, only an unstated implementation detail being
filled the same way the rest of this codebase already fills it.

## DC-3

The `Entry` output type is:

```go
type Entry struct {
    Ref         string // "spec/<name>" - the canonical kind/name identity
    Source      Source // default | local | remote | both
    StatusGroup string // feature dc-2's four-value vocabulary
    SpecStatus  string // the raw frontmatter status, where a spec was readable; "" otherwise
    Disclosed   *disclosure.Disclosure // non-nil only for a degraded entry (ac-4)
}
```

`Source` is a closed, always-populated four-value enum, not the
design-branch-only three values an earlier draft of this spec declared:
`default` names an entry read from the default branch's own tree (dc-4) —
neither a local nor a remote-tracking design ref at all — and `local` /
`remote` / `both` name a design-branch draft's ref sourcing exactly as
before. Closing a decision-conflict finding raised against the earlier
draft: without a fourth value, dc-4's default-branch entries would have no
defined `Source`, leaving parent dc-5's "each entry disclosed by source"
obligation unmet for that whole class of entry — misstating a
default-branch spec as `local` or `remote` would be actively wrong, since
it did not come from a design ref at all.

`StatusGroup` is always populated too, including for ac-4's degraded entry:
per ac-3, every design-branch entry's group is the unconditional constant
`drafts-in-progress`, derived from being on a design branch at all, never
from that branch's spec content — so a degraded entry (whose `SpecStatus`
is correctly empty, there being no content to read) still has a
well-defined `StatusGroup`. Only a `default`-sourced entry's `StatusGroup`
is content-derived (from its real, always-present frontmatter `status:`
field), and a `default`-sourced entry is never degraded in this story's
scope (dc-4 walks the default branch's own committed tree, which does not
have ac-4's "not yet authored" failure mode a fresh design branch does).

`Disclosed` reuses `internal/disclosure`'s existing shared shape
(`disclosure.New`, `disclosure.Render`) rather than a bespoke ad hoc
string, for the no-draft-spec case (ac-4) and any other degraded case this
story's implementer discovers — so `directory-home`'s later disclosed-notice
rendering (the parent feature's ac-5) speaks the same vocabulary every
other disclosure in this store already renders in, rather than inventing a
second one.

**Two distinct disclosures, closing a decision-conflict finding raised
against an earlier draft of this spec:** `Source` itself is the mechanism
that satisfies parent dc-5's "each entry discloses its source" and "a
remote-only branch renders sealed with its remoteness disclosed" — `Source`
is a plain, always-populated field on every entry (never omitted, never
silently defaulted), so a remote-only entry's remoteness, or a
default-branch entry's default-ness, is disclosed simply by
`directory-home` rendering its `Source` value; no `disclosure.Disclosure`
wrapper is created for that ordinary case. `Disclosed` is reserved narrowly
for a genuinely degraded entry whose content could not be read at all
(ac-4's no-draft-spec case, and any future case like it) — materially
different from an ordinary remote-only or default-branch entry, whose
content is present, just sourced from a particular place. The two fields
are not in tension: they disclose two different kinds of fact (sourcing vs.
absence), and only one of them ever needs the heavier shared-disclosure
shape.

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

**Scope note, closing a decision-conflict finding raised against an
earlier draft of this spec:** this is not an extension of parent dc-5's
source taxonomy that needs dc-5's own blessing. dc-5 resolves oq-2 — "does
`refs/remotes/origin/design/*` join the enumeration" — a strictly narrower
question than the parent feature's ac-2 base text, which already requires
"every spec on the default branch" independent of dc-5 entirely. dc-4
realizes that pre-existing ac-2 requirement; `Source: default` (dc-3)
discloses it by source exactly as dc-5's disclosed-by-source rule asks of
every entry, and parent dc-2's `accepted-pending-build` / `active
components` / `terminal` groups can only ever be populated from
default-branch content in the first place — dc-4 is required scaffolding
for dc-2's own grouping vocabulary, not a competing or undeclared
enumeration source.

## DC-5

A design branch already merged into the default branch — its tip is an
ancestor of the default-branch tip, tested with the same `gitx.IsAncestor`
primitive parent feature dc-4 already prescribes for `verdi gc`'s merged
signal — is EXCLUDED from the design-branch enumeration entirely, for local
and remote-tracking design refs alike. Closing a decision-conflict finding
raised against an earlier draft: parent feature dc-4 makes a
merged-but-not-yet-`gc`'d design branch a normal, expected state (`gc`
reclaims lazily on its own schedule; "directory reads never delete"), so
`ComputeIndex` must have a defined answer for it. Without this exclusion, a
just-accepted spec would show up TWICE — once from the default-branch walk
(dc-4, correctly grouped `accepted-pending-build`/`active`/`terminal`) and
once from the design-branch walk (always `drafts-in-progress`, ac-3) —
violating parent dc-2's "the status is the distinction" premise (one spec,
two statuses) however the duplicate was resolved: two entries contradicts
dc-2 directly, and a single entry that silently dropped one source would
have contradicted parent dc-5's "each entry disclosed by source" instead.

This exclusion is a merged/unmerged filter, orthogonal to parent dc-5's
local/remote axis — it applies identically whether the merged branch is
local, remote-tracking, or both. `Source: both` (dc-3) is thereby reserved
specifically for a genuinely still-open (unmerged) local+remote design
branch; it is never emitted for a merged leftover, since a merged branch
now contributes no design-branch entry at all.

**Narrower claim than an earlier draft made, closing a decision-conflict
finding raised against it (confidence 0.65 — the highest of this story's
review, taken seriously):** this exclusion test is REFS-ONLY (an ancestry
check, co-1), and is therefore structurally blind to one thing parent
dc-4's reclaim rule also depends on: whether the branch's MANAGED WORKTREE
(if one exists) carries uncommitted changes. Parent dc-4 forbids `gc` from
reaping a dirty worktree even when its branch is merged, and requires that
case be disclosed and kept — a worktree-state fact ref-index cannot see and
must not try to, since co-1 forbids it from ever reading a worktree's
dirty/clean status (that is a filesystem check, not a ref-scoped one, and
is exactly the `worktree-manager` story's domain, not this one's). So the
correct claim is narrower than "by construction agreement": `ComputeIndex`'s
merged-branch exclusion answers the DIRECTORY-LISTING half of dc-4 honestly
(a merged branch is not re-listed as a draft — dc-2's one-spec-one-status
premise, above) using only the signal it is allowed to read; the DIRTY-
WORKTREE-DISCLOSURE half of dc-4 (a merged-but-dirty worktree must still be
disclosed and kept, never silently reaped) is realized entirely by
`worktree-manager` (ac-3/ac-4), the story that actually manages worktrees
and can see their dirtiness — not by this story, and not by this exclusion
rule. The two stories' outputs are expected to be composed by
`directory-home`, exactly as dc-1 already scopes the forge/in-review chip;
no `exempts` edge is needed against parent dc-4 because this decision
narrows nothing dc-4 promises — `verdi gc`'s own dirty-worktree carve-out
is `worktree-manager`'s obligation to satisfy, not this refs-only index's.
Equally, no `exempts` edge is needed against parent dc-5's unqualified "the
index reads local design refs and remote-tracking design refs alike": that
sentence describes which REF NAMESPACES join the enumeration (local vs.
remote-tracking — the axis dc-5 itself is about, resolving oq-2), not
whether every ref that resolves is retained regardless of merge state: a
merged branch's ref still joins the read (it is inspected by the ancestry
check), it is only excluded from the OUTPUT once recognized as already
counted elsewhere (dc-2's one-status premise) — the same "read, then dedup"
relationship dc-4's own default-branch entries already have to the design-
branch entries they are never confused with.

**Direct engagement with parent dc-5's own enumeration wording, closing a
further decision-conflict finding:** parent dc-5's "local design refs and
remote-tracking design refs alike join the enumeration, each entry
disclosed by source" resolves oq-2 — a question about WHICH REF NAMESPACES
are read at all (both, not local-only, was the thing genuinely in doubt
before dc-5). Both namespaces are still read here, unconditionally,
including for a since-merged ref (the ancestry check itself reads it to
learn it is merged) — nothing is skipped un-inspected. dc-5's sentence is
not, in addition, a promise that every ref that resolves also yields a
retained, undeduplicated OUTPUT entry regardless of merge state; that is a
question dc-5's own text never reaches, phrased entirely in terms of ref
namespace membership, not merge state. This story's dc-2-derived one-
status premise answers the question dc-5 leaves open, rather than
narrowing an answer dc-5 already gave.

**No collision with an in-flight revision of an already-accepted spec,
closing a further decision-conflict finding raised against an earlier
draft:** could an UNMERGED design branch ever share its `Ref` with an
already-accepted default-branch entry, producing the very one-spec-two-
statuses duplicate this exclusion exists to prevent, in a case the ancestry
filter cannot catch because the branch is not yet merged? No — this store's
existing invariants rule it out structurally, not by any new rule this
story adds: an accepted spec is frozen and immutable (VL-010) and never
amended after acceptance (02 §Kind registry) — the ONLY forward path is a
superseding revision under a genuinely NEW, distinct spec name, because the
predecessor's own directory and name persist unchanged in `specs/active/`
(02: "a superseded spec stays in `specs/active/`") and VL-002 enforces
global ref uniqueness. So a design branch can never carry an unmerged draft
under the SAME name as an already-merged, accepted default-branch spec —
by the time a name is `accepted-pending-build` or later on the default
branch, no legal design branch exists, or ever will exist, proposing a
same-named draft revision; a proposed successor is always a different
`Ref` from its predecessor's. The `Ref`-collision scenario this finding
raised does not arise under the contract already in force.

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
