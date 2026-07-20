---
id: obligation/ritual-traps--ac-3--behavioral
kind: obligation
title: "VL-003 gains a root-discovery path so the module-root verdi.bindings.yaml becomes a checked target for the first time (P2-3(b))"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/ritual-traps" }
frozen: { at: 2026-07-20, commit: 853f1c91ad9493e808eca1422d7991fa7d86692e }
---
# VL-003 gains a root-discovery path so the module-root verdi.bindings.yaml becomes a checked target for the first time (P2-3(b))

The behavioral evidence must show a red-to-green demonstration in
`internal/lint/vl003_test.go`, not merely an assertion. First, a fixture
reproducing TODAY's shape — a repository root carrying a
`verdi.bindings.yaml` with a deliberately-wrong bare AC id and no
`.flowmap.yaml` at the module root — must be shown to pass lint silently
before this story's fix (proving `checkBindings` genuinely does not see
the root file today, grounding the P2-3(b) finding rather than assuming
it). After the fix, the identical fixture must red, naming the offending
entry. A second case must show a root bindings file with entries that
are all correct continues to pass cleanly after the fix — the discovery
path must not itself introduce false positives on a clean file. This
obligation is a prerequisite the ac-4 obligation depends on: ac-4's own
fragment-qualified cross-check test is only meaningful once this leg
makes the root file visible to `checkBindings` at all. Green in CI's
test step, as part of `make verify`.
