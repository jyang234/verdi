---
id: obligation/judged-sweep--ac-3--static
kind: obligation
title: "DiagramSweepFrontmatter's integrity/judge_integrity fields and Validate mirror DecisionConflictFrontmatter exactly"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/judged-sweep" }
frozen: { at: 2026-07-14, commit: 1d78c2776983c7c08ae4a065727a828b2fd28825 }
---
# DiagramSweepFrontmatter's integrity/judge_integrity fields and Validate mirror DecisionConflictFrontmatter exactly

The static evidence must show `internal/artifact/diagramsweep.go` declares
`DiagramSweepFrontmatter` with `Integrity string` and
`JudgeIntegrity *JudgeIntegrity` fields (reusing the existing
`JudgeIntegrity` type, not a new one), and a `Validate` method that
requires `Integrity` to be `sha256:<64 hex>` when present and requires
`JudgeIntegrity` to imply a non-empty `Integrity` — the same one-directional
rule `DecisionConflictFrontmatter.Validate` already enforces. The evidence
must show `RunDiagramSweep` calls `computeIntegrity` (the existing
function in `judge.go`) rather than a new hash formula.
