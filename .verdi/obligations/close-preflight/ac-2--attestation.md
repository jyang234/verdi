---
id: obligation/close-preflight--ac-2--attestation
kind: obligation
title: "The operator affirms no write path is reachable under --preflight, under any fixture"
owners: [platform-team]
for_kind: attestation
links:
  - { type: verifies, ref: "spec/close-preflight" }
frozen: { at: 2026-07-16, commit: 20b0525430727bbeb168bb1a0cb5d0593f40a70d }
---
# The operator affirms no write path is reachable under --preflight, under any fixture

The attestation must affirm, after reading the merged diff: no code path
reachable when `--preflight` is set ever calls `os.WriteFile`, any `gitx`
mutation (`CheckoutNewBranch`, `AddAll`, `CreateCommit`, `UpdateRef`), or a
provider `PublishRollup` call — checked by reading the dispatch from
`cmdClose`/`runClose` down to the point `--preflight` returns, for every
fixture class in ac-1's test, including the ready one where a real close
would proceed to mutate.
