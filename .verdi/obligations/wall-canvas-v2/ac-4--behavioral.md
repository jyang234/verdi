---
id: obligation/wall-canvas-v2--ac-4--behavioral
kind: obligation
title: "Drag-to-thread offers only the legal edge types"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The Playwright test `wall-canvas › Drag-to-thread offers only the legal edge types` in e2e/tests/89-wall-canvas.spec.ts passes when CI job verify's producer runs that file alone. For each source and target kind pair on the fixture, the test drags a pin and asserts the picker lists exactly the legal types with their consequence labels, a gate-bearing type asks for confirmation, and choosing writes the edge through the typed-edge path and selects it; a drag from a stub pin opens no picker; and a sticky's attribution yarn and graduation drop still write what they write today."
  falsifier: "The test fails, times out, is interrupted, needs another attempt, is skipped, or does not run (its title path absent from the report), or any assertion the claim describes does not hold."
  scope: "e2e/tests/89-wall-canvas.spec.ts, run once with one worker and no retries by CI job verify's Playwright producer, against the e2e harness's fixture stores in Chromium."
  producer: { kind: test, ref: "playwright:e2e/tests/89-wall-canvas.spec.ts:wall-canvas › Drag-to-thread offers only the legal edge types" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun playwright:e2e/tests/89-wall-canvas.spec.ts:wall-canvas › Drag-to-thread offers only the legal edge types in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/wall-canvas-v2" }
frozen: { at: 2026-09-30, commit: ca6ecec16b315d74bdfac2a0f9f422cf141ebcf2 }
---
# Drag-to-thread offers only the legal edge types

CI job `verify` must record producer `playwright:e2e/tests/89-wall-canvas.spec.ts:wall-canvas › Drag-to-thread offers
only the legal edge types` at the exact candidate commit. For each source and target kind pair on the fixture, the test
drags a pin and asserts the picker lists exactly the legal types with their consequence labels, a gate-bearing type asks
for confirmation, and choosing writes the edge through the typed-edge path and selects it; a drag from a stub pin opens
no picker; and a sticky's attribution yarn and graduation drop still write what they write today. The test passes when
its file runs alone (Playwright producer design §6; BL-98).
