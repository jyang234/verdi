---
id: obligation/verification-extractor--ac-3--behavioral
kind: obligation
title: "A table-driven comparison test covers all three classifications, a rename as two facts, and witness resolution/absence"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/verification-extractor" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# A table-driven comparison test covers all three classifications, a rename as two facts, and witness resolution/absence

The behavioral evidence must show a table-driven test that: (1) classifies
a proposal element present in truth as `exists`; (2) classifies a proposal
element absent from truth (no base) as `proposed-new`; (3) classifies a
base-inherited element truth has since dropped as `kept-but-gone`; (4)
feeds a "rename" scenario (a proposal that dropped node A and added node B
relative to its base, where a human would call it a rename) and asserts
the output is the two independent facts `kept-but-gone(A)` +
`proposed-new(B)`, never a single combined fact. It must also show a
fixturegit-backed test proving witness-commit resolution: a repository
with a scripted history where a known commit removed a known identity
string, asserting the comparison names that exact commit sha as witness;
and a companion test where no commit in the fixture history ever touched
the identity string, asserting the witness is disclosed absent rather than
guessed.
