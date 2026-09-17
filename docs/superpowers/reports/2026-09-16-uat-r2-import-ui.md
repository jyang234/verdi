# UAT Round 1, wave 2, lane W2-B: import dialog mapping by selection (ac-8)

## Status

Complete and proven. Every gate named in the lane brief ran green on the
head below; the negative and positive Playwright cases were RED before the
script change and GREEN after it. No specification conflict was met.

## Risk tier

Tier 2 (spec/uat-round-1 dc-5; ac-8 closes UAT-006).

## Base..Head

Base `19dca4f5` (wave-1 integration head) .. code head `a38bc6df`, report
`4b740e70`, review fix `2b72a40b`; this report update is the last commit.
Branch `agent/uat-r2-import-ui`, worktree `verdi-wt/uat-r2-import-ui`.

## Commits

- `f7d6f003` Serve selection-mapping affordances on the import page
- `ae607e36` Map a selection in a rendered source to a picked target
- `a38bc6df` Prove selection mapping end to end with multi-byte sources
- (this report, last)

## Files changed

- `internal/workbench/specimportrender.go` (+14/-2): page-local CSS rules
  for `.import-source-view`, `.import-source-text`, `.import-map-controls`,
  `.import-map-note`, `.import-mapping-excerpt`; source-step and Advanced
  hint copy naming the Map selection path.
- `internal/workbench/specimportselection_test.go` (new): render test
  pinning the rules and copy above and the surviving free-text grammar.
- `internal/workbench/assets/specimport.js` (+379/-2): the behavior.
- `e2e/tests/76-import-selection-mapping.spec.ts` (new): two cases.
- `internal/dex/assets/style.css`: untouched — the page's stylesheet lives
  in `specImportCSS` inside the renderer by the established pattern.

## Contract implemented

1. Each source is rendered read-only beneath its row in a `<details>` (open
   unless >256 KiB): a `<pre class="import-source-text">` filled via
   `textContent`, `pre-wrap`, `var(--mono)`, `user-select: text`, scrollable.
   The text is the SELECTED SLICE: `selectedSlice` mirrors the server's
   `selectLineRange` byte-for-byte (offsets are slice-relative). Decoding is
   strict (`TextDecoder` fatal, BOM kept) and display-only; invalid UTF-8 or
   an invalid line range shows a note instead of text.
2. Map selection resolves the live selection against that source's block;
   `byteOffsetsForUnits` walks the byte array rune by rune, counting the
   UTF-16 units per rune (2 for four-byte sequences), to turn code-unit
   offsets into byte offsets; the slice is decoded back and must equal the
   selected text. Visible refusals (`data-refused="true"`): empty, outside
   any source, spanning two sources, inside another source, whitespace-only,
   boundary mismatch.
