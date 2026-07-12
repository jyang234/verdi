---
schema: verdi.deviation/v1
covers: 769eeeafa77468c3f39c24204bc85ba27c548b0a
findings:
  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd=\"claude -p --output-format json\")", disposition: accepted-deviation, note: "judge timed out in this environment (round5-divergences.md D-11: internal/specalign's own self-hosted checklist test already exercises real claude -p round-trips against this checkout as an incidental side effect of go test, straining the same judge command); the build's changes are a mechanical, behavior-preserving migration (three existing disclosure call sites construct-and-render through the new internal/disclosure seam, no producer's decision logic changed) fully covered by this story's own behavioral ac-1/ac-2 exercisers (cmd/verdi/disclosure_seam_test.go) and the updated per-site unit tests, so the missing judged sweep is accepted rather than re-run" }
digest: sha256:cfb49a0135b5240b31d095765b6a1d735d5b79a39cb45b0b0f35f13c5ac73257
provenance: { generator: verdi-align, version: v0, inputs: [spec/disclosure-seam-v2@769eeeafa77468c3f39c24204bc85ba27c548b0a, spec/disclosure-seam-v2@a66de5b6b656ebe9b123ed0e44aadf38a9ba762d], digest: sha256:cfb49a0135b5240b31d095765b6a1d735d5b79a39cb45b0b0f35f13c5ac73257 }
---
# Alignment report

## Computed

(none)

### Boundary diff vs acceptance baseline

(no impacted services)

## Judged

- **judged-coverage-absent** [accepted-deviation]: judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd="claude -p --output-format json") — judge timed out in this environment (round5-divergences.md D-11: internal/specalign's own self-hosted checklist test already exercises real claude -p round-trips against this checkout as an incidental side effect of go test, straining the same judge command); the build's changes are a mechanical, behavior-preserving migration (three existing disclosure call sites construct-and-render through the new internal/disclosure seam, no producer's decision logic changed) fully covered by this story's own behavioral ac-1/ac-2 exercisers (cmd/verdi/disclosure_seam_test.go) and the updated per-site unit tests, so the missing judged sweep is accepted rather than re-run
