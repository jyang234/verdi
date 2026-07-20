---
id: spec/verdi-store-layout
kind: spec
class: component
title: "Store layout: filesystem as database"
status: active
owners: [platform-team]
links:
  - { type: depends-on, ref: spec/verdi-artifact-contract, note: "frontmatter and kind registry" }
schema: verdi.layout/v1
---

# Store layout: filesystem as database

## Purpose

The store is the system's only database: a directory tree in the monorepo plus a
per-checkout working area. Every other component (lint, index, fold, workbench,
MCP, dex) is a reader or writer of this layout and nothing else. The design goal
is that the store is legible to three audiences with zero translation: humans
(`ls`, `cat`), agents (`grep`, MCP tools over paths), and git (history, blame,
CODEOWNERS).

Store-root discovery is nearest-ancestor: every verdi command operates on the
nearest ancestor directory (from cwd) containing `.verdi/verdi.yaml`. An
explicit `--store` flag overrides the walk entirely, naming the root
directly with no ancestor search.

Principles inherited from verdi-go and binding here:

1. **Derived state is disposable.** Anything not in git must be rebuildable from
   git plus CI. Losing the working area is a non-event.
2. **The commit boundary is the audit line and the sharing line.** Committed
   content is reviewed, owned, and durable. Uncommitted content is one
   developer's working state and travels nowhere.
3. **Strict decode.** Unknown entries fail loudly. The layout is a versioned
   schema, not a convention.
4. **Views are never authoritative.** A rendering is not a record.

## Directory layout

```
.verdi/                            # committed zone (versioned, reviewed, audited)
  verdi.yaml                       # store manifest (see below)
  .gitignore                       # contains exactly: data/
  specs/
    active/<name>/                 # one directory per spec
      spec.md
      layout.json                  # board coordinate sidecar, schema verdi.boardlayout/v1 (R4-I-5)
      board.json                   # frozen at commit-to-design (feature class only)
    archive/<name>/                # closed feature specs move here whole
      spec.md
      layout.json
      board.json
      rollup.json                  # frozen at closure
      deviation-report.md          # frozen at closure
  adr/<name>.md
  diagrams/<name>.mermaid          # authored diagrams only; generated views are never committed
  attestations/<story-slug>/<ac-id>.md   # <story-slug> = RefSlug(scheme-prefixed story ref)
  waivers/<story-slug>/<ac-id>.md
  reaffirmations/<story-slug>/<object-id>.md   # rung-4 re-affirmation records; one per (story, amended object)
  conflicts/<name>.md              # challenges to closed decisions (evidence-model spec)
  bin/                             # committed shims: verdi-mcp, groundwork-mcp (surfaces spec)
  data/                            # working area — gitignored, per-checkout
    writer.lock                    # single-writer enforcement (D3)
    serve.path                     # pointer file naming the real socket path (D3); the
                                    #   socket itself binds off-tree at
                                    #   $TMPDIR/verdi-<hash>/serve.sock
    derived/<ref-slug>/<commit>/   # materialized CI bundles + local regenerations
      verdicts.json
      boundary-diff.json
      tests.json
      review.json
      graph-<service>.json         # one per impacted service (groundwork-mcp's input)
      views/                       # regenerated renderings (graphs, mermaid)
    mutable/
      annotations/<kind>--<name>.jsonl
      annotations/board--<story-slug>.jsonl   # board-only (no target) annotations
      boards/<story>.json          # live board state (autosave) — superseded for new work by
                                    #   layout.json (see note below); existing files remain
                                    #   valid until their branch dies
    cache/
      index-<layout-version>-<tree-hash>
```

Notes:

- `data/` lives **inside** `.verdi/` and is excluded by a **committed**
  `.verdi/.gitignore`. Rationale: each git worktree gets its own isolated
  working area for free, which matters for parallel local agents; and the
  entire system's footprint is one directory. Rejected alternative: a sibling
  `.verdi-data/` at repo root (second top-level entry, no worktree benefit).
  The gitignore prevents accidents; VL-013 catches intent — nothing under
  `data/` may ever be git-tracked, `git add -f` included.
