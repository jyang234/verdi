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
- **Fragment ref (R4-I-3).** Either ref form gains an optional
  `#<object-id>` suffix naming a spec object declared in the target's
  frontmatter (§Object model): `<kind>/<name>#<object-id>` (e.g.
  `spec/loan-update#ac-2`) and, pinned, `<kind>/<name>@<commit>#<object-id>`.
  A fragment is the only way an edge targets an object rather than a whole
  artifact — `implements`, `resolves`, and `exempts` edges (§Link taxonomy)
  require one. VL-003 resolves a fragment by parsing the target's
  frontmatter objects and matching `<object-id>` exactly; an unresolved
  fragment fails closed like any other broken link.
- **Path derivation:** single-file kinds live at
  `.verdi/<kind-dir>/<name>.md`; directory kinds (specs) at
  `.verdi/specs/<status-dir>/<name>/`. Lint enforces agreement (VL-002).
  Because status is encoded in the path for specs, an active→archive move
  changes the path but never the ref; permalinks and links use refs.
- **Never duplicate git.** Frontmatter carries no created/updated dates —
  git owns time. The exceptions are load-bearing stamps: `frozen:` and
  `decided:` (ADRs), which record doctrine-relevant moments, not file history.
- **Attestation/waiver/reaffirmation names.** These kinds nest their path by
  story and object (`attestations/<story-slug>/<ac-id>.md`,
  `waivers/<story-slug>/<ac-id>.md`,
  `reaffirmations/<story-slug>/<object-id>.md`), which the `kind/name`
  grammar does not otherwise express. Their canonical `name` is the compound
  slug `<story-slug>--<ac-id>` / `<story-slug>--<object-id>` (e.g.
  `attestation/jira-loan-1482--ac-2`, `reaffirmation/jira-loan-1482--ac-2`),
  where `<story-slug>` is `RefSlug` (store-layout spec) of the owning
  **story** spec's scheme-prefixed `story:` ref (the required scalar moved
  from the feature class to the story class, R4-I-2) — never a bare tracker
  key, which collides across schemes. **Feature outcome-attestations**
  (evidence-model spec §Attestations and waivers) reuse the attestation kind
  with the same compound grammar — name `<feature-slug>--<ac-id>`, path
  `attestations/<feature-slug>/<ac-id>.md` — but in the `<story-slug>`
  position they carry the **feature spec's name** (the `name` half of its
  ref; amended at V1-P3 phase review from the earlier `RefSlug(id)` form —
  08 §Round 4 E2), because features carry only an
  OPTIONAL tracker ref, so the slug is never tracker-derived. The path stays
  the nested two-level form; VL-002 defers path/id agreement for these kinds
  to VL-011, which maps the compound name back onto the nested path.
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
kind: spec | adr | diagram | attestation | waiver | reaffirmation | conflict
title: string              # required
schema: string             # optional; this document's own schema, e.g. verdi.artifact/v1
status: <per-kind enum>    # required
owners: [string]           # required; team or CODEOWNERS-resolvable handles
links:                     # optional, typed edges (see taxonomy)
  - { type: <link-type>, ref: <kind>/<name or name#object-id>, note: string? }
problem: { text: string, anchor: string }   # required; feature/story only — §Object model
outcome: { text: string, anchor: string }   # required; feature/story only — §Object model
acceptance_criteria: [ {id, text, evidence, anchor, synopsis?}, ... ]  # optional; feature/story only — §Object model
constraints: [ {id, text, anchor, synopsis?}, ... ]                    # optional; feature/story only — §Object model
decisions: [ {id, text, anchor, links?, synopsis?}, ... ]              # optional; feature/story only — §Object model
open_questions: [ {id, text, anchor, synopsis?}, ... ]                 # optional; feature/story only — §Object model

`synopsis` (round 5.3) is an optional authored headline on any object —
the card face renders it in place of body truncation when present; absent
means truncate. Presentation-only: no gate, fold, or edge ever reads it.
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

`schema:` is optional and unconstrained in form — a free string identifying
a document's own schema version. Every component spec in this system's
own reading order carries one (e.g. this document's `verdi.artifact/v1`);
strict decode accepts the field on every kind so a self-hosted schema
document is never rejected for describing itself.

## Object model

**Attributes** (R4 concept §1) are distinct from objects below: exactly
one each, attached to the spec itself rather than relatable or
independently identified — no `id`, no `links:`, never a board-yarn
endpoint. Feature and story specs carry two required attributes: `problem`
(the problem statement — what this spec exists to solve) and `outcome`
(what good looks like: when this exists, what will be different?). Each is
`{ text, anchor }` — same anchor-resolution rule as objects, below — and
renders on the board as the spec's attribute placards (surfaces spec
§Workbench). Component and ADR specs, having no object model, carry
neither.

Feature and story specs (§Kind registry) declare their acceptance criteria,
constraints, design decisions, and open questions as
**frontmatter-declared objects** — typed array entries, each with a stable
`id`, its `text`, and (except `constraints` and `open_questions`, see
below) evidence or edge fields:

```yaml
acceptance_criteria:
  - { id: ac-2, text: "...", evidence: [static, behavioral], anchor: "#ac-2" }
constraints:
  - { id: co-1, text: "...", anchor: "#co-1" }
decisions:
  - { id: dc-1, text: "...", anchor: "#dc-1",
      links: [ { type: exempts, ref: adr/0012-outbox-loansvc-events, note: "..." } ] }
open_questions:
  - { id: oq-1, text: "...", anchor: "#oq-1" }
```

Every object — and both attributes, above — carries an `anchor:` naming
the body heading where its prose lives. **Anchor resolution is
exact-match**: the named heading must exist verbatim in the document body
— the VL-014 `where`-anchor check, restated here as the general rule for
every object and attribute, not only dispositions. The document body is
ordered free prose — narrative, rationale, considered alternatives — and
is never itself schema; the object model is a **deterministic parse**:
frontmatter objects and attributes plus their resolved anchors, nothing
inferred from prose structure.

`acceptance_criteria` objects are outcome- or implementation-scoped
depending on class (§Kind registry) and declare their expected evidence
kinds like any AC (VL-006). `constraints` objects state a rule that applies
wherever relevant; they are deliberately not mappable to stories and carry
no `links:` of their own — a feature constraint inherits downward to its
stories, checked wherever relevant, never assigned to one. `decisions`
objects may carry their own `links:` — the same shape as document-level
`links:` (§Common frontmatter) — for `supersedes`/`exempts` edges against
ADRs or other decisions (§Link taxonomy). `open_questions` objects (added
at ratification round four's phase review — the block VL-017's "declared
open-question object" and the spike variant's `resolves` target had named
without a home in this contract) carry no `links:` of their own: they are
the *targets* of `resolves` edges (a spike's deliverable is answering
them, §Kind registry) and the graduation destination of the board's
carried open-question stickies (VL-017; surfaces spec §Workbench's
scratch tier); a resolved open question graduates into a real object or
prose by an ordinary edit, and the entry is removed in the same edit. For a
FROZEN declarer (an accepted feature whose open question a spike later
answers), resolution surfaces mechanically as the computed `resolved-by`
backlink (round 5, D-8): the spike's `resolves` edge inverts through §Link
taxonomy's table and renders in the declarer's connections panel (dex) and
on its board's open-question card — the frozen document is never edited;
the backlink is the record. A
story's `implements`/`resolves` edges are declared at the document level
(top-level `links:`), targeting a feature object fragment (§Identity and
references), not inside an `acceptance_criteria` or `constraints` entry.

Object **IDs are immutable once published** (first appearance in a spec
that reaches `accepted-pending-build` or later). An object's cross-revision
identity is the content hash of `(kind, id, text)` (the I-37 identity) —
the identity the supersession manifest classifies objects by (§Kind
registry, R4-I-4): the same `id` with an unchanged hash across revisions is
`carried`; the same `id` with a changed hash is `amended` or
`amended_advisory`.

## Kind registry

| Kind        | Dir              | Form | Statuses                          | Temporal class            |
|-------------|------------------|------|-----------------------------------|---------------------------|
| spec (feature)   | specs/{active,archive}/ | dir  | draft → accepted-pending-build → closed(archive) \| superseded | frozen at acceptance (merge of the spec MR) |
| spec (story)     | specs/{active,archive}/ | dir  | draft → accepted-pending-build → closed(archive) \| superseded | frozen at acceptance (merge of the spec MR) |
| spec (component) | specs/active/           | dir  | draft → active → superseded       | authored-living           |
| adr         | adr/             | file | proposed → accepted → superseded  | frozen at acceptance      |
| diagram     | diagrams/        | file | active → superseded               | authored-living           |
| attestation | attestations/    | file | (none — existence is the record)  | frozen at commit          |
| waiver      | waivers/         | file | active → expired                  | frozen at commit          |
| reaffirmation | reaffirmations/ | file | (none — existence is the record)  | frozen at commit          |
| conflict    | conflicts/       | file | open → superseded \| dismissed    | frozen at resolution      |

Attestation, waiver, and reaffirmation paths nest by story and object —
`attestations/<story-slug>/<ac-id>.md`, `waivers/<story-slug>/<ac-id>.md`,
`reaffirmations/<story-slug>/<object-id>.md` — per §Identity and
references' compound-name rule; VL-011 enforces the mapping.

Spec classes:

- **feature** — the birds-eye class: a grouping of stories that all deliver
  a tangible business outcome (R4 concept §1). Requires the two spec
  attributes `problem:` and `outcome:` (§Object model) plus an
  `acceptance_criteria:` block whose ACs are strictly outcome-level,
  implementation-blind, and declare their expected evidence kinds
  (§Object model) including the outcome-evidence floor — `attestation` at
  minimum (evidence-model spec). Frontmatter carries an OPTIONAL `story:`
  **scalar** — an epic/objective tracker ref, not a per-story binding
  (R4-I-2) — plus optional `constraints:` and `decisions:` objects
  (§Object model), `context:` (pinned manifest), `impacts: [service...]`,
  and `declares:` (intended boundaries). It also carries `stubs:` — the
  acceptance-time scoping record, one entry per intended story:
  `{ slug: <title-slug>, acceptance_criteria: [<ac-id>...] }`. A stub may
  instead be a **spike stub** (round 5.4, mirroring the story level's own
  discriminator): `{ slug, spike: true, resolves: [<oq-id>...] }` — the
  intended spike named with the open questions it will answer. The grammar
  fails closed: `resolves` requires `spike: true`; a spike stub declares
  `resolves` and no `acceptance_criteria`; a plain stub the reverse. One
  spike may resolve many questions; a question claimed by multiple spike
  stubs is a norm-level smell, never an error. On a
  superseding revision, a `supersession:` block is required (R4-I-4):
  `{ carried: [ids], amended: [{id, note}], amended_advisory: [{id, note}],
  removed: [{id, note}], added: [ids] }`, classifying every predecessor
  object exactly once (VL-015). Lifecycle is two-MR as before: the spec
  gets its **own MR** from a design branch, and *merging that MR is
  acceptance*. `verdi accept <spec>` performs the mechanical flip as the
  final action on the design branch — sets `status: accepted-pending-build`
  and writes the frozen stamp with `commit` = the content-final sha it
  supersedes — and VL-004 keeps drafts off main. The spec is **never
  amended** after acceptance: supersession is the only forward path
  (R4 concept §3b). The feature is **downward-blind**: it is never amended
  when stories are added, split, or superseded; the authoritative AC→story
  mapping is only ever the computed inverse of stories' `implements` edges
  (§Link taxonomy), never a field on the feature itself. **Superseded is a
  terminal status (round 5, D-12/D-16):** accepting a spec that carries a
  `supersedes` edge to a predecessor **story** spec (the rung-3 chain)
  flips that predecessor's `status:` to `superseded` in the same
  `verdi accept` ritual — a sanctioned, status-only edit (VL-004 gains the
  accepted-pending-build → superseded transition, performed only by the
  ritual; VL-010 gains the matching narrow exception alongside the
  active→archive rename: the diff may touch only the status line). A
  superseded spec stays in `specs/active/` (its supersession chain is live
  reading during the build), is excluded from the feature fold's computed
  AC→story mapping, and is refused by `verdi build start`, which names the
  successor. A superseded **feature** predecessor's status remains
  governed by the rung-4 cascade machinery for now — its terminal-state
  question is carried to round 6 (round 5's D-12 fix pass, deliberately
  scoped).
- **story** (NEW) — the unit of work, and the unit of review. Same status
  lifecycle as feature, frozen at acceptance. Requires the two spec
  attributes `problem:` and `outcome:` (§Object model), exactly one `story:`
  **scalar** field (`jira:KEY` form) as the canonical tracker reference —
  VL-005 validates the scalar's scheme and configuredness here, moved from
  the feature class (R4-I-2) — and ≥1 `implements` edge (top-level
  `links:`, §Link taxonomy) to a feature AC fragment
  (`spec/<feature-name>#<ac-id>`, §Identity and references). Carries its
  own `acceptance_criteria:`, `constraints:`, and `decisions:` objects
  (§Object model), implementation-scoped. **Spike variant:** `spike: true`
  plus ≥1 required `resolves` edge to an open-question fragment. A spike
  carries NO `implements` edges: the `≥1 resolves edge` requirement REPLACES
  the story class's `≥1 implements edge` requirement — a spike answers
  questions, it implements no outcome AC. The required `story:` scalar still
  applies. Spikes are exempt from the evidence model and their build branch's
  diff is fenced to designated non-product paths (VL-016).
- **component** — system source-of-truth documents like this one. No story, no
  ACs, maintained by ordinary MRs, superseded rather than archived.

**Superseded: v0's story-grained feature class (R4-I-9).** Before this
round, `spec (feature)` was itself the unit of work: a single required
`story:` scalar bound 1:1, ACs were a flat implementation-level list, and
there was no separate story spec. That definition is superseded by the
two-class model above; existing v0 feature-spec artifacts remain valid
frozen records under their original shape — history is never rewritten,
the new contract applies forward. VL-014 (disposition completeness) is
retained but scoped to those grandfathered artifacts: it fires only on
specs carrying a `dispositions:` block; new specs use the readiness rules
in §Lint rules (VL-017 in place of dispositions) instead.

The `dispositions:` block, where it still applies (grandfathered
artifacts), is the commit-to-design ritual's durable output (surfaces
spec): every sticky in the frozen `board.json` lands here as `incorporated`
(`where` required — a heading anchor in this spec that must resolve),
`contradicted` (`note` required), or `open-question`. Completeness is
bidirectional: an undispositioned sticky and a disposition naming no board
sticky are both VL-014 errors.

