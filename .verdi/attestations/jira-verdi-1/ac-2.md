---
id: attestation/jira-verdi-1--ac-2
kind: attestation
title: "AC-2 attested: verdi-evidence CI assembles + uploads the source:ci derived tree; make verify is the gate"
owners: [platform-team]
links:
  - { type: verifies, ref: spec/remote-and-ci }
frozen: { at: 2026-07-13, commit: 79d9e4ce6b3dc978809f3582aa1074c5e485aa20 }
---
# AC-2 outcome attestation

Operator attests (round 6 real-remote proof, 2026-07-13): main's `verify.yml`
runs the CI-provenance producer after `make verify` passes in the same job,
uploading the `data/derived/` tree with `provenance.source: ci` records, and
`make verify` is the CI gate (trust parity). Verified against the real run at
`79d9e4c`: `verdi sync` fetched the `source: ci` evidence bundle that run
produced (6 files, landing at the reader key), confirming the workflow
assembles and uploads the authoritative tree as ac-2 requires. No local
record was load-bearing.
