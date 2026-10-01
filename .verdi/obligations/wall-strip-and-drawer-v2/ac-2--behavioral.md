---
id: obligation/wall-strip-and-drawer-v2--ac-2--behavioral
kind: obligation
title: "The strip's chips keep the kept badge and flag properties"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The Playwright test `wall-strip-and-drawer › The strip's chips keep the kept badge and flag properties` in e2e/tests/91-wall-strip-drawer.spec.ts passes when CI job verify's producer runs that file alone. On a story wall and a feature wall in every board mode, the test asserts each badge chip is a button carrying data-badge-source and its derivation record and opens the derivation drawer; no write is refused because a badge is present; ladder flags appear on the story wall and size-smell on a wall with acceptance criteria; the flag names match the dex story-lens badges; and a disclosed-unproven value is drawn with the disclosure class, not the flag class."
  falsifier: "The test fails, times out, is interrupted, needs another attempt, is skipped, or does not run (its title path absent from the report), or any assertion the claim describes does not hold."
  scope: "e2e/tests/91-wall-strip-drawer.spec.ts, run once with one worker and no retries by CI job verify's Playwright producer, against the e2e harness's fixture stores in Chromium."
  producer: { kind: test, ref: "playwright:e2e/tests/91-wall-strip-drawer.spec.ts:wall-strip-and-drawer › The strip's chips keep the kept badge and flag properties" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun playwright:e2e/tests/91-wall-strip-drawer.spec.ts:wall-strip-and-drawer › The strip's chips keep the kept badge and flag properties in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/wall-strip-and-drawer-v2" }
frozen: { at: 2026-09-30, commit: da729113549288efdee9961dcfa089c2d4ac199a }
---
# The strip's chips keep the kept badge and flag properties

CI job `verify` must record producer `playwright:e2e/tests/91-wall-strip-drawer.spec.ts:wall-strip-and-drawer › The
strip's chips keep the kept badge and flag properties` at the exact candidate commit. On a story wall and a feature wall
in every board mode, the test asserts each badge chip is a button carrying data-badge-source and its derivation record
and opens the derivation drawer; no write is refused because a badge is present; ladder flags appear on the story wall
and size-smell on a wall with acceptance criteria; the flag names match the dex story-lens badges; and a
disclosed-unproven value is drawn with the disclosure class, not the flag class. The test passes when its file runs
alone (Playwright producer design §6; BL-98).
