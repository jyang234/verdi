---
id: obligation/attest-helper--ac-2--behavioral
kind: obligation
title: "Go tests prove exit 1 and a byte-for-byte-unchanged working tree for every AC-2 refusal case"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/attest-helper" }
frozen: { at: 2026-07-16, commit: a42d24e2ed017bcf7fa839417755b98b90bb0f34 }
---
# Go tests prove exit 1 and a byte-for-byte-unchanged working tree for every AC-2 refusal case

The behavioral evidence must show `cmd/verdi/attest_test.go`'s
`TestRunAttest_RefusesUnknownStoryRef`, `TestRunAttest_RefusesWrongClass`,
`TestRunAttest_RefusesUndeclaredAC`, and `TestRunAttest_RefusesAlreadyExists`
driving the verb's core over a fixturegit-backed store, each asserting exit
1 (the verdict, never exit 0 and never exit 2) and that the working tree is
byte-for-byte unchanged after the refusal (co-2: a refused invocation writes
nothing). The already-exists case must prove the race-safe `O_CREATE|O_EXCL`
create idiom (I-12): a file that appears at the fold path is caught by the
OS and never silently overwritten. No case may exec a subprocess or touch
the network (co-1).
