---
id: obligation/localop-story--ac-1--behavioral
kind: obligation
title: "Local-operator story AC-1 (fixture obligation)"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The local-operator claim resolves when the store's Git identity is bound."
  falsifier: "The local-operator claim does not resolve when the store's Git identity is bound."
  scope: "Fixture-only; not a real test."
  producer: { kind: test, ref: "go-test:localoperator:TestLocalOperatorResolves" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun TestLocalOperatorResolves in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/localop-story" }
frozen: { at: 2026-09-05, commit: deadbeefdeadbeefdeadbeefdeadbeefdeadbeef }
---
# Local-operator story AC-1 (fixture obligation)

Fixture-only scaffold obligation: exists solely so `verdi build start`'s
obligation-quality precondition is satisfied and the run reaches the
policy-conflict gate under test. It is not a real evidence claim about
production code.
