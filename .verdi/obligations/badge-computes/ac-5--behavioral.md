---
id: obligation/badge-computes--ac-5--behavioral
kind: obligation
title: "Badges render in every mode and block nothing"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/badge-computes" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# Badges render in every mode and block nothing

The behavioral evidence must include a Playwright e2e (verdi/e2e/) showing
badge chips on cards and stamps on the case file, and prove the render in
authoring, review, and read-only modes alike. On a badged authoring wall
it must exercise at least one write path (add a sticky, or draw and
commit yarn) and prove it succeeds unchanged — a badge is disclosure,
never refusal (co-2): no write handler consults badge state.
