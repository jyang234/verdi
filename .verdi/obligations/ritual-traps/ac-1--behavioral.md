---
id: obligation/ritual-traps--ac-1--behavioral
kind: obligation
title: "ResolveAnchor slugifies the frontmatter anchor: value symmetrically with heading text — a mixed-case anchor resolves"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/ritual-traps" }
frozen: { at: 2026-07-20, commit: 853f1c91ad9493e808eca1422d7991fa7d86692e }
---
# ResolveAnchor slugifies the frontmatter anchor: value symmetrically with heading text — a mixed-case anchor resolves

The behavioral evidence must show a table-driven test in
`internal/artifact/object_test.go` (or its sibling test file) pinning
X-1's exact witness: a document whose frontmatter carries `anchor: AC-1`
(mixed case) and whose body carries a `## AC-1` heading must resolve
successfully through `ResolveAnchor` after the fix — asserted as a
negative pin that fails against the pre-fix code (the case must
genuinely have failed before, proving the test exercises the real
defect, not a vacuous case). A second case with an already-lowercase
`anchor: ac-1` against `## Ac 1` must continue to resolve exactly as
before, proving the fix only adds resolution power and narrows nothing.
A third case with a heading that still does not match after slugifying
both sides must continue to fail to resolve, proving the fix is not
overly permissive. Green in CI's test step, as part of `make verify`.