Feature-spec frontmatter additions:

```yaml
class: feature
problem: { text: "borrowers cannot self-serve an update to a submitted application", anchor: "#problem" }
outcome: { text: "a borrower can update their application and see the change reflected", anchor: "#outcome" }
story: okr:LOAN-Q3          # optional; epic/objective ref (R4-I-2)
impacts: [loansvc, notification-svc]
context:
  - adr/0012-outbox-loansvc-events@3e91ab2
  - spec/messaging-gateway@9c41f2e
declares:
  boundaries:
    - { from: loansvc, to: notification-svc, via: events }
acceptance_criteria:
  - { id: ac-1, text: "a borrower can update their application", evidence: [attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "a borrower can see the change reflected", evidence: [behavioral, attestation], anchor: "#ac-2" }
constraints:
  - { id: co-1, text: "...", anchor: "#co-1" }
decisions:
  - { id: dc-1, text: "...", anchor: "#dc-1" }
stubs:
  - { slug: borrower-update-api, acceptance_criteria: [ac-1] }
  - { slug: borrower-update-ui, acceptance_criteria: [ac-1, ac-2] }
supersession:            # required only on a superseding revision (R4-I-4)
  carried: [ac-1]
  amended: [ { id: ac-2, note: "..." } ]
  amended_advisory: []
  removed: []
  added: []
```

