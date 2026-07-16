---
id: obligation/home-status-glance--ac-3--behavioral
kind: obligation
title: "An empty glance bucket still renders its heading and an explicit empty-state notice"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/home-status-glance" }
frozen: { at: 2026-07-16, commit: d11cd50bf4840109ef8834b16e97a1920805c178 }
---
# An empty glance bucket still renders its heading and an explicit empty-state notice

The behavioral evidence must show `e2e/tests/43-home-status-glance.spec.ts`
driving a fixture store where at least one glance bucket has no matching
entries (e.g. a store with no `accepted-pending-build` spec at all) and
asserting: the `glance-group-in-flight` section is still present and
visible, its count reads "(0)", and it carries a visible empty-state
notice (mirroring the existing `.empty` "None." convention) rather than
being absent from the DOM entirely.
