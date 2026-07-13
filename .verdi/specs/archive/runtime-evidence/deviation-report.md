---
schema: verdi.deviation/v1
covers: d54c19f3585da58fdc7464d5d8fcd327085ff51a
findings:
  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd=\"claude -p --output-format json\")", disposition: accepted-deviation, note: "Judge timed out at 2m on the 16-file diff (+1393/-41) — the third build in a row where the align judge cannot finish on a real change set (D6-5, D6-8; now a confirmed recurring process finding, logged D6-21). Reviewer performed the alignment review and undeclared-conflict sweep manually: no decision conflicts with another decision or an ADR (ADR corpus empty). Two disclosed design points, both spec-aligned: (1) derivedFileNames left at four files — runtime.json is dc-2's SEPARATELY-written sibling, never part of the regenerated per-service bundle those tests assert, so widening it would have been wrong, not the narrower reading; (2) internal/runtime.Query is exercised in unit tests but not on the close path — co-2's queryable-by-(story,AC) is met by the fold's own location(story)+EvidenceFor(AC) scoping (ac-2's 'consume exactly as static/behavioral'), and Query is the explicit realization of the same, proven crisply in isolation. Provenance mirrors D6-10 exactly (source:ci only when InCI \u0026\u0026 !ForceLocal). The one real build gap — no verdi.bindings.yaml entries for runtime-evidence#ac-1/#ac-2 — was fixed by the reviewer before this report. Judged coverage accepted as absent for this build." }
digest: sha256:8a86ab02d3e9db6b5a15d7b8bea477204754b12f6cdceb60932979af464d3f4e
frozen: { at: 2026-07-13, commit: d54c19f3585da58fdc7464d5d8fcd327085ff51a }
provenance: { generator: verdi-align, version: v0, inputs: [spec/runtime-evidence@d54c19f3585da58fdc7464d5d8fcd327085ff51a, spec/runtime-evidence@f9b1597affa00a6570a1f1e28763372d462fe5b6], digest: sha256:8a86ab02d3e9db6b5a15d7b8bea477204754b12f6cdceb60932979af464d3f4e }
---
# Alignment report

## Computed

(none)

### Boundary diff vs acceptance baseline

(no impacted services)

## Judged

- **judged-coverage-absent** [accepted-deviation]: judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd="claude -p --output-format json") — Judge timed out at 2m on the 16-file diff (+1393/-41) — the third build in a row where the align judge cannot finish on a real change set (D6-5, D6-8; now a confirmed recurring process finding, logged D6-21). Reviewer performed the alignment review and undeclared-conflict sweep manually: no decision conflicts with another decision or an ADR (ADR corpus empty). Two disclosed design points, both spec-aligned: (1) derivedFileNames left at four files — runtime.json is dc-2's SEPARATELY-written sibling, never part of the regenerated per-service bundle those tests assert, so widening it would have been wrong, not the narrower reading; (2) internal/runtime.Query is exercised in unit tests but not on the close path — co-2's queryable-by-(story,AC) is met by the fold's own location(story)+EvidenceFor(AC) scoping (ac-2's 'consume exactly as static/behavioral'), and Query is the explicit realization of the same, proven crisply in isolation. Provenance mirrors D6-10 exactly (source:ci only when InCI && !ForceLocal). The one real build gap — no verdi.bindings.yaml entries for runtime-evidence#ac-1/#ac-2 — was fixed by the reviewer before this report. Judged coverage accepted as absent for this build.
