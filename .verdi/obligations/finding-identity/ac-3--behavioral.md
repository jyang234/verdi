---
id: obligation/finding-identity--ac-3--behavioral
kind: obligation
title: "a non-reproduced dispositioned finding lands in not-resurfaced: and survives a judge-re-roll replay with the spec-stale count unchanged"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/finding-identity" }
frozen: { at: 2026-07-20, commit: fb6fb43180469f29545ca99f4d649930222b91a0 }
---
# a non-reproduced dispositioned finding lands in not-resurfaced: and survives a judge-re-roll replay with the spec-stale count unchanged

The behavioral evidence must show a test where a finding is
dispositioned in report N, then the canned judge is reconfigured to NOT
emit that slug at all in report N+1's regeneration — asserting the
finding now appears under a `not-resurfaced:` section (not silently
dropped, and not misreported as resolved). It must show the exact X-18
laundering replay: compute the spec-stale accepted-deviation count before
the re-roll, regenerate with the same non-reproducing judge, and assert
the count is byte-identical afterward — a judge simply failing to
re-emit a finding must never uncount a standing accepted deviation. It
must also show the section's two documented consumers working correctly:
a `not-resurfaced` finding that DOES resurface on a later regeneration
still pre-fills as a candidate (not as a brand-new, context-free
finding), and the deviations counterweight reads `not-resurfaced:`
findings identically to `findings:` ones for counting purposes. Green in
CI's test step, as part of `make verify`.
