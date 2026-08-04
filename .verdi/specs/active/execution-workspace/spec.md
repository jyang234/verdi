---
kind: spec
id: spec/execution-workspace
title: "Execution workspace"
class: component
status: active
schema: verdi.execution-workspace/v1
owners: [platform-team]
links:
  - { type: depends-on, ref: spec/worktree-manager }
  - { type: depends-on, ref: spec/verdi-store-layout }
---

# Execution workspace

## Purpose

This component exists by owner ruling OD-6
(`docs/superpowers/specs/2026-08-03-four-feature-owner-adjudications-design.md`
§OD-6): a narrowly scoped shared component for Context Integrity (CI) and
Comparative Spike Experiments (CSE) owning exact workspace materialization,
application of isolation controls and execution grants, environment
fingerprint collection, and safe cleanup. It reuses existing
`internal/wtmanager`, `internal/gitx`, and `internal/filelock` primitives but
does not broaden the closed local-design-branch-only `worktree-manager`
story.

Its workspaces live under `data/execution/<workspace-id>/` with sibling
`data/execution/<workspace-id>.lock` — admitted by the landed
`verdi-store-layout` amendment (OD-12): its §Directory layout rows and its
`data/execution/` lifecycle note, which rode along inside the committed-zone
amendment under the governing plan's severability rule as ratified by that
amendment's owner merge (ledger SI-8). This spec consumes the amended
layout as landed authority, not as a proposal it re-argues.

Consumers are CI and CSE only. ASD is not a consumer (see Non-goals).

## Exact workspace materialization

Two request shapes, both keyed under `data/execution/`:

(a) an exact commit SHA materialized as a DETACHED worktree — a new `gitx`
wrapper over `git worktree add --detach <path> <sha>`; no such primitive
exists today (`gitx.WorktreeAdd` takes a branch name only).

(b) a base commit plus a canonical patch — patch application is new code (no
`git apply` wrapper exists anywhere in `internal/gitx`).

Invariants: materialization never mints a local branch (preserving
worktree-manager ac-2's hard-won gate — its deviation history records that
`git worktree add` DWIM-mints branches unless actively refused) and never
touches the serving checkout's branch, index, or working tree
(worktree-manager ac-1's invariant; `gitx.WorktreeAdd`'s
`ErrBranchCheckedOut` pattern generalized to a fresh commit or patch target
rather than a branch name).

Either shape's FIRST materialization also writes the immutable
request-identity sidecar `data/execution/<workspace-id>.request`; a request
landing on an already-existing `<workspace-id>` is verified against that
sidecar before any reuse (§Workspace naming).

## Workspace naming

This is invention SI-10 (plan handle L-3), disclosed in §Inventions
disclosed below. The scheme: `<workspace-id>` is a deterministic pure
function of the materialization request — `<run-slug>--<sha12>` for the
exact-SHA shape and `<run-slug>--<sha12>-p<patch12>` for the base-plus-patch
shape, where `<run-slug>` is the consumer-supplied run identity passed
through the store's normative RefSlug rule (`verdi-store-layout` §Directory
layout notes: "Ref slugging is normative" — the ref lowercased, `/` mapped
to `--`, every remaining byte outside `[a-z0-9._-]` mapped to `-`), `<sha12>`
is the 12-hex abbreviation of the exact/base commit, and `<patch12>` is the
first 12 hex digits of sha256 over the canonical patch bytes. Two requests
colliding after slugging are a hard error naming both, never a silent merge
(the RefSlug collision rule, applied unchanged). No wall clock, no
randomness — the id is recomputable from the request alone, matching this
store's deterministic-output discipline for generated identifiers.