- **Ref slugging is normative.** `<ref-slug>` = the ref lowercased, with `/`
  mapped to `--` and every remaining byte outside `[a-z0-9._-]` mapped to
  `-`. Two refs that collide after mapping are a hard error naming both —
  never a silent merge. `feature/stale-decline` → `feature--stale-decline`.
  **Which ref it is depends on the bundle (round 6; 08 §Round 6).** A
  whole-branch or per-service regeneration bundle is keyed by the **git
  ref** it was produced on — the transport and gc unit (§gc's
  merged/deleted-ref pruning applies to these). The **per-spec evidence
  records the fold consumes** are keyed by the **owning spec's ref**
  (`RefSlug(spec.id)`): the fold accumulates a story's records across every
  branch and commit that ever produced evidence for it, so a git-ref key
  would scatter records the fold must see together. `verdi sync` fetches the
  per-(git-ref, commit) `verdi-evidence` artifact and preserves its internal
  per-spec keys on write, so CI's per-spec producer output lands exactly
  where the fold's readers look; gc of per-spec dirs follows the owning
  spec's active/archive lifecycle, not git-ref merge detection.
- Flowmap and groundwork artifacts (`.flowmap.yaml`, `policy.json`, goldens,
  boundary contracts) stay in their service directories. The store **reads
  them in place** and never relocates them. The index unifies; the layout does
  not annex. The one verdi-owned file in a service root is the
  `verdi.bindings.yaml` AC-binding sidecar (evidence-model spec), discovered
  and validated like the rest. A service's boundary contract lives at the
  fixed, upstream-written path `<service-root>/.flowmap/boundary-contract.json`
  — `flowmap boundary` has no stdout mode or output flag; it always writes
  there, so verdi reads that literal path rather than any configured location.
- The `<story-slug>` segment under `attestations/` and `waivers/` (and the
  matching half of the artifact contract's `<story>--<ac-id>` name grammar)
  has two forms, round four having split it by class: **story**
  attestations/waivers use `RefSlug` of the owning **story** spec's
  required, scheme-prefixed `story:` ref (R4-I-2) — e.g. `jira:LOAN-1482` →
  `jira-loan-1482` — never a bare tracker key, which collides across
  schemes. **Feature outcome-attestations** use the owning **feature**
  spec's own ref slug instead (`RefSlug` of the feature spec's `id`, not
  tracker-derived, since a feature's `story:` is only an optional
  epic/objective ref) — see the artifact-contract spec's §Identity and
  references and the evidence-model spec's §Attestations and waivers, which
  define both forms in full.
- `layout.json` is the board's coordinate sidecar (`verdi.boardlayout/v1`,
  R4-I-5): `{schema, positions: {<object-id>: {x, y}}}`, positions only,
  never content. It lives on the spec's design branch during authoring —
  autosaved to the working tree, committed when the author commits, never
  per-drag — merges to main with the spec's acceptance, and is locked from
  then on with it (see Temporal classes, below). A superseding spec revision
  seeds its own `layout.json` copy from its predecessor's; a rejected or
  abandoned design branch's `layout.json` dies with the branch — no
  coordinate litter on main. Fallback and strictness are distinct rules, not
  one: an **absent** `layout.json` (or a present one with no stored position
  for a given object) falls back to the zoned-incremental layout algorithm
  for that object — this never gates. A **present** `layout.json` must
  strict-decode against `verdi.boardlayout/v1`, and every key in its
  `positions` map must resolve to a real object ID declared in that spec's
  frontmatter (§Object model, artifact-contract spec) — a dangling key is a
  VL-018 lint error, the same dangling-bindings posture as every other
  reference in this system, never a silent fallback.
