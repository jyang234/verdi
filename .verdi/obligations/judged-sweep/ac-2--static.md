---
id: obligation/judged-sweep--ac-2--static
kind: obligation
title: "The sweep's prompt builder and finding decode reuse judge.go's exported plumbing, no second exec path"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/judged-sweep" }
frozen: { at: 2026-07-14, commit: 1d78c2776983c7c08ae4a065727a828b2fd28825 }
---
# The sweep's prompt builder and finding decode reuse judge.go's exported plumbing, no second exec path

The static evidence must show `internal/align/diagram_judge.go`'s
`RunDiagramSweep` (or equivalently named entry point) calls
`execJudgeEnvelope` (the same function `decision_judge.go`'s
`RunDecisionSweep` calls) rather than a new exec wrapper, and that its
inner-result decode produces `[]artifact.ConflictFinding` values using the
existing `ConflictFinding` struct and `ConflictDisposition` enum with no
new Go type declared for a finding or a disposition. The evidence must
also show the prompt-building function includes, verbatim or by
equivalent structured content, the ADR corpus and the target spec's (or
corpus-wide, per this story's own scoping decision) declared
constraints/decisions — not merely the diagram's mermaid text alone.
