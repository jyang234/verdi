---
id: obligation/model-schema--ac-2--behavioral
kind: obligation
title: "A parity test proves the embedded canonical.yaml agrees with the code's own state enums and ritual verbs, failing on either side drifting"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/model-schema" }
frozen: { at: 2026-07-17, commit: ca7906782cabdd9fdfe535fdd7591d3a2e8b63dd }
---
# A parity test proves the embedded canonical.yaml agrees with the code's own state enums and ritual verbs, failing on either side drifting

The behavioral evidence must show a Go parity test proving the embedded
`internal/model/canonical.yaml` decodes and is equivalent to the code it
claims to describe — not asserting it, proving it: its state set checked
equal to `internal/artifact/status.go`'s own status enums and its
transition/ritual-verb set checked equal to `cmd/verdi/dispatch.go`'s own
dispatch table, compared through exported helpers on both sides rather than
reflection on either side's private maps. The test must be constructed so
that drift on either side fails it — a status or verb added to the Go code
with no matching entry in the YAML, or an entry in the YAML with no matching
Go code — so the embedded default can never silently diverge from the
hard-coded model it exists to describe. Green in CI's test step.
