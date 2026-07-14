---
schema: verdi.deviation/v1
covers: e731d476f3c0e1354fdd076a2deb30341fa0daaf
findings:
  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd=\"claude -p --output-format json\")", disposition: accepted-deviation, note: "Judge timed out at 2m again — the fourth consecutive real build (D6-21). Reviewer performed the alignment review and undeclared-conflict sweep manually and in depth: no decision conflicts with another decision or an ADR (ADR corpus empty). ac-1 (accept.go): the flip was factored into a shared flipPredecessorToSuperseded helper used by both the existing story path and the new feature path; all D-12 guards preserved (accepted-pending-build-only per dc-2 — closed NOT flipped; idempotent; self-validating decode + frozen-stamp; single-line status-only edit); supersedesTargetsFeature fails closed on fragment refs; cascade/blast-radius untouched. ac-2: matrix gained a story-rung status line + a feature-rung superseded-implementing-story marker (was silently dropped); the reviewer additionally added the feature's OWN status line to printFeatureMatrix (dc-3 under-specified it) so a superseded feature is legible when matrix'd directly; dex badge proven latent-working at both rungs (no fix needed); board gained a superseded head badge reusing the shared .badge-superseded vocabulary; superseded-by backlink proven; Playwright e2e added. Disclosed judgment calls (bracket-tag marker format; board single-status scope; closed-\u003esuperseded deferral) all reviewed and accepted as smallest-reversible. make verify green (127 e2e). Judged coverage accepted as absent." }
digest: sha256:0f6f025f8a3a1adab3517aa09e1163c292b1f38d4e39342098ce9bfd58455099
frozen: { at: 2026-07-13, commit: e731d476f3c0e1354fdd076a2deb30341fa0daaf }
provenance: { generator: verdi-align, version: v0, inputs: [spec/feature-supersession-state@e731d476f3c0e1354fdd076a2deb30341fa0daaf, spec/feature-supersession-state@dab21cfcca85a497b80a1bc8be9ba7cdde856476], digest: sha256:0f6f025f8a3a1adab3517aa09e1163c292b1f38d4e39342098ce9bfd58455099 }
---
# Alignment report

## Computed

(none)

### Boundary diff vs acceptance baseline

(no impacted services)

## Judged

- **judged-coverage-absent** [accepted-deviation]: judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd="claude -p --output-format json") — Judge timed out at 2m again — the fourth consecutive real build (D6-21). Reviewer performed the alignment review and undeclared-conflict sweep manually and in depth: no decision conflicts with another decision or an ADR (ADR corpus empty). ac-1 (accept.go): the flip was factored into a shared flipPredecessorToSuperseded helper used by both the existing story path and the new feature path; all D-12 guards preserved (accepted-pending-build-only per dc-2 — closed NOT flipped; idempotent; self-validating decode + frozen-stamp; single-line status-only edit); supersedesTargetsFeature fails closed on fragment refs; cascade/blast-radius untouched. ac-2: matrix gained a story-rung status line + a feature-rung superseded-implementing-story marker (was silently dropped); the reviewer additionally added the feature's OWN status line to printFeatureMatrix (dc-3 under-specified it) so a superseded feature is legible when matrix'd directly; dex badge proven latent-working at both rungs (no fix needed); board gained a superseded head badge reusing the shared .badge-superseded vocabulary; superseded-by backlink proven; Playwright e2e added. Disclosed judgment calls (bracket-tag marker format; board single-status scope; closed->superseded deferral) all reviewed and accepted as smallest-reversible. make verify green (127 e2e). Judged coverage accepted as absent.
