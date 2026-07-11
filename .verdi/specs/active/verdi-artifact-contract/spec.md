---
id: spec/verdi-artifact-contract
kind: spec
class: component
title: "Artifact contract: identity, frontmatter, links, and lint"
status: active
owners: [platform-team]
links:
  - { type: depends-on, ref: spec/verdi-store-layout }
schema: verdi.artifact/v1
---

# Artifact contract

## Purpose

The contract is the product. Every surface — lint, index, fold, workbench, MCP,
dex — is a consumer of the schemas in this document. A change here is a
coordinated change everywhere, so the contract is versioned
(`verdi.artifact/v1`), decoded strictly, and gated by CODEOWNERS.

## Identity and references

- **Canonical ref:** `<kind>/<name>`, e.g. `adr/0012-outbox-loansvc-events`,
  `spec/loansvc-stale-decline`. `name` is kebab-case, unique within its kind;
  the pair is globally unique.
- **Pinned ref:** `<kind>/<name>@<commit>` — the only form permitted in
  context manifests, evidence records, and board pins.
- **Path derivation:** single-file kinds live at
  `.verdi/<kind-dir>/<name>.md`; directory kinds (specs) at
  `.verdi/specs/<status-dir>/<name>/`. Lint enforces agreement (VL-002).
  Because status is encoded in the path for specs, an active→archive move
  changes the path but never the ref; permalinks and links use refs.
- **Never duplicate git.** Frontmatter carries no created/updated dates —
  git owns time. The exceptions are load-bearing stamps: `frozen:` and
  `decided:` (ADRs), which record doctrine-relevant moments, not file history.
- **External refs (provisional).** In-place service artifacts — boundary
  contracts, obligations, goldens, OpenAPI files — get read-only identity
  minted by the index from discovery: `svc/<service>/<artifact>[/<name>]`,
  e.g. `svc/loansvc/boundary-contract`,
  `svc/loansvc/obligations/audit-before-publish`. They are valid link
  targets, participate in backlinks, and get dex permalinks under
  `/a/svc/...`, but are never authored under `.verdi/` and never linted as
  verdi kinds — VL-003 resolves them against discovery instead of the
  committed zone. Marked provisional: the ref grammar may change once the
  first real cross-links exist; the read-only, index-minted property will
  not.

## Common frontmatter

```yaml
id: <kind>/<name>          # required, must agree with path
kind: spec | adr | diagram | attestation | waiver
title: string              # required
status: <per-kind enum>    # required
owners: [string]           # required; team or CODEOWNERS-resolvable handles
links:                     # optional, typed edges (see taxonomy)
  - { type: <link-type>, ref: <kind>/<name>, note: string? }
frozen: { at: date, commit: sha }   # required iff temporal class is frozen
provenance:                # required iff generated
  generator: string        # e.g. verdi-close, commit-to-design skill
  version: string
  inputs: [<pinned-ref | path@commit>]
  digest: sha256           # computed content only: recomputable from inputs
  integrity: sha256        # judged content only: hash of the text as written
```

Frontmatter is a **restricted YAML dialect**, decoded strictly: unknown fields
fail (VL-001), and YAML anchors, aliases, and custom tags are rejected outright
— the accepted dialect is deliberately smaller than YAML so that independent
implementations converge on identical decodes. One vendored parser, behind a
single import seam, decodes every schema in this contract.

## Kind registry

| Kind        | Dir              | Form | Statuses                          | Temporal class            |
|-------------|------------------|------|-----------------------------------|---------------------------|
| spec (feature)   | specs/{active,archive}/ | dir  | draft → accepted-pending-build → closed(archive) | frozen at acceptance (merge of the spec MR) |
| spec (component) | specs/active/           | dir  | draft → active → superseded       | authored-living           |
| adr         | adr/             | file | proposed → accepted → superseded  | frozen at acceptance      |
| diagram     | diagrams/        | file | active → superseded               | authored-living           |
| attestation | attestations/    | file | (none — existence is the record)  | frozen at commit          |
| waiver      | waivers/         | file | active → expired                  | frozen at commit          |
| conflict    | conflicts/       | file | open → superseded \| dismissed    | frozen at resolution      |

Spec classes:

