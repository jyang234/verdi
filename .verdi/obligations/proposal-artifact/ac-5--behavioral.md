---
id: obligation/proposal-artifact--ac-5--behavioral
kind: obligation
title: "A lint fixture test proves VL-021 catches a dangling derived_from.ref and a malformed digest, and admits a clean proposal"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/proposal-artifact" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# A lint fixture test proves VL-021 catches a dangling derived_from.ref and a malformed digest, and admits a clean proposal

The behavioral evidence must show `make lint-store` (or a direct
`internal/lint` test) run over three fixture proposals: one whose
`derived_from.ref` names a diagram that does not exist in the corpus
(VL-021 must fire, naming the dangling ref); one whose `derived_from.digest`
is not `sha256:<64-hex>` (VL-021 must fire, naming the malformed value); and
one whose `derived_from` correctly names a real diagram with a well-formed
digest (VL-021 must be silent). A test that only exercises the clean case
does not satisfy this obligation.
