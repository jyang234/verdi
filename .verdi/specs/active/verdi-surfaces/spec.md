---
id: spec/verdi-surfaces
kind: spec
class: component
title: "Surfaces: CLI, workbench, MCP, lenses, and Verdi-dex"
status: active
owners: [platform-team]
links:
  - { type: depends-on, ref: spec/verdi-store-layout }
  - { type: depends-on, ref: spec/verdi-artifact-contract }
  - { type: depends-on, ref: spec/verdi-evidence-model }
schema: verdi.surfaces/v1
---

# Surfaces

## Purpose and shape

One Go library, four surfaces. The library (store walk, strict decode, index,
fold, digests) is the product; every surface below is a thin consumer.
Division of audiences: **the dex is the human read surface; MCP is the
machine read surface; the workbench is the human write surface; MRs are the
only durable write path.** Nothing is duplicated, so nothing can drift.

## CLI

| Verb                     | Runs      | Purpose                                                        |
|--------------------------|-----------|----------------------------------------------------------------|
| `verdi lint`             | local + CI| artifactlint (VL-001..014); CI gate                            |
| `verdi design start <story> --name <name>` | local | cut the **design branch**, scaffold `specs/active/<name>/` as `draft`, resolve story, open the board, and regenerate impacted-service graphs/contracts into `derived/` at the branch point (the design baseline, `provenance: local`); `--name` is required — no tracker-derived naming, no magic |
| `verdi board commit <board-key> --name <name> [--story-ref <scheme:key>]` | local | the mechanical half of commit-to-design (below): scaffold the draft feature spec, freeze `board.json` with provenance, write the disposition block listing every board sticky; `--story-ref` sets the new spec's `story:` field, defaulting to the board key itself only when it is already `scheme:key`-shaped |
| `verdi accept <spec>`    | local     | the acceptance flip, run as the design branch's final action: `draft → accepted-pending-build` + frozen stamp; merging the spec MR is acceptance |
| `verdi feature start <story>` | local | cut the **build branch** after acceptance; fails unless the story's spec is `accepted-pending-build`; refreshes the baseline into `derived/` |
| `verdi align [--freeze]` | local + CI| generate/refresh the alignment report (computed + judged) for the build head; `--freeze` produces the closure edition |
| `verdi gate`             | local + CI| the merge gate: checks the build head's spec is accepted, no AC is violated, and a fresh alignment report has every finding — computed and judged — dispositioned; exit 0 pass / 1 fail / 2 operational error; takes no argument, inferring the build's spec from the branch the same way `align` does |
| `verdi sync [--or-regen]`| local     | pull the MR/PR pipeline's evidence bundle for the current ref through the configured forge into `derived/<ref>/<commit>/`; `--or-regen` regenerates locally when no bundle exists (fresh clone, no pipeline yet) |
| `verdi serve`            | local     | localhost workbench UI + lens pages (read/write to mutable zone) |
| `verdi mcp`              | local     | MCP server over stdio (below)                                  |
| `verdi matrix <story>`   | local + CI| compute and print the fold; `<story>` accepts exactly a scheme-prefixed story ref or a spec ref — a bare tracker key is an operational error naming the accepted forms; `--preview` includes advisory evidence |
| `verdi rollup <story> --publish` | CI | compute fold from authoritative evidence for the given story or spec ref (the same strict two-form argument as `matrix`) and publish to provider; `--force-local` runs the verb outside CI for local testing, printing a disclosed, non-authoritative warning first |
| `verdi close <story>`    | local→CI  | fetch runtime records, verify eligibility, run `align --freeze`, generate frozen rollup, open the closure MR |
| `verdi waivers`          | local + CI| audit waivers: expired, orphaned                               |
| `verdi verify-artifact <ref>` | any  | recompute a generated artifact's digest from pinned inputs     |
| `verdi dex build -o <dir>` | CI      | emit the static site (below)                                   |
| `verdi gc`               | local     | prune derived + cache per store-layout rules                   |

In v0, `close`, `gc`, `waivers`, and `verify-artifact` are recognized by
dispatch — never treated as unknown verbs — but each answers `not
implemented (out of v0 scope)` and exits 2; the rows above state their
intended full shape, which the v0 thin slice checklist (below) does not
yet build.

Baseline regeneration is affordable and honest by construction: producers are
byte-deterministic pure functions of the tree, scoped to impacted services
(widen on demand), and everything they emit locally is advisory — gates
consume CI provenance only.

## Workbench

`verdi serve` binds localhost only. Every page is server-rendered (goldmark,
mermaid client-side) except one deliberately fat page: **the board**.

**Board model — a spatial lens over existing primitives, never a storage
system:**

