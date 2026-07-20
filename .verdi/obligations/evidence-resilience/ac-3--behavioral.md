---
id: obligation/evidence-resilience--ac-3--behavioral
kind: obligation
title: "VL-009 tightens from is-a-real-commit to reachable-from-HEAD; a dangling-but-local frozen.commit reds (X-11b), a legitimate one is unaffected"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/evidence-resilience" }
frozen: { at: 2026-07-20, commit: 8cd7e52f3d9c69014d6a020a55bb4284e67eef55 }
---
# VL-009 tightens from is-a-real-commit to reachable-from-HEAD; a dangling-but-local frozen.commit reds (X-11b), a legitimate one is unaffected

The behavioral evidence must show `internal/lint/vl009_test.go` gaining
X-11b's exact fixture: a `frozen.commit` naming an object that genuinely
exists in the local object database (so a naive "is a real commit" check
passes it) but that no branch or ref anywhere reaches — constructed by
committing on a throwaway branch, then deleting that branch without ever
merging it, leaving the commit dangling but present. The test must
assert this fixture reds under the tightened `VL-009` check. A second,
unmodified case must show an ordinary spec whose `frozen.commit` is
reachable through real history continues to pass exactly as before —
proving the tightening is additive (closes the false green) and not a
narrowing that would regress legitimate frozen stamps. Green in CI's
test step, as part of `make verify`.
