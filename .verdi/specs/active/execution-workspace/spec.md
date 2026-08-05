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

Either shape's first materialization writes the immutable request-identity
sidecar `data/execution/<workspace-id>.request` LAST, after the worktree
exists, so that the sidecar is the materialization's COMPLETION WITNESS; a
request landing on an already-existing `<workspace-id>` is verified against
that sidecar before any reuse, a directory without one is incomplete residue
to be rebuilt, and a RELEASED id is refused outright until gc has reclaimed
it (§Workspace naming's ordered state machine).

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

Worktree creation and the sidecar write are NOT jointly atomic — no
primitive spans `git worktree add` and a rename — so `.request` is written
LAST and is thereby the COMPLETION WITNESS: its presence means a
materialization finished and returned to a consumer; its absence beside an
existing directory means one did not. This ordered state machine, and the
idempotent recovery it gives, is invention SI-17, disclosed. Every step runs
under the workspace's `.lock` (a per-operation hold, never an idle-lifetime
one):

After the lock is acquired and every path is typed, the machine branches on
the UNIT PATH first, so every reachable combination of
`{unit path, .request, .request.staging, .released, .lock, registry entry}`
reaches exactly one outcome. The lock is held CONTINUOUSLY across steps 1
through 6, so no other mutator can interleave a materialization — and every
other mutator of a unit acquires the same lock: the RELEASE operation
(below) and the gc reclaim alike.

GIT'S WORKTREE REGISTRY IS PART OF A UNIT'S STATE. A registration naming a
unit path that no longer holds a real directory is STALE ADMINISTRATIVE
RESIDUE, and it must be cleared, or `git worktree add` will refuse that path
forever ("missing but already registered") since this component never passes
`-f`. Both absent-unit branches — materialization step 2 and gc rank 0 —
RECONCILE THE REGISTRY FOR A UNIT under that unit's lock before proceeding.

Git offers NO TARGETED PRUNE, and bare `git worktree prune` is neither
target-specific nor trustworthy on its own: stock Git SKIPS a locked missing
registration and still exits 0, so a zero exit does not prove this unit's
registration is gone; and it WILL remove a registration whose directory
still exists with content when that directory's `.git` linkage is broken, so
a repo-wide prune can mutate another slice's administrative state. The
reconciliation is therefore VERIFIED and SCOPED, in three parts:

1. SCOPE FENCE — before any mutation, inspect what a prune would remove
   (`git worktree prune --dry-run -v`). If ANY candidate's path lies OUTSIDE
   `data/execution/`, the reconciliation REFUSES: an operational error on
   the materialization side, that unit's disclosed partial on gc's,
   fail-closed and disclosed. This is the cross-slice exclusion applied to
   ADMINISTRATIVE state — a broken `.git` link under `data/worktrees/` is
   that slice's problem to resolve, never this component's to prune.
2. MUTATE — only with an all-in-scope dry-run may the prune run.
3. VERIFIED POSTCONDITION — success is established by `gitx.WorktreeList`
   showing NO registration naming this unit path afterwards, NEVER by
   prune's exit status. A surviving registration is an operational error
   (materialization) or that unit's disclosed partial (gc); the unit is
   KEPT, and the outcome is never reported as reconciled.

A HUMAN-LOCKED registration naming a unit path is outside the states this
component produces (nothing here locks a worktree). Its outcome is that
same disclosed, retryable REFUSAL — never a silent wedge, and never a false
success on Git's zero exit.

Because success is defined by that verified postcondition rather than by a
command's exit code, the crash windows still SELF-HEAL: a crash or failure
between a directory removal and its reconciliation leaves the next attempt
to re-run the identical verified reconciliation, which either establishes
the postcondition or refuses and discloses.

