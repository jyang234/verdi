---
id: obligation/creation-form--ac-4--behavioral
kind: obligation
title: "commit-to-design renders through the shared producer: override honored, no-override output byte-identical to the retired builder"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/creation-form" }
frozen: { at: 2026-07-21, commit: cbd7eba9edb7770d0c4a10bd881b831c2feb48ba }
---
# commit-to-design renders through the shared producer: override honored, no-override output byte-identical to the retired builder

The behavioral evidence must show tests in `internal/commitdesign`
pinning both halves of the switch. The parity pin: the existing
`TestScaffoldSpec_BytePin` fixture — its `want` bytes kept byte-for-byte
unchanged from before the switch — passes against the new producer path
with no store override present, proving the embedded commit-to-design
canonical template reproduces the retired `strings.Builder` output
exactly for the inputs the old producer handled (pins present/absent,
dispositions present/absent must both be exercised across the byte-pin
and the package's existing `Run` fixtures, which must keep passing
unmodified). The override leg: a fixture store carrying a
`.verdi/templates/` override for the feature class's declared
`Class.Template` produces a committed spec carrying the override's shape
— the first time this path honors a store override, L-M12's discharge —
and the result still self-validates and passes `CheckClass(feature)`. A
negative pin: an override that renders a non-feature class refuses
before anything is written. Green in CI's test step, as part of
`make verify`.
