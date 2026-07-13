---
schema: verdi.deviation/v1
covers: b9b45d9100aef43044458149e8cbee0f871f6d71
findings:
  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd=\"claude -p --output-format json\")", disposition: accepted-deviation, note: "Judge timed out at 2m again — the fourth consecutive real build (D6-21). Reviewer performed the alignment review and undeclared-conflict sweep manually and in depth: no decision conflicts with another decision or an ADR (ADR corpus empty). ac-1 (accept.go): the flip was factored into a shared flipPredecessorToSuperseded helper used by both the existing story path and the new feature path; all D-12 guards preserved (accepted-pending-build-only per dc-2 — closed NOT flipped; idempotent; self-validating decode + frozen-stamp; single-line status-only edit); supersedesTargetsFeature fails closed on fragment refs; cascade/blast-radius untouched. ac-2: matrix gained a story-rung status line + a feature-rung superseded-implementing-story marker (was silently dropped); the reviewer additionally added the feature's OWN status line to printFeatureMatrix (dc-3 under-specified it) so a superseded feature is legible when matrix'd directly; dex badge proven latent-working at both rungs (no fix needed); board gained a superseded head badge reusing the shared .badge-superseded vocabulary; superseded-by backlink proven; Playwright e2e added. Disclosed judgment calls (bracket-tag marker format; board single-status scope; closed->superseded deferral) all reviewed and accepted as smallest-reversible. make verify green (127 e2e). Judged coverage accepted as absent." }
digest: sha256:bf0f516c41e21cac0579a711f2ce68da78dc7fb5be97d227a7941aa5fe196a43
provenance: { generator: verdi-align, version: v0, inputs: [spec/feature-supersession-state@b9b45d9100aef43044458149e8cbee0f871f6d71, spec/feature-supersession-state@dab21cfcca85a497b80a1bc8be9ba7cdde856476], digest: sha256:bf0f516c41e21cac0579a711f2ce68da78dc7fb5be97d227a7941aa5fe196a43 }
---
# Alignment report

## Computed

(none)

### Boundary diff vs acceptance baseline

(no impacted services)

## Judged

- **judged-coverage-absent** [UNDISPOSITIONED]: judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd="claude -p --output-format json")
