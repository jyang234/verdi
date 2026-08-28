---
id: obligation/sealed-codex-execution--ac-1--behavioral
kind: obligation
title: "The public execution command fails closed around sealed Codex start"
owners: ["platform-team"]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The built binary accepts exactly verdi context execution --request <path|-> [--out <path>], strict-decodes the start-or-resume request, and launches the pinned Codex argument vector only after every declared prerequisite is proven; repository and corpus fixtures remain provenance-wrapped data even when they contain imperative prose, and the canonical result is emitted only after durable execution-result and receipt-event acknowledgment with exit 0 for a completed result, 1 for an authoritative-prerequisite verdict, and 2 for malformed, process, or storage failure."
  falsifier: "The command or flags differ, stdin and path requests are not equivalent, an unknown field or invalid arm reaches launch, the adapter starts after a missing, mismatched, dirty, ambient, or unproven prerequisite, repository or corpus prose becomes an instruction, advisory authority is upgraded, output precedes receipt-event acknowledgment, result bindings are incomplete, or an exit class is wrong."
  scope: "Built-binary context execution start and resume invocations over stdin or path and stdout or --out, fixture Git, fake process, isolated profile, execution-workspace, classified repository data including adversarial imperative prose, policy, capability, recorder, and advisory, authoritative, malformed, process-failure, and storage-failure requests; no network or real Codex process."
  producer: { kind: test, ref: "go-test:cmd/verdi:TestContextExecutionPublicContract_Behavioral" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:cmd/verdi:TestContextExecutionPublicContract_Behavioral in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/sealed-codex-execution" }
frozen: { at: 2026-08-25, commit: 0e8006b8b20270c9792ef6bf1a81ce165cbdcde9 }
---
# The public execution command fails closed around sealed Codex start

CI job `verify` must record producer `go-test:cmd/verdi:TestContextExecutionPublicContract_Behavioral` at the exact candidate commit.

The evidence must prove: The built binary accepts exactly `verdi context execution --request <path|-> [--out <path>]`, strict-decodes the start-or-resume request, and launches the pinned Codex argument vector only after every declared prerequisite is proven; repository and corpus fixtures remain provenance-wrapped data even when they contain imperative prose, and the canonical result is emitted only after durable execution-result and receipt-event acknowledgment with exit 0 for a completed result, 1 for an authoritative-prerequisite verdict, and 2 for malformed, process, or storage failure.

It is falsified when: The command or flags differ, stdin and path requests are not equivalent, an unknown field or invalid arm reaches launch, the adapter starts after a missing, mismatched, dirty, ambient, or unproven prerequisite, repository or corpus prose becomes an instruction, advisory authority is upgraded, output precedes receipt-event acknowledgment, result bindings are incomplete, or an exit class is wrong.

Scope: Built-binary `context execution` start and resume invocations over stdin or path and stdout or `--out`, fixture Git, fake process, isolated profile, execution-workspace, classified repository data including adversarial imperative prose, policy, capability, recorder, and advisory, authoritative, malformed, process-failure, and storage-failure requests; no network or real Codex process.