- `mutable/boards/<story>.json` (live board state, autosave) is
  **superseded for new work** by `layout.json`: under the spec-realignment
  model (ratification round four), the board is a deterministic projection
  of the spec document plus `layout.json`, so authoring no longer maintains
  a separate live-board file — edits autosave straight into the spec's
  working tree. This is a stop-writing, not a deletion: existing
  `mutable/boards/<story>.json` files remain valid working state until the
  branch that produced them dies (merges or is abandoned), per the mutable
  zone's own lifecycle.
- `reaffirmations/<story-slug>/<object-id>.md`: rung-4 re-affirmation
  records (spec-realignment concept §3b) — one file per (story, amended
  feature object) pair, filed when a feature supersession's object manifest
  marks an object the story's edges touch as `amended`. `<story-slug>`
  follows the same `RefSlug` rule as `attestations/` and `waivers/` (the
  story's own scheme-prefixed ref); `<object-id>` is the amended object's
  stable id (an AC, constraint, or design-decision id) from the feature
  spec's object model. Attestation-shaped, CODEOWNERS-routed to the story
  owner, and embeds the old→new content-hash pair; frozen at commit
  (Temporal classes, below).
- Service discovery skips `.git`, `.verdi/data`, `node_modules`, and
  `testdata/` directories — the same noise class, so a fixture service root
  under a module's own `testdata/` (needed to exercise discovery itself) is
  never mistaken for a live corpus service, matching the Go toolchain's own
  testdata invisibility convention.

## Store manifest: `verdi.yaml`

```yaml
schema: verdi.layout/v1
forge: gitlab                  # or github; auto-detected from the remote URL when omitted
toolchain:                     # the pinned verdi-go dependency verdi execs, never links
  module: github.com/OWNER/verdi-go
  commit: 7a1c9e0b3f2d         # 12-hex pseudo-version commit; migrates to a tag form once upstream cuts one
providers:                     # story-provider spec owns semantics
  jira:
    base_url: https://koalafi.atlassian.net
    rollup_field: customfield_10142     # ids only; secrets come from env/CI vars
lint:
  gated_generated: []          # committed generated artifacts that are currency-gated
align:                         # evidence-model spec owns semantics
  judge_cmd: ["claude", "-p"]  # argv array, never a shell string — no quoting/injection ambiguity
  judge_required: false        # true: `verdi align` fails outright without a judge
audit:                         # R4-I-10; exemption/deviation counterweights (spec-realignment concept §2, §3b)
  exempts_conflict_threshold: 3      # active exemptions filed against one ADR before an auto-filed conflict record
  deviations_stale_threshold: 3      # accepted-deviations on one story before a spec-stale closure flag;
                                      #   0 (or absent) means the default (3), so a store cannot configure a
                                      #   zero threshold — the loosest configurable value is 1
spike_paths: []                # VL-016 fence: path globs a spike MR's diff may touch; empty by
                                #   default (fails closed) until a repo declares its own spike
                                #   workspace and doc paths
derived:
  retention_days: 14           # gc horizon for merged/deleted refs
services:
  discovery: flowmap           # any directory containing .flowmap.yaml is a service root
                                # (skips .git, .verdi/data, node_modules, testdata/)
```

Decode is strict: unknown top-level keys fail `verdi lint`. Layout migrations
bump `verdi.layout/vN` and regenerate affected paths in one coordinated change.

`toolchain.module` and `toolchain.commit` pin the verdi-go dependency every
flowmap/groundwork invocation execs (`go run <module>/cmd/<tool>@<commit>`):
a 12-hex pseudo-version commit today, migrating to a human-legible tag in one
manifest edit once upstream starts tagging releases. CI sets
`GROUNDWORK_REQUIRE_STAMP=1` and passes `--expect` so a boundary check fails
loudly on an unpinned or drifted toolchain; `GOPROXY` must stay reachable
even with a warm module cache — module metadata lookups need it, so CI must
never set `GOPROXY=off`.

`audit.exempts_conflict_threshold` and `audit.deviations_stale_threshold`
(R4-I-10) are both tunable, both documented as spec-realignment concept
OQ-iii watch items, and default to `3` and `3` — the smallest reversible
starting point, not a value derived from data; retuning is a manifest edit,
never a release. The exemption audit (concept §2) auto-files a conflict
record against an ADR once its active-exemption count reaches the first
threshold; the rung arbitrage counter-pressure (concept §3b) raises a
story's `spec-stale` closure flag once its accepted-deviation count reaches
the second.

`spike_paths` (VL-016) is the path-glob fence a spike MR's diff must stay
inside (concept §3b's spike evidence exemption); a spike diff touching a
path outside this list fails closed. It defaults to the empty list —
mirroring `lint.gated_generated`'s empty-by-default posture above — so a
repo must explicitly declare its spike workspace and doc paths before any
spike diff is accepted; an unconfigured store admits no spike diffs at all.

## Zones

| Zone      | Path                    | Versioned | Writer                   | Lifecycle                          |
|-----------|-------------------------|-----------|--------------------------|-------------------------------------|
| Committed | `.verdi/` (minus data/) | yes       | humans + closure MRs     | permanent; mutations are MRs        |
| Derived   | `.verdi/data/derived/`  | no        | `verdi sync` + local regeneration | disposable; pruned by `verdi gc` |
| Mutable   | `.verdi/data/mutable/`  | no        | the local verdi process  | one developer's working state       |

Graduation: a mutable record becomes durable by being rewritten as a committed
artifact (annotation → attestation, waiver, spec edit, or ADR) inside an MR.
There is no sync mechanism and none is wanted: sharing **is** committing.

## Temporal classes

Every committed artifact has exactly one temporal class, and the dex renders
the class honestly (see surfaces spec).

| Class           | Claim to currency            | Kept honest by                       | Examples                                  |
|-----------------|------------------------------|--------------------------------------|-------------------------------------------|
| living-gated    | current, machine-maintained  | CI currency gate (regenerate + fail on drift) | boundary contracts, goldens (in service dirs) |
| authored-living | maintained by humans         | MR review; dex shows last-modified from git | component specs, ADR index pages, authored diagrams |
| frozen          | none — point-in-time record  | immutability lint (VL-010)           | feature and story specs at acceptance, board.json, rollup.json, final alignment reports, attestations, re-affirmation records |

Class transitions are part of the doctrine: a feature spec is
authored-living while `draft` and becomes frozen at acceptance (the merge of
its spec MR); the alignment report is living-gated during the build (its
currency gate is `covers` = MR head) and becomes frozen at closure. A
transition is always a ritual (`verdi accept`, `verdi close`), never a hand
edit.

`layout.json` is not independently classed: it **inherits its spec's class**
at every point — authored-living while the spec is `draft` (autosaved,
committed alongside working-tree spec edits), frozen the instant the spec
accepts, and re-seeded (as its own copy, same rule) on a superseding
revision — because it is coordinate data for that one spec and nothing
else; it has no currency claim of its own to keep honest. Re-affirmation
records (concept §3b's rung-4 attestation) follow the incumbent attestation
rule instead: **frozen at commit**, not at some later ritual — an
attestation-shaped record is a point-in-time claim from the moment it
exists, so there is no living interval to transition out of.

Frozen artifacts carry a stamp in frontmatter: `frozen: { at: 2026-05-14, commit: 3e91ab2 }`.
The normative rule (enforced as VL-008 in the artifact contract): any artifact
with generated provenance in the committed zone MUST be either on the
`lint.gated_generated` allowlist or frozen-stamped. There is no third state,
which is how the store answers "how do diagrams not go stale": generated views
are simply never committed, and regeneration is cheap because every producer is
a byte-deterministic pure function of the tree.

## Disciplines

- **D1 — Versioned layout.** The tree shape is schema `verdi.layout/v1`.
  Unknown entries directly under `.verdi/` fail lint.
- **D2 — Identity.** The canonical id lives in frontmatter; the path encodes
  kind and (for specs) status. Lint enforces global ref uniqueness and
  path/frontmatter agreement. Active→archive moves preserve identity; git
  rename detection preserves history.
- **D3 — Atomic writes, immutable records, one writer.** One artifact per
  file, written temp-then-rename. Streams (annotations) are append-only
  JSONL. Evidence is keyed by commit and never edited in place: new evidence
  is a new record. Exactly one writer process per checkout, enforced by
  `data/writer.lock`: an atomically created (`O_CREATE|O_EXCL`) file whose
  JSON body is `{pid, start}`. A holder is live iff its pid answers a
  liveness probe *and* the OS's own process-start time for that pid
  (`ps -o lstart=`) agrees with the recorded `start` within tolerance —
  closing the PID-reuse gap a bare liveness probe would leave open. A dead
  or reused-pid holder's lock is stale and eligible for takeover; the JSON
  body stays legible with `cat`, per this document's legibility goal.
  `verdi serve` is the writer, and it additionally exposes the MCP endpoint
  on the checkout's unix socket. Realistic worktree checkout paths can
  breach a unix socket's ~103-byte `sun_path` ceiling, so the socket does
  not bind at a store-relative path: it binds at
  `$TMPDIR/verdi-<hash>/serve.sock`, where `<hash>` is a short hash of the
  checkout's absolute path, and the store records the real bound path in a
  legible, cat-able pointer file, `data/serve.path`. `verdi mcp` is a stdio
  shim — it reads the pointer file, then proxies to a running serve over
  that socket, or acquires the lock and serves standalone when none is
  running. Per-checkout sockets (and pointer files) mean worktrees never
  collide; if even the short bound form overflows the ceiling, binding
  fails loudly naming the path and the limit, rather than a cryptic `bind:
  invalid argument`. CI writes only commit-scoped derived paths.
- **D4 — Disposable caches.** Cache filenames embed layout version and tree
  hash; staleness is detected, never guessed. The tree hash algorithm is
  sha256 over the sorted `(path, git-blob-sha)` pairs of the committed zone
  (minus `data/`) plus every discovered corpus-contributing file in service
  roots — so a boundary-contract or obligation change invalidates the cache
  exactly like a spec change does. Every blob sha is computed the way git
  would hash the file's *current on-disk content*, so a dirty (uncommitted)
  edit changes the hash immediately: a new untracked file counts the moment
  it exists, and a tracked file deleted from the working tree is omitted
  from the pair set entirely (a deletion is a corpus change to detect, not
  an error) rather than guessed at by mtime. `rm -rf .verdi/data/cache` is
  always safe.

## Garbage collection

`verdi gc` (local hygiene verb, also runnable in CI images):

- prunes `derived/<ref>/` for refs merged or deleted more than
  `derived.retention_days` ago (the durable record is the pipeline and main's
  history). Mechanism: a prune-aware `git fetch --prune` first, then deleted
  = the ref exists neither locally nor on the remote, merged = its last known
  tip is an ancestor of the default branch tip. Offline fallback: mtime-based
  pruning of ref directories older than the retention window. Both are safe
  precisely because derived is disposable;
- prunes cache entries whose layout version or tree hash no longer matches;
- optionally, on explicit opt-in, prunes a LOCAL branch and its worktree (if
  any) when the branch is fully merged into the default branch, its worktree
  carries no uncommitted changes, and the worktree is not the primary
  checkout — reads never delete without that opt-in; every run names
  verbatim what it did and did not touch.
- never touches the committed zone or `mutable/`.

## Scale envelope and non-goals

Comfortable operating range: low thousands of artifacts, tens of megabytes of
markdown. Startup walk plus in-memory index build stays under a second at that
scale; full-text search over the corpus is in-memory, stdlib-only. If the
corpus outgrows this, the escape hatch is a persistent index cache — never a
database.

Non-goals, stated so nobody fixes them later: no server component in this spec
(the dex is a build artifact; the workbench is localhost — see surfaces spec),
no HA, no concurrent multi-writer support, no cross-repo federation (v2+).

## Open questions

- OQ-3: migration of existing `.claude/skills/spec` artifacts and any active
  specs into this layout (one-time chore; lint grandfathers `archive/`).