The `.request` write is the only temp-then-rename artifact this component
has (`.released` needs no staging: it is `O_EXCL`-created and zero-byte), and
its temporary MUST be staged inside the OWNING UNIT'S SIBLING NAMESPACE, at
exactly `data/execution/<workspace-id>.request.staging`, written and then
renamed under the unit lock. A generic same-directory helper temporary
outside this namespace is NON-CONFORMING: it would place crash residue
outside the unit grammar, where nothing could classify it. `.request.staging`
is therefore one of the unit's sibling forms.

The staging write TRUNCATES any existing REGULAR FILE at the staging path,
which is lstat-typed like every other path this component touches; a
NON-REGULAR object there — a symlink, a directory, anything else — is an
OPERATIONAL ERROR for the request, never followed and never written through.
Staging residue is never load-bearing — it is never read and never a witness
— so overwriting a regular file there is always safe, and an
EXCLUSIVE-CREATE staging write is NON-CONFORMING for exactly the wedge it
would cause: a crash during step 6 leaves a `.staging` behind, and an
exclusive create would then fail against that residue on every later
attempt, forever.

Crash residue is therefore classified rather than ambiguous, and it is
disposed of at three sites: both absent-unit branches delete it with the
other orphaned metadata; step 4c's removal unlinks it alongside the
directory; and a `.staging` surviving beside a COMPLETE `.request` is
equally residue, left for the next reclaim, whose fixed order deletes it
immediately before `.request`.

BOTH PATHS ARE EXAMINED WITH LSTAT, never a following stat: at the marker
path as already stated, and at the UNIT PATH itself, so that "directory
present" means a REAL DIRECTORY. Without that, a symlink planted at
`<workspace-id>` would read as a present directory and a reclaim could
delete through it into its target. Any object at the unit path that is not a
real directory is an OPERATIONAL ERROR on this path — the unit is kept for
human attention, the step-3b posture applied one level up. An LSTAT FAILURE
at the unit path — anything other than a clean not-found — is likewise an
OPERATIONAL ERROR for this request, never read as absence, so it can never
route into step 2's sibling deletion: the materialization mirror of gc's
rank-0 keep-malformed rule:

1. acquire `data/execution/<workspace-id>.lock`. `filelock.Acquire` is
   NON-BLOCKING, so nobody waits: an acquisition that fails — a live holder,
   or any other acquisition failure such as an undecodable lock body — is an
   OPERATIONAL ERROR for this request, disclosed and retryable;
2. NOTHING AT ALL at the unit path — any siblings present (`.request`,
   `.request.staging`, `.released`) are orphaned metadata from a partial
   reclaim, a crashed write, or tampering, describing content that no longer
   exists: each is deleted by a plain unlink under the lock, one disclosed
   line. The registry is then RECONCILED FOR THIS UNIT under the same lock
   (the verified, scoped operation above), clearing any stale registration
   naming this unit path, and materialization proceeds fresh at step 5. An
   unexpected object kind at a sibling path, a failed unlink, or a
   reconciliation that refuses or cannot establish its postcondition is an
   operational error for this request, disclosed and retryable — the same
   posture step 4c takes for a failed removal. The unit lock is not among
   these siblings: it is this operation's own, released normally at the end.
   Release state does not survive the workspace it described: an orphaned
   marker describes nothing, so it is deleted with the other orphans rather
   than making the id permanently unusable;
3. directory present AND anything at all at the marker path (lstat
   semantics; symlinks never followed) — branch on what that object is:
   1. a REGULAR FILE — the RELEASED WORKSPACE is TERMINAL for this
      lifecycle: a hard error naming the released id. A released workspace
      awaits gc reclamation and is NEVER re-materialized or reused while its
      marker survives BESIDE ITS DIRECTORY; once a complete reclaim has
      removed every trace, the same deterministic id is legitimately fresh
      for a new request. This closes the hazard of a released-but-reused
      workspace being reclaimed out from under a live consumer;
   2. a NON-REGULAR object — a directory, a symlink, anything else — an
      OPERATIONAL ERROR: the workspace is kept for human attention, never
      treated as released and never fallen through to the `.request` branch.
      This is the materialization mirror of gc's keep-malformed rank, so a
      malformed marker is a decided state on both paths, not a gap;
