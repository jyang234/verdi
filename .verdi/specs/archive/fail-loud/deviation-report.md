---
schema: verdi.deviation/v1
covers: 3a36af20e48bd42610d5cfd06544ca070a091caf
findings:
  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd=\"claude -p --output-format json\")", disposition: accepted-deviation, note: "Judge timed out at 2m (D6-21, the persistent align-judge timeout). Reviewer adjudicated manually: every diff was personally reviewed against the spec during the build (four topic commits, each AC's witness test verified red-then-green); the closure gate passed 3/3 on source:ci evidence; no decision here conflicts with another decision or an ADR (ADR corpus empty). Two build-time events already disclosed at merge are recorded for the ledger: (1) VL-020 activated mid-build (obligation-gate merged ahead), so this story's obligations were authored post-acceptance in the #29 hotfix rather than at accept; (2) that hotfix also added artifactview's obligation arm — an integration necessity (dex failed closed on the first real-store obligations), scoped to Base+ForKind, with wall/matrix presentation left to obligation-wall. Judged coverage accepted as absent for this closure." }
digest: sha256:2df17d46d5fe73e6c82ba1c726f42d92e13cf02de48a100f73feacc9bd1c132f
frozen: { at: 2026-07-13, commit: 3a36af20e48bd42610d5cfd06544ca070a091caf }
provenance: { generator: verdi-align, version: v0, inputs: [spec/fail-loud@3a36af20e48bd42610d5cfd06544ca070a091caf, spec/fail-loud@15d86d18f456796ff9c011cae8a2c691933d6a8a], digest: sha256:2df17d46d5fe73e6c82ba1c726f42d92e13cf02de48a100f73feacc9bd1c132f }
---
# Alignment report

## Computed

(none)

### Boundary diff vs acceptance baseline

(no impacted services)

## Judged

- **judged-coverage-absent** [accepted-deviation]: judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd="claude -p --output-format json") — Judge timed out at 2m (D6-21, the persistent align-judge timeout). Reviewer adjudicated manually: every diff was personally reviewed against the spec during the build (four topic commits, each AC's witness test verified red-then-green); the closure gate passed 3/3 on source:ci evidence; no decision here conflicts with another decision or an ADR (ADR corpus empty). Two build-time events already disclosed at merge are recorded for the ledger: (1) VL-020 activated mid-build (obligation-gate merged ahead), so this story's obligations were authored post-acceptance in the #29 hotfix rather than at accept; (2) that hotfix also added artifactview's obligation arm — an integration necessity (dex failed closed on the first real-store obligations), scoped to Base+ForKind, with wall/matrix presentation left to obligation-wall. Judged coverage accepted as absent for this closure.
