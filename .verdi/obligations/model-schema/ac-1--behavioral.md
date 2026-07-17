---
id: obligation/model-schema--ac-1--behavioral
kind: obligation
title: "DecodeModel enforces every kernel rule table-driven, one violation fixture per rule, unknown scheme/kind failing closed naming the catalog"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/model-schema" }
frozen: { at: 2026-07-17, commit: ca7906782cabdd9fdfe535fdd7591d3a2e8b63dd }
---
# DecodeModel enforces every kernel rule table-driven, one violation fixture per rule, unknown scheme/kind failing closed naming the catalog

The behavioral evidence must show a table-driven Go test in
`internal/model` proving `DecodeModel` (strict-decode via the shared
`internal/artifact` seam, then the kernel validation pass) enforces every
kernel rule the Outcome section lists — obligations list required per
transition (a present-but-empty list distinct from an absent key: decode
tells `nil` from `[]`), terminal states drawn from `states`, every state
reachable, every transition's `from`/`to` naming a declared state, every
class's `parent` naming a declared class, every class carrying a non-empty
`template`, `count` legal only on `countersign`, and `hook` legal only with
a non-empty `Hook`. Each rule must be proven, not merely asserted: one
committed violation fixture per rule under `internal/model/testdata/` that
trips exactly that rule and no other, alongside the canonical fixture that
decodes clean. It must also show that an obligation `scheme` or `kind`
outside the closed catalog (`author-vouch`, `countersign`, `gate-pass`,
`fold-green`, `hook`, `stubs-reconciled`) fails closed with an error that
names the legal catalog itself, never a bare "invalid value" — green in
CI's test step.
