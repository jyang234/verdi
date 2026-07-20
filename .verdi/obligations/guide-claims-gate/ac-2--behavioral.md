---
id: obligation/guide-claims-gate--ac-2--behavioral
kind: obligation
title: "every EXISTS/PARTIAL row's witness is bound three ways — name-in-corpus, // guide-claim: anchor, and PASS-coupling — each independently red-tested"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/guide-claims-gate" }
frozen: { at: 2026-07-20, commit: 1b0976c1039e0aa95e2be207dad8256b6d3b509e }
---
# every EXISTS/PARTIAL row's witness is bound three ways — name-in-corpus, // guide-claim: anchor, and PASS-coupling — each independently red-tested

The behavioral evidence must show `verdi/internal/specalign/
guideclaims_test.go` (following `vocabprose_test.go`'s witness
conventions) with three independently red-tested cases, each isolating
exactly one of the three bindings so a reader can tell which one failed.
Case one: an `EXISTS` row naming a witness whose name does not exist
anywhere in the corpus reds, naming both the row and the missing
witness. Case two: a row naming a witness that DOES exist in the corpus
but is not marked with a `// guide-claim: <row-id>` anchor at its own
declaration reds — constructed so the witness is otherwise real and
passing, isolating the missing-anchor case specifically (the ADJ-50
lying-gate class this obligation's parent AC names: name existence alone
must not be sufficient). Case three: a row naming a witness that exists
and is correctly anchored but is skipped or gated behind an unexercised
build tag in `make verify` reds via the `require-pass.sh` mechanism —
constructed so the anchor and corpus-name checks both pass, isolating
the PASS-coupling case specifically. A fourth, positive case with all
three bindings genuinely satisfied must pass cleanly. Green in CI's test
step, as part of `make verify`.
