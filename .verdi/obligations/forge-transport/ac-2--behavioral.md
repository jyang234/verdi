---
id: obligation/forge-transport--ac-2--behavioral
kind: obligation
title: "The decisive item past page one is seen"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/forge-transport" }
frozen: { at: 2026-07-13, commit: b52051b1058e17bb26f1f54c79bdaa8d2dbec71d }
---
# The decisive item past page one is seen

The behavioral evidence must show multi-page fakes where the decisive item
sits beyond page one: the unresolved review thread at position >100
reported unresolved (the gate-pass witness), the page-2 open MR seen by the
supersession scan, a repeated next-signal failing loud, malformed/absent
page headers stopping cleanly after one request.
