---
id: obligation/model-schema--ac-3--behavioral
kind: obligation
title: "Tests driving the built binary prove verdi model check's 0/1/2 exit discipline, including absent model.yaml and the pinned frontier text"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/model-schema" }
frozen: { at: 2026-07-17, commit: ca7906782cabdd9fdfe535fdd7591d3a2e8b63dd }
---
# Tests driving the built binary prove verdi model check's 0/1/2 exit discipline, including absent model.yaml and the pinned frontier text

The behavioral evidence must show end-to-end Go tests driving the real
built `verdi` binary (mirroring `close_test.go`'s own style, not a
package-internal unit test standing in for it) proving `verdi model check`
keeps the same three-valued exit discipline every other verb does: exit 0
with an OK line naming the schema (`verdi.model/v1`), the class/transition
counts, and the resolved model's digest on valid input — including the
absent-`model.yaml` case, which resolves to the embedded canonical, and a
valid hand-written `model.yaml` (vocabulary/template changes only) over
that manifest's own counts and digest; exit 1 with the pinned frontier
error text printed verbatim, never a paraphrase, on a structurally deviant
manifest; and exit 2 on operational trouble (a missing store, an
unreadable or undecodable manifest). Green in CI's test step.
