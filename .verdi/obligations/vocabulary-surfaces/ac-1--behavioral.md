---
id: obligation/vocabulary-surfaces--ac-1--behavioral
kind: obligation
title: "CLI verdict and status output resolves display names through DisplayState/DisplayVerb over a vocab-rename fixture, with the entire pre-existing golden/substring suite proving byte-identical output when no model.yaml is present"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/vocabulary-surfaces" }
frozen: { at: 2026-07-17, commit: 6fb386f1c7d53f9318519b7710144c9adcb4e33d }
---
# CLI verdict and status output resolves display names through DisplayState/DisplayVerb over a vocab-rename fixture, with the entire pre-existing golden/substring suite proving byte-identical output when no model.yaml is present

The behavioral evidence must show Go tests driving the built `verdi`
binary (mirroring `cmd/verdi`'s existing built-binary, exact-substring
convention — `feature_test.go`'s `contains(stderr.String(), ...)` style,
never a package-internal unit test standing in for it) proving that CLI
verdicts naming a state or a verb resolve through `model.DisplayState`/
`model.DisplayVerb` before printing, rather than the bare id: over
`internal/model/testdata/vocab-rename.yaml` (model-schema's own fixture,
renaming `accept` to "Sign off" and `accepted-pending-build` to "Ready
to build," reused rather than duplicated), `build start`'s
status-mismatch refusal, `accept`/`supersede`'s flipped-predecessor
confirmation, and `close`'s own verdict lines must each print the
renamed label in place of today's literal. It must also show that the
entire pre-existing set of exact-substring and golden CLI-output
assertions already committed across `cmd/verdi`'s test suite keeps
passing completely unmodified over a store carrying no `model.yaml` at
all — the parity floor is proven by those tests requiring no change,
not by a new assertion of sameness written alongside them. Green in
CI's test step.
