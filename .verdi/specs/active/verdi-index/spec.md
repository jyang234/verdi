---
id: spec/verdi-index
kind: spec
class: component
title: "Verdi: knowledge corpus and design workbench — spec index"
status: active
owners: [platform-team]
links:
  - { type: depends-on, ref: spec/verdi-store-layout }
  - { type: depends-on, ref: spec/verdi-artifact-contract }
  - { type: depends-on, ref: spec/verdi-evidence-model }
  - { type: depends-on, ref: spec/verdi-story-provider }
  - { type: depends-on, ref: spec/verdi-surfaces }
---

# Verdi: spec index

One Go library over a filesystem store in the monorepo, with four surfaces:
a local design **workbench** (spec ⇄ murder board projection — the spec is
the source, the board is its deterministic projection, ratification round
four), a stdio **MCP** server
for agents, a **CLI** whose verbs double as CI gates, and **Verdi-dex** — a
static, read-only, main-only documentation site the pipeline publishes on
every merge. No servers, no database, no new auth: a binary, a repo, a Jira
token, and a Pages site.

Forge support: verdi runs on GitLab and GitHub through a small forge port
with per-forge adapters. These specs state forge mechanics in GitLab terms
(MRs, CODEOWNERS section approvals, `gitlab-generated`, member-restricted
Pages); the GitHub adapter maps each to its equivalent (PRs, CODEOWNERS plus
branch-protection approvals, `linguist-generated`, Pages — noting that
private-repo Pages requires GitHub Enterprise Cloud). The adapter is selected
by `verdi.yaml`'s `forge:` key, auto-detected from the remote URL when
omitted. "MR" throughout these specs reads as "PR" on GitHub.

## Constitution

Inherited from verdi-go and binding on every component:

1. Every output is a pure function of its inputs; derived state is disposable.
2. Three-valued honesty everywhere: proven, violated-with-witness, or
   disclosed-as-unproven. Silence is never a pass. (Applied here to evidence,
   freshness, provenance, and anchor drift.)
3. A human via CODEOWNERS is always the oracle; attestations make the
   oracle's answers durable.
4. Artifacts that gate come from trusted CI, never from the author under
   review; local regeneration is advisory.
5. Strict decode, versioned schemas, unknown fields fail loudly.
6. Views are never authoritative; gated artifacts carry recomputable digests.
7. The commit boundary is the audit line and the sharing line.
8. Every committed artifact is living-gated, authored-living, or frozen —
   no undisclosed staleness.
9. Human gates are placed at agreement points and priced by information
   content — computed answers get deterministic fast paths, quorum scales
   with computed blast radius.
10. An author's own assertion is never a fold's only input — every
    self-asserted claim is paired with a deterministic cross-check or an
    independent oracle.

## Reading order

| Spec | One line |
|------|----------|
| [01 — store layout](01-store-layout.md) | the filesystem-as-database: zones, temporal classes, disciplines, GC; per-spec sidecars (coordinate files) |
| [02 — artifact contract](02-artifact-contract.md) | identity, frontmatter, kinds, links/edges (`implements`/`resolves`/`supersedes`/`exempts`/`depends-on`, object-fragment refs), the spec object model, the object manifest, schemas, VL lint rules v2 |
| [03 — evidence model](03-evidence-model.md) | ACs, evidence kinds, the story fold and the feature fold with its outcome floor, stub reconciliation, merge and closure gates, the amendment ladder, deviation reports |
| [04 — story provider](04-story-provider.md) | the two-method port and the Jira adapter; story specs own the tracker ref, feature specs carry it optionally; tracker is a read model |
| [05 — surfaces](05-surfaces.md) | CLI, the murder board as a deterministic projection (scratch tier, review stickies, board-owned git affordance), MCP, lenses, Verdi-dex |

## Glossary

- **corpus** — everything the store indexes: the committed zone plus
  in-place service artifacts.
- **ref / pinned ref** — `kind/name`, optionally `@commit`.
- **the fold** — the derivation of status from evidence and records; now
  names both levels — the story fold (AC and story status from evidence
  records, unchanged) and the feature fold (feature AC status aggregated
  from implementing stories, plus the outcome floor).
