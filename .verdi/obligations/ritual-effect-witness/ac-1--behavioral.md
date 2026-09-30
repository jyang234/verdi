---
id: obligation/ritual-effect-witness--ac-1--behavioral
kind: obligation
title: "The harness's sensors and verdicts are right"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "Over synthetic rituals: a ritual that stays within its declaration passes; one that stages outside its declared paths, moves an undeclared ref, adds an undeclared worktree, or carries a foreign entry while declaring scoped fails naming the effect; one that refuses while declaring scoped fails; and an effect the sensors cannot attribute is reported unproven."
  falsifier: "Any listed synthetic ritual gets a different verdict, or an unattributable effect is counted within scope."
  scope: "internal/ritualwitness over fixturegit repositories with a local bare remote."
  producer: { kind: test, ref: "go-test:internal/ritualwitness:TestHarness_SensorsAndVerdicts" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/ritualwitness:TestHarness_SensorsAndVerdicts in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/ritual-effect-witness" }
frozen: { at: 2026-09-30, commit: 5d5dbfc29e44d1e955dd94ca470cc82a895f4409 }
---
# The harness's sensors and verdicts are right

CI job `verify` must record producer `go-test:internal/ritualwitness:TestHarness_SensorsAndVerdicts` at the exact
candidate commit. The harness's own table-driven test, with no network.