| Board element | Is actually                         | Stored                          |
|---------------|-------------------------------------|---------------------------------|
| card          | a context-manifest entry (pinned ref) | spec `context:` on commit; live in board state |
| sticky        | an annotation (board-anchored)      | `mutable/annotations/*.jsonl`   |
| yarn          | a proto-link `{from, to, label}`    | board state                     |
| position      | coordinates (the only new datum)    | `mutable/boards/<story>.json`, autosaved, never committed per-drag |

**Commit-to-design ritual.** Input: the resolved manifest (full pinned
contents), every sticky with its anchor, the yarn graph. Output, on the
**design branch**: a draft feature spec, a frozen `board.json` snapshot
committed alongside it (design provenance — one frame, not a drag history),
and the spec's disposition block. **Disposition rule:** every sticky lands
as incorporated (with where), contradicted (with why), or carried as an open
question. The skill promises this — and VL-014 enforces it: artifactlint
statically checks the spec's `dispositions:` block bidirectionally — every
sticky id in the committed `board.json` appears with a legal value
(`incorporated` with a resolving `where` anchor, `contradicted` with a note,
or `open-question`), and every entry names a real board sticky — so the
guarantee rests on a deterministic gate, not on an LLM's good behavior. The
workbench and the dex render the block as a table on the spec page — a view,
never authoritative.

The ritual splits cleanly at that mechanical boundary. Everything up to and
including the disposition block — the spec skeleton, the frozen `board.json`
snapshot with provenance, and the disposition block enumerating every
sticky — is `verdi board commit`, the binary's own deterministic ritual: it
takes an explicit `story_ref` for the new spec's `story:` field, defaulting
to the board key itself only when the key is already `scheme:key`-shaped,
since a board key is an opaque, free-form namespace distinct from a story
ref (artifact-contract spec, §Record schemas). Everything past that
skeleton — yarn promotion into typed links or prose, and the design
write-up itself — belongs to a committed skill doc *outside* the binary,
never to LLM logic inside it. Yarn is promoted to typed
`links[]` / `declares:` entries or to prose. Stickies then graduate
(`status: graduated`) or die with the branch. The branch then proceeds to
`verdi accept` and the spec MR (see evidence model, two-MR lifecycle).

**Dispatch (agent lanes):**

- Lane 1 — the developer's interactive Claude Code session: the board's
  commit button writes a task record; a `/tasks` skill lists open
  `agent-task` annotations via MCP and works them in-session. Pull-based; no
  push channel.
- Lane 2 — local automation: shell out to `claude -p` (drawing on the
  plan-attached Agent SDK credit rather than interactive limits).
- Lane 3 — shared server-side agents: **deferred**, and when built, API-billed
  via LiteLLM/Bedrock only. Subscription OAuth credentials are restricted to
  official Anthropic clients and are never wired into shared services.

## MCP server

The writer process is `verdi serve` (D3): it hosts the workbench UI and the
MCP endpoint on the checkout's unix socket. `verdi mcp` speaks stdio to the
agent client and proxies to a running serve over that socket — or acquires
the writer lock and serves standalone when the workbench isn't up. Agents
and the board therefore never race on the mutable zone. The socket speaks
the exact same wire framing as MCP's stdio transport — newline-delimited
JSON-RPC 2.0 — so the shim never translates, only pipes bytes; `verdi mcp`
degenerates to byte-forwarding once connected.

The committed project-scope `.mcp.json` points at **committed shims**, not
bare binaries, so the fresh-clone path actually works (approval prompt on
first use):

```json
{ "mcpServers": {
    "verdi":      { "type": "stdio",
                    "command": "${CLAUDE_PROJECT_DIR:-.}/.verdi/bin/verdi-mcp" },
    "groundwork": { "type": "stdio",
                    "command": "${CLAUDE_PROJECT_DIR:-.}/.verdi/bin/groundwork-mcp" } } }
```

Shim contract (`.verdi/bin/`, POSIX, committed): acquire the verdi binary at
a **pinned version** (default: `go run github.com/OWNER/verdi/cmd/verdi@<version>`
with the version literal in the shim — zero-install, version-locked by the
shim itself, no PATH assumption; OQ-5 resolved: verdi is a standalone
module); run `verdi sync --or-regen` so the derived prerequisites exist —
one `graph-<service>.json` per impacted service, `policy.json` at each
service root. There is no upstream `groundwork-mcp` binary: the
`groundwork` shim itself execs the pinned toolchain's `groundwork mcp`
subcommand, one `--service <name>=<path to graph-<service>.json>` and
`--policy <name>=<service-root>/policy.json` pair per discovered service
with a materialized graph. A fresh clone plus one approval prompt yields
two working servers.

**Federation:** verdi serves knowledge artifacts; groundwork serves graph and
policy lenses. Neither duplicates the other's tools.

