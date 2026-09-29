---
id: obligation/index-data--ac-2--static
kind: obligation
title: "Quiet after fourteen days, against an injected clock"
owners: [platform-team]
for_kind: static
quality:
  state: elaborated
  claim: "An entry in the drafts-in-progress group reads quiet exactly when its last change is more than fourteen days before the injected now, and no entry outside that group reads quiet."
  falsifier: "The quiet mark flips at any boundary other than fourteen days, reads the real clock, or marks an entry outside the drafts-in-progress group."
  scope: "Boundary cases at thirteen, fourteen, and fifteen days; a design-branch entry and a default-branch entry in the drafts group; an unreadable date; and an entry in another group."
  producer: { kind: test, ref: "go-test:internal/refindex:TestQuiet_FourteenDays" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/refindex:TestQuiet_FourteenDays in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/index-data" }
frozen: { at: 2026-09-29, commit: be706198be544721484b0d2fed7974477b3c68d7 }
---
# Quiet after fourteen days, against an injected clock

CI job `verify` must record producer `go-test:internal/refindex:TestQuiet_FourteenDays` at the exact candidate commit. A
table-driven test of the quiet rule with an injected clock.