Story-spec frontmatter additions:

```yaml
class: story
problem: { text: "the update API has no PUT route for a submitted application", anchor: "#problem" }
outcome: { text: "PUT /applications/:id/update returns 200 with the new state", anchor: "#outcome" }
story: jira:LOAN-1482
links:
  - { type: implements, ref: "spec/loan-update#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "PUT /applications/:id/update returns 200 with the new state", evidence: [static, behavioral], anchor: "#ac-1" }
constraints:
  - { id: co-1, text: "...", anchor: "#co-1" }
decisions:
  - { id: dc-1, text: "...", anchor: "#dc-1" }
```

Spike-story frontmatter additions:

```yaml
class: story
spike: true
story: jira:LOAN-1490
links:
  - { type: resolves, ref: "spec/loan-update#oq-1" }
```

## Link taxonomy

Backlinks are computed by inverting this table at index/dex-build time.

| Type          | Inverse (computed)  | Semantics                                            |
|---------------|---------------------|------------------------------------------------------|
| implements    | implemented-by      | story → feature-AC fragment: this story realizes that outcome AC (R4 concept §1); the feature's AC→story mapping is only ever this edge's computed inverse — the feature is downward-blind |
| resolves      | resolved-by         | spike story → open-question fragment: this spike's deliverable is answering that question |
| supersedes    | superseded-by       | decision/spec replacement chain                      |
| exempts       | exempted-by         | decision → ADR or feature-decision fragment: the target stays valid org-wide; this spec-scoped decision is excused from it, with a required reason |
| verifies      | verified-by         | evidence artifact → AC/spec (also via evidence-for)  |
| derived-from  | source-of           | generated artifact → its inputs                      |
| annotates     | annotated-by        | annotation → target                                  |
| depends-on    | depended-on-by      | reading-order/knowledge dependency                   |
| story         | —                   | spec → tracker item (scheme-prefixed ref): required on the story class (the canonical tracker binding, R4-I-2), optional epic/objective ref on the feature class |
| impacts       | impacted-by         | spec → service                                       |
| challenges    | challenged-by       | conflict → the closed decision or rollup it disputes |

