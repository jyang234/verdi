# Execution-Workspace + Store-Layout Authority Materialization Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps use
> checkbox (`- [ ]`) syntax for tracking.

**Goal:** Materialize the two still-unlanded authority deliverables named by
the owner adjudications — the `execution-workspace` component specification
(OD-6) and the single shared `verdi-store-layout` amendment (OD-12) — as two
serialized owner-merged PRs, mapping every new path and ownership claim to
existing authority before any authority bytes are authored.

**Architecture:** One early store-layout amendment (PR-A) admits all four
features' committed paths plus the execution-workspace data-zone root and its
`verdi gc` scope growth, with an explicit severability rule so CSE-only rows
cannot stall the gate ASD needs; a second PR (PR-B) then creates the
`execution-workspace` component spec, which consumes the amended layout as
landed authority. Both are documentation-only authority PRs; all runtime
implementation remains divided among the owning feature units (OD-12).

**Tech stack:** Markdown authority artifacts under `.verdi/specs/active/`,
the workspace-sibling `docs/design/specs/` origin + `08-revision-notes.md`
ratification record, and the existing gates (`make lint-store`,
`make spec-align`, `make verify`).

**Review provenance:** this plan was adversarially reviewed (boundary +
over-engineering pass) before this PR was opened; verdict REWORK, all P1/P2
findings adjudicated and folded in, including: the PR-A severability rule
(P1-1), dropping the `data/receipts/` admission (P1-2), re-grounding PR-B on
landed authority instead of the non-authoritative audit §3 (P1-3), the
unmanaged-reclaim cross-slice exclusion for `data/execution/` (P1-4), and
routing the plan's own inventions to §10 (P1-5).

## Global constraints

- Specs are authoritative; nothing here resolves a spec ambiguity silently —
  unresolved semantics and this plan's own inventions are enumerated in §10
  for the successor invention ledger (`docs/superpowers/invention-ledger.md`,
  OD-9 Option A). Until each is ratified, it is *disclosed as unproven*, not
  decided.
- The closed local-design-branch-only worktree contract is never broadened:
  worktree-manager ac-1/ac-2 and dc-1 (`.verdi/specs/archive/worktree-manager/spec.md`),
  enforcing the archived feature `spec/workbench-directory` dc-5's
  local-branches-only rule. Execution-workspace reuses the primitives and
  adds new ones beside them (OD-6).
- Policy decisions and feature proof semantics remain outside
  execution-workspace (OD-6, verbatim).
- Terminology firewall (OD-7): ASD capabilities = adapter-surface discovery;
  CI/CSE capabilities = execution grants from one shared strict vocabulary.
  Documentation and authority contracts must preserve the distinction.
- Promotion coupling (adjudication dependency statement): the OD-12
  amendment must be owner-merged before **either** ASD or CSE promotion; the
  OD-6 component spec must be owner-merged before the **CSE** promotion
  only. Each promotion consumes landed authority only, never audit-packet
  proposals — and so do PR-A and PR-B themselves: the audit packet's §3
  "ratifies nothing" and may be cited as design rationale only, never as
  authority.
- Three-valued honesty everywhere: proven, violated-with-witness, or
  disclosed-as-unproven. A gate that SKIPs is a disclosure, never a pass.
- PR bytes are verdi-repo files only. Workspace-side files
  (`docs/design/specs/01-store-layout.md`, `08-revision-notes.md`) are
  outside the git repo; their edits are same-change obligations verified
  locally by the fidelity gate, not PR content (see §8).

---

## 1. Authority map — every path and ownership claim

Each row states the claim, the authority that grounds it today, which
deliverable carries it, and its severability class (see §7.1): **CORE** rows
implement OD-12's committed-zone mandate and gate both promotions; **SEV**
rows are severable at PR-A review — any SEV row may be dropped and re-landed
as a follow-up amendment without re-opening CORE rows.

