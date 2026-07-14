---
schema: verdi.deviation/v1
covers: 3a36af20e48bd42610d5cfd06544ca070a091caf
findings:
  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd=\"claude -p --output-format json\")", disposition: accepted-deviation, note: "Judge timed out at 2m (D6-21). Reviewer adjudicated manually: closure gate 3/3 on source:ci evidence; no decision conflicts (ADR corpus empty). Recorded adjudications: (1) dc-4's premise was FALSE against the built system — no fetched DerivedTree ever carried graph.json or any tool field; surfaced per ac-4's own disclosure clause and owner-adjudicated 2026-07-13 as the optional toolchain.json carrier (Assemble stamps Graph.Tool; intake strict-decodes and runs CheckToolPin; absence is a disclosed-unproven notice — a path exercised by this very closure's own bundle intake, which printed the notice for the tool-less self-hosted bundle). (2) dc-1's seam landed as internal/httpjson per its implementer-choice clause. Judged coverage accepted as absent for this closure." }
digest: sha256:2df17d46d5fe73e6c82ba1c726f42d92e13cf02de48a100f73feacc9bd1c132f
frozen: { at: 2026-07-13, commit: 3a36af20e48bd42610d5cfd06544ca070a091caf }
provenance: { generator: verdi-align, version: v0, inputs: [spec/forge-transport@3a36af20e48bd42610d5cfd06544ca070a091caf, spec/forge-transport@8ff365db1bc3f149f7b6475598a6cea01ad10fef], digest: sha256:2df17d46d5fe73e6c82ba1c726f42d92e13cf02de48a100f73feacc9bd1c132f }
---
# Alignment report

## Computed

(none)

### Boundary diff vs acceptance baseline

(no impacted services)

## Judged

- **judged-coverage-absent** [accepted-deviation]: judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd="claude -p --output-format json") — Judge timed out at 2m (D6-21). Reviewer adjudicated manually: closure gate 3/3 on source:ci evidence; no decision conflicts (ADR corpus empty). Recorded adjudications: (1) dc-4's premise was FALSE against the built system — no fetched DerivedTree ever carried graph.json or any tool field; surfaced per ac-4's own disclosure clause and owner-adjudicated 2026-07-13 as the optional toolchain.json carrier (Assemble stamps Graph.Tool; intake strict-decodes and runs CheckToolPin; absence is a disclosed-unproven notice — a path exercised by this very closure's own bundle intake, which printed the notice for the tool-less self-hosted bundle). (2) dc-1's seam landed as internal/httpjson per its implementer-choice clause. Judged coverage accepted as absent for this closure.