4. directory present and `.released` absent — branch on `.request`:
   1. present as a REGULAR FILE and DECODABLE — the request path is typed
      with lstat like the others, and a non-regular object there is treated
      as UNDECODABLE (outcome b below), so the lstat discipline is uniform
      across all three sibling paths and mints no new state. Byte-compare
      the full request identity.
      Equal: the sidecar is the completion witness, the materialization is
      complete, so this is idempotent reuse; return. Different: a hard error
      naming both requests, never a silent merge — the RefSlug collision
      rule extended from the slug level to the full request identity. A
      stale `.request.staging` beside the complete `.request` is IGNORED
      here — never read, never a witness — and is removed by the next
      reclaim (the disposal sentence above);
   2. present but UNDECODABLE — an operational error; the workspace is KEPT
      for human attention, never silently reused and never silently deleted
      BY THIS PATH. The promise is scoped deliberately: a valid release
      marker still authorizes gc reclamation whatever `.request`'s state,
      because deletion authority is the MARKER, and a gc reclaim is not a
      reuse;
   3. ABSENT — incomplete residue of a crashed attempt that never returned
      to any consumer: removed under the lock and re-materialized, never
      reused as-is (the never-silent-reuse rule preserved by REBUILDING
      rather than by trusting what a crash left behind). The removal
      primitive is named: direct filesystem removal of the directory, an
      unlink of any `.request.staging` residue beside it, plus
      the verified, scoped registry reconciliation for this unit —
      deliberately NOT `gitx.WorktreeRemove` and NOT any `--force`. The
      never-force rule exists to protect consumer-visible work, and
      `.request`'s absence is the mechanical proof that no consumer ever
      received this directory, so there is no such work to protect. A failed
      removal is an operational error for THIS request, disclosed and
      retryable; the retry re-enters this same step, so the id is never
      permanently wedged. A PARTIAL of this pair — the removal done, the
      reconciliation refused or unverified or the process crashed between
      them — leaves no unit path but a surviving registration, which is
      exactly step 2's absent-unit state: the retry lands there and re-runs
      the same verified reconciliation, so the pair is self-healing rather
      than a wedge;
5. materialize the worktree (either request shape). A materialization that
   FAILS is an operational error for this request, disclosed; any partial
   directory it leaves behind carries no witness, so it is exactly the 4c
   residue the next identity-equal attempt removes and rebuilds;
6. write `.request` by staging `.request.staging` and renaming it into place
   — the atomic completion witness — and release the lock.

State 4c therefore has two producers, both self-healing on the next
identity-equal request, with no permanently wedged `<workspace-id>`: a
step-5 materialization failure, and a crash between steps 5 and 6. The
other partial states this store can reach are named where they arise —
§GC slice's reclaim ordering below.

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
ordered, one-reason-per-item decisions — extending
`wtmanager.decideReclaim`'s total ordered shape with the ranks this
component's state space needs (`decideReclaim` and `internal/reclaim`'s
compile-time-exhaustive `KeptReason` enum are the named precedents), never
`--force` (`gitx.WorktreeRemove` deliberately omits it — git's own
dirty-tree refusal stays a second, independent guard), one disclosed result
line per unit. The ordered outcome set is reclaim-orphaned, keep-malformed,
keep-not-eligible, keep-dirty, keep-locked, reclaim, plus the disclosed
partial outcome of a failed reclaim step (§GC slice states the same
membership and order, with its predicates); a reclaim deletes the completion
witness first, in fixed order — `.request.staging`, then `.request`, then
the unit path, then `.released`, then `.lock`, that last deletion being the
holder's own release of the unit lock — with any per-step failure disclosed
on that unit alone while the sweep continues. A later invocation resolves
any survivor.

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
| `filelock.Acquire/Release/Peek` | `data/execution/<workspace-id>.lock` ownership: all three mutators — materialization, release, and the gc reclaim — ACQUIRE it before mutating (never merely Peek, which would leave a check-then-act race); Peek remains available for read-only reporting | generic path-keyed lock; per-operation hold only |
| `gitx.StatusDirty` | dirty check before any destructive op | none needed |
| `gitx.WorktreeList` | the gc scan set's registry half (registrations resolving under `data/execution/`), and the registry reconciliation's verified postcondition | read-only — one `git worktree list --porcelain` call; never `add`, `remove`, or `prune` |
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

