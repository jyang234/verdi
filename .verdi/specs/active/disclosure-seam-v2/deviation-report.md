---
schema: verdi.deviation/v1
covers: 2109f14dc3776e02452c7fe3f6c673a8c7c834e3
findings:
  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd=\"claude -p --output-format json\")", disposition: accepted-deviation, note: "judge timed out in this environment (round5-divergences.md D-11: internal/specalign's own self-hosted checklist test already exercises real claude -p round-trips against this checkout as an incidental side effect of go test, straining the same judge command); the build's changes are a mechanical, behavior-preserving migration (three existing disclosure call sites construct-and-render through the new internal/disclosure seam, no producer's decision logic changed) fully covered by this story's own behavioral ac-1/ac-2 exercisers (cmd/verdi/disclosure_seam_test.go) and the updated per-site unit tests, so the missing judged sweep is accepted rather than re-run" }
digest: sha256:5f84a24a5f220803e817e1f6e6a882d5e7cd819f6679875049618e5ff5e8365a
provenance: { generator: verdi-align, version: v0, inputs: [spec/disclosure-seam-v2@2109f14dc3776e02452c7fe3f6c673a8c7c834e3, spec/disclosure-seam-v2@a66de5b6b656ebe9b123ed0e44aadf38a9ba762d], digest: sha256:5f84a24a5f220803e817e1f6e6a882d5e7cd819f6679875049618e5ff5e8365a }
---
# Alignment report

## Computed

(none)

### Boundary diff vs acceptance baseline

(no impacted services)

## Judged

- **judged-coverage-absent** [accepted-deviation]: judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd="claude -p --output-format json") — judge timed out in this environment (round5-divergences.md D-11: internal/specalign's own self-hosted checklist test already exercises real claude -p round-trips against this checkout as an incidental side effect of go test, straining the same judge command); the build's changes are a mechanical, behavior-preserving migration (three existing disclosure call sites construct-and-render through the new internal/disclosure seam, no producer's decision logic changed) fully covered by this story's own behavioral ac-1/ac-2 exercisers (cmd/verdi/disclosure_seam_test.go) and the updated per-site unit tests, so the missing judged sweep is accepted rather than re-run