3. Per-source picker: Statements; Existing objects (last preview's fields
   plus current mapping targets, per kind, numerically sorted); next id per
   kind (one past the highest known — the importer's ordinal numbering),
   labeled "New" only under a current preview (see Review-fix range). The
   Advanced free-text target remains; placeholder "problem, outcome, ac-1,
   co-1, dc-1 or oq-1". A selection mapping replaces the target's earlier
   span/text, keeps its evidence kinds, and the note says so.
4. Each source-backed mapping row shows `Bytes [start,end) of <label>:
   "<first line, ≤140 chars>"` from the page's own bytes, refreshed on
   offset, source or line-range change; an invalid range says so.
5. Unchanged: preview, coverage, apply, confirmation/race handling and the
   request JSON (`mappingWire` untouched; a selection mapping is
   `{target, source_id, start, end, transform:"identity"}`). The picker is
   not a request edit and never invalidates the preview.

## Explicit exclusions

- `internal/specimport/**`, the spec-import contract document, and
  `e2e/tests/72-spec-import.spec.ts` were not edited (72 was only run).
- No JS-level unit test (the repository has no JS test harness; the brief
  forbids adding one); Playwright is the JS proof.
- No `cmd/e2eharness` change: multi-byte sources are supplied inline via
  `setInputFiles({buffer})`, as 72's escaping case does.

## RED command and observed failure

Go (before the renderer change):
`go test -count=1 -run TestSpecImport_PageServesSelectionMappingAffordances ./internal/workbench/`
→ `page-local stylesheet missing selector ".import-source-text"` (x3),
`missing selector ".import-mapping-excerpt"`, `source step copy missing
"Map selection"`, `... "select a passage"`, `advanced hint does not point
the reader back to the selection path`; FAIL.

Playwright (before the script change):
`cd e2e && VERDI_E2E_PORT_BASE=4390 npx playwright test tests/76-import-selection-mapping.spec.ts --workers=1 --trace=off`
→ both cases failed at `toBeVisible()` for `import-source-text-widget-plan-md`
/ `import-source-text-notes-md`: "element(s) not found". 2 failed.

## GREEN commands and results

- `go test -count=1 -run 'TestSpecImport_PageServesSelectionMappingAffordances|...PageExplainsChoicesBeforeSelection|...HomeAndPageDiscoverable' ./internal/workbench/` → ok.
- `node --check internal/workbench/assets/specimport.js` → ok.
- `cd e2e && rm -rf test-results && VERDI_E2E_PORT_BASE=4390 npx playwright test tests/76-import-selection-mapping.spec.ts tests/72-spec-import.spec.ts --workers=1 --trace=off`
  → 20 passed (30.6s): 72-spec-import 18/18, 76-import-selection-mapping 2/2.
- Artifact scan: `find e2e/test-results -type f` → only `.last-run.json`;
  png/jpg/jpeg/webm/mp4/zip/trace count = 0.
- `gofmt -l ./internal/workbench ./internal/dex` → none;
  `go vet ./internal/workbench/... ./internal/dex/...` → clean;
  `golangci-lint run ./internal/workbench/... ./internal/dex/...` → 0 issues.
- `go test -race -count=1 ./internal/workbench/... ./internal/dex/...` → ok, ok.
- `go test ./internal/specalign/... -count=1` → ok (vocab witness passed; no
  `vocab:identity` marker was needed — the new Go literals carry no bare
  lifecycle or class word).
- `go test ./internal/showcasealign/... -count=1` → ok.

## Residual risks

- Next-id rule: one past the highest numbered id among the last preview's
  fields and current mapping targets (the importer's ordinal numbering); a
  coincidence with an automatic id is the contract's explicit override and,
  since the review fix, is disclosed rather than called new.
- Selection preservation relies on the Map selection button taking no focus
  on mousedown; `<select>` interaction proven via Playwright `selectOption`,
  not a real pointer on the dropdown; Chromium-only suite.
- Large sources render fully (collapsed above 256 KiB), no virtualisation;
  the excerpt shows a mapping's first line only (the preview card shows all).

## Review-fix range

Opus review returned REVISE on `4b740e70`; fixed in one commit on top (see
handback for the SHA), no amend/rebase. Important: before any preview or
with a stale one, the picker's next-id option read "New <kind> (ac-1)" and
the note "(new)" although automatic recognition may own ac-1 and the
mapping silently replaces it. Now `fillMapPicker`/`mapSelection` assert
novelty only while `previewCurrent()`; otherwise the label reads
"<kind> ac-1 (next id; no current preview — if recognition owns ac-1 this
mapping replaces it)" and the note "(next id, unverified until preview;
replaces any automatically recognized ac-1)"; pickers refresh in
`invalidate()`. Minor 1: whitespace-only and mid-character (astral
surrogate) refusals added to spec 76. Minor 2: a source's last note is kept
on state and restored by `renderSourceView`. Spec 76 gained a third case
(override disclosed, then shown by the preview; "(new)" after a current
preview; stale again after the edit) and the note-survival check.
GREEN: `go test -race -count=1 ./internal/workbench/...` ok; gofmt none,
vet clean, golangci-lint 0 issues; `go test ./internal/specalign/ -run
TestVocabProseWitness -count=1` ok; `VERDI_E2E_PORT_BASE=4790 npx playwright
test tests/76-import-selection-mapping.spec.ts tests/72-spec-import.spec.ts
--workers=1 --trace=off` → 21 passed (72: 18, 76: 3); artifacts 0.

## Integration prerequisites

- None beyond merging the four commits; no schema, contract, ledger, CLI or
  MCP registry change. `e2e/node_modules` must exist (`cd e2e && npm install`)
  wherever the suite runs — it was absent in this worktree.
- Full `make verify` is the controller's gate; only the focused specs ran here.