The truncated hexes appear ONLY in the path, for legibility; IDENTITY IS
ALWAYS THE FULL DIGESTS. Truncation alone would let two distinct requests
alias one directory and one lock, so at first materialization the component
atomically writes (temp-then-rename, the store's D3 atomic-write idiom) an
immutable request-identity sidecar, `data/execution/<workspace-id>.request`:
canonical JSON — sorted keys, trailing newline, this store's canonical-JSON
discipline — recording the consumer run identity, the FULL 40-hex commit SHA
(exact or base), and, for the base-plus-patch shape, the FULL 64-hex sha256
of the canonical patch bytes. The sidecar is written once and never edited
in place.

Reuse rule: any request mapping to an already-existing `<workspace-id>` MUST
verify its full request identity byte-for-byte against that sidecar before
reusing the path. Equal — idempotent reuse of the existing workspace.
Different — a hard error naming both requests, never a silent merge: the
RefSlug collision rule extended from the slug level to the full request
identity. A missing or undecodable sidecar at an existing path is an
OPERATIONAL ERROR, never silent reuse.

## Isolation-control application

The component constructs the isolated profile — clean environment,
controlled home/config discovery, sandbox and network policy application —
as MECHANISM only. A primitive that cannot provide a required control
returns an OPERATIONAL ERROR, never a silent reinterpretation. This is
grounded on both consumer sides:

- CI dc-10 (`.verdi/specs/active/context-integrity-v2/spec.md`, verbatim):
  "authoritative launch fails when isolation cannot be proven; Verdi may
  offer a visibly new advisory run, but no adapter or harness may silently
  reinterpret the failed launch as authoritative".
- CSE's operational-error clause (`docs/superpowers/specs/2026-07-30-comparative-spike-experiments-design.md`
  §Execution, isolation, and recovery, verbatim): "An evaluator crash,
  malformed response, protected-input mismatch, missing round, environment
  mismatch, or unavailable required isolation control invalidates the run
  and returns an operational error."

A missing required control must never be silently reinterpreted as
authoritative execution.

## Execution-grant enforcement

OD-7 ratifies THAT CI/CSE execution grants come from one shared strict
vocabulary; the vocabulary's CONTENTS are not fixed by any landed authority
and are this spec's invention SI-12 (handle L-6). Proposed contents — six
kinds: network, path-read scopes, path-write scopes, process execution,
resource ceilings, and timeouts. Unknown grant kinds fail closed (fail-closed
decoding is landed store discipline — `spec/verdi-index` §Constitution
item 5, strict decode).

Enforcement means: the workspace is constructed with exactly the granted
controls, and the component reports which grants it could and could not
apply as operational facts.

ASD's `get_design_capabilities` is outside this vocabulary by ruling — OD-7
(`docs/superpowers/specs/2026-08-03-four-feature-owner-adjudications-design.md`
§OD-7, verbatim): "ASD capabilities mean adapter-surface discovery. CI/CSE
capabilities mean execution grants from one shared strict vocabulary." Named
explicitly: `get_design_capabilities` is an ASD discovery surface, never a
member of this component's execution-grant vocabulary.

## Environment fingerprint collection

OD-6 ratifies that this component owns fingerprint COLLECTION; the collected
field set is not fixed by landed authority and is this spec's invention
SI-13 (handle L-7). Proposed field set — the intersection the two consumers
already demand: operating system and architecture; tool/adapter versions;
declared environment variables; input digests (drawn from the CSE design's
fingerprint enumeration — operating system and architecture, runtime and
evaluator versions, relevant environment variables, workload and fixture
digests, among the fields CSE's own schema demands as a superset — and CI
ac-2's manifest identity fields, its "adapter version, accepted authority,
repository state, declared scope, and capability grant" that compile into
the canonical context manifest).

Two fields CSE's own fingerprint enumeration additionally demands — network
policy and resource allocation — are not collected twice: they reach CSE as
this component's grant-application operational facts (§Execution-grant
enforcement's could-and-could-not-apply report) and are embedded by CSE's
feature-owned superset schema, never by a second collection path.

Output is canonical and sorted. COLLECTION IS SHARED; SCHEMAS ARE NOT: CSE's
fingerprint schema and CI's manifest fields embed the output as
feature-owned supersets; this spec states both halves without defining
either feature's schema.

## Safe cleanup

Cleanup and reclamation follow the shipped fail-closed idiom: total,
ordered, one-reason-per-item decisions (`wtmanager.decideReclaim`'s own
total ordered outcome shape; `internal/reclaim`'s compile-time-exhaustive
`KeptReason` enum are the named precedents), never `--force`
(`gitx.WorktreeRemove` deliberately omits it — git's own dirty-tree refusal
stays a second, independent guard), one disclosed result line per
workspace. The ordered outcome set is keep-not-eligible, keep-dirty,
keep-locked, reclaim (§GC slice states the same order and its predicates);
a reclaim removes the worktree first, then deletes the workspace's
`.request`, `.released`, and `.lock` siblings, leaving no orphaned sidecar
behind a removed workspace.