- **advisory / authoritative** — local vs CI provenance; only authoritative
  evidence feeds gates.
- **living-gated / authored-living / frozen** — the three temporal classes.
- **graduation** — rewriting a mutable-zone record as a committed artifact.
- **acceptance** — the merge of a spec's own MR to main; `verdi accept`
  performs the mechanical flip to `accepted-pending-build` and the freeze.
- **alignment report** — the iterative, per-build-head record of how the
  implementation deviates from the accepted spec; living-gated during the
  build, frozen at closure.
- **conflict** — a committed challenge to a closed decision; resolved by
  supersession (two Code Owner approvals) or dismissal, never deleted.
- **external ref** — index-minted, read-only identity for in-place service
  artifacts: `svc/<service>/<artifact>`.
- **the quartet** — an archived story's spec, board.json, rollup.json, and
  deviation-report.md. Round four: new specs archive `layout.json` (the
  board coordinate sidecar, §01) in the quartet's board-artifact slot in
  place of v0's frozen `board.json`; `board.json` is the grandfathered form,
  still valid and unrewritten in pre-R4 archives (§02, §03 §Alignment
  report).
- **feature spec** — the birds-eye spec: a grouping of user stories that
  together deliver one tangible business outcome; its ACs are strictly
  user-observable, implementation-blind.
- **story spec** — a user story's own spec document: implementation-scoped
  ACs, decisions, and constraints, plus `implements` edges to the feature
  AC(s) it serves.
- **stub** — a feature spec's acceptance-time scoping record (title +
  outcome + the ACs it serves) for an intended story; reconciled against
  real stories at feature closure.
- **implements edge** — the authored link from a story spec's AC to the
  feature AC it serves, CODEOWNERS-routed to the feature's owners; the
  feature's live AC→story mapping is computed from these, never authored on
  the feature.
- **object / object ID** — a typed, anchored block embedded in a spec's
  prose (AC, constraint, design decision, etc.) carrying a stable ID that
  is anchorable, relatable, and renderable as a board card.
- **the feature fold** — see **the fold**.
- **outcome floor** — the requirement that every feature AC bind at least
  one direct outcome-level record — an automated producer or a
  CODEOWNERS-routed outcome attestation — so an outcome claim is never
  inferred solely from story bookkeeping.
- **stub reconciliation** — the feature-closure check that every
  acceptance-time stub is realized-by named closed stories or explicitly
  withdrawn-with-note.
- **the amendment ladder (rungs 1–4)** — the priced escalation for
  mid-build spec change: (1) deviation — spec intent intact; (2)
  wrong-for-me — an `exempts` edge, no amendment; (3) story-spec
  supersession — the story's own ACs/approach were wrong; (4) feature-spec
  supersession — the cascade, computed downstream verdicts, priced by blast
  radius.
- **object manifest** — the structured `supersession:` block on a
  superseding spec classifying every predecessor object as `carried`
  (lint-enforced byte-identical), `amended`, `amended-advisory`, or
  `removed`, plus objects `added`.
- **re-affirmation** — a per (story, amended object) attestation-shaped
  record, CODEOWNERS-routed to the story owner and embedding the
  old→new content-hash pair, required before a stale story may close.
- **scratch tier** — the board's authoring-mode annotation layer:
  free-floating stickies and untyped relates-threads that never enter the
  spec document until graduated by an ordinary edit.
- **review sticky** — a commit-free board annotation that materializes as
  an MR inline comment carrying a `[vd:<object-id>]` token; lives in the
  forge's review system, never in the document.
- **spike** — a timeboxed story subtype required to carry `resolves` edges
  to at least one open question, attachable to a draft or accepted
  feature; exempt from the evidence model but path-fenced from product
  source.

## delivered (v0)

History, not work — the v0 thin slice, complete and frozen. See
`docs/design/specs/08-revision-notes.md` for the ratification record.

