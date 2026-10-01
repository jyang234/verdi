---
id: obligation/wall-canvas-v2--ac-1--behavioral
kind: obligation
title: "Cards at the new footprint with their receipts, and yarn in two layers"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The Playwright test `wall-canvas › Cards at the new footprint with their receipts, and yarn in two layers` in e2e/tests/89-wall-canvas.spec.ts passes when CI job verify's producer runs that file alone. Over a fixture wall carrying every card kind and every receipt, the test asserts each card's footprint and no rotation; the obligation rows, evidence slots, attestation chips, and coverage chips with their exact existing texts; a stub card's slug as its first line; the readiness mark on a card named by a Focus next concern; and the base yarn layer under the cards with the selection's threads drawn in the layer above them."
  falsifier: "The test fails, times out, is interrupted, needs another attempt, is skipped, or does not run (its title path absent from the report), or any assertion the claim describes does not hold."
  scope: "e2e/tests/89-wall-canvas.spec.ts, run once with one worker and no retries by CI job verify's Playwright producer, against the e2e harness's fixture stores in Chromium."
  producer: { kind: test, ref: "playwright:e2e/tests/89-wall-canvas.spec.ts:wall-canvas › Cards at the new footprint with their receipts, and yarn in two layers" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun playwright:e2e/tests/89-wall-canvas.spec.ts:wall-canvas › Cards at the new footprint with their receipts, and yarn in two layers in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/wall-canvas-v2" }
frozen: { at: 2026-09-30, commit: ab64413fd48345dc5ee87d71f8151a0d1288869d }
---
# Cards at the new footprint with their receipts, and yarn in two layers

CI job `verify` must record producer `playwright:e2e/tests/89-wall-canvas.spec.ts:wall-canvas › Cards at the new
footprint with their receipts, and yarn in two layers` at the exact candidate commit. Over a fixture wall carrying every
card kind and every receipt, the test asserts each card's footprint and no rotation; the obligation rows, evidence
slots, attestation chips, and coverage chips with their exact existing texts; a stub card's slug as its first line; the
readiness mark on a card named by a Focus next concern; and the base yarn layer under the cards with the selection's
threads drawn in the layer above them. The test passes when its file runs alone (Playwright producer design §6; BL-98).