| # | Path / claim | Existing authority | Carried by | Class |
|---|---|---|---|---|
| 1 | `.verdi/specs/active/<name>/design-provenance.jsonl` (ASD sidecar, append-only, non-authoritative, schema `verdi.design-provenance/v1`) | ASD design §Provenance (line 274); OD-8 fixes exclusion reason code `design-provenance-sidecar`; CX-17 marks the path unadmitted | PR-A | CORE |
| 2 | `.verdi/specs/archive/<name>/design-provenance.jsonl` (sidecar follows spec to archive) | ASD design line 287; ASD promotion plan §6 | PR-A | CORE |
| 3 | `.verdi/specs/{active,archive}/<spike>/experiments/<experiment-id>/{experiment.yaml, candidates/*.patch, observations.jsonl, result.json, recommendation.md, ratification.yaml, selected/capsule-manifest.json}` | CSE design §Architecture (lines 199–213, verbatim tree) + line 217 (follows spike to archive); CX-17 marks it unadmitted | PR-A | CORE |
| 4 | CSE durable support files only under configured non-product `spike_paths` | CSE design line 239; `spike_paths` is an existing `verdi-store-layout` §Store manifest key | Already admitted; PR-A restates, does not redefine | CORE |
| 5 | `.verdi/policy/` — new committed top-level home for CI constitution/policy/overlay/exemption artifacts; internal grammar owned by CI's future `policy-authority` stub | CI spec.md names **no** `.verdi/` path anywhere; CI dc-3 says only that authority "is stored in ID-addressable policy, overlay, and exemption artifacts"; delivery-sequence item 1 assigns the *artifacts* (not their location) to `policy-authority`; CX-17 requires admission. The directory name `policy/` is this plan's invention (§10 L-1) | PR-A | SEV |
| 6 | GLG human records — five kinds per GLG ac-4: deviations, attestations, waivers, exemptions, semantic dispositions | Attestations → `attestations/<story-slug>/<ac-id>.md`; waivers → `waivers/…`; deviations → archive `deviation-report.md`; semantic dispositions → spec frontmatter (`dispositions`, already schema-admitted). **Exemption records have no admitted home** — deferred with §10 L-2b, seam flagged against row 5's CI exemption artifacts (a two-units-one-path hazard for the R-5/OD-2 kernel-contract lane to resolve) | PR-A records mapping + deferral in 08 entry; no new paths | CORE |
| 7 | GLG journey event receipts (AC-8) — storage **deferred, not admitted** | GLG spec leaves storage unspecified (CX-17 verbatim). GLG AC-8 makes receipts *immutable* and GLG elsewhere calls receipts authority-bearing (ac-4 "can never mint an authoritative verdict, receipt, or closure"); only *outcome events* are "telemetry, never lifecycle authority." A data-zone home would put an immutable authority-adjacent record in a disposable per-checkout zone, and the name `receipts` collides with CI's canonical context receipts. Deferred to the GLG unit (§10 L-2) | PR-A records the deferral in the 08 entry; **admits nothing** | CORE |
| 8 | `data/execution/<workspace-id>/` + sibling `data/execution/<workspace-id>.lock` — execution-workspace per-run workspaces | OD-6 ruled the component into existence; the data zone and gc scope are owned by `verdi-store-layout`, so the admission must be a ratified store-layout amendment (CX-6 resolution clause: OD-6 "must name the `verdi-store-layout` amendment explicitly"); CX-6: neither CI's per-run sealed worktree nor CSE's base-commit+patch workspace fits `data/worktrees/<name>` | PR-A admits root + gc scope; naming scheme fixed in PR-B (§10 L-3 until then) | SEV |
| 9 | `data/worktrees/<name>/` + `data/worktrees/<name>.lock` — managed design-branch worktrees | Shipped authority: worktree-manager dc-1 (`internal/wtmanager/naming.go:32–70`), CO-1 (data zone, never committed); absent from 01 §Directory layout. Backfill justified as **sibling legibility** for the new `data/execution/` line (the omission violates no rule — D1 governs entries directly under `.verdi/`, and lint skips `data/` entirely), not as a rule-violation repair | PR-A | SEV |
| 10 | `obligations/<story-ref-slug>/<ac-id>--<for-kind>.md` — committed top-level | Enforced in code (`internal/lint/walk.go:29` `knownTopLevelEntries`, `ClassifyPath`), ratified via spec/obligation-artifact (VL-019), absent from 01 §Directory layout **and** 02 §Kind registry — silent code-first admission, the failure mode OD-12 heads off. PR-A backfills the 01 half; the 02 half is outside store-layout authority and gets §10 L-9 with a named owner | PR-A | SEV |
| 11 | Workspace materialization, isolation-control application, execution-grant enforcement, fingerprint collection, safe cleanup — owned by `execution-workspace` | OD-6 verbatim charter | PR-B | — |
| 12 | CI context manifests/receipts and CSE observation records/results/recommendations/capsule manifests/ratifications remain separate feature-owned proof types; neither feature may mint the other's | Orchestration plan line 118 (landed): "context receipts and experiment recommendations remain separate proof types" | PR-B non-goals restate; owned by CI/CSE specs | — |
| 13 | Isolation *claims* stay feature-owned: CI owns the claim "an agent run was project-sealed and the vendor base remained opaque" (orchestration line 116, verbatim; CI dc-2 is the in-spec grounding); CSE owns "registered environment policy honored, weaker isolation disclosed" (CSE design line 459) | Orchestration lines 113–119; CI dc-2; CSE §Execution | PR-B non-goals restate | — |
| 14 | `verdi gc` reclamation authority over `data/` roots | store-layout §Garbage collection (managed-worktree scope bullet; derived-zone pruning), workbench-directory dc-4 (reclamation signals), gc-reclaim dc-1/dc-2 (unmanaged mode). The zones table itself grants gc authority over `derived/` only — scope for any new root is new ratification, not existing coverage | PR-A grows scope (execution root); PR-B defines the slice's decision semantics | SEV (with row 8) |

No path or claim in either deliverable is unmapped: rows 1–4 transcribe
feature designs verbatim; rows 5 and 8 are new admissions this plan proposes
and marks for owner ratification at PR-A review (each an invention, §10);
rows 6–7 are recorded mappings/deferrals that admit nothing; rows 9–10 are
disclosed backfills; rows 11–14 transcribe landed rulings.

## 2. Committed-zone authority vs `.verdi/data/` runtime storage

