---
id: obligation/sealed-claude-vatc-events--ac-1--behavioral
kind: obligation
title: "Claude fixtures prove complete Codex invariant parity"
owners: ["platform-team"]
for_kind: behavioral
quality:
  state: elaborated
  claim: "Canned Claude Code streams prove the same isolated profile, runway and child workspace, immutable authority, capabilities, expansion, invalidation, interruption, resume, recorder, advisory, result, receipt, and receipt-event behavior required of Codex."
  falsifier: "Any Codex invariant lacks a Claude parity case, a provider limitation silently relaxes authority, the live provider or network is invoked, or equivalent fixtures produce different authority outcomes."
  scope: "Hermetic Claude and Codex parity tables across preparation, start, observable stream normalization, denial, invalidation, stop, resume, advisory completion, authoritative completion, and receipt finalization."
  producer: { kind: test, ref: "go-test:internal/sealedexec/claude:TestClaudeAdapterParityContract_Behavioral" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/sealedexec/claude:TestClaudeAdapterParityContract_Behavioral in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/sealed-claude-vatc-events" }
frozen: { at: 2026-08-25, commit: 0e8006b8b20270c9792ef6bf1a81ce165cbdcde9 }
---
# Claude fixtures prove complete Codex invariant parity

CI job `verify` must record producer `go-test:internal/sealedexec/claude:TestClaudeAdapterParityContract_Behavioral` at the exact candidate commit.

The evidence must prove: Canned Claude Code streams prove the same isolated profile, runway and child workspace, immutable authority, capabilities, expansion, invalidation, interruption, resume, recorder, advisory, result, receipt, and receipt-event behavior required of Codex.

It is falsified when: Any Codex invariant lacks a Claude parity case, a provider limitation silently relaxes authority, the live provider or network is invoked, or equivalent fixtures produce different authority outcomes.

Scope: Hermetic Claude and Codex parity tables across preparation, start, observable stream normalization, denial, invalidation, stop, resume, advisory completion, authoritative completion, and receipt finalization.
