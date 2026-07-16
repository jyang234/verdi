---
id: obligation/attest-helper--ac-4--behavioral
kind: obligation
title: "A Go test writes a scaffold, reads it back from the fold's own path, and DecodeAttestation succeeds byte-for-byte before any claim is authored"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/attest-helper" }
frozen: { at: 2026-07-16, commit: a42d24e2ed017bcf7fa839417755b98b90bb0f34 }
---
# A Go test writes a scaffold, reads it back from the fold's own path, and DecodeAttestation succeeds byte-for-byte before any claim is authored

The behavioral evidence must show `cmd/verdi/attest_test.go`'s
`TestRunAttest_ScaffoldRoundTrips` driving the verb's core over a
fixturegit-backed store: it writes a scaffold, reads the file back from disk
at the exact path the fold reads for that (story, AC), and asserts
`internal/artifact.DecodeAttestation` succeeds against the read-back bytes —
while the unauthored marker is still present, before any claim is authored.
The evidence must also show the verb's own pre-write self-check (mirroring
`design start`/stub-instantiate): a scaffold that fails to self-validate is
refused with an internal-error operational exit (2) rather than ever leaving
a malformed attestation on disk (CLAUDE.md: never fake success). No network,
no subprocess exec (co-1).
