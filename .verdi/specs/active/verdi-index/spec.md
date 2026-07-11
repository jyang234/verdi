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
a local design **workbench** (murder board → spec), a stdio **MCP** server
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

## Reading order

| Spec | One line |
|------|----------|
| [01 — store layout](01-store-layout.md) | the filesystem-as-database: zones, temporal classes, disciplines, GC |
| [02 — artifact contract](02-artifact-contract.md) | identity, frontmatter, kinds, links, schemas, VL lint rules |
| [03 — evidence model](03-evidence-model.md) | ACs, evidence kinds, the fold, merge and closure gates, deviation reports |
| [04 — story provider](04-story-provider.md) | the two-method port and the Jira adapter; spec owns ACs, tracker is a read model |
| [05 — surfaces](05-surfaces.md) | CLI, workbench and board ritual, MCP, lenses, Verdi-dex |

## Glossary

- **corpus** — everything the store indexes: the committed zone plus
  in-place service artifacts.
- **ref / pinned ref** — `kind/name`, optionally `@commit`.
- **the fold** — the derivation of AC and story status from evidence records.
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
  deviation-report.md.

## v0 thin slice checklist

- [ ] `verdi.yaml` + layout scaffold committed; `.verdi/.gitignore` (`data/`)
- [ ] `artifactlint` VL-001..014, wired as a CI gate
- [ ] store walk + in-memory index; `search` and `get_artifact` correct
- [ ] `design start` → board → commit-to-design (VL-014 backstop) →
      `accept` → spec MR; `feature start` refuses non-accepted specs
- [ ] `sync --or-regen`, `matrix` (with `--preview`), `align` (computed +
      judged, digest/integrity split)
- [ ] workbench: rendered corpus, verdict viewer with cross-commit diff,
      board with autosave
- [ ] `verdi serve` as the single writer (lock + socket); `verdi mcp` shim;
      committed `.verdi/bin/` shims + `.mcp.json`
- [ ] merge gate: accepted spec + no violated AC + fresh fully-dispositioned
      alignment report (authoritative evidence only)
- [ ] `rollup --publish` with the Jira adapter (field + change-only comment)
- [ ] `dex build` publishing to member-restricted Pages: by-kind and
      by-service axes, temporal banners, backlinks, search index, changelog

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

## How these documents are maintained

These six files are component-class specs: authored-living, updated by
ordinary MRs, superseded rather than archived, and — once the layout exists —
resident at `.verdi/specs/active/` as the first citizens of the system they
describe. They are drafted as `status: draft` and activate on merge, which
VL-004 will insist on.
