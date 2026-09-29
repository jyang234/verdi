---
id: obligation/wall-strip-and-drawer--ac-1--behavioral
kind: obligation
title: "The one-line case-file strip, edited in place"
owners: [platform-team]
for_kind: behavioral
quality:
  state: unresolved-design-debt
links:
  - { type: verifies, ref: "spec/wall-strip-and-drawer" }
frozen: { at: 2026-09-29, commit: 49220b472016f65dabd052f5edc38138483cfbd2 }
---
# The one-line case-file strip, edited in place

The behavioral evidence is the Playwright file `e2e/tests/91-wall-strip-drawer.spec.ts`. The test asserts the strip's
problem and outcome lines, edits each in place with Enter and asserts one set-problem or set-outcome operation, cancels
with Escape with nothing written, opens the full case file, and asserts the badges, flags, and disclosures as chips in
the strip. No producer grammar exists yet for a Playwright test (SI-228 parses go-test refs only), so this obligation is
declared unresolved design debt (SI-288) and names its evidence here.
