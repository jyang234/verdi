---
id: obligation/wall-strip-and-drawer--ac-2--behavioral
kind: obligation
title: "The strip's chips keep the kept badge and flag properties"
owners: [platform-team]
for_kind: behavioral
quality:
  state: unresolved-design-debt
links:
  - { type: verifies, ref: "spec/wall-strip-and-drawer" }
frozen: { at: 2026-09-29, commit: fe196661a03bda803eaa0863c6514279c102fd7c }
---
# The strip's chips keep the kept badge and flag properties

The behavioral evidence is the Playwright file `e2e/tests/91-wall-strip-drawer.spec.ts`. On a story wall and a feature
wall in every board mode, the test asserts each badge chip is a button carrying data-badge-source and its derivation
record and opens the derivation drawer; no write is refused because a badge is present; ladder flags appear on the story
wall and size-smell on a wall with acceptance criteria; the flag names match the dex story-lens badges; and a
disclosed-unproven value is drawn with the disclosure class, not the flag class. No producer grammar exists yet for a
Playwright test (SI-228 parses go-test refs only), so this obligation is declared unresolved design debt (SI-288) and
names its evidence here.
