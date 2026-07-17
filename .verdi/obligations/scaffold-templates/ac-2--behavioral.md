---
id: obligation/scaffold-templates--ac-2--behavioral
kind: obligation
title: "A test proves a store's template override scaffolds an added section and a custom: field that survive strict decode and canonical re-emit, while an anchor/alias/tag inside custom: still fails closed"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/scaffold-templates" }
frozen: { at: 2026-07-17, commit: 88f8a0a6236e71f403d768f45140718624b8cd2c }
---
# A test proves a store's template override scaffolds an added section and a custom: field that survive strict decode and canonical re-emit, while an anchor/alias/tag inside custom: still fails closed

The behavioral evidence must show a Go test that drops a
`.verdi/templates/story.md` override — adding a new body section and a
`custom:` frontmatter field carrying a real value — into a fixture store,
then drives both reachable scaffold call sites (`internal/designscaffold`'s
render path directly, and `cmd/verdi/design.go`'s `design start` over the
fixture) proving the resulting spec's body carries the added section and
its `custom:` field decodes with the given value in place of the embedded
canonical `story.md`. It must also prove the round trip: the scaffolded
spec's `custom:` content, put through `artifact.DecodeSpec` and the
canonical re-emit path, comes back unchanged — the same
strict-decode-then-re-emit property every other frontmatter field already
holds today. Finally, it must show one committed violation fixture
(mirroring `internal/model/testdata`'s one-fixture-per-rule convention)
whose `custom:` block carries a YAML anchor, alias, or tag, and prove
decode fails closed on it — operating-model dc-2's dialect wall, extended
verbatim to this namespace, with no carve-out. Green in CI's test step.
