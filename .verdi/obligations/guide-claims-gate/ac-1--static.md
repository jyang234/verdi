---
id: obligation/guide-claims-gate--ac-1--static
kind: obligation
title: "guide-claims.yaml strict-decodes as atomic capability rows; a bundled multi-capability row shape fails decode"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/guide-claims-gate" }
frozen: { at: 2026-07-20, commit: 1b0976c1039e0aa95e2be207dad8256b6d3b509e }
---
# guide-claims.yaml strict-decodes as atomic capability rows; a bundled multi-capability row shape fails decode

The static evidence must show the manifest's schema declared and strict-
decoded through the single `internal/artifact` seam (`KnownFields(true)`,
unknown enum values failing closed), with `verdi/docs/guide-claims.yaml`
itself committed and transcribing the guide's current Appendix B
atomically: each capability, including 7.2/6.2/8.4/5.3's bundled prose,
decomposed into its own one-capability/one-status/one-witness-set row per
the Task-0 adjudication. It must also show a fixture proving a bundled
multi-capability row shape (a single row attempting to describe more than
one capability, status, or witness set) is rejected at decode with a
named, unknown-shape error — never silently accepted as one merged
claim. The schema declaration and the committed manifest file are
themselves the static artifact; the decode-rejection fixture is the
proof that the schema is actually enforced, not merely documented. Green
in CI's test step, as part of `make verify`.
