# UAT Round 1, wave 2, lane W2-B: import dialog mapping by selection (ac-8)

## Status

Complete and proven. Every gate named in the lane brief ran green on the
head below; the negative and positive Playwright cases were RED before the
script change and GREEN after it. No specification conflict was met.

## Risk tier

Tier 2 (spec/uat-round-1 dc-5; ac-8 closes UAT-006).

## Base..Head

Base `19dca4f5` (wave-1 integration head) .. code head `a38bc6df`; this
report is committed on top as the last commit (its SHA is in the handback).
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
- `internal/dex/assets/style.css`: untouched. The import page's stylesheet
  lives in `specImportCSS` inside the renderer by the established pattern
  ("the page's small stylesheet lives here with the component"), so the
  new rules went there and no shared-stylesheet rule was needed.

## Contract implemented

1. Each selected source is rendered read-only beneath its row in a
   `<details>` (open unless the file exceeds 256 KiB) holding a `<pre
   class="import-source-text">` filled via `textContent` (no source byte
   becomes markup), `white-space: pre-wrap`, `var(--mono)`,
   `user-select: text`, scrollable at 24rem. The text shown is the source's
   SELECTED SLICE: `selectedSlice` mirrors the server's `selectLineRange`
   byte-for-byte (both lines zero = whole file; inclusive 1-based physical
   LF lines, unterminated last line counted, CRLF kept), because every
   mapping offset is slice-relative. Decoding is strict (`TextDecoder`
   fatal, BOM kept as text) and for display only; invalid UTF-8 or an
   invalid line range shows a note instead of text.
2. Map selection: the live selection is resolved against that source's
   block. Code-unit offsets within the block are turned into byte offsets by
   `byteOffsetsForUnits`, which walks the byte array rune by rune counting
   the UTF-16 units each rune occupies (2 for a four-byte sequence, else
   1); the resulting slice is decoded back and must equal the selected text,
   otherwise the mapping is refused. Refused with a visible message
   (`data-refused="true"` on the per-source status line): empty selection,
   selection outside any source, selection spanning two sources, selection
   inside another source, whitespace-only selection, boundary mismatch.
3. Target picker per source: Statements (problem, outcome); Existing
   objects (ids from the last preview's fields plus current mapping
   targets, per kind, numerically sorted); New object for each of the four
   kinds with the id it will take shown in the label. New id = one past the
   highest numbered id the page knows for that kind (the importer's own
   1-based ordinal numbering; see Residual risks). The free-text target
   input under Advanced remains, placeholder now
   "problem, outcome, ac-1, co-1, dc-1 or oq-1". A mapping made by
   selection replaces the target's earlier span/text but keeps its evidence
   kinds; the note says so.
4. Every source-backed mapping row in the Advanced list carries an excerpt:
   `Bytes [start,end) of <label>: "<first line, ≤140 chars>"`, computed from
   the page's own bytes and refreshed when the offsets, the source or the
   source's line range change; an invalid or boundary-cutting range says
   so instead.
5. Unchanged: preview, coverage table, apply, confirmation/race handling,
   and the request JSON (`mappingWire` untouched; a selection mapping is
   `{target, source_id, start, end, transform:"identity"}`). Changing the
   picker is not a request edit and does not invalidate the preview.

## Explicit exclusions

- `internal/specimport/**`, the spec-import contract document, and
  `e2e/tests/72-spec-import.spec.ts` were not edited (72 was only run).
- No JS-level unit test: the repository has no JS test harness (no
  vitest/jest/node:test configuration anywhere outside `e2e/node_modules`),
  and the brief forbids adding one; Playwright is the JS proof.
- No `cmd/e2eharness` change: the multi-byte source is supplied inline via
  `setInputFiles({buffer})`, the same technique 72's escaping case uses.
- No live "selected N bytes" readout before mapping; the post-mapping note
  and the list excerpt carry the verification.

## RED command and observed failure

Go (before the renderer change):
`go test -count=1 -run TestSpecImport_PageServesSelectionMappingAffordances ./internal/workbench/`
→ `page-local stylesheet missing selector ".import-source-text"` (x3),
`missing selector ".import-mapping-excerpt"`, `source step copy missing
"Map selection"`, `... "select a passage"`, `advanced hint does not point
the reader back to the selection path`; FAIL.

Playwright (before the script change):
`cd e2e && VERDI_E2E_PORT_BASE=4390 npx playwright test tests/76-import-selection-mapping.spec.ts --workers=1 --trace=off`
→ both cases failed at `expect(locator).toBeVisible()` for
`getByTestId('import-source-text-widget-plan-md')` /
`('import-source-text-notes-md')`: "element(s) not found". 2 failed.

An intermediate run after the script landed failed 8/20 (both 76 cases and
six 72 cases) because the state source object omitted `bytes`, so
`renderSources` threw; fixed in `ae607e36` before commit.

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
- No `make verify`, no push/rebase/squash/amend, per the brief.

## Residual risks

- Next-id rule: the dialog had no explicit rule before this lane; the
  implemented one (one past the highest numbered id among the last
  preview's fields and current mapping targets) follows the importer's
  ordinal numbering. If a stale preview's automatic ids later renumber, a
  "new" id can coincide with an automatic one; the contract makes that an
  explicit override, not an error, and the next preview shows it. Worth a
  one-line note in the ledger if the controller wants it pinned.
- Selection preservation relies on the Map selection button taking no focus
  on mousedown; native `<select>` interaction in Chromium leaves the page
  selection intact (proven via Playwright's `selectOption`, not a real
  pointer on the dropdown). Firefox/Safari untested (suite is Chromium-only).
- Large sources render fully (collapsed above 256 KiB); no virtualisation.
  A 2 MiB file is within Chromium's comfort but selection over it is manual.
- The excerpt shows the first line only; a multi-line selection is verified
  by its first line plus the byte range, then fully by the preview card.

## Integration prerequisites

- None beyond merging the four commits; no schema, contract, ledger, CLI or
  MCP registry change. `e2e/node_modules` must exist (`cd e2e && npm install`)
  wherever the suite runs — it was absent in this worktree.
- The full `make verify` (including the whole e2e suite) is the
  controller's integration gate; only the focused specs ran here.
