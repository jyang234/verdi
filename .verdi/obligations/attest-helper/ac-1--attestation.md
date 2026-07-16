---
id: obligation/attest-helper--ac-1--attestation
kind: obligation
title: "The operator affirms the scaffold writes structure only — never a claim, never claim-shaped prose, and leaves the frozen stamp for the human to own"
owners: [platform-team]
for_kind: attestation
links:
  - { type: verifies, ref: "spec/attest-helper" }
frozen: { at: 2026-07-16, commit: a42d24e2ed017bcf7fa839417755b98b90bb0f34 }
---
# The operator affirms the scaffold writes structure only — never a claim, never claim-shaped prose, and leaves the frozen stamp for the human to own

The attestation must affirm, after reading the merged diff: the scaffold
writer copies `owners` verbatim from the resolved story spec, derives an
identifier-shaped `title` from identifiers already on hand, writes the bare
`verifies` edge and the `frozen` stamp, and emits a body that is exactly the
`evidence.UnauthoredAttestationMarker` literal plus fixed instructional
prose — and that NO code path in the diff templates, defaults, or otherwise
generates a single claim-shaped sentence (parent dc-2: verdi writes
structure, the human writes every word of the claim). The operator must also
affirm that the pre-filled `frozen.commit` is left as a convenience for the
human to correct to the tree they actually verified against — never
presented as a machine-vouched fact (dc-2, ADJ-30).
