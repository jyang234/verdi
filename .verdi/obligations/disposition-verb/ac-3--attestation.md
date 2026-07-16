---
id: obligation/disposition-verb--ac-3--attestation
kind: obligation
title: "an operator records a real disposition, freezes it, and confirms verbatim survival end to end"
owners: [platform-team]
for_kind: attestation
links:
  - { type: verifies, ref: "spec/disposition-verb" }
frozen: { at: 2026-07-16, commit: 2911a957eda4f96d5ccfda5fc7ed1dfa388231d6 }
---
# an operator records a real disposition, freezes it, and confirms verbatim survival end to end

The attestation must record a named operator's affirmation that, on a
real (not fixture) build's living `deviation-report.md`, they ran `verdi
disposition` to record a `fixed` or `accepted-deviation` decision with a
real rationale on a named finding; that the report's digest and integrity
were unchanged and independently reverified afterward; and that running
`verdi align --freeze` subsequently produced a frozen report carrying
that exact decision and rationale, byte-for-byte, for that finding. The
attestation must name the spec ref, the finding id, the decision
recorded, and the commit at which the freeze was taken — a generic "it
works" statement without these specifics does not satisfy this
obligation (the same specificity bar spec/alignment-section's own ac-4
attestation obligation already set in this store).
