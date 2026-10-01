---
id: obligation/wall-canvas-v2--ac-2--behavioral
kind: obligation
title: "Selecting a card emphasizes its threads and names them"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The Playwright test `wall-canvas › Selecting a card emphasizes its threads and names them` in e2e/tests/89-wall-canvas.spec.ts passes when CI job verify's producer runs that file alone. The test clicks a card and asserts the card and its threaded cards emphasized, the rest receded, and the status pill naming the card and its threads; a card with no threads says so; clicking the wall and the same card clears the selection; and clicking a thread's chip selects the thread."
  falsifier: "The test fails, times out, is interrupted, needs another attempt, is skipped, or does not run (its title path absent from the report), or any assertion the claim describes does not hold."
  scope: "e2e/tests/89-wall-canvas.spec.ts, run once with one worker and no retries by CI job verify's Playwright producer, against the e2e harness's fixture stores in Chromium."
  producer: { kind: test, ref: "playwright:e2e/tests/89-wall-canvas.spec.ts:wall-canvas › Selecting a card emphasizes its threads and names them" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun playwright:e2e/tests/89-wall-canvas.spec.ts:wall-canvas › Selecting a card emphasizes its threads and names them in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/wall-canvas-v2" }
frozen: { at: 2026-09-30, commit: ca6ecec16b315d74bdfac2a0f9f422cf141ebcf2 }
---
# Selecting a card emphasizes its threads and names them

CI job `verify` must record producer `playwright:e2e/tests/89-wall-canvas.spec.ts:wall-canvas › Selecting a card
emphasizes its threads and names them` at the exact candidate commit. The test clicks a card and asserts the card and
its threaded cards emphasized, the rest receded, and the status pill naming the card and its threads; a card with no
threads says so; clicking the wall and the same card clears the selection; and clicking a thread's chip selects the
thread. The test passes when its file runs alone (Playwright producer design §6; BL-98).