`implements`, `resolves`, `supersedes`, `exempts`, and `depends-on` are the
**closed spec-object edge vocabulary** (R4 concept §1): a decision object's
own `links:` (§Object model) and a story or spike's top-level `links:` may
target an object fragment (§Identity and references) only with one of these
five types — unknown edge types fail closed, same as any other strict-decode
violation. An `implements`/`resolves`/`exempts` edge added or changed on a
story-spec MR is CODEOWNERS-routed to the owners of the spec it targets —
the party who owns the AC or ADR being claimed against, not the party
claiming credit (R4 concept §1, §2).

Proto-links (board yarn) are `{ from, to, label }` with no type — the v0
authoring-time shape, produced before commit-to-design's promotion pass.
**Superseded (R4-I-9):** `verdi board commit` is retired — board editing on
a design branch edits the spec's objects and typed edges directly
(§Object model; R4 concept §5), so this promotion step no longer runs. The
mutable board's `yarn` list (§Record schemas) is retained for the
annotation layer's untyped `relates` threads only.

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

Every `digest` and `integrity` value in this contract is computed over a
**canonical JSON byte form**, mirroring upstream verdi-go's own `canonjson`:
object keys sorted, no HTML escaping, one trailing newline. Two
semantically equal values always serialize identically regardless of map
iteration or struct field order — the property digests depend on;
`encoding/json`'s unordered default is digest-unstable and MUST NOT be used
directly for hashed content.