- [x] `verdi.yaml` + layout scaffold committed; `.verdi/.gitignore` (`data/`)
- [x] `artifactlint` VL-001..014, wired as a CI gate
- [x] store walk + in-memory index; `search` and `get_artifact` correct
- [x] `design start` → board → commit-to-design (VL-014 backstop) →
      `accept` → spec MR; `feature start` refuses non-accepted specs
- [x] `sync --or-regen`, `matrix` (with `--preview`), `align` (computed +
      judged, digest/integrity split)
- [x] workbench: rendered corpus, verdict viewer with cross-commit diff,
      board with autosave
- [x] `verdi serve` as the single writer (lock + socket); `verdi mcp` shim;
      committed `.verdi/bin/` shims + `.mcp.json`
- [x] merge gate: accepted spec + no violated AC + fresh fully-dispositioned
      alignment report (authoritative evidence only)
- [x] `rollup --publish` with the Jira adapter (field + change-only comment)
- [x] `dex build` publishing to member-restricted Pages: by-kind and
      by-service axes, temporal banners, backlinks, search index, changelog

## v1 checklist

Delivered 2026-07-11 (the v1 build, PLAN-V1.md; every item's exit
criteria proven, final whole-branch review READY-AFTER-FIXES with the one
fix applied — see the round-4 execution ledger). Originally: the live
list, per ratification round four (the Verdesign spec
realignment). Build contract: `PLAN-V1.md`.

- [x] contract v2: object model, story class, edges, object manifest
- [x] lint v2: VL-015..018 + rescopes of the v0 rules
- [x] feature fold + outcome floor + stub reconciliation
- [x] ladder machinery: spec-stale / pending-supersession flags, cascade
      verdicts, re-affirmations
- [x] conflict gate (declared + judged) + sweep + exemption audit
- [x] lifecycle verbs v2 (`design start --kind`, `build start`, deprecation
      alias for `feature start`)
- [x] board v2: spec-is-source projection, scratch tier, board-owned git
      affordance
- [x] review-sticky forge round-trip (`[vd:<object-id>]` tokens, inbox tray)
- [x] dex/lens updates for the two-level model
- [x] v0 grandfathering (frozen v0 artifacts remain valid under their own
      schemas)

## Open questions (carried, owned, not hidden)

- **OQ-1** — Jira workflow validator on the Done transition needs a Jira
  admin; fallback is field-plus-convention.
- **OQ-2** — runtime evidence mechanism; record schema is
  mechanism-agnostic, defer to the first runtime AC — but it MUST be
  queryable by (story, AC) at close time.
- **OQ-3** — one-time migration of existing spec artifacts; lint
  grandfathers `specs/archive/`.
- **OQ-4** — waiver-culture friction: audit is the counterweight; tune in
  the first month.
- **OQ-5** — **resolved**: verdi is a standalone repository and Go module
  (`github.com/OWNER/verdi`, single binary `cmd/verdi`), hosting its own
  `.verdi/` store. verdi-go is an upstream dependency: verdi execs its
  pinned flowmap/groundwork CLIs and strict-decodes their JSON artifacts,
  never linking their internal packages. The shims pin
  `github.com/OWNER/verdi/cmd/verdi` directly.

Carried from the Verdesign spec realignment concept (round four):

- **OQ-i** — feature-AC granularity norm: the ship-independently heuristic
  is the default; tune against real reviews.
- **OQ-ii** — rung-4 re-affirmation rubber-stamping: hash-pair
  re-affirmations make it countable; watch the count, revisit if it trends
  to 100% same-day approvals.
- **OQ-iii** — threshold tuning: the spec-stale deviation count, the
  per-ADR exemption threshold, and stub-match strictness are config values;
  tune in the first month, mirroring the OQ-4 posture.
- **OQ-iv** — outcome-evidence producers: the attestation floor is settled;
  which automated outcome-level producers (e2e journeys, runtime checks)
  exist and how they're declared follows the pluggable-producer seam and
  OQ-2.

## How these documents are maintained

These six files are component-class specs: authored-living, updated by
ordinary MRs, superseded rather than archived, and — once the layout exists —
resident at `.verdi/specs/active/` as the first citizens of the system they
describe. They are drafted as `status: draft` and activate on merge, which
VL-004 will insist on.