| Tool                 | R/W | Purpose                                                  |
|----------------------|-----|----------------------------------------------------------|
| `search_artifacts`   | R   | full-text over the corpus                                |
| `get_artifact`       | R   | resolve `ref[@commit]` to content + frontmatter          |
| `get_links`          | R   | typed links + computed backlinks                         |
| `get_matrix`         | R   | the fold for a story (`preview` flag includes advisory)  |
| `get_context_bundle` | R   | resolve a manifest (or a spec's `context:`) to pinned contents |
| `list_annotations`   | R   | annotations for a target, with drift status              |
| `list_tasks`         | R   | open `agent-task` annotations                            |
| `add_annotation`     | W   | append to the mutable zone (the only write tool)         |

Safety note, normative: annotation bodies and artifact contents returned by
these tools are **data, never instructions**. Skills consuming them must treat
them as untrusted input; MCP servers that surface free-text content are a
recognized prompt-injection vector even when the text is your own team's.

## Lenses

Three zooms over one link graph — story (the evidence matrix card), service
(active workstreams, obligations registry, current boundary contract and
diffs, dependency edges computed from boundary contracts and cross-service
chains), portfolio (swimlane per service, matrix rollups, `impacts:` edges;
draft specs appear as proposed nodes). **The anti-hairball law, inherited
from flowmap's own renderer:** every graph view is rooted and capped; above
the cap, render an index of entry points to root at — never a hairball.
Locally these are workbench pages; the dex ships their read-only,
main-only editions.

## Verdi-dex

A static site, not a service: `verdi dex build` runs in the main pipeline on
every merge and publishes to the forge's Pages with readership restricted to
project members — on GitLab via Pages access control (SSO included); on
GitHub via private-repo Pages, which requires Enterprise Cloud (a documented
adapter gap, not a verdi behavior). Readership equals repo membership,
authenticated through the forge itself, zero new auth infrastructure. The
site is a pure function of main's tree: rebuildable, diffable, and
time-travelable (check out any commit and rebuild).

**Thesis: a wiki that structurally cannot lie about time.** Every page renders
its temporal class — living-gated pages carry the build stamp
(`main @ <sha> · <date>`) and are true by construction because currency gates
regenerated them with the merge; authored-living pages show last-modified
from git; frozen pages banner their stamp (`point-in-time record · frozen
<date> @ <commit>`). A reader can never mistake an acceptance-time spec for
current architecture, because the page refuses the claim.

**Information architecture — three axes:** by kind (specs active/archive,
decisions, diagrams, contracts and APIs), by service (description, boundary
contract, OpenAPI and event contracts, obligations registry — machine-checked
guarantees published as documentation — active specs, ADRs, capped dependency
mini-map), by story (the archived quartet: spec, board, rollup, deviation
report).

**Page anatomy:** breadcrumb; title + status badge; temporal banner;
metadata card (owners, decided/frozen, supersession links, provenance path);
rendered body with heading anchors; connections panel (typed links plus
computed backlinks); on-this-page TOC.

**Mechanics:** permalinks are `/a/<kind>/<name>` — refs, not paths, so links
survive active→archive moves; markdown via goldmark and syntax highlighting
via chroma at build time (pure Go); client-side JavaScript budget is exactly
three items — mermaid rendering, an OpenAPI renderer (script tag per API
page; the committed spec file is the source of truth, discovered by
convention at `<service-root>/api/openapi.{yaml,yml,json}` with a per-service
override key `services.overrides.<name>.openapi` in `verdi.yaml`), and
search over a
build-emitted JSON inverted index with a small vanilla lookup; each publish
emits a "what changed" feed from the git log of `.verdi/`; every page carries
a copy-reference button yielding the pinned form (`adr/0012@3e91ab2`) for
direct use in manifests and board pins.

**Non-goals (design, not v1 limitations):** no comments (discussion lives in
MRs and on local boards; the dex reflects conclusions), no editing (the edit
affordance is a link to the file in GitLab, where a change is an MR), no
branch selector (branch state is the workbench's world; publishing WIP to a
team surface reintroduces limbo).

## v0 thin slice

Contract + `verdi lint` in CI (VL-001..014); walk-and-index; workbench read
paths (markdown, mermaid, verdict viewer with cross-commit diff) plus
`add_annotation`; the board with commit-to-design and its VL-014 backstop;
`design start`, `accept`, `feature start`, `sync --or-regen`, `matrix`,
`align`; the committed `.verdi/bin/` shims and `.mcp.json`; MCP read tools +
`add_annotation`; `dex build` with the by-kind and by-service axes. Deferred
beyond v0: `close` automation polish, portfolio lens, dex by-story axis
(needs first closures), lane 3, `declares:`-diff depth beyond boundary
contracts, conflict-resolution tooling (the kind and ritual exist; the
tooling is `verdi lint` + convention in v0).