- **feature** — story-linked. Requires exactly one `story:` link
  (`jira:KEY` form), an `acceptance_criteria:` block (evidence-model spec),
  and optionally `context:` (pinned manifest), `impacts: [service...]`, and
  `declares:` (intended boundaries). Lifecycle is two-MR: the spec gets its
  **own MR** from a design branch, and *merging that MR is acceptance*.
  `verdi accept <spec>` performs the mechanical flip as the final action on
  the design branch — sets `status: accepted-pending-build` and writes the
  frozen stamp with `commit` = the content-final sha it supersedes — and
  VL-004 keeps drafts off main. Build branches may only reference specs in
  `accepted-pending-build` (gated; see evidence model). The spec is **never
  amended** after acceptance: deviation is expected, and its sanctioned
  ledger is the iterative alignment report on the build branch, not spec
  edits. Adding sibling files to a frozen spec's directory
  (deviation-report.md, rollup.json) is legal — VL-010 governs files, not
  directories.
- **component** — system source-of-truth documents like this one. No story, no
  ACs, maintained by ordinary MRs, superseded rather than archived.

Feature-spec frontmatter additions:

```yaml
class: feature
story: jira:LOAN-1482
impacts: [loansvc, notification-svc]
context:
  - adr/0012-outbox-loansvc-events@3e91ab2
  - spec/messaging-gateway@9c41f2e
declares:
  boundaries:
    - { from: loansvc, to: notification-svc, via: events }
acceptance_criteria:
  - { id: ac-1, text: "...", evidence: [static] }
  - { id: ac-2, text: "...", evidence: [static, behavioral] }
  - { id: ac-3, text: "...", evidence: [behavioral] }
  - { id: ac-4, text: "...", evidence: [runtime] }
dispositions:                # written by commit-to-design (surfaces spec); VL-014
  - { sticky: a-01J8Z0K3, disposition: incorporated, where: "#ac-2" }
  - { sticky: a-01J8Z0K4, disposition: contradicted, note: "duplicates ac-1" }
  - { sticky: a-01J8Z0K5, disposition: open-question }
```

The `dispositions:` block is the commit-to-design ritual's durable output
(surfaces spec): every sticky in the frozen `board.json` lands here as
`incorporated` (`where` required — a heading anchor in this spec that must
resolve), `contradicted` (`note` required), or `open-question`. Completeness
is bidirectional: an undispositioned sticky and a disposition naming no board
sticky are both VL-014 errors.

## Link taxonomy

Backlinks are computed by inverting this table at index/dex-build time.

| Type          | Inverse (computed)  | Semantics                                            |
|---------------|---------------------|------------------------------------------------------|
| implements    | implemented-by      | this artifact realizes that one                      |
| supersedes    | superseded-by       | decision/spec replacement chain                      |
| verifies      | verified-by         | evidence artifact → AC/spec (also via evidence-for)  |
| derived-from  | source-of           | generated artifact → its inputs                      |
| annotates     | annotated-by        | annotation → target                                  |
| depends-on    | depended-on-by      | reading-order/knowledge dependency                   |
| story         | —                   | feature spec → tracker item (scheme-prefixed ref)    |
| impacts       | impacted-by         | spec → service                                       |
| challenges    | challenged-by       | conflict → the closed decision or rollup it disputes |

Proto-links (board yarn) are `{ from, to, label }` with no type; the
commit-to-design skill promotes each to a typed link or prose (surfaces spec).

## Generated artifacts and digests

Generated committed artifacts (alignment reports, rollup snapshots, board
snapshots) MUST carry `provenance` with verifiability declared honestly per
section, three-valued about our own claims:

- **Computed content** (fold results, boundary diffs, `declares:` checks,
  board snapshots) carries a `digest` recomputable by any verifier from the
  pinned inputs — the unfakeable-artifact posture groundwork applies to
  review artifacts.
