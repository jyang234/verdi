---
id: obligation/index-data--ac-2--static
kind: obligation
title: "Quiet after fourteen days, against an injected clock"
owners: [platform-team]
for_kind: static
quality:
  state: elaborated
  claim: "A design-branch entry reads quiet exactly when its last change is more than fourteen days before the injected now, and a default-branch entry never reads quiet."
  falsifier: "The quiet mark flips at any boundary other than fourteen days, reads the real clock, or marks a default-branch entry."
  scope: "Boundary cases at thirteen, fourteen, and fifteen days, an unreadable date, and a default-branch entry."
  producer: { kind: test, ref: "go-test:internal/refindex:TestQuiet_FourteenDays" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/refindex:TestQuiet_FourteenDays in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/index-data" }
frozen: { at: 2026-09-29, commit: 18b943da44eb8d366b56a31ae92c88b2a190ac6d }
---
# Quiet after fourteen days, against an injected clock

CI job `verify` must record producer `go-test:internal/refindex:TestQuiet_FourteenDays` at the exact candidate commit. A
table-driven test of the quiet rule with an injected clock.
