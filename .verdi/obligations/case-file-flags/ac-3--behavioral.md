---
id: obligation/case-file-flags--ac-3--behavioral
kind: obligation
title: "Same badge, same drawer, at two viewport sizes"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/case-file-flags" }
frozen: { at: 2026-07-14, commit: c464efb6133e449257c48738ab66ae93a0e071e5 }
---
# Same badge, same drawer, at two viewport sizes

The behavioral evidence must include a Playwright e2e (verdi/e2e/) that
loads the same size-smell-badged wall at two distinct browser viewport
sizes (one shorter than the reference constant, one taller) and asserts
the badge is present in both and its drawer content is identical —
including that no rendered drawer value equals either actual viewport
height, the falsifiable form of "the drawer never cites a client
viewport measurement".
