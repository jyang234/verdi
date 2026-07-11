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
      board.json                   # frozen at commit-to-design (feature class only)
    archive/<name>/                # closed feature specs move here whole
      spec.md
      board.json
      rollup.json                  # frozen at closure
      deviation-report.md          # frozen at closure
  adr/<name>.md
  diagrams/<name>.mermaid          # authored diagrams only; generated views are never committed
  attestations/<story>/<ac-id>.md
  waivers/<story>/<ac-id>.md
  conflicts/<name>.md              # challenges to closed decisions (evidence-model spec)
  bin/                             # committed shims: verdi-mcp, groundwork-mcp (surfaces spec)
  data/                            # working area — gitignored, per-checkout
    writer.lock                    # single-writer enforcement (D3)
    serve.sock                     # per-checkout MCP endpoint (D3)
    derived/<ref-slug>/<commit>/   # materialized CI bundles + local regenerations
      verdicts.json
      boundary-diff.json
      tests.json
      review.json
      views/                       # regenerated renderings (graphs, mermaid)
    mutable/
      annotations/<kind>--<name>.jsonl
      boards/<story>.json          # live board state (autosave)
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
- Flowmap and groundwork artifacts (`.flowmap.yaml`, `policy.json`, goldens,
  boundary contracts) stay in their service directories. The store **reads
  them in place** and never relocates them. The index unifies; the layout does
  not annex. The one verdi-owned file in a service root is the
  `verdi.bindings.yaml` AC-binding sidecar (evidence-model spec), discovered
  and validated like the rest.

## Store manifest: `verdi.yaml`

```yaml
schema: verdi.layout/v1
forge: gitlab                  # or github; auto-detected from the remote URL when omitted
providers:                     # story-provider spec owns semantics
  jira:
    base_url: https://koalafi.atlassian.net
    rollup_field: customfield_10142     # ids only; secrets come from env/CI vars
lint:
  gated_generated: []          # committed generated artifacts that are currency-gated
align:                         # evidence-model spec owns semantics
  judge_cmd: claude -p         # the alignment judge; this is the default
  judge_required: false        # true: `verdi align` fails outright without a judge
derived:
  retention_days: 14           # gc horizon for merged/deleted refs
services:
  discovery: flowmap           # any directory containing .flowmap.yaml is a service root
```

Decode is strict: unknown top-level keys fail `verdi lint`. Layout migrations
bump `verdi.layout/vN` and regenerate affected paths in one coordinated change.

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
| frozen          | none — point-in-time record  | immutability lint (VL-010)           | feature specs at acceptance, board.json, rollup.json, final alignment reports, attestations |

Class transitions are part of the doctrine: a feature spec is
authored-living while `draft` and becomes frozen at acceptance (the merge of
its spec MR); the alignment report is living-gated during the build (its
currency gate is `covers` = MR head) and becomes frozen at closure. A
transition is always a ritual (`verdi accept`, `verdi close`), never a hand
edit.

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
  `data/writer.lock`: `verdi serve` is that writer, and it additionally
  exposes the MCP endpoint on the checkout's unix socket `data/serve.sock`.
  `verdi mcp` is a stdio shim — it proxies to a running serve over the
  socket, or acquires the lock and serves standalone when none is running.
  Per-checkout sockets mean worktrees never collide. CI writes only
  commit-scoped derived paths.
- **D4 — Disposable caches.** Cache filenames embed layout version and tree
  hash; staleness is detected, never guessed. The tree hash covers the whole
  **corpus**, not just `.verdi/`: a hash over the sorted (path, blob-sha)
  pairs of the committed zone (minus `data/`) plus every discovered
  corpus-contributing file in service roots — so a boundary-contract or
  obligation change invalidates the cache exactly like a spec change does.
  `rm -rf .verdi/data/cache` is always safe.

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
