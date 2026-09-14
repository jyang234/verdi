# F13 spec-import prototype handoff

Status: data-only design/prototype prepared; independent review blocked before
execution; no importer implementation or revised-release acceptance claimed.

## Deliverables

- [Proposed import design](../specs/2026-09-14-existing-spec-import-design.md)
- [Proposed F13 card content](../proposals/2026-09-14-spec-import-f13/board-content-preview.md)
- [Pinned source inventory](../proposals/2026-09-14-spec-import-f13/source-inventory.json)
- [Coverage and transformation map](../proposals/2026-09-14-spec-import-f13/coverage-map.json)

The exact proposal head is `42a7e39efac0bc14dbbd950774f22445e3838cec`.
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

The preview carries eight proposed criteria, two constraints, inferred
problem/outcome summaries, and retained implementation/source notes. It preserves
full F13 requirements alongside the narrower slices' exclusions. No implemented
slice, checkbox, source command or external approval label becomes gate evidence.

The proposed design handles native drafts and reviewed ordinary Markdown, one
target per explicit file bundle, with create-only import and retained provenance.
Before implementation, settle the exact import/provenance contracts, governed
pre-draft AI-context integration and explicit amendment of the v0 import exclusion.
No missing policy can be bypassed by confirming a model-generated preview.
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
