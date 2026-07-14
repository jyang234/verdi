---
id: attestation/jira-verdi-2--ac-2
kind: attestation
title: "AC-2 attested: the closed story's rollup publishes to the configured tracker and reads back through it"
owners: [platform-team]
links:
  - { type: verifies, ref: spec/close-verb }
frozen: { at: 2026-07-13, commit: 9fce5b8cced19879330daa1009fd29cf628a5db2 }
---
# AC-2 outcome attestation

Operator attests (round 6, 2026-07-13): `verdi close`'s publish step reached
the configured tracker for real and the rollup read back through it. For
`spec/remote-and-ci` the close printed `rollup published to jira:VERDI-1
(eligible=true)`, and for `spec/runtime-evidence` `rollup published to
jira:VERDI-3 (eligible=true)` — the round-6 hermetic fake provider, the
configured tracker (dc-2: real Jira is a config change, not a code change). The
published rollup is readable back through the provider's own `PublishedField`
(asserted green in `close_test.go` / `close_runtime_test.go`), each carrying the
story's evidenced criteria. The publish step ran to completion, not a stub — ac-2
satisfied.