- **Judged content** (the alignment report's LLM-written section) is not a
  pure function of inputs and MUST NOT claim to be: it carries an
  `integrity` hash of the text as written — tamper-evident, not
  reproducible.

An artifact containing both kinds of section carries both fields, and its
rendered form labels each section `computed` or `judged`.
`verdi verify-artifact <ref>` recomputes digests and checks integrity
hashes, reporting each separately.

## Record schemas (working area)

**Annotation** (`data/mutable/annotations/<kind>--<name>.jsonl`, append-only;
schema `verdi.annotation/v1`):

```json
{ "id": "a-01J...", "ts": "2026-07-10T14:02:11Z", "author": "john",
  "target": { "ref": "spec/loansvc-stale-decline@7f3c2a1",
              "selector": { "heading": "ac-2", "quote": "charge API", "line": null } },
  "board": { "story": "STORY-1482", "x": 262, "y": 132 },
  "type": "comment | question | decision-needed | agent-task",
  "body": "string", "status": "open | resolved | graduated" }
```

`target` is optional: a free-floating sticky — the normal early state of a
murder board — is a record with `board` and no `target`. At least one of
`target` or `board` MUST be present. Anchors pin to a commit; drift against
the working tree is computed three-valued (fresh / moved / gone) and
displayed, never silently healed.

**Board state** (`data/mutable/boards/<story>.json`, autosaved; schema
`verdi.board/v1`): `pins` (pinned refs + x/y), `stickies` (annotation ids +
x/y), `yarn` (proto-links). The frozen `board.json` committed at
commit-to-design is this schema plus a `frozen` stamp and provenance.

## Lint rules

`artifactlint` is dependency-light Go — no frameworks and no services in the
gate path; its only parser is the contract's single vendored YAML decoder —
run locally and as a CI gate. Rules:

| Rule    | Enforces                                                                      |
|---------|-------------------------------------------------------------------------------|
| VL-001  | frontmatter present, decodes strictly against kind schema; the restricted dialect is enforced here (anchors, aliases, custom tags fail) |
| VL-002  | id/path agreement; global ref uniqueness. Status-in-path applies to the feature class only: superseded component specs remain in `specs/active/` |
| VL-003  | all link refs resolve — verdi refs against the committed zone, `svc/...` external refs against discovery, and `evidence-for` bindings in discovered `verdi.bindings.yaml` sidecars (evidence-model spec) against the named spec's ACs; pins name real commits |
| VL-004  | status transitions legal per kind; `status: draft` MUST NOT exist on the default branch |
| VL-005  | feature spec has exactly one `story:` link with a configured scheme           |
| VL-006  | every AC declares ≥1 expected evidence kind (activation lint)                 |
| VL-007  | unknown entries directly under `.verdi/` fail (D1)                            |
| VL-008  | generated provenance in committed zone ⇒ on `gated_generated` allowlist OR frozen-stamped |
| VL-009  | frozen artifacts carry valid `frozen` stamp and provenance where generated    |
| VL-010  | frozen artifacts are immutable: any diff touching a frozen file fails, except a pure rename within an active→archive move |
| VL-011  | attestation/waiver files live under the story/AC they name; waiver has owner + reason, expiry optional |
| VL-012  | `.gitattributes` marks all committed-generated paths with the configured forge's generated attribute (`gitlab-generated` on GitLab, `linguist-generated` on GitHub) |
| VL-013  | nothing under `.verdi/data/` is ever git-tracked (`git add -f` is intent; lint catches it) |
| VL-014  | disposition completeness, bidirectional: every sticky id in a committed `board.json` appears in the spec's `dispositions:` block as incorporated (with a resolving `where` anchor), contradicted (with `note`), or open-question — and every entry names a real board sticky. The deterministic backstop for the commit-to-design skill's promise |

## Repository plumbing

```
# .gitattributes (repo root) — gitlab-generated here; linguist-generated on GitHub (VL-012)
.verdi/specs/*/*/board.json          gitlab-generated
.verdi/specs/*/*/rollup.json         gitlab-generated
.verdi/specs/*/*/deviation-report.md gitlab-generated
```

Marked files render collapsed in MR diffs but remain fully reviewable on
expand, and CODEOWNERS routing still fires on them — gated artifacts stay
unbypassable, just not in your face. CODEOWNERS SHOULD route
`.verdi/attestations/`, `.verdi/waivers/`, `verdi.yaml`, and this contract to
their designated owners; the human-as-oracle property depends on it.

## Open questions

- OQ-3: grandfathering rules for pre-contract documents during migration
  (lint skips `specs/archive/` for VL-001..006 on import).
