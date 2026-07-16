---
id: obligation/close-preflight--ac-3--attestation
kind: obligation
title: "The operator affirms the agreement pairs are exercised end to end, not asserted independently"
owners: [platform-team]
for_kind: attestation
links:
  - { type: verifies, ref: "spec/close-preflight" }
frozen: { at: 2026-07-16, commit: 20b0525430727bbeb168bb1a0cb5d0593f40a70d }
---
# The operator affirms the agreement pairs are exercised end to end, not asserted independently

The attestation must affirm, after reading the merged diff and the test
file(s): every defect-class agreement pair in ac-3's test(s) genuinely
constructs one fixture and drives BOTH the preflight call and the real
close call against it inside the same test function, in that order, on the
identical on-disk state — not two separate tests each hand-asserting its
own expected string that happens to match today but is free to drift
tomorrow — and that the ready-fixture pair's second half is a real,
unmodified `verdi close` run reaching an actual archive move, not a
stubbed or mocked success.