Keep dirty, locked, ambiguous, or unverifiably eligible workspaces — a
predicate that cannot verify eligibility keeps, the same corrective posture
this store already carries elsewhere for a misclassifying predicate.

Locks (`data/execution/<workspace-id>.lock`, via `internal/filelock`) are
held only for the duration of a single mutating operation, never a
workspace's idle lifetime — worktree-manager dc-2's own correction:
reintroducing lifetime-long holds is a named regression this component does
not repeat.

## Reused primitives

| Primitive | Reused for | Limit preserved |
|---|---|---|
| `filelock.Acquire/Release/Peek` | `data/execution/<workspace-id>.lock` ownership | generic path-keyed lock; per-operation hold only |
| `gitx.StatusDirty` | dirty check before any destructive op | none needed |
| `gitx.WorktreeRemove` | cleanup | never `--force` |
| `wtmanager.WorktreesRoot`/`WorktreePath` | addressing (not cutting) managed worktrees when a design-branch worktree is the materialization source | pure path assemblers; addressing only, never cutting or reclaiming |
| `wtmanager.EnsureWorktree` | **not reused for execution workspaces** | its contract is local design branches with `design/<name>`↔`<name>` naming, and stays that way — the closed contract, unchanged |

`wtmanager.EnsureWorktree` is NOT reused for execution workspaces. Its
contract is the closed local-design-branch-only contract
(worktree-manager ac-1/ac-2, dc-1; `workbench-directory` dc-5's
local-branches-only rule) and stays that way — this component never
broadens it. Where an execution workspace's materialization source happens
to be a design-branch worktree, this component only *addresses* the already
managed path via `wtmanager.WorktreesRoot`/`WorktreePath`; it never cuts,
mutates, or reclaims a managed worktree itself.

## GC slice (the execution slice of `verdi gc`)

Ownership split: `verdi-store-layout` owns WHICH roots gc may touch (the
landed amendment grew scope to `data/execution/`); this spec owns HOW the
execution slice decides and WHICH invocation surface selects it; feature
units own WHEN cleanup is requested — quote CSE verbatim
(`docs/superpowers/specs/2026-07-30-comparative-spike-experiments-design.md`
§Retention and reproducibility): "Cleanup runs only after the human decision
is durably recorded. A cleanup failure is operational and disclosed; it does
not rewrite the decision."

Decision semantics: scan `data/execution/`; decide per workspace among a
TOTAL outcome set that must include keep-not-eligible, keep-dirty,
keep-locked (via `filelock.Peek`), and reclaim; reads never delete —
explicit `verdi gc` is the only deleter (worktree-manager dc-4's
non-forcing discipline; `workbench-directory` dc-4's reclamation-signal
authority). One disclosed line per workspace.

Eligibility is RUN-scoped, not branch-deletion-scoped: worktree-manager's
"deleted" signal is deliberately local-design-branch-only (its dc-3) and is
not silently transferred to execution workspaces. An execution workspace's
eligibility derives instead from its run's declared lifecycle, recorded by
the release contract below — invention SI-16, disclosed.

The component exposes a RELEASE operation; the consuming feature invokes it
when its own lifecycle permits cleanup (features own WHEN — the CSE
cleanup-ordering clause quoted above). Release is durably recorded as a
marker file whose EXISTENCE IS THE RECORD: the sibling
`data/execution/<workspace-id>.released`, written atomically
temp-then-rename (the store's D3 atomic-write idiom). The marker is an
operational FACT — never a proof, never a verdict, never a ratification.
Writing it requires the consuming feature's own lifecycle to have already
produced whatever durable record its authority demands — for CSE, the
durably recorded human decision — a record this component never inspects
and never interprets.

The execution slice's decision is TOTAL and ORDERED, mirroring
`wtmanager.decideReclaim`'s shape, exactly one reason per workspace:

1. no readable `.released` marker — keep-not-eligible;
2. uncommitted changes (`gitx.StatusDirty`) — keep-dirty;
3. a live lock (`filelock.Peek`) — keep-locked;
4. otherwise — reclaim: the worktree is removed first
   (`gitx.WorktreeRemove`, never `--force`), then the workspace's siblings
   (`.request`, `.released`, `.lock`) are deleted.

Any marker state the slice cannot read or decode KEEPS, fail-closed: an
eligibility that cannot be verified is never reclaimed.

Whole-checkout disposal loses the markers together with the workspaces they
describe — a non-event for this root, which the landed store-layout
lifecycle note already declares "per-run and disposable by declared
lifecycle" (unlike `data/journey/`, whose own note makes the same loss a
disclosed coverage event).

