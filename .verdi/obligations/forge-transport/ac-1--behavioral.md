---
id: obligation/forge-transport--ac-1--behavioral
kind: obligation
title: "Contract suites prove the seam changed nothing"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/forge-transport" }
frozen: { at: 2026-07-13, commit: b52051b1058e17bb26f1f54c79bdaa8d2dbec71d }
---
# Contract suites prove the seam changed nothing

The behavioral evidence must show the forge and provider contract suites
(forgetest, providertest) passing UNCHANGED — zero suite-file edits —
against both fakes and the seam-riding adapters: the byte-equivalence proof
that collapsing three transports into one altered no observable behavior.
