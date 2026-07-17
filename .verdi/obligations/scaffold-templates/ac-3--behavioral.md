---
id: obligation/scaffold-templates--ac-3--behavioral
kind: obligation
title: "End-to-end tests driving the built binary prove verdi model check instantiates and strict-decodes every resolved template, failing closed and naming the offending file on a broken one"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/scaffold-templates" }
frozen: { at: 2026-07-17, commit: 88f8a0a6236e71f403d768f45140718624b8cd2c }
---
# End-to-end tests driving the built binary prove verdi model check instantiates and strict-decodes every resolved template, failing closed and naming the offending file on a broken one

The behavioral evidence must show end-to-end Go tests driving the real
built `verdi` binary (mirroring `cmd/verdi/model_test.go`'s existing
built-binary convention for `verdi model check`, never a package-internal
unit test standing in for it) proving that `model check` now also
instantiates every template the resolved model can reach — the embedded
canonical fallback for every declared class on a store with no
`.verdi/templates/` override, and a store override in its place when one
exists — and strict-decodes the rendered result, over both a store with no
overrides and one with a valid override, exiting 0 alongside the existing
schema/count/digest OK line. It must also show a broken-template case
(malformed template syntax, or rendered output that fails strict decode)
failing `model check` closed with an error naming the specific offending
template file, never a bare "model.yaml invalid" message, and wired into
`make verify`'s `lint-store` step so this exit discipline runs on every
gate pass, not only in this story's own tests. Green in CI's test step.
