---
schema: verdi.deviation/v1
covers: 3a36af20e48bd42610d5cfd06544ca070a091caf
findings:
  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd=\"claude -p --output-format json\")", disposition: accepted-deviation, note: "Judge timed out at 2m (D6-21). Reviewer adjudicated manually: closure gate 3/3 on source:ci evidence; no decision conflicts (ADR corpus empty). Recorded adjudications: (1) ac-5's fold-load collapse landed four-of-five — featurematrix's per-story variant interpolates the story id mid-message and was left in place, disclosed rather than force-fit (dc-4's bit-for-bit rule outranked the count). (2) The two disclosed behavior additions landed as specced (dc-1 fsync, dc-3 reaffirmation indexing); healing the index required artifactview's reaffirmation arm — the same integration class as fail-loud's obligation arm, dex having never received the kind from a real store. (3) A fifth writeFileAtomic consumer (boardio/delete.go) was found at build time and converted in-package. Judged coverage accepted as absent for this closure." }
digest: sha256:2df17d46d5fe73e6c82ba1c726f42d92e13cf02de48a100f73feacc9bd1c132f
frozen: { at: 2026-07-13, commit: 3a36af20e48bd42610d5cfd06544ca070a091caf }
provenance: { generator: verdi-align, version: v0, inputs: [spec/shared-homes@3a36af20e48bd42610d5cfd06544ca070a091caf, spec/shared-homes@2b4a8ef2d66b10fc1d3af201da1d4e8919ea3b5d], digest: sha256:2df17d46d5fe73e6c82ba1c726f42d92e13cf02de48a100f73feacc9bd1c132f }
---
# Alignment report

## Computed

(none)

### Boundary diff vs acceptance baseline

(no impacted services)

## Judged

- **judged-coverage-absent** [accepted-deviation]: judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd="claude -p --output-format json") — Judge timed out at 2m (D6-21). Reviewer adjudicated manually: closure gate 3/3 on source:ci evidence; no decision conflicts (ADR corpus empty). Recorded adjudications: (1) ac-5's fold-load collapse landed four-of-five — featurematrix's per-story variant interpolates the story id mid-message and was left in place, disclosed rather than force-fit (dc-4's bit-for-bit rule outranked the count). (2) The two disclosed behavior additions landed as specced (dc-1 fsync, dc-3 reaffirmation indexing); healing the index required artifactview's reaffirmation arm — the same integration class as fail-loud's obligation arm, dex having never received the kind from a real store. (3) A fifth writeFileAtomic consumer (boardio/delete.go) was found at build time and converted in-package. Judged coverage accepted as absent for this closure.
