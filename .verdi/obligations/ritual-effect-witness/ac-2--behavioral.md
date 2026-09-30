---
id: obligation/ritual-effect-witness--ac-2--behavioral
kind: obligation
title: "Every declared ritual stays within its declaration on every publication path"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "Each ritual in internal/writescope's registry, run through its real entry point on each publication path in both seeded states, has every observed effect within its declaration, and completes in the clean-index state when declared to."
  falsifier: "Any ritual has an effect outside its declaration, fails to complete when declared to, or a path is skipped without an unproven report."
  scope: "Every registry declaration, driven as the built binary, the workbench server, or the MCP server over fixturegit repositories."
  producer: { kind: test, ref: "go-test:cmd/verdi:TestRitualEffects_EveryDeclaredRitual" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:cmd/verdi:TestRitualEffects_EveryDeclaredRitual in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/ritual-effect-witness" }
frozen: { at: 2026-09-30, commit: 5d5dbfc29e44d1e955dd94ca470cc82a895f4409 }
---
# Every declared ritual stays within its declaration on every publication path

CI job `verify` must record producer `go-test:cmd/verdi:TestRitualEffects_EveryDeclaredRitual` at the exact candidate
commit. One subtest per ritual and publication path; the test fails when the registry holds a ritual it has no case for.
