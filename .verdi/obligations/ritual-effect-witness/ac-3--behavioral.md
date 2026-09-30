---
id: obligation/ritual-effect-witness--ac-3--behavioral
kind: obligation
title: "No ritual carries a foreign staged entry, close refuses, and only Commit and push is carried"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "In the fully seeded state, design start, design start --supersedes, verdi board commit, the workbench's commit-to-design, accept diagram, and constitution propose create commits that omit the pre-staged foreign entry; close exits 2 with no ref, index, or working-tree change; and the registry's only carried declaration is the board's Commit and push."
  falsifier: "A listed ritual's commit contains the foreign entry, close mutates anything or exits other than 2, or another declaration is carried."
  scope: "The listed rituals driven through their real entry points over fixturegit repositories."
  producer: { kind: test, ref: "go-test:cmd/verdi:TestUAT036_RitualsNeverCarryForeignEntries" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:cmd/verdi:TestUAT036_RitualsNeverCarryForeignEntries in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/ritual-effect-witness" }
frozen: { at: 2026-09-30, commit: 4a5da769127ba8da923e667a43dfe243eb5f85da }
---
# No ritual carries a foreign staged entry, close refuses, and only Commit and push is carried

CI job `verify` must record producer `go-test:cmd/verdi:TestUAT036_RitualsNeverCarryForeignEntries` at the exact
candidate commit. Each ritual is one subtest; close's case keeps the existing refusal tests as they are.
