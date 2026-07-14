---
id: obligation/judged-sweep--ac-2--behavioral
kind: obligation
title: "A fake-judge test produces a decoded ConflictFinding, and a judge-absent test degrades to the synthetic finding"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/judged-sweep" }
frozen: { at: 2026-07-14, commit: 1d78c2776983c7c08ae4a065727a828b2fd28825 }
---
# A fake-judge test produces a decoded ConflictFinding, and a judge-absent test degrades to the synthetic finding

The behavioral evidence must show a test using the SAME fake `JudgeRunner`
seam `decision_judge_test.go` already establishes, feeding a canned judge
response with one finding, and asserting `RunDiagramSweep` returns exactly
one `artifact.ConflictFinding` with the expected id/text/target. A second
test must configure no judge command (or a failing one) and assert the
result degrades to a synthetic absence finding, mirroring
`decision_judge.go`'s `decisionAbsentResult` pattern, rather than erroring
out or silently returning zero findings.
