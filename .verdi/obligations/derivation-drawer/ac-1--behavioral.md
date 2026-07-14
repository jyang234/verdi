---
id: obligation/derivation-drawer--ac-1--behavioral
kind: obligation
title: "Playwright opens and inspects a drawer on both wall surfaces"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/derivation-drawer" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# Playwright opens and inspects a drawer on both wall surfaces

The behavioral evidence must include a Playwright e2e (verdi/e2e/) over a
fixture spec with at least one card badge and one case-file badge that:
opens each badge's drawer by click AND by keyboard activation, asserts the
drawer names the badge's namespaced source rule id, at least one pinned
input with a non-empty revision, and at least one firing record, then
closes it (Esc and close control) and asserts the wall is unchanged. A
test that only asserts the drawer element exists, without reading its
rule/inputs/records content, does not satisfy this obligation.