The store-layout §Zones table defines **three** zones — Committed (`.verdi/`
minus `data/`, versioned, "mutations are MRs"), Derived
(`.verdi/data/derived/`, disposable, pruned by `verdi gc`), and Mutable
(`.verdi/data/mutable/`, one developer's working state). Several shipped
`data/` paths sit in **no** zone row (`worktrees/`, `cache/`, `writer.lock`,
`serve.path`); their lifecycles are ratified individually (worktree-manager
CO-1/dc-4, D4). The binding floor under all of it: `data/` is gitignored via
the committed `.verdi/.gitignore`, and "nothing under `data/` may ever be
git-tracked, `git add -f` included" (VL-013, `internal/lint/vl013.go`).

Applied to this program:

**Committed (authority-bearing or committed-evidence):**
- ASD sidecar (rows 1–2) — committed but *non-authoritative* by its own
  schema posture; excluded from CI context compilation under reason code
  `design-provenance-sidecar` (OD-8).
- CSE experiment tree (row 3) — committed proof artifacts; disposable
  execution byproducts are excluded from retention ("worktrees and
  temporary branches; containers; compiled binaries; profiles and traces;
  verbose logs; and transient caches," CSE §Retention lines 603–608) and
  live only in data-zone workspaces.
- CI policy artifacts (row 5) — canonical, ID-addressable → committed.
- GLG human records (row 6) — already-committed artifact kinds; reused.

**Data zone (runtime, never committed, per-checkout):**
- Execution workspaces (row 8) — per-run, disposable by declared lifecycle
  (like `worktrees/`, they join the individually-ratified unzoned paths;
  PR-A's text must state their lifecycle explicitly rather than lean on the
  derived-zone row).
- Managed design-branch worktrees (row 9) — existing.

**Deliberately unplaced:** GLG event receipts (row 7). Immutable,
authority-adjacent records fit neither a disposable data zone nor an
unratified committed home; zone placement is the GLG unit's decision
(§10 L-2). The "sharing is committing" graduation rule stands meanwhile:
anything a human record needs durably is rewritten as a committed artifact
through existing kinds, never synced.

The rule both deliverables restate: **committed-zone admission is
store-layout authority (PR-A); data-zone behavior within an admitted root is
component authority (PR-B for `data/execution/`); neither PR ships runtime
code.**

## 3. Deliverable 1 (PR-B): the `execution-workspace` component specification

Landed second, but specified first here because its contract fixes what the
amendment must admit. New file
`.verdi/specs/active/execution-workspace/spec.md` (spec.md-only is a valid
component directory — witnessed by all six shipped component specs, each
spec.md-only aside from verdi-store-layout's review sidecars).

Frontmatter (strict-decoded; restricted YAML dialect):

```yaml
kind: spec
id: spec/execution-workspace
title: Execution workspace
class: component
status: active
schema: verdi.execution-workspace/v1
owners: [platform-team]
links:
  - { type: depends-on, ref: spec/worktree-manager }
  - { type: depends-on, ref: spec/verdi-store-layout }
```

(Task B step 2 verifies this block against `internal/artifact`'s
`validateBase`/`validateComponent` before authoring; the `schema:` value
follows the shipped `verdi.<component>/v1` pattern but is itself unverified
until that step.)

Two structural consequences of `class: component`, disclosed now rather than
discovered at authoring time:

- **No decision objects.** `validateComponent`
  (`internal/artifact/spec.go:518`) rejects `acceptance_criteria`,
  `constraints`, `decisions`, `open_questions` on component specs (02's "no
  object model") — worktree-manager dc-3 hit exactly this against
  store-layout. `spec/execution-workspace` will have **no** `#ac-N`/`#dc-N`
  anchors; the CSE promotion's required restatement of it as landed
  authority is prose-to-prose. Its owned-scope and gc-slice rules are
  normative prose sections, cited by section heading.
- **First origin-less component spec.** It sits outside the fidelity gate's
  closed six-spec set (`internal/specalign/fidelity_test.go:26`), and this
  plan chooses to give it no `docs/design/specs/` origin file — a precedent
  no landed text either sanctions or forbids (00-index enumerates the six
  "first citizens"; the fidelity comment sanctions growth by feature/story/
  spike specs, silent on new components). Recorded as an invention, §10 L-8.

### 3.1 Owned scope (the OD-6 charter, made normative)

The spec must contain exactly these five owned concerns, no more:

1. **Exact workspace materialization.** Two request shapes, both keyed
   under `data/execution/`: (a) an exact commit SHA materialized as a
   detached worktree — new `gitx` wrapper over
   `git worktree add --detach <path> <sha>`; no such primitive exists today
   (`gitx.WorktreeAdd` takes a branch name only,
   `internal/gitx/worktree.go:89`); (b) a base commit plus a canonical
   patch — patch application is new code (no `git apply` wrapper exists
   anywhere in `internal/gitx`). Materialization never mints a local branch
   (preserving worktree-manager ac-2's hard-won gate — its deviation-report
   records that `git worktree add` DWIM-mints branches unless actively
   refused) and never touches the serving checkout's branch/index/working
   tree (worktree-manager ac-1 invariant, `gitx.WorktreeAdd`'s
   `ErrBranchCheckedOut` pattern generalized).
2. **Isolation-control application.** The component constructs the isolated
   profile (clean environment, controlled home/config discovery, sandbox
   and network policy application) as *mechanism*. A primitive that cannot
   provide a required control returns an **operational error, never a
   silent reinterpretation** — grounded in landed authority on both
   consumer sides: CI dc-10 ("authoritative launch fails when isolation
   cannot be proven… no adapter or harness may silently reinterpret the
   failed launch as authoritative") and CSE's operational-error clause (an
   unavailable required isolation control invalidates the run and returns
   an operational error, design §Execution ~line 469). No isolation code
   exists today; the spec defines the contract, the CI/CSE runtime units
   implement against it.
3. **Execution-grant enforcement.** OD-7 ratifies that CI/CSE execution
   grants come from **one shared strict vocabulary**; the vocabulary's
   *contents* are not fixed by any landed authority and are therefore an
   invention this spec makes and PR-B's owner merge ratifies (§10 L-6).
   PR-B's proposed contents: network, path-read scopes, path-write scopes,
   process execution, resource ceilings, timeouts; unknown grant kinds fail
   closed (fail-closed decoding is landed store discipline — constitution
   #5). Enforcement means: the workspace is constructed with exactly the
   granted controls, and the component reports which grants it could and
   could not apply as operational facts. ASD's `get_design_capabilities` is
   outside this vocabulary by ruling (OD-7); the spec must say so by name.
4. **Canonical environment fingerprint collection.** OD-6 ratifies that
   this component owns fingerprint *collection*; the collected field set is
   not fixed by landed authority and is an invention this spec makes
   (§10 L-7), proposed as the intersection the two consumers already
   demand: operating system and architecture, tool/adapter versions,
   declared environment variables, input digests (drawn from CSE's schema,
   design lines 447–455, and CI's manifest identity fields, CI AC-2).
   Output is canonical and sorted. **Collection is shared; schemas are
   not** — CSE's fingerprint schema and CI's manifest fields embed the
   output as feature-owned supersets, and this spec must state both halves
   without defining either feature's schema.
5. **Safe cleanup.** Cleanup and reclamation follow the shipped fail-closed
   idiom: total, ordered, one-reason-per-item decisions
   (`wtmanager.decideReclaim`'s 4-outcome shape, `internal/wtmanager/gc.go:62`;
   `reclaim.KeptReason`'s compile-time-exhaustive enum,
   `internal/reclaim/predicate.go:49–96`), never `--force`
   (`gitx.WorktreeRemove` deliberately omits it — git's own dirty-tree
   refusal stays a second, independent guard), one disclosed line per
   workspace. Locks are held only for the duration of a single mutating
   operation, never a workspace's idle lifetime (worktree-manager dc-2's
   own correction — reintroducing lifetime-long holds is a named
   regression).

### 3.2 Reused primitives (with their unbroadened limits)

| Primitive | Reused for | Limit preserved |
|---|---|---|
| `filelock.Acquire/Release/Peek` (`internal/filelock/filelock.go:154,235,256`) | `data/execution/<workspace-id>.lock` ownership | Generic path-keyed lock; per-operation hold only |
| `gitx.StatusDirty` (`worktree.go:29`) | Dirty check before any destructive op | none needed |
| `gitx.WorktreeRemove` (`worktree.go:121`) | Cleanup | never `--force` |
| `wtmanager.WorktreesRoot/WorktreePath` (`naming.go:44,62`) | Addressing (not cutting) managed worktrees when a design-branch worktree is the materialization *source* | Pure path assemblers, exported for exactly this |
| `wtmanager.EnsureWorktree` (`ensure.go:41`) | **Not reused for execution workspaces.** Its contract is local design branches with `design/<name>`↔`<name>` naming, and stays that way | The closed contract, verbatim |

New primitives (detached-SHA worktree add, patch application, isolation
profile, grant decode, fingerprint collection, execution naming scheme, and
the residue/reclaim managed-root exclusion of §3.4) are **named as runtime
gaps in the spec's delivery section, not implemented by the spec PR** —
implementation is Wave-1+ feature-unit work (OD-12's "runtime implementation
remains divided among the owning feature units," and the adjudication
record: "only the runtime/shared-seam implementation may remain pending").

### 3.3 Non-goals (the do-not-encroach list, each with its citation)

The spec must carry a non-goals section restating, with citations:

- "Policy decisions and feature proof semantics remain outside this
  component" (OD-6, verbatim — quote it).
- Verdict and outcome taxonomy stays feature-owned. CSE's own
  classification (design §"Failures retain their meaning", lines
  ~462–472): a correctness or safety failure is a **valid candidate
  verdict with a witness**; a crash or timeout is a **candidate result**
  when harness and workload stayed healthy; evaluator crash, malformed
  response, protected-input mismatch, environment mismatch, or an
  unavailable required isolation control is an **operational error**; and
  excessive variance or a practical tie is **disclosed-unproven**. CI's
  conflict verdicts (CI AC-3) likewise. Execution-workspace surfaces raw
  operational **facts** — which controls were requested, applied, or
  refused; exit status; timeout — never a proof, never a verdict, and
  never a reclassification of a run's meaning.
- Isolation claims stay feature-owned (row 13 of §1, quoting orchestration
  line 116 for CI's claim).
- Proof types stay separate (row 12 of §1; quote orchestration line 118:
  "context receipts and experiment recommendations remain separate proof
  types").
- Fingerprint *schemas* stay feature-owned (§3.1 item 4).
- ASD is not a consumer: ASD's v1 scope "owns semantic draft objects only"
  and "does not switch branches, commit, push, open or merge PRs" (ASD
  design §Shared draft-mutation architecture — a version-scoped statement;
  the non-goal binds until ASD's own authority says otherwise), and its
  capability term is discovery, not grants (OD-7).

### 3.4 GC slice

The spec defines `verdi gc`'s execution-workspace slice: scan
`data/execution/`, decide per workspace among a total outcome set that must
include keep-not-eligible, keep-dirty, keep-locked (via `filelock.Peek`),
and reclaim; reads never delete — gc is the only deleter, invoked
explicitly (worktree-manager dc-4's non-forcing discipline; workbench-
directory dc-4's reclamation-signal authority). Eligibility signals are
run-scoped, not branch-scoped: worktree-manager's "deleted" signal is
deliberately local-design-branch-only (its dc-3) and must not be silently
transferred. Two decisions this spec must make explicitly:

- **Invocation surface.** gc-reclaim dc-1 makes the two existing *modes*
  (bare `verdi gc`; `verdi gc --reclaim-unmanaged`) mutually exclusive per
  invocation. Whether the execution slice joins bare `verdi gc`, gets its
  own flag, or extends an existing mode is a CLI-surface decision **not**
  settled by PR-A's scope ratification — PR-B must state it, and until PR-B
  merges it is open (§10 L-5).
- **Cross-slice exclusion (data-loss guard).** The shipped unmanaged sweep
  classifies as "managed" only paths under `data/worktrees/`
  (`internal/reclaim/predicate.go:266` `looksManagedAnywhere`;
  `internal/residue/survey.go:123`). Without a stated exclusion, anything
  under `data/execution/` — including a live CSE candidate workspace on a
  temporary branch — is *unmanaged residue* to
  `verdi gc --reclaim-unmanaged --apply` and can be destroyed mid-run.
  Both PR-A's gc prose and PR-B must state that `data/execution/` is a
  managed root excluded from the unmanaged slice, and PR-B names the
  residue/reclaim predicate change as a runtime gap.

CSE's ordering constraint binds the consumer side: "Cleanup runs only after
the human decision is durably recorded. A cleanup failure is operational and
disclosed; it does not rewrite the decision" (CSE lines 618–619) — the
component exposes cleanup; CSE decides when to call it.

## 4. Deliverable 2 (PR-A): the single `verdi-store-layout` amendment

Landed **first**. One amendment through the ratified component-spec flow
(OD-12: "Land one early shared amendment… covering all four features' new
artifact paths"), following the 77bfe501 precedent exactly (08-revision-notes
entry + both spec copies edited identically in one change). The adjudication
record characterizes OD-12's deliverable as the *committed-zone* amendment;
the data-zone rows ride along because CX-6's resolution requires the
store-layout amendment to be named as the vehicle for the data-zone/gc
growth — the severability rule (§7.1) keeps that ride-along from ever
holding the committed-zone rows hostage.

### 4.1 Exact text additions to §Directory layout

The ASCII block gains these lines (comments included as shown):

```
  specs/
    active/<name>/design-provenance.jsonl     # ASD sidecar: append-only, non-authoritative (OD-8)
    active/<spike>/experiments/<experiment-id>/   # CSE experiment tree; follows spike to archive
    archive/<name>/design-provenance.jsonl
    archive/<spike>/experiments/<experiment-id>/
  policy/                                     # CI constitution/policy/overlay/exemption artifacts;
                                              # internal grammar owned by spec/policy-authority   [SEV]
  obligations/<story-ref-slug>/<ac-id>--<for-kind>.md   # backfill: shipped via spec/obligation-artifact (VL-019)  [SEV]
  data/
    worktrees/<name>/                         # sibling legibility backfill (spec/worktree-manager dc-1)  [SEV]
    worktrees/<name>.lock
    execution/<workspace-id>/                 # execution-workspace runs (OD-6); naming owned by spec/execution-workspace  [SEV]
    execution/<workspace-id>.lock
```

(The `[SEV]` markers are plan notation, not amendment text.) The CSE tree's
internal file enumeration (experiment.yaml, candidates/,
observations.jsonl, result.json, recommendation.md, ratification.yaml,
selected/capsule-manifest.json) is transcribed **verbatim from CSE design
lines 199–213** into the amendment's prose, following the gc-reclamation
precedent of landing candidate language "words unchanged."

### 4.2 Exact prose additions

- **§Garbage collection (not the zones table):** gc scope prose lives where
  the managed-worktree scope already lives. New text: `verdi gc`'s ratified
  scope now covers three root families — managed design-branch worktrees
  (worktree-manager dc-4; workbench-directory dc-4/dc-5 signals), unmanaged
  residue (gc-reclaim dc-1/dc-2), and execution workspaces under
  `data/execution/` (scope only; decision semantics and invocation surface
  owned by spec/execution-workspace, named per OD-6 — a landed owner
  ruling, citable as authority). `data/execution/` is a **managed root,
  excluded from the unmanaged-residue sweep** (§3.4's data-loss guard).
  Fail-closed: keep on dirty, keep on locked, keep on ambiguous; never
  forced. The existing modes' mutual exclusivity (gc-reclaim dc-1) is
  restated untouched; no new invocation surface is ratified here.
- **Lifecycle line for `data/execution/`:** per-run, disposable by declared
  lifecycle, reclaimed only by gc's execution slice — stated explicitly
  because the zones table's Derived/Mutable rows do not cover it (it joins
  `worktrees/` among the individually-ratified `data/` paths).
- **D1 note:** the amendment is the enumeration change D1 demands
  ("Unknown entries directly under `.verdi/` fail lint") for the two new
  top-level entries `policy/` and (backfilled) `obligations/`. Until the
  policy-authority unit lands the `knownTopLevelEntries` addition, creating
  `.verdi/policy/` fails VL-007 — fail-closed in the safe direction,
  disclosed in the 08 entry, with the lint-enumeration change assigned to
  the policy-authority unit by name.
- **08-revision-notes entry:** one dated `##` section citing OD-12 as the
  ratifying event, enumerating all additions above, recording the GLG
  human-records mapping and the two explicit deferrals (rows 6–7: exemption
  storage, receipt storage — GLG-owned, ledger entries L-2/L-2b),
  disclosing the two backfills (rows 9–10) with their witnesses
  (`internal/lint/walk.go:29` comment vs. absent 01 text), and carrying the
  plan's PR-A-scoped inventions (§10) verbatim if the OD-9 ledger has not
  yet landed.

### 4.3 What the amendment does *not* do

No runtime code changes. `knownTopLevelEntries` (`internal/lint/walk.go:29`)
and `ClassifyPath` (`internal/artifact/classify.go`) additions for
`policy/`, the residue/reclaim managed-root exclusion, and any
VL/`.gitattributes` rules for the new artifact kinds are feature-unit
runtime work. Witnessed safe: VL-007 governs only entries directly under
`.verdi/`; lint's walks ignore unclassified files inside spec directories
(`internal/lint/walk.go:78` early return; `snapshot.go:161,198` read only
board.json/layout.json), so sidecar/experiment-tree admission is textual and
the store stays green without code edits. Kind-registry rows in
02-artifact-contract (a different component's authority) are out of OD-12's
store-layout scope — deferred: §10 L-4 for the four features, §10 L-9 for
the obligations backfill's orphaned 02 half.

## 5. GC ownership and fail-closed behavior (summary of both deliverables)

- **Owner of scope:** `verdi-store-layout` owns *which roots* gc may touch
  (amended in PR-A — new-root scope is new ratification; the zones table's
  own gc grant covers `derived/` only). `execution-workspace` owns *how*
  the execution slice decides and *which invocation surface* selects it
  (PR-B §3.4). Feature units own *when* cleanup is requested (CSE lines
  618–619).
- **Fail-closed invariants carried into both texts:** reads never delete;
  gc is the only deleter and is invoked explicitly; total
  one-reason-per-item decision enums; keep on dirty/locked/ambiguous; never
  `--force`; a predicate that cannot verify eligibility keeps (the R4-I-84
  `KeptDefaultBranch` episode — a predicate that could misclassify was
  corrected toward keeping — is the named precedent); and the cross-slice
  exclusion of §3.4, so no slice's sweep can claim another slice's root.
- **Out of gc scope in this program:** everything under committed zones,
  and all deferred paths (GLG receipts have no home to reclaim).

## 6. Separation preservation (verbatim guardrails both texts must quote)

- CI manifests/receipts vs CSE experiment proof types: orchestration plan
  line 118 (landed) — "context receipts and experiment recommendations
  remain separate proof types." The audit's sharper phrasing
  ("experimental/spike posture cannot mint authoritative receipts") may be
  *paraphrased as rationale* but not cited as authority (the packet
  ratifies nothing).
- Policy/verdicts outside execution-workspace: OD-6 — "Policy decisions and
  feature proof semantics remain outside this component."
- ASD discovery vs CI/CSE grants: OD-7 — "ASD capabilities mean
  adapter-surface discovery. CI/CSE capabilities mean execution grants from
  one shared strict vocabulary." Existing public names (e.g.
  `get_design_capabilities`) remain; contracts preserve the distinction.
- No silent reinterpretation of missing controls: CI dc-10 ("no adapter or
  harness may silently reinterpret the failed launch as authoritative") and
  CSE's operational-error clause (design ~line 469) — the landed grounding
  for §3.1 item 2.

## 7. Amendment ordering and PR topology — the decision

**Two serialized PRs. PR-A (store-layout amendment) merges first; PR-B
(execution-workspace component spec) is authored only after PR-A is
owner-merged.** Rationale:

1. CX-6's resolution requires the store-layout amendment to be explicitly
   named as the vehicle for the data-zone/gc growth — ratified layout
   first, then a component spec that cites it. If the two traveled in one
   PR, the component spec would cite co-traveling, unlanded layout text —
   the same consume-landed-authority-only discipline the dependency
   statement imposes on promotions, applied one level up.
2. The blocking scopes differ: OD-12 gates both promotions; OD-6 gates CSE
   only. The severability rule (§7.1) handles the residual coupling
   *inside* PR-A.
3. The review surfaces differ in kind: PR-A is an enumeration change to an
   existing authored-living component with an exact precedent (77bfe501);
   PR-B creates new component authority with novel semantics. The
   adjudication record already lists them as two separately owner-merged
   bullets.
4. The change shapes differ mechanically: PR-A obligates workspace-side
   origin + 08-revision-notes edits bound by the fidelity gate; PR-B is a
   single new in-repo file outside the fidelity set.

### 7.1 Severability rule (binding on PR-A's author and reviewers)

PR-A's rows divide into CORE (rows 1–4, 6–7: the four features'
committed-zone admissions, mappings, and deferrals — OD-12's own subject
matter, gating both promotions) and SEV (rows 5, 8–10, 14: the `policy/`
admission, the execution data-zone/gc rows, and the two backfills). A
review challenge to any SEV row severs that row (and its dependent prose)
from PR-A; the row re-lands as a follow-up store-layout amendment on its
own timeline. A severed row 8/14 delays PR-B and therefore the CSE
promotion only; it never delays PR-A's merge or the ASD promotion.
Challenges to CORE rows stop PR-A itself (§9.3-2).

Resulting order: **PR-A → PR-B → CSE promotion**; ASD promotion needs PR-A
only (plus its own non-execution gates). The OD-9 ledger PR is independent
of both but must land before either promotion begins (dependency
statement); it is not a deliverable of this plan.

## 8. Exact file inventories

**This planning PR (now):**
- Create: `docs/superpowers/plans/2026-08-03-execution-workspace-store-layout-authority.md` (this file). Nothing else.

**PR-A — store-layout amendment (verdi repo bytes):**
- Modify: `.verdi/specs/active/verdi-store-layout/spec.md` (§Directory
  layout block + §Garbage collection prose, per §4.1–4.2)

Same-change workspace-side obligations (NOT PR bytes — these files live at
the workspace root, outside the verdi git repo; the fidelity gate
`TestSelfHostedSpecFidelity` binds them to the in-repo copy byte-for-byte
except the `status:` line, and SKIPs with disclosure in a verdi-only
checkout):
- Modify: `../docs/design/specs/01-store-layout.md` (identical edit)
- Modify: `../docs/design/specs/08-revision-notes.md` (new dated `##`
  ratification entry per §4.2)

**PR-B — execution-workspace component spec (verdi repo bytes):**
- Create: `.verdi/specs/active/execution-workspace/spec.md` (per §3; no
  layout.json/board.json required — six-component-spec precedent)
- No workspace-side obligations (outside the fidelity six-set; origin-less
  by §10 L-8's disclosed choice).

**Either PR, conditionally:** append its §10 entries to
`docs/superpowers/invention-ledger.md` if the OD-9 PR has landed by then
(Task A step 3 / Task B step 2).

**Neither PR touches:** any file under `internal/` or `cmd/`, any
`testdata/`, `.gitattributes`, or `02-artifact-contract.md`.

## 9. Validation commands, rollback, STOP conditions

### 9.1 Validation

This planning PR: `git diff --check`, `make lint-store`, `make spec-align`
(plan files are outside both gates' subject matter; green proves no
collateral breakage), plus the repo's `merge-gate` required check on the
draft PR.

PR-A and PR-B, each: from `verdi/`, run `git diff --check`,
`make lint-store`, `make spec-align`, and full `make verify`
orchestrator-side before the PR is opened. **For PR-A the fidelity gate
must RUN, not SKIP** — authoring from a verdi-only checkout is a STOP
(§9.3-3), because a skipped fidelity gate is a disclosure, not a pass, and
PR-A's whole risk is mirror drift.

### 9.2 Rollback / amendment posture

Both deliverables are authored-living component authority
(`validateComponent` never requires frozen): the amendment mechanism is the
rollback mechanism. A defective ratification is corrected by a forward
amendment with its own dated 08-revision-notes entry (for PR-A) or an
ordinary spec edit (for PR-B) — never a silent revert of the ratification
record. A git revert of PR-A without a compensating 08 entry would leave
the workspace-side record asserting an admission the store no longer
carries; the 08 entry is part of any rollback. Severed SEV rows (§7.1)
re-land forward, never by rewriting PR-A's history.

### 9.3 STOP conditions

STOP and report (no improvisation) if any of these holds:
1. `origin/main` has moved off the pinned base and rebase changes any file
   this plan cites as authority.
2. Owner or Codex review challenges a **CORE** row of §1: the challenge
   resolves at review; nothing proceeds on the challenged row. (SEV-row
   challenges sever per §7.1 instead of stopping PR-A.)
3. The fidelity gate SKIPs during PR-A authoring (verdi-only checkout).
4. Any gate in §9.1 is red, or any lint rule turns out to enumerate files
   inside spec directories after all (would force runtime code into an
   authority PR — a scope violation to re-adjudicate, not absorb).
5. A second unit claims ownership of any path in §1 (orchestration
   stop-condition on shared-registry ownership, line 406) — the known
   near-miss is the row 5 / row 6 exemption seam, already deferred to the
   kernel-contract lane rather than resolved here.
6. PR-B authoring begins before PR-A is owner-merged.

## 10. Unresolved semantics and inventions → successor ledger (post-OD-9)

Owner: the lane that lands each authority PR transcribes that PR's entries;
Task A step 3 and Task B step 2 carry the action. If the OD-9 ledger has
not landed when a PR is ready, the PR carries its entries verbatim in its
own ratification record (08 entry for PR-A; the spec's disclosure section
for PR-B) and flags them for transcription. Nothing below is resolved
silently by this plan; each is either deferred to a named owner or is an
invention a specific PR ratifies with disclosure.

**Deferred to named owners:**
- **L-1** — internal grammar of `.verdi/policy/` (file naming, frontmatter,
  kind rows) and the `knownTopLevelEntries` lint admission: CI's
  `policy-authority` unit. PR-A admits only the top-level entry.
- **L-2** — GLG journey-event-receipt storage: zone (committed vs data),
  path, retention/pruning, and the naming collision with CI's canonical
  context receipts: GLG unit. No admission until decided.
- **L-2b** — GLG exemption-record storage, and the seam with CI's exemption
  artifacts under row 5 (two units, one concept): the R-5/OD-2 joint
  GLG/CI kernel-contract lane.
- **L-4** — whether the four features' artifact kinds need
  02-artifact-contract Kind-registry rows and `.gitattributes` VL-012
  globs at runtime-lint time: feature-unit scope, outside OD-12.
- **L-5** — gc invocation surface for the execution slice (bare `verdi gc`
  vs a dedicated flag; interaction with gc-reclaim dc-1's mode
  exclusivity): fixed by PR-B, open until PR-B merges.
- **L-9** — the obligations backfill's orphaned half: `obligations/` has no
  02 §Kind registry row (witnessed; only the VL-019 rule row exists).
  Outside store-layout authority; owner: the artifact-contract component's
  next amendment lane (platform-team).
- **L-10** — CSE AMB-6 (patch *content* under `spike_paths` vs VL-016
  intent) and CSE oq-1/oq-2 (evaluator protocol; corroborated-measurement
  trust class): CSE-owned, carried exactly as the CSE promotion plan
  carries them; they touch execution-workspace only at the
  evaluator-invocation seam, which PR-B names as a consumer-side boundary
  and does not define.

**Inventions this plan or its PRs make (ratified by the named PR's owner
merge, with disclosure):**
- **L-3** — `<workspace-id>` naming scheme under `data/execution/`
  (commit-keyed vs run-keyed vs grant-keyed; deterministic): fixed by PR-B.
- **L-6** — the execution-grant vocabulary's contents (§3.1 item 3's six
  kinds): OD-7 ratifies *that* one strict vocabulary exists, not *what* it
  contains. Invention; PR-B.
- **L-7** — the fingerprint collection field set (§3.1 item 4): OD-6
  ratifies collection ownership, not fields. Invention; PR-B.
- **L-8** — first origin-less component spec (no `docs/design/specs/`
  origin, outside the fidelity six-set): precedent neither sanctioned nor
  forbidden by landed text. Invention; PR-B.
- **L-11** — the directory name `policy/` itself (row 5): no source names
  a location; the name is plan-proposed. Invention; PR-A (severable).
- **L-12** — admitting execution data-zone rows inside the OD-12 amendment
  under a severability rule (§7.1), reading the adjudication's
  "committed-zone amendment" characterization as non-exclusive: invention;
  PR-A's owner merge ratifies the reading.
- **L-13** — whether the archive-zone enumeration lists `experiments/` and
  `design-provenance.jsonl` per-line or via a follows-the-spec rule: PR-A
  author's drafting choice, ratified at review.

---

## Task breakdown for the two future authority sessions

### Task A: author and land PR-A (store-layout amendment)

**Files:** modify `.verdi/specs/active/verdi-store-layout/spec.md`;
workspace-side `../docs/design/specs/01-store-layout.md` and
`../docs/design/specs/08-revision-notes.md`.
**Interfaces:** produces the amended layout PR-B and both promotions cite;
consumes §4 of this plan verbatim.

- [ ] Step 1: verify full-workspace checkout (`ls ../docs/design/specs/01-store-layout.md`) — STOP if absent (§9.3-3)
- [ ] Step 2: apply §4.1 block (without `[SEV]` markers) and §4.2 prose to `.verdi/specs/active/verdi-store-layout/spec.md`
- [ ] Step 3: apply the identical edit to `../docs/design/specs/01-store-layout.md`; add the §4.2 dated entry to `../docs/design/specs/08-revision-notes.md`; transcribe PR-A's §10 entries (L-1, L-2, L-2b, L-11, L-12, L-13) to `docs/superpowers/invention-ledger.md` if it exists, else verbatim into the 08 entry
- [ ] Step 4: run `make lint-store && make spec-align` — fidelity must RUN and pass; then full `make verify`
- [ ] Step 5: `git diff --check`; commit the in-repo file(s); open draft PR citing OD-12 and §7.1's severability rule; STOP for Codex/owner review

### Task B: author and land PR-B (execution-workspace component spec)

**Files:** create `.verdi/specs/active/execution-workspace/spec.md`.
**Interfaces:** consumes PR-A's merged layout (verify merged before
starting — §9.3-6); produces the component authority the CSE promotion's
relationship section restates as landed (prose-to-prose — no ac/dc anchors
exist on component specs, §3).

- [ ] Step 1: witness PR-A merged on origin/main (`git log origin/main -- .verdi/specs/active/verdi-store-layout/spec.md`), and confirm rows 8/14 were not severed (if severed, their follow-up amendment must merge first)
- [ ] Step 2: verify §3's frontmatter block against `internal/artifact` (`validateBase`, `validateComponent`, schema-value acceptance); author spec.md per §3 (scope §3.1, reuse table §3.2, non-goals §3.3 with §6's landed quotes, gc slice §3.4 including the cross-slice exclusion and invocation-surface decision, runtime gaps named as pending delivery, inventions L-3/L-5/L-6/L-7/L-8 disclosed in the spec and transcribed to the ledger if landed)
- [ ] Step 3: run `make lint-store && make spec-align && make verify`
- [ ] Step 4: `git diff --check`; commit; open draft PR citing OD-6; STOP for Codex/owner review
