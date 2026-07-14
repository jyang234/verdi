---
id: obligation/alignment-section--ac-4--attestation
kind: obligation
title: "An operator affirms a real accepted, genuinely-diverged proposal surfaces and dispositions end to end"
owners: [platform-team]
for_kind: attestation
links:
  - { type: verifies, ref: "spec/alignment-section" }
frozen: { at: 2026-07-14, commit: bd4e93b262179dc1ff3d3c363fd66addb1a875c9 }
---
# An operator affirms a real accepted, genuinely-diverged proposal surfaces and dispositions end to end

The attestation must record a named operator's affirmation, on a real
merged-and-accepted `class: proposal` diagram (not a fixture) whose truth
has genuinely diverged since acceptance (a real commit removed or altered
what the proposal claimed), that running `verdi align` on a build touching
that diagram surfaces the divergence in the `### Diagram alignment`
subsection with a real, correct candidate witness commit, and that the
operator's own `fixed`/`accepted-deviation` disposition on that finding is
honored by a subsequent `verdi align` run without being silently reset.
The attestation must name the diagram ref, the commit that diverged it,
and the disposition applied — a generic "it works" statement without these
specifics does not satisfy this obligation (the same specificity bar
close-verb's own attestation obligations already set in this store).