Sibling naming under `data/execution/` rides this component's naming
authority over the admitted root — the landed layout's own tree comment,
"naming owned by spec/execution-workspace". A follow-up store-layout
enumeration line naming `.request` and `.released` explicitly remains
available to a later amendment if the owner wants one.

Invocation surface — invention SI-11 (handle L-5), disclosed: the execution
slice joins BARE `verdi gc`, alongside the existing managed-worktree slice.
What is preserved untouched is gc-reclaim dc-1's per-invocation MODE
exclusivity: bare `verdi gc` and `verdi gc --reclaim-unmanaged` remain
mutually exclusive modes, never combined in one run, and
`--reclaim-unmanaged` keeps its own unmanaged-only slice. What this decision
GROWS is the bare mode's slice set. dc-1's wording that "bare verdi gc
(unchanged) runs only the existing managed-worktree slice", and ac-3's
requirement that the scope-disclosure line "grows to name both slices as a
closed pair on every invocation", were written when the managed-worktree
slice was the bare mode's only slice; this decision adds a second one, so
the bare mode's slice set gains the execution slice and the disclosure grows
from a closed pair to a closed triple — grown, never replaced and never
narrowed, the same incremental-delivery posture worktree-manager dc-5
already establishes for slices landing behind this same verb ("incremental
delivery of an already-ratified … verb, not a redefinition").

Rationale: both bare-mode slices reclaim only roots verdi itself
materialized (managed roots), both are fail-closed keep-by-default with
per-item disclosure, so adding the execution slice does not change
bare gc's mutating-or-not character — the property dc-1's
mode exclusivity exists to protect; smallest reversible option (no new flag
or mode; a later amendment can still split out a dedicated flag). The
scope-disclosure line grows to name the execution slice as run or not-run
on every invocation (gc-reclaim ac-3's closed-pair disclosure idiom, now
covering three slices).

Cross-slice exclusion (data-loss guard): `data/execution/` is a MANAGED
root excluded from the unmanaged-residue sweep, as the landed store-layout
amendment states. The shipped sweep's predicate today classifies as managed
only paths under `data/worktrees/`, so without the exclusion a live CSE
candidate workspace would be unmanaged residue to
`verdi gc --reclaim-unmanaged --apply` and destroyable mid-run. The
predicate change is a runtime gap of the shared seam (see §Implementation
seam below). No slice's sweep may claim another slice's root.

## Non-goals

- Quote OD-6 verbatim
  (`docs/superpowers/specs/2026-08-03-four-feature-owner-adjudications-design.md`
  §OD-6): "Policy decisions and feature proof semantics remain outside this
  component."
- Verdict and outcome taxonomy stays feature-owned: this component surfaces
  raw operational FACTS — which controls were requested, applied, or
  refused; exit status; timeout — never a proof, never a verdict, and never
  a reclassification of a run's meaning. CSE's own failure taxonomy
  (`docs/superpowers/specs/2026-07-30-comparative-spike-experiments-design.md`
  §Execution, isolation, and recovery, "Failures retain their meaning"): a
  correctness or safety failure is a valid candidate verdict with a witness;
  a candidate crash or timeout is a candidate result if the harness and
  workload remained healthy; an evaluator crash, malformed response,
  protected-input mismatch, missing round, environment mismatch, or
  unavailable required
  isolation control invalidates the run and returns an operational error;
  excessive variance, a practical tie, conflicting bounds, or no eligible
  candidate yields disclosed-unproven. CI's conflict verdicts (CI ac-3:
  "the acceptance, launch, evidence-intake, and closure gates share one
  conflict verdict…") remain their own.
- Isolation CLAIMS stay feature-owned: CI owns the claim that "an agent run
  was project-sealed and the vendor base remained opaque"
  (`docs/superpowers/plans/2026-08-01-four-feature-orchestration.md`
  §Worktree, isolation, and capability mechanics, verbatim; CI dc-2 is the
  in-spec grounding — "Verdi certifies a project-sealed context, not an
  unknowable whole-harness context…"); CSE owns registered-environment-
  policy-honored / weaker-isolation-disclosed (CSE design §Execution,
  isolation, and recovery: "An experiment that declares network access may
  run only when policy permits and must disclose its weaker isolation in the
  result").
- Proof types stay separate — quote the landed orchestration plan verbatim
  (`docs/superpowers/plans/2026-08-01-four-feature-orchestration.md`
  §Worktree, isolation, and capability mechanics): "Shared Git/worktree
  primitives may be reused; context receipts and experiment recommendations
  remain separate proof types."
- Fingerprint SCHEMAS stay feature-owned (per §Environment fingerprint
  collection above).
- ASD is not a consumer: ASD's v1 scope
  (`docs/superpowers/specs/2026-07-30-ai-assisted-spec-design.md`, verbatim)
  "owns semantic draft objects only" and "does not switch branches, commit,
  push, open or merge PRs" — version-scoped statements binding until ASD's
  own authority says otherwise — and its capability term is discovery, not
  grants (OD-7). Preserve the OD-7 distinction between ASD discovery
  capabilities and CI/CSE execution grants.
- Authority grounding: this spec cites only LANDED authority as authority
  (OD-6/OD-7/OD-12 adjudications, the landed store-layout amendment, the
  archived worktree-manager/gc-reclaim/workbench-directory stories, the
  landed orchestration plan); the cross-feature audit packet may be design
  rationale but ratifies nothing.

## Implementation seam

One shared implementation seam, owned by the execution-workspace unit — the
future implementation unit named `execution-workspace enforcement` (owner:
platform-team), implementing this component's mechanics ONCE, in one shared
internal package beside `internal/wtmanager`, `internal/gitx`,
`internal/filelock` (the exact Go package name is that unit's decision).
This naming is invention SI-15 (handle L-14), disclosed.

Pending runtime gaps — this spec ships no runtime code; only the
runtime/shared-seam implementation may remain pending: detached-SHA
worktree creation; patch application; isolation-profile construction; grant
decoding/enforcement; fingerprint collection; execution workspace naming and
its request-identity/release sidecars; the execution gc slice; the
managed-root exclusion from residue/reclaim.

CI and CSE consume this seam. They retain only feature policy, proof and
receipt semantics, isolation/authority claims, and their own integration
behavior — never a second implementation of materialization, isolation,
grant enforcement, fingerprinting, or cleanup. OD-12's division clause
("Runtime implementation remains divided among the owning feature units")
governs the four features' STORAGE runtime for the amendment's paths; it
does not license per-feature implementations of this component's
mechanics, which OD-6 assigns here.

## Inventions disclosed

This specification's owner merge ratifies seven inventions, none of which was
settled by OD-6 or OD-7, each recorded in the successor invention ledger
(`docs/superpowers/invention-ledger.md`) in the same reviewed change:

- SI-10 — workspace naming scheme (handle L-3)
- SI-11 — gc invocation surface (handle L-5)
- SI-12 — execution-grant vocabulary contents (handle L-6)
- SI-13 — fingerprint field set (handle L-7)
- SI-14 — first origin-less component spec outside the fidelity six-set —
  this spec has no `docs/design/specs/` origin file, a precedent no landed
  text sanctions or forbids (handle L-8)
- SI-15 — the enforcement unit name and single-package shape (handle L-14)
- SI-16 — the durable release signal for the execution gc slice (no plan
  handle: raised as a P1 by the owner-gate review of this PR, after the
  governing plan's handle list was written)

Until each is amended by later ratification it stands as recorded here;
each L-* value is a plan-local traceability handle, never a ledger ID.