## Record schemas (working area)

**Annotation** (`data/mutable/annotations/<kind>--<name>.jsonl` for targeted
annotations, append-only; schema `verdi.annotation/v1`). Board-only
annotations (no `target`) have no ref to derive a filename from, so they
get their own stream: `annotations/board--<story-slug>.jsonl`, where
`<story-slug>` is `RefSlug` of the board key (see the board-state entry
below for the board-key namespace this slug is drawn from).

```json
{ "id": "a-01J...", "ts": "2026-07-10T14:02:11Z", "author": "john",
  "target": { "ref": "spec/loansvc-stale-decline@7f3c2a1",
              "selector": { "heading": "ac-2", "quote": "charge API", "line": null } },
  "target_b": { "ref": "...", "selector": { "heading": "...", "quote": "...", "line": null } },
  "board": { "story": "STORY-1482", "x": 262, "y": 132 },
  "type": "comment | question | decision-needed | agent-task | frame | note | pin | relates | review | spike | story",
  "body": "string", "status": "open | resolved | graduated" }
```

`id` is `a-<ULID>`: a sortable, monotonic identifier matching the shape
above. `target` is optional: a free-floating sticky — the normal early
state of a murder board — is a record with `board` and no `target`. At
least one of `target` or `board` MUST be present.

