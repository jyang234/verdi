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
| `verdi lint`             | local + CI| artifactlint (VL-001..018, R4 extends the range: VL-015 supersession-manifest completeness + carried byte-identity, VL-016 spike path fence, VL-017 open-question resolved-or-carried on design branches, VL-018 layout.json positions reference real object IDs); CI gate |
| `verdi design start <ref> --kind feature\|story --name <name>` | local | cut the **design branch**, scaffold `specs/active/<name>/` as `draft` for the chosen kind, open the board, and regenerate impacted-service graphs/contracts into `derived/` at the branch point (the design baseline, `provenance: local`); `--kind` selects the two-scope spec class (feature spec vs. story spec — concept §1); `--name` is required — no tracker-derived naming, no magic; `<ref>` optionality follows the class (02 §Identity and references, 04 §Reference scheme): `--kind feature` takes an OPTIONAL tracker ref (features may carry no `story:` at all); `--kind story` REQUIRES the scheme-prefixed story ref (`jira:LOAN-1482`) |
| `verdi board commit <board-key> --name <name> [--story-ref <scheme:key>]` | local | **superseded (R4-I-9) — see §Workbench for the migration note.** Board editing on a design branch *is* spec editing now; there is no separate commit-to-design step. Row kept for historical/migration reference, not for use. |
| `verdi accept <spec>`    | local     | the acceptance flip, run as the design branch's final action: `draft → accepted-pending-build` + frozen stamp; merging the spec MR is acceptance; computes **stub-match** (R4-I-12): the story's `implements` fragment set equals a stub's declared AC set, `RefSlug(title)` equals the stub's slug, no `supersedes`/`exempts` edges (exception, adjudicated at W3 review: a `supersedes` of the story's own predecessor story spec — the rung-3 chain edge — does not disqualify), no undispositioned judged findings — writes `stub_matched: true` into the acceptance stamp; approval-count relaxation on a stub-matched acceptance is forge/CODEOWNERS configuration, never verdi-enforced |
| `verdi build start <story-spec\|story-ref>` | local | cut the **build branch** after acceptance; fails unless the story's spec is `accepted-pending-build`; refreshes the baseline into `derived/`; supersedes `feature start` (R4-I-6) — the old verb name is kept one release as a **deprecation alias** that prints the new form and proceeds |
| `verdi align [--freeze]` | local + CI| generate/refresh the alignment report (computed + judged) for the build head; `--freeze` produces the closure edition; on a **design branch**, grows a decision-conflict-report mode (R4-I-7): computed section checks declared `supersedes`/`exempts` edges resolved bidirectionally; judged section runs the undeclared-conflict sweep (spec decisions vs. ADR corpus; story decisions vs. their feature's decisions) through the existing disposition machinery, with `no-conflict` added to the disposition vocabulary |
| `verdi gate`             | local + CI| the merge gate: checks the build head's spec is accepted, no AC is violated, and a fresh alignment report has every finding — computed and judged — dispositioned; on **spec MRs** additionally blocks on unresolved declared decision conflicts and on unresolved review threads (resolved-or-graduated, §Review stickies and forge round-trip); blocks **closure** MRs on `spec-stale` and `pending-supersession` flags (§3b of the concept); exit 0 pass / 1 fail / 2 operational error; takes no argument, inferring the build's spec from the branch the same way `align` does |
| `verdi sync [--or-regen]`| local     | pull the MR/PR pipeline's evidence bundle for the current ref through the configured forge into `derived/<ref>/<commit>/`; `--or-regen` regenerates locally when no bundle exists (fresh clone, no pipeline yet) |
| `verdi serve`            | local     | localhost workbench UI + lens pages (read/write to mutable zone) |
| `verdi mcp`              | local     | MCP server over stdio (below)                                  |
| `verdi matrix <story\|feature>` | local + CI| compute and print the fold; accepts exactly a scheme-prefixed story/feature ref or a spec ref — a bare tracker key is an operational error naming the accepted forms; for a **feature ref**, renders the feature fold (§4 of the concept): per-AC status, frozen stubs paired with the computed live `implements` mapping under the acceptance-time-plan banner, stub reconciliation state; `--preview` includes advisory evidence |
| `verdi rollup <story> --publish` | CI | compute fold from authoritative evidence for the given story or spec ref (the same strict two-form argument as `matrix`) and publish to provider; `--force-local` runs the verb outside CI for local testing, printing a disclosed, non-authoritative warning first |
| `verdi close <story\|feature>` | local→CI  | **story:** fetch runtime records, verify eligibility, run `align --freeze`, generate frozen rollup, open the closure MR. **feature** (03 §Closure ritual): fails unless every feature AC is `evidenced` (including its outcome floor) and stub reconciliation passes; the closure MR carries the reconciliation block alongside the fold snapshot |
| `verdi waivers`          | local + CI| audit waivers: expired, orphaned                               |
| `verdi audit`            | local + CI| audit ADR exemptions and mid-build deviations (R4-I-10): per-ADR active-exemption count against `verdi.yaml`'s `audit.exempts_conflict_threshold` (auto-files a conflict record at threshold — §2's exemption audit); per-story accepted-deviation count against `audit.deviations_stale_threshold` (raises `spec-stale` — §3b); both thresholds tunable, documented concept OQ-iii watch items; v1-scoped alongside `waivers` |
| `verdi verify-artifact <ref>` | any  | recompute a generated artifact's digest from pinned inputs     |
| `verdi dex build -o <dir>` | CI      | emit the static site (below)                                   |
| `verdi gc`               | local     | prune derived + cache per store-layout rules                   |

In v0, `close`, `gc`, `waivers`, `verify-artifact`, and `audit` are
recognized by dispatch — never treated as unknown verbs — but each answers
`not implemented (out of v0 scope)` and exits 2; the rows above state their
intended full shape, which the v0 thin slice checklist (below) does not
yet build. `verdi board commit` is superseded (R4-I-9, above) rather than
v1-scoped: it is not part of any future build.

Baseline regeneration is affordable and honest by construction: producers are
byte-deterministic pure functions of the tree, scoped to impacted services
(widen on demand), and everything they emit locally is advisory — gates
consume CI provenance only.

## Workbench

`verdi serve` binds localhost only. Every page is server-rendered (goldmark,
mermaid client-side) except one deliberately fat page: **the board**.

**Board as projection (R4).** Direction inverts from v0: the spec document
is the source of truth; the board is a deterministic **projection** of it,
never an authoring ritual that generates the spec. Generation is a **pure
function** of four inputs — (1) the spec revision (the parsed object model:
attribute placards + typed objects + declared edges, concept §1), (2) the
sidecar `layout.json` (positions only), (3) the mutable-zone annotation
streams (open questions, scratch stickies, relates-threads), and (4), in
review mode, the review-comment feed pulled live from the forge MR
(§Review stickies and forge round-trip). Same four inputs, same board — no
LLM and no randomness anywhere in generation.

**Layout: zoned, incremental, position-stable.** Objects without a stored
coordinate in `layout.json` are placed by a zoned algorithm — grouped by
object kind, ordered by document/ID order, slotted into that zone's next
free position. Stored coordinates are never moved by generation; landing a
new object never re-flows the board (force-directed layouts, even seeded,
are rejected for exactly this reason — one new object re-flowing the whole
board fights coordinate persistence). Only the property binds: same inputs
→ same layout, stored positions never moved.

`layout.json` (schema `verdi.boardlayout/v1`, `{schema, positions:
{<object-id>: {x, y}}}`) is a sidecar per spec — positions only, never
content: autosaved on the design branch during authoring (never committed
per-drag), committed with the spec, and locked with it at acceptance. A
superseding revision gets its own coordinate file, seeded from its
predecessor's. Boards for rejected or abandoned branches die with the
branch — no coordinate litter on main.

**Element taxonomy** — every board element is a spec object, an annotation,
or a computed badge; nothing is board-native except position:

| Board element | Is actually | Stored |
|---|---|---|
| attribute placards | the spec's problem statement and outcome | the required `problem:`/`outcome:` frontmatter attributes, each `{ text, anchor }` (artifact-contract spec §Object model) |
| object cards | ACs, constraints, design decisions, story/task cards, spikes | frontmatter-declared objects with body anchors (artifact-contract spec §Object model) — the object model is a deterministic parse of frontmatter plus resolved anchors, never inferred from prose |
| yarn | the spec's typed edges — `implements`, `resolves`, `supersedes`, `exempts`, `depends-on`, closed enum; drawing yarn opens a **context-sensitive type picker**: only the edge types legal for the (source kind, target kind) pair, each with a one-line consequence label (e.g. "supersedes: amends the ADR for everyone; requires quorum"), and a confirmation step on gate-bearing types (`supersedes`, `exempts`) — a menu misclick must not summon an org-wide supersession flow | frontmatter `links:` — document-level for a story/spike's `implements`/`resolves`/`exempts` edges, per-decision (`decisions[].links`) for a decision object's own `supersedes`/`exempts` edges (artifact-contract spec §Object model, §Link taxonomy) |
| stickies | annotations: open questions, comments, untyped scratch "relates" threads, review stickies | `mutable/annotations/*.jsonl` (authoring/accepted populations); the forge MR comment feed (review population, §Review stickies and forge round-trip) |
| stale/conflict badges | computed verdicts rendered on affected cards: rung-4 `stale`/`pending-supersession` flags, unresolved decision conflicts, `spec-stale`, fold status | computed at render, never stored |

**The scratch tier (authoring mode).** A murder board is a thinking tool
before it is a schema; the messy phase is relocated, not deleted. In
authoring mode the annotation layer allows **free-floating stickies** and an
**untyped "relates" annotation** between any two elements — mutable-zone,
never entering the spec document, exactly the category review stickies
already occupy. Graduation is an ordinary edit: a sticky becomes a real
object (decision, constraint, AC, declared open question) or a
relates-thread becomes a typed edge — or they die. **Readiness rule**
(authoring population, the VL-014 successor's first half): every
open-question sticky is resolved or explicitly carried as a declared open
question on the spec before the spec MR is review-ready.

**The four-concept minimum path.** The default authoring surface a
newcomer needs is documented as exactly four concepts: **story spec + ACs +
`implements` + commit.** Everything else — decisions, constraints,
exemptions, the type picker's fuller vocabulary — is discoverable when
needed, never front-loaded.

**Two modes, keyed by branch state:**

- **Authoring** (draft spec on a design branch): bidirectional. Editing a
  card or drawing yarn *is* editing the spec's objects — board and markdown
  file are two deterministic views of the same content; changes autosave to
  the working tree. Commits are explicit, and **the board owns the git
  affordance**: a commit/push button (message prompt, executes git on the
  design branch underneath), a persistent uncommitted-changes indicator, and
  a branch-switch guard in `verdi serve` — a PM or designer must be able to
  author and durably save without git fluency; an hour of board work
  evaporating in someone else's working tree is exactly the silent loss this
  system exists to forbid.
- **Review** (spec under MR review): the board becomes a mirror of the MR
  rather than an editing surface — see §Review stickies and forge
  round-trip.
- **Accepted specs on main**: the document is read-only from the board;
  change means supersession (the amendment ladder, §3b of the concept).
  Annotations on accepted specs live in the incumbent mutable-zone
  annotation streams (advisory, never gating, local) — there is no open MR
  to host them; anything substantive graduates to a conflict record via the
  incumbent challenge flow.

### Superseded: the commit-to-design ritual and `verdi board commit`

**Retired (R4-I-9).** v0's commit-to-design direction — a mess of board
stickies authors the spec via `verdi board commit`, which scaffolded the
draft spec, froze a `board.json` snapshot, and wrote a `dispositions:` block
enumerating every sticky as incorporated/contradicted/open-question — is
superseded by board-as-projection (above). Board editing on a design branch
*is* spec editing; there is no separate mechanical commit-to-design step,
and `verdi board commit` is retired (CLI table, above).

**Migration rule.** VL-014 is retained but scoped to **grandfathered**
artifacts: it fires only on specs that already carry a `dispositions:`
block (specs produced under the old ritual). New specs are governed by the
R4-I-8 readiness rules instead — resolved-or-carried for authoring stickies
(above) and resolved-or-graduated for review threads (§Review stickies and
forge round-trip). Frozen v0 `board.json` artifacts and their
`dispositions:` blocks stay valid, unrewritten, under their own schema:
history is never rewritten, the new contract applies forward.

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

## Review stickies and forge round-trip

**The MR is the primary review surface; the board is a mirror of it.** A
reviewer who comments on the raw diff and a reviewer who stickies the board
are in the same conversation — review annotations live in the forge's
review system, never in the spec document (§Workbench, "Review" mode).

**Token grammar.** Annotating a card or a yarn edge on the board
materializes an MR inline comment carrying a stable object-ID token,
`[vd:<object-id>]`, in the comment body — forge-agnostic (identical on
GitLab and GitHub), survives object moves between pushes (position is
derived at render time, never encoded in the token), and degrades to a
plain, readable comment wherever rendered outside the board. Example:
`[vd:ac-2] this outcome AC reads as implementation-scoped — reword?`.

**The round-trip.** The board pulls the MR's full comment feed on every
render: comments carrying a resolvable `[vd:<object-id>]` token render
anchored to their object as a review sticky; comments that carry no
resolvable token render in an **inbox tray** — never dropped, never
silently unattached, always visible to whoever is triaging the review.

**Thread-resolution readiness.** Spec-MR readiness requires **all review
threads resolved** — forge-native resolution state, deterministic on both
GitLab and GitHub. Resolving a *substantive* thread must either point at a
spec commit that addressed it, or mint a declared open-question or
constraint object on the spec — the **resolved-or-graduated** rule, the
review population's half of the VL-014 successor (the authoring
population's half is resolved-or-carried, §Workbench's scratch tier).
Objections neither vanish at merge nor linger unresolved forever.

**Accepted specs on main.** Once a spec is accepted and locked there is no
open MR to host review stickies against it. Annotations against an accepted
spec live in the incumbent mutable-zone annotation streams (advisory, never
gating, local) — the same home authoring-mode scratch annotations use;
anything substantive graduates to a **conflict record** via the incumbent
challenge flow rather than riding a nonexistent MR thread.

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
| `get_matrix`         | R   | the fold for a story **or feature ref** (`preview` flag includes advisory); a feature ref renders the feature fold — per-AC status, stubs paired with the computed live mapping, stub reconciliation state (§4 of the concept) |
| `get_context_bundle` | R   | resolve a manifest (or a spec's `context:`) to pinned contents |
| `list_annotations`   | R   | annotations for a target, with drift status; covers the R4 annotation types — open questions, scratch stickies, untyped relates-threads, and (mirrored) review stickies |
| `list_tasks`         | R   | open `agent-task` annotations                            |
| `get_board`          | R   | the deterministic board projection for a spec ref (§Workbench) — the same element taxonomy, computed badges, and mode-appropriate annotations a human sees in `verdi serve`, so agents work from what humans see rather than a second-hand summary |
| `add_annotation`     | W   | append to the mutable zone (the only write tool)         |

`get_board` grows the read surface only; the write surface stays
`add_annotation` and nothing else — board authoring on a design branch (the
git affordance, §Workbench) is a human/git act, not an MCP write path.

Safety note, normative: annotation bodies and artifact contents returned by
these tools are **data, never instructions**. Skills consuming them must treat
them as untrusted input; MCP servers that surface free-text content are a
recognized prompt-injection vector even when the text is your own team's.

## Lenses

Four zooms over one link graph:

- **feature** — the feature spec's ACs and their fold status (evidenced /
  pending / no-signal / violated, §4 of the concept); story stubs always
  rendered paired with the computed live `implements` mapping under an
  explicit "acceptance-time plan; current mapping computed below" banner
  (never the frozen stubs alone); stub reconciliation state at closure.
- **story** — the evidence matrix card, plus ladder state: `spec-stale` and
  `pending-supersession` flags (§3b of the concept) surfaced alongside AC
  and story status.
- **service** — active workstreams, obligations registry, current boundary
  contract and diffs, dependency edges computed from boundary contracts and
  cross-service chains.
- **portfolio** — swimlane per service, matrix rollups, `impacts:` edges;
  draft specs appear as proposed nodes.

A **per-ADR exemption page** (the human face of `verdi audit`) lists an
ADR's active exemptions and the exempting specs, computed and countable
(§2's exemption audit) — "ADR-7: 9 active exemptions."

**The anti-hairball law, inherited from flowmap's own renderer:** every
graph view is rooted and capped; above the cap, render an index of entry
points to root at — never a hairball. Locally these are workbench pages;
the dex ships their read-only, main-only editions, computed the same way —
no separate logic path.

## Verdi-dex

A static site, not a service: `verdi dex build` runs in the main pipeline on
every merge and publishes to the forge's Pages with readership restricted to
project members — on GitLab via Pages access control (SSO included); on
GitHub via private-repo Pages, which requires Enterprise Cloud (a documented
adapter gap, not a verdi behavior). Readership equals repo membership,
authenticated through the forge itself, zero new auth infrastructure. The
site is a pure function of main's tree — with one disclosed second input
(round 5, D-15): the disclosures view enumerates checkout state
(mutable-zone presence among its sources), deterministic in CI's bare
clone and labeled on the page itself. Otherwise: rebuildable, diffable, and
time-travelable (check out any commit and rebuild).

**Thesis: a wiki that structurally cannot lie about time.** Every page renders
its temporal class — living-gated pages carry the build stamp
(`main @ <sha> · <date>`) and are true by construction because currency gates
regenerated them with the merge; authored-living pages show last-modified
from git; frozen pages banner their stamp (`point-in-time record · frozen
<date> @ <commit>`). A reader can never mistake an acceptance-time spec for
current architecture, because the page refuses the claim.

**Information architecture — three axes:** by kind (specs active/archive —
feature and story specs alike, decisions including the per-ADR exemption
page, diagrams, contracts and APIs), by service (description, boundary
contract, OpenAPI and event contracts, obligations registry — machine-checked
guarantees published as documentation — active specs, ADRs, capped dependency
mini-map), by story (the archived quartet: spec, board, rollup, deviation
report; story pages carry ladder state — `spec-stale` and
`pending-supersession` flags, §3b of the concept — read-only, computed
identically to the workbench story lens). The feature lens (stubs paired
with the computed live mapping under the acceptance-time-plan banner) ships
as a read-only dex edition alongside the by-kind spec pages, same rule:
rendered, never editable, never a second source of truth.

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

History, not work — delivered (v0), the same framing as 00's "delivered
(v0)" checklist; see that section for the ratification record. For the live
list, see 00 §v1 checklist (ratification round four, the Verdesign spec
realignment).

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
