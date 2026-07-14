---
id: attestation/code-health--ac-3
kind: attestation
title: "AC-3 attested: one home per shared behavior, proven equivalent"
owners: [platform-team]
links:
  - { type: verifies, ref: spec/code-health }
frozen: { at: 2026-07-14, commit: 49b779af64f9584f55cd3f0940e6c38fda544ed8 }
---
# AC-3 outcome attestation

Operator attests (round 6, 2026-07-14): the atomic write lives once in
internal/atomicfile (fsync closing the uniform durability gap), the
digest tail once in canonjson.Digest (golden pinned, ten copies
collapsed), the YAML double-quote once at the artifact seam (15-case
byte table), and the path-classification table once in
artifact.ClassifyPath — with index gaining the reaffirmation case its
copy silently lost, witnessed red from the index side before the heal.
The small pairs collapsed with scaffold bytes pinned; the one disclosed
partial (featurematrix) is dispositioned in the archived report. Proven
by spec/shared-homes' archived closure (jira:VERDI-QH-3).