`type: relates` is the annotation layer's untyped scratch thread between two
elements (R4 concept §5): `target_b`, same shape as `target`, is present
only for this type and names the thread's second endpoint. It never enters
the spec document; graduation to a real object edge (`implements`,
`resolves`, `exempts`, `supersedes`, `depends-on`, §Link taxonomy) is an
ordinary spec edit, not an automatic promotion.

`type: note` is the fast lane's explicitly unclassified thought — minted
by quick capture with `board` and no required classification. Choosing the
fast lane is itself the choice (R4-I-36's no-silent-default ruling is
satisfied by the lane, not a form); a note classifies at graduation like
any sticky — or it dies.

`type: frame` is a named annotation-layer region — declared clustering,
carrying the label so proximity alone never has to: `body` is the frame's
name, and its `board` record alone may carry the optional `w`/`h` extent
fields (fail closed on every other type). A frame never enters the spec
document and never constrains layout; it is a label over space, not a
container with semantics.

`type: story` and `type: spike` (round 5.4) are the scoping canvas's
typed proto-stickies — a feature wall's claim that a story (or spike)
will exist: `board` carries the parking spot, `body` the working title.
Their untyped relates-threads to acceptance criteria (story) or open
questions (spike) carry the coverage and resolution attribution — and for
exactly this, a relates endpoint may name a board annotation by id
(`a-<ULID>`) as well as an artifact ref; the endpoint pair is the
meaning, so the edge vocabulary is untouched.
Graduation mints the frontmatter stub (spike stubs carrying `resolves`);
like every sticky, they graduate or they die, and they are legal only on
feature-class walls (fail closed elsewhere).

`type: pin` pins an existing artifact to a board as planning material — the
murder-board move of putting a fact on the wall before any claim is made
about it: `target` names the pinned artifact (a ref; no selector required),
`board` carries its wall position, and `body` optionally says why it is
pinned. A pin never enters the spec document. Its graduation is drawing a
typed edge to the pinned target (an ordinary spec edit, §Link taxonomy) —
or it dies (the record is deleted), taking its untyped `relates` threads
with it.

`type: review` records a review sticky. It never enters the document
either, and its canonical home is a forge MR inline comment, not this
stream — see the comment-token grammar below. Where a local mirror is kept
(the incumbent mutable-zone streams accepted specs retain, R4 concept §5),
it carries in `body` the same `[vd:<object-id>]` token the forge comment
carries.

**Comment-token grammar.** A forge MR inline comment whose body begins with
`[vd:<object-id>]` anchors that comment to the named spec object
(§Object model), independent of which diff line it was left on or how
objects have moved between pushes. The board renders a token-bearing
comment on its object's current card; a comment whose token does not
resolve, or that carries no token, renders in an unanchored inbox tray —
never dropped (R4 concept §5). This is the addressing scheme `type: review`
annotations reference rather than duplicate.

Anchor drift is computed three-valued against the *current* working tree
(never re-resolved against the pinned commit — drift measures change since
the pin): **fresh** — the selector's quote is still found within the
section under its pinned heading; **moved** — the quote is not under its
pinned heading but is found verbatim elsewhere in the current document;
**gone** — the quote is not found anywhere (including the target artifact
no longer resolving at all). Matching is exact, never fuzzy: a near-miss
is `moved` or `gone`, honestly, not silently healed to `fresh`.