Decision semantics: scan `data/execution/`; decide per unit among a TOTAL
outcome set of seven result kinds — the six ranked outcomes
reclaim-orphaned, keep-malformed, keep-not-eligible, keep-dirty,
keep-locked, and reclaim, plus the disclosed PARTIAL outcome of a reclaim
step that failed — all defined below; reads never delete — explicit
`verdi gc` is the only deleter (worktree-manager dc-4's non-forcing
discipline; `workbench-directory` dc-4's reclamation-signal authority). One
disclosed line per unit.

Eligibility is RUN-scoped, not branch-deletion-scoped: worktree-manager's
"deleted" signal is deliberately local-design-branch-only (its dc-3) and is
not silently transferred to execution workspaces. An execution workspace's
eligibility derives instead from its run's declared lifecycle, recorded by
the release contract below — invention SI-16, disclosed.

The component exposes a RELEASE operation; the consuming feature invokes it
when its own lifecycle permits cleanup (features own WHEN — the CSE
cleanup-ordering clause quoted above). Release is durably recorded by the
sibling `data/execution/<workspace-id>.released`, defined normatively as a
ZERO-BYTE REGULAR FILE created with `O_CREATE|O_EXCL` — this store's own
write-once primitive, the idiom D3 already gives `data/writer.lock`. The
marker path is always examined with LSTAT semantics: symlinks are never
followed, and a symlink at the path is a non-regular object. Release is
IDEMPOTENT, but qualified: it succeeds when `O_CREATE|O_EXCL` creates the
marker, and on `EEXIST` when the existing object IS a regular file;
`EEXIST` against a directory, a symlink, or any other non-regular object is
an OPERATIONAL ERROR — a consumer is never told that a wedged marker path
was a successful release. Release ACQUIRES the unit's `.lock` before
creating the marker and holds it only for that operation, the same
per-operation discipline materialization and the gc reclaim follow: release
therefore CANNOT LAND INSIDE A MATERIALIZATION, because both hold the unit
lock and materialization holds it continuously across its steps 1 through 6.
Acquisition is non-blocking here too: a release whose acquisition fails — a
live holder, or any other acquisition failure such as an undecodable lock
body — is an OPERATIONAL ERROR for that invocation, disclosed and
retryable, never a wait and never a marker created outside the lock.
Without that gate a release could slip between materialization's worktree
creation and its completion witness, minting a released-yet-returned
workspace the next bare `verdi gc` would reclaim under a live consumer. The
EXISTENCE OF A REGULAR FILE at that path is the ENTIRE record; content is
ignored, so a nonempty file at the path still witnesses release — the record
is existence, not bytes, and there is nothing to decode. The marker is an
operational FACT — never a proof, never a verdict, never a ratification.
Creating it requires the consuming feature's own lifecycle to have already
produced whatever durable record its authority demands — for CSE, the
durably recorded human decision — a record this component never inspects and
never interprets.

Release may be invoked for an ABANDONED run regardless of how complete its
materialization is: abandonment is the consuming feature's own lifecycle
decision, not a judgment this component makes. gc then applies the ordered
decision below as usual, and a partial worktree whose cleanliness cannot be
proven KEEPS, disclosed — this store's kept-until-a-human-resolves posture.

The SCAN UNIT is the `<workspace-id>` itself, and the scan set is the UNION
of two sources: (a) filesystem grammar entries under `data/execution/` — the
unit path itself, or any of its siblings `.request`, `.request.staging`,
`.released`, `.lock` — and (b) GIT WORKTREE REGISTRATIONS whose recorded
paths resolve under `data/execution/`, read through the existing read-only
`gitx.WorktreeList` seam. A partial state is thereby a first-class decision
unit rather than invisible residue, and so is a REGISTRY-ONLY unit — a
surviving registration with nothing left on disk, which the filesystem alone
could never surface. Such a unit classifies at rank 0, where its action is
the registry reconciliation, one disclosed line.

Any entry under `data/execution/` matching NO unit grammar at all is
DISCLOSED AND KEPT: unclassified, held for human attention. The slice never
deletes what it cannot classify. This is a SCAN-LEVEL disclosure, not a
per-unit outcome — such an entry names no unit, so it takes no unit lock
and joins no rank.

Locking is ACQUISITION, not inspection: before any mutation the slice
ATTEMPTS TO ACQUIRE the unit's `.lock` and holds it for that operation only
(the per-operation discipline), rather than merely peeking. This closes the
check-then-act race in which gc could delete a workspace another process is
mid-materialization inside.

The decision has TWO LAYERS, and they must not be conflated: RANKS CLASSIFY
— they only read state — while MUTATION AT ANY RANK HAPPENS ONLY UNDER THE
ACQUIRED UNIT LOCK, and a lock that cannot be acquired converts any
would-be mutating outcome into keep-locked. Both mutating outcomes, rank 0
and rank 5, are gated this way; the gate is not itself a rank, because a
rank below a mutating outcome could never protect it. The decision is
RE-DERIVED under the acquired lock immediately before mutating; a decision
that no longer holds under the lock is re-decided, never applied —
classification is ADVISORY until re-established inside the gate. This binds
both mutating outcomes here and materialization's own step-2 orphan
deletion, so a two-pass implementation can never apply a classification the
world has since invalidated: without it, a stale rank 0 could delete a
completed workspace's `.request` and degrade a live unit to state 4c.

That gate is load-bearing for a case the ranks alone would misread: a unit
consisting SOLELY of a `.lock` held by a LIVE process is a materialization
in flight whose directory does not yet exist — `filelock.Acquire` creates
the lock file at materialization step 1, and the directory only appears at
step 5 — so it classifies rank 0 by shape but is keep-locked in fact, and
its lock is never deleted out from under its live holder. A lone `.lock`
whose holder is STALE is taken over by `filelock.Acquire`'s own stale-lock
detection, and its deletion is that holder's release — the same single
operation rank 0 names — disclosed as reclaim-orphaned.

Every path this list examines — the unit path, the marker path, the request
path, the staging path — is typed with LSTAT, never a following stat;
symlinks are never followed, so a symlink is always a non-regular object and
never a directory.
The per-unit decision is then TOTAL and ORDERED — extending
`wtmanager.decideReclaim`'s total ordered shape with the ranks this
component's state space needs — exactly one disclosed reason per unit.
MALFORMATION IS TESTED BEFORE ELIGIBILITY, so a malformed unit path is never
disclosed as the ordinary not-yet-released case:

0. NOTHING AT ALL at the unit path (siblings only, or NOTHING ON DISK AT ALL
   for a registry-only unit) — reclaim-orphaned: any siblings are deleted
   under the acquired lock and the registry is RECONCILED FOR THIS UNIT
   under it too (the verified, scoped operation above), one disclosed line;
   deleting the unit's `.lock` IS this holder's release of that lock — one
   operation, never an unlink followed by a release, so no gap exists in
   which a concurrent `Acquire`'s fresh lock could be removed underneath it.
   For a registry-only unit the reconciliation IS the whole action. A
   reconciliation that refuses on the scope fence, or whose postcondition
   cannot be established, is this unit's disclosed partial outcome — never
   a reported success;
1. a NON-DIRECTORY object at the unit path, or a NON-REGULAR object at the
   marker path — keep-malformed, its own disclosed reason, fail-closed, so a
   human can tell "not yet released" apart from "something is wrong at one
   of this unit's paths";
2. NOTHING AT ALL at the marker path — keep-not-eligible;
3. uncommitted changes (`gitx.StatusDirty`) — keep-dirty;
4. the unit's lock CANNOT BE ACQUIRED — a live holder, or any other
   acquisition failure such as an undecodable lock body — keep-locked, the
   disclosure naming which case, fail-closed;
5. otherwise — reclaim, under the acquired lock, in the order below.

A PREDICATE THAT CANNOT BE EVALUATED — a `gitx.StatusDirty` error, an lstat
failure — KEEPS, fail-closed, disclosed under the rank whose predicate
failed, the disclosure naming the check that failed. This mints no new
reason kind: an unevaluable predicate is a keep at its own rank, never a
silent fall-through to a later one. At RANK 0, whose only own kind is the
mutating reclaim-orphaned, the keep kind is KEEP-MALFORMED — the unit's
state cannot be established, which is the same cannot-tell disclosure class
— never reclaim-orphaned. So an lstat failure at the unit path keeps as
malformed rather than being read as absence and driving a deletion.

A reclaim deletes the COMPLETION WITNESS FIRST, in this FIXED ORDER:
`.request.staging`, then `.request`, then the unit path, then `.released`,
then `.lock` — and that final `.lock` deletion IS this holder's release of
the unit lock, one operation rather than an unlink following a release, so
nothing is double-unlinked and no gap exists in which a concurrent `Acquire`
could win a fresh lock only to have it removed underneath.

Deleting the witness first means every partial failure degrades into a
state this spec has already defined, and the landings are these: a failure
at the DIRECTORY step leaves the directory with its marker still present
(the marker is deleted third), which materialization reads as
released-terminal (step 3a) and which gc re-decides at rank 5 and
re-reclaims; a failure at the `.released` or `.lock` step leaves siblings
with no directory, which is rank 0's orphaned metadata; and step 4c — a
directory with neither marker nor witness — is reached by the
crash-between-materialization-steps-5-and-6 case, not by a reclaim partial.
So RECLAIM IS RE-ENTRANT: a later invocation re-decides the same unit and
finishes the job. Any step's
failure is that unit's disclosed PARTIAL outcome, named on its own line
rather than folded into a generic failure bucket, and the sweep continues
to the next unit — gc-reclaim's own per-item partial-outcome idiom (its
ac-2, a per-item refusal disclosed while the sweep continues; its dc-4, a
partial outcome printed as a line distinct from both full success and a row
kept before anything was touched). A later invocation resolves any
survivor.

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
worktree creation; patch application; a verified, scoped
registry-reconciliation primitive (dry-run scope fence, then prune, then a
`gitx.WorktreeList` postcondition — no targeted prune exists in Git, and
`internal/gitx` has no prune primitive at all today); isolation-profile
construction; grant
decoding/enforcement; fingerprint collection; execution workspace naming and
its request-identity/release sidecars and their partial-state recovery; the
execution gc slice; the managed-root exclusion from residue/reclaim.

CI and CSE consume this seam. They retain only feature policy, proof and
receipt semantics, isolation/authority claims, and their own integration
behavior — never a second implementation of materialization, isolation,
grant enforcement, fingerprinting, or cleanup. OD-12's division clause
("Runtime implementation remains divided among the owning feature units")
governs the four features' STORAGE runtime for the amendment's paths; it
does not license per-feature implementations of this component's
mechanics, which OD-6 assigns here.

## Inventions disclosed

This specification's owner merge ratifies eight inventions, none of which was
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
- SI-17 — the ordered materialization/cleanup state machine and its
  idempotent recovery (no plan handle: raised as a P1 by the owner-gate
  round-3 review of this PR, after the governing plan's handle list was
  written)

Until each is amended by later ratification it stands as recorded here;
each L-* value is a plan-local traceability handle, never a ledger ID.
