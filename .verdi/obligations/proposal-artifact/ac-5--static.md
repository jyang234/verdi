---
id: obligation/proposal-artifact--ac-5--static
kind: obligation
title: "VL-021 is declared, and id/path plus class/status agreement are shown already covered"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/proposal-artifact" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# VL-021 is declared, and id/path plus class/status agreement are shown already covered

The static evidence must show `internal/lint/vl021.go` declaring a rule
with `ID() string { return "VL-021" }` that, for every decoded `class:
proposal` diagram document, resolves `derived_from.ref` against the
snapshot's known refs (refusing, naming the ref, when it does not resolve
to any diagram artifact) and checks `derived_from.digest` against the
existing `sha256:[0-9a-f]{64}` pattern (refusing, naming the value, when
malformed). The evidence must also point to `internal/lint/vl002.go`'s
`singleFileKindDir["diagram"]` entry and to whichever baseline rule
surfaces a `DecodeErr` as a finding, demonstrating — not merely asserting —
that id/path and class/status agreement need no new rule.
