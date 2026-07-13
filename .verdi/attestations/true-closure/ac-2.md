---
id: attestation/true-closure--ac-2
kind: attestation
title: "AC-2 attested: the closure's rollup is published to the configured tracker and is readable there"
owners: [platform-team]
links:
  - { type: verifies, ref: spec/true-closure }
frozen: { at: 2026-07-13, commit: 6185f58a6d34ca38059c317576b1da4c5c87e3fe }
---
# AC-2 outcome attestation

Operator attests (round 6, 2026-07-13): each archived story's rollup was
published to the configured tracker and reads back through it. The four closes
printed `rollup published to jira:VERDI-1/2/3/4 (eligible=true)` respectively,
publishing to the round-6 hermetic fake provider — the configured tracker
(true-closure dc-2: real Jira is a config change, not a code change). The
published rollup is readable back through the provider's own `PublishedField`
(asserted green in `close_test.go` / `close_runtime_test.go`), each carrying the
story's evidenced criteria. The publish step ran to completion for real, not a
stub.
