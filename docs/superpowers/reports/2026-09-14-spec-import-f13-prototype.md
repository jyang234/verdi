# F13 spec-import prototype handoff

Status: data-only design/prototype prepared; independent review blocked before
execution; no importer implementation or revised-release acceptance claimed.

## Deliverables

- [Proposed import design](../specs/2026-09-14-existing-spec-import-design.md)
- [Proposed F13 card content](../proposals/2026-09-14-spec-import-f13/board-content-preview.md)
- [Pinned source inventory](../proposals/2026-09-14-spec-import-f13/source-inventory.json)
- [Coverage and transformation map](../proposals/2026-09-14-spec-import-f13/coverage-map.json)

The initial proposal head was `42a7e39efac0bc14dbbd950774f22445e3838cec`.
It is superseded by the mechanical-mapping revision described below.
Main authored the proposal and mapping. Source snapshots reproduce ATC revision
`c346c005d117dfb090e1dedd5a89f45080875a5e`; the user's active adoption checkout
was read only and was not used as a mutable source of truth for these snapshots.
The prototype is not an automatic extraction result or visual implementation.

## Verified locally

A Python check reread each file from pinned Git, recomputed full-file/selection
SHA-256 values, and byte-compared all four retained snapshots: **4/4 match**.
All **668 selected lines** are retained. Primary F13 lines 602–671 are partitioned
exactly once into eight accounting units: **70/70 lines accounted for**.
Semantic completeness still requires review; file retention is not that proof.

`git diff --check 2c919deadd5aceeb23551ef55bb620d2d85b9079 42a7e39efac0bc14dbbd950774f22445e3838cec`
reports one trailing blank line in the primary source snapshot. It is deliberately
preserved to retain the exact selected bytes. The same check excluding only the
`sources/*` snapshots exits 0. No source snapshot was normalized to silence it.
No runtime files were changed and no runtime gate was rerun for this data-only
proposal. The existing selected release and its earlier verification remain
separate from a future importer implementation.

## Scope and remaining decisions

The revised preview carries eight source-span criterion mappings and retained
interface/state-catalog/implementation notes. Problem/outcome remain explicitly
unmapped; no inferred summary or constraint text is presented as imported content. It preserves
full F13 requirements alongside the narrower slices' exclusions. No implemented
slice, checkbox, source command or external approval label becomes gate evidence.

The proposed design handles native drafts and reviewed ordinary Markdown, one
target per explicit file bundle, with create-only import and retained provenance.
Before implementation, settle the exact import/provenance and format-mapping
contracts and explicit amendment of the v0 import exclusion. The importer has
no AI-context bootstrap dependency. Agent-operated mutation still retains its
existing policy checks; ordinary human mechanical import does not invoke AI.
Whether this addition enters the MVP release must also be explicit; both
qualifying journeys would then use the new identified release. Hosted validation
remains deferred. The earlier owner-run session is paused/assisted feedback,
not an independent pass on that future release.

## Independent review status

Prepared one tool-free Claude Code Opus 5 review of the consolidated exact head,
with the proposal, four pinned source snapshots and relevant Verdi governing
specifications. Automatic approval review rejected process creation because this
is a new private payload to Anthropic and the earlier authorization concerned a
different policy packet. **No Claude review executed, no model provenance or
review verdict is claimed, and no workaround/retry was attempted.**

Explicit owner permission is needed to send this packet to Anthropic for the
required one independent review and, after at most one main-authored correction,
one same-reviewer closure. This is the root AGENTS spec-only reviewer exception;
Codex retains authoring/adjudication. Runtime work, when authorized, still uses
`/fable-orchestration` and genuine FABLE 5.1 / Sonnet / Opus 5 assignments.

Prepared packet metadata and the rejection record are retained locally in
`/Users/johnyang/code/verdi-system/.local/verdi-system/development/spec-import-f13-20260914/`.

## Mechanical-mapping revision after owner feedback

The owner challenged whether AI is needed for faithful import. The revised
proposal excludes AI extraction from this importer version. Native specs use
strict decoding; supported external formats use explicit versioned structural
mappings; unknown structure stays attached for manual field assignment.

`mechanical-field-map.json` contains eight explicit criterion source spans with
byte offsets, original line numbers, selected-text digests and one formatting
operation: collapse source whitespace for display. Main recomputed all eight
spans and verified that no word or punctuation changes occur. This is a
mechanically checked mapping artifact, not an implemented generic parser. The
source snapshot and whole-selection coverage checks continue to pass unchanged.

Missing problem/outcome fields are displayed as unmapped. The user can assign
existing text or explicitly use the already-supported statement deferral; the
latter leaves an incomplete draft, not a ready or accepted design. Inference of
missing requirements, completion, relationships and evidence is outside import.

The prepared initial Opus packet is now obsolete and must be regenerated for
the revised exact head before any authorized review. The rejected call was never
executed; no initial review, author correction round or closure has occurred.
Design review by another model is a development verification requirement,
separate from whether importing a document calls a model. The review transfer
remains pending explicit authorization; no retry occurred during this revision.

## Owner clarification: labeled statements are directly importable

The owner identified F13's absent Problem/Outcome labels as a source-structure
finding. The design now explicitly requires mechanical extraction when those
labels exist, including multiline content and exact source mapping. Positive
labeled-input tests accompany F13's missing-label case; the importer must not
mistake F13's structural gap for a general limitation of Markdown import.
See the [F13 structure finding](../proposals/2026-09-14-spec-import-f13/f13-structure-finding.md).
No original ATC source or pinned snapshot was changed.