**Board state** (`data/mutable/boards/<story>.json`, autosaved; schema
`verdi.board/v1`). **Grandfathered (superseded R4-I-9 — see §Link
taxonomy's superseded note).** This file's `pins`/`yarn` as spec content,
and the frozen board.json below, were produced only by the retired
commit-to-design ritual; new specs never write either. The schema is
retained (existing artifacts must remain decodable), and the mutable board
file's `yarn` list survives solely for the annotation layer's untyped
`relates` threads (§Link taxonomy). Fields: `pins` (pinned refs + x/y),
`stickies` (annotation ids + x/y), `yarn` (proto-links). The `<story>` key
here is an **opaque, verbatim filename stem** — a separate namespace from
the scheme-prefixed `story:` ref feature specs carry, and not required to
parse as one (a board can exist before a story ref is even chosen). The
retired commit-to-design ritual took an explicit `story_ref` parameter for
the new spec's `story:` field, defaulting to the board key itself only when
the board key was already shaped like a scheme-prefixed ref (`scheme:key`)
— no invented bridge between the two namespaces otherwise.

The frozen `board.json` — **grandfathered, produced only by the retired
commit-to-design ritual; new specs never write one, and existing frozen
board.json artifacts remain valid under their own schema** — is this schema
plus a `frozen` stamp and provenance: `digest` is sha256 of the canonical
JSON (§Generated artifacts and digests) of `{pins, stickies, yarn}` only —
never the raw file bytes, which would couple the digest to formatting — and
`inputs` is the mutable board file named as `path@commit` plus every
distinct pinned ref among `pins`.

**Board layout** (`.verdi/specs/<status-dir>/<name>/layout.json`, a sibling
of the spec inside its own directory; schema `verdi.boardlayout/v1`,
R4-I-5):

```json
{ "schema": "verdi.boardlayout/v1",
  "positions": { "ac-1": { "x": 40, "y": 20 }, "dc-1": { "x": 40, "y": 180 },
                 "stub:borrower-update-api": { "x": 990, "y": 40 } } }
```

A `positions` key names a declared object by id, or — round 5.5, the
scoping canvas's stubs made draggable on demand — a declared stub as
`stub:<slug>`. Same verbatim-pass-through, prune, and display-resolution
semantics either way; positions only, never content, as ever.

Positions only, keyed by object ID (§Object model) — never content. It is
committed with the spec, frozen with it at acceptance (VL-018 checks its
keys resolve to real object IDs; it never gates otherwise), and a
superseding revision's directory gets its own `layout.json`, seeded from its
predecessor's. Distinct from the mutable board state above: that file is
authoring-time working state (pins, stickies, yarn) for a design branch
still in flux; `layout.json` is the frozen, per-spec coordinate record that
persists once the spec is accepted, and it never moves stored coordinates
(R4 concept §5).

**Re-affirmation** (`reaffirmations/<story-slug>/<object-id>.md`, nested by
story and the amended feature object per §Identity and references'
compound-name pattern; attestation-shaped — existence is the record, no
`status` field, frozen at commit; R4-I-4, R4 concept §3b):

```yaml
id: reaffirmation/jira-loan-1482--ac-2
kind: reaffirmation
title: "re-affirm ac-2 as amended for jira:LOAN-1482"
schema: verdi.reaffirmation/v1
owners: [loansvc-team]
frozen: { at: 2026-07-14, commit: <sha> }
object: spec/loan-update@<v2-commit>#ac-2
hash: { old: sha256:..., new: sha256:... }
```

CODEOWNERS-routed to the story owner. `hash.old`/`hash.new` are the
`(kind, id, text)` content hash (§Object model) of the amended object
before and after the feature supersession that triggered it, so even a
same-day rubber stamp attests to the specific diff and is audit-countable
later (R4 concept §9 OQ-ii). A story whose edges are flagged `stale`
against a feature supersession (§Kind registry, VL-015) must carry one of
these per amended object it depends on, or supersede itself (rung 3),
before the merge gate proceeds.

## Lint rules

`artifactlint` is dependency-light Go — no frameworks and no services in the
gate path; its only parser is the contract's single vendored YAML decoder —
run locally and as a CI gate. Rules:

| Rule    | Enforces                                                                      |
|---------|-------------------------------------------------------------------------------|
| VL-001  | frontmatter present, decodes strictly against kind schema; the restricted dialect is enforced here (anchors, aliases, custom tags fail) |
| VL-002  | id/path agreement; global ref uniqueness. Status-in-path applies to the feature and story classes only: superseded component specs remain in `specs/active/` |
| VL-003  | all link refs resolve — verdi refs against the committed zone, `svc/...` external refs against discovery, and `evidence-for` bindings in discovered `verdi.bindings.yaml` sidecars (evidence-model spec) against the named spec's ACs; pins name real commits; object-id fragments (`#<object-id>`, §Identity and references) resolve against the target's parsed frontmatter objects (§Object model), and their edge types are the closed five-value enum (§Link taxonomy) — unknown types fail closed |
| VL-004  | status transitions legal per kind; `status: draft` MUST NOT exist on the default branch — enforced when linting the default branch itself or a change targeting it (CI vars, or a local merge-base against the default branch); elsewhere a bare warning, since always-enforcing would break ordinary design branches |
| VL-005  | story spec has exactly one `story:` link with a configured scheme (moved from the feature class, R4-I-2); a feature spec's optional `story:` epic ref, when present, is validated against the same configured schemes |
| VL-006  | every AC declares ≥1 expected evidence kind (activation lint)                 |
| VL-007  | unknown entries directly under `.verdi/` fail (D1)                            |
| VL-008  | generated provenance in committed zone ⇒ on `gated_generated` allowlist OR frozen-stamped |
| VL-009  | frozen artifacts carry valid `frozen` stamp and provenance where generated    |
| VL-010  | frozen artifacts are immutable: any diff touching a frozen file fails, except a pure rename within an active→archive move — the diff base is `merge-base(HEAD, default branch)`; uncommitted edits to a frozen file are errors too |
| VL-011  | attestation/waiver/reaffirmation files live under the story/object they name; feature outcome-attestations (`attestations/<feature-slug>/<ac-id>.md`, name `<feature-slug>--<ac-id>`, §Identity and references) validate the same nested form with `<feature-slug>` = the feature spec's name (not tracker-derived; E2 as amended); waiver has owner + reason, expiry optional |
| VL-012  | `.gitattributes` marks all committed-generated paths with the configured forge's generated attribute (`gitlab-generated` on GitLab, `linguist-generated` on GitHub) |
| VL-013  | nothing under `.verdi/data/` is ever git-tracked (`git add -f` is intent; lint catches it) |
| VL-014  | disposition completeness, bidirectional, **grandfathered** (R4-I-9(a)): fires only on specs carrying a `dispositions:` block (pre-R4 artifacts) — every sticky id in a committed `board.json` appears there as incorporated (with a resolving `where` anchor), contradicted (with `note`), or open-question, and every entry names a real board sticky. New specs use VL-017 instead |
| VL-015  | supersession manifest completeness and fidelity: every object in the predecessor spec (at its `frozen.commit`) is classified exactly once across the superseding revision's `supersession:` block (`carried`/`amended`/`amended_advisory`/`removed`, plus `added`); every `carried` object's `(kind, id, text)` content is byte-identical to its predecessor (§Object model) — fail closed on drift |
| VL-016  | spike path fence: a build branch built from a `spike: true` story touches only paths matched by `verdi.yaml`'s `spike_paths:` allowlist; any other path in the diff fails closed |
| VL-017  | open-question stickies resolved-or-carried: on a design branch targeting the default branch, every open-question annotation (§Record schemas) is either `status: resolved` or explicitly carried as a declared open-question object on the spec — the VL-014 successor for new specs. Scoped by mutable-zone presence: open-question annotations live in the per-checkout mutable zone (`data/mutable/annotations/*.jsonl`), which is never committed. The rule enforces **where the mutable zone is present** — author-local lint and the workbench's review-ready indicator. **Where the mutable zone is absent (CI clone), lint reports the check disclosed-unproven for that spec** — never a silent pass — honoring three-valued honesty (constitution 2); a vacuous green is never emitted. The disclosure is a printed notice, not a verdict failure — a CI run with no other findings exits 0 with the disclosure on the record (adjudicated at W2 wave close) |
| VL-018  | `layout.json` positions: every key in a spec directory's `positions` map (`verdi.boardlayout/v1`, §Record schemas) resolves to a real object ID declared in that spec's frontmatter, or — as `stub:<slug>` — to a declared stub of that spec |
| VL-019  | an obligation's `verifies` edge targets a whole STORY spec (bare `spec/<story>` ref, no fragment) that genuinely declares the `<ac-id>` named in the obligation's own id — a feature-class target, a fragment, an unresolvable ref, or an undeclared AC is refused naming the offending target (obligations are a story-level concern; 03 §The feature fold carried to obligations). Ratified through spec/obligation-artifact (its ac-2/dc-3); recorded in this table per spec/fail-loud dc-4 |

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
`.verdi/attestations/`, `.verdi/waivers/`, `.verdi/reaffirmations/`,
`verdi.yaml`, and this contract to their designated owners; the
human-as-oracle property depends on it.

## Open questions

- OQ-3: grandfathering rules for pre-contract documents during migration
  (lint skips `specs/archive/` for VL-001..006 on import).
