---
id: obligation/ritual-traps--ac-2--behavioral
kind: obligation
title: "a freshly minted judged finding id carries exactly one judged- prefix; an archived report fixture with the old doubled form still round-trips"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/ritual-traps" }
frozen: { at: 2026-07-20, commit: 853f1c91ad9493e808eca1422d7991fa7d86692e }
---
# a freshly minted judged finding id carries exactly one judged- prefix; an archived report fixture with the old doubled form still round-trips

The behavioral evidence must show two proofs in `internal/align/
judged_test.go`. First, a test driving the exact regeneration path that
today produces the doubled `judged-judged-...` prefix, asserting the
freshly minted id after the fix carries exactly one `judged-` prefix —
this must be a genuine regression reproduction (the test must be shown
to fail against the pre-fix code), not a synthetic id constructed by
hand. Second, a committed fixture file standing in for an already-
archived report whose findings carry the OLD doubled `judged-judged-...`
form must still decode through `internal/artifact`'s strict-decode seam
and round-trip byte-for-byte unchanged — proving the fix is prospective-
only and never rewrites or breaks an id a real archived disposition
already references. Green in CI's test step, as part of `make verify`.
