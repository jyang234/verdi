---
id: obligation/case-file-flags--ac-2--behavioral
kind: obligation
title: "The size-smell badge raises on the estimate, discloses its proxy"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/case-file-flags" }
frozen: { at: 2026-07-14, commit: c464efb6133e449257c48738ab66ae93a0e071e5 }
---
# The size-smell badge raises on the estimate, discloses its proxy

The behavioral evidence must render two fixtures around the threshold: a
spec whose AC count puts the dc-1 estimate at or under the reference
constant shows no size-smell badge; one whose count puts the estimate
over it shows the badge on the case file, in observation register, with a
drawer disclosing the constant names and values, the AC count, and the
computed estimate. Dragging AC cards on the badged wall must not change
the badge (positions are not an operand), and every write path must still
succeed with the badge present.
