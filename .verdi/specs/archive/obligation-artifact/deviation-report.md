---
schema: verdi.deviation/v1
covers: da822c245134feb70b3616b6bed094cef610a2f0
findings:
  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd=\"claude -p --output-format json\")", disposition: accepted-deviation, note: "Judge timed out at 2m (D6-21). Reviewer performed the alignment review + conflict sweep manually; no decision conflicts with another decision or an ADR (ADR corpus empty). Three disclosed deviations, all reviewed and accepted: (1) dc-1 said the obligation's `verifies` edge targets an AC FRAGMENT, which conflicts with 02's ratified closedEdgeVocab (verifies may not target a fragment) — the reviewer directed a fix so obligations `verifies` the WHOLE story spec (AC in id/path), mirroring attestations and touching NO ratified invariant (the ValidateLinkForKind carve-out was fully reverted, common.go/vl003.go byte-identical to pre-build); VL-019 enforces story-AC-only from the obligation's own id. (2) The on-disk keying uses the SPEC NAME (`.verdi/obligations/\u003cspec-name\u003e/\u003cac\u003e--\u003ckind\u003e.md`) rather than dc-2's story-ref-slug — a deliberate, self-consistent choice (backend + frontend agree, VL-011 path/id check passes) that AVOIDS the D6-18 story-ref-slug ambiguity the board would otherwise reintroduce; accepted, and Wave 2/3 will follow the same keying. (3) The board UX (a class-keyed obligation pushpin on story walls + a yarn-drop→for_kind picker, mirroring the scoping canvas) is Fable's disclosed design choice for the ac-3 authoring ambiguity — an invention-ledger candidate. make verify green (129 e2e). Judged coverage accepted as absent." }
digest: sha256:072a811a2c233b7c3de88e2411e72be0cc0edc1a4dfe3159f1c231c875b869db
frozen: { at: 2026-07-13, commit: da822c245134feb70b3616b6bed094cef610a2f0 }
provenance: { generator: verdi-align, version: v0, inputs: [spec/obligation-artifact@da822c245134feb70b3616b6bed094cef610a2f0, spec/obligation-artifact@800094b6d688dfaa2a9063078065fc75d7858a72], digest: sha256:072a811a2c233b7c3de88e2411e72be0cc0edc1a4dfe3159f1c231c875b869db }
---
# Alignment report

## Computed

(none)

### Boundary diff vs acceptance baseline

(no impacted services)

## Judged

- **judged-coverage-absent** [accepted-deviation]: judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd="claude -p --output-format json") — Judge timed out at 2m (D6-21). Reviewer performed the alignment review + conflict sweep manually; no decision conflicts with another decision or an ADR (ADR corpus empty). Three disclosed deviations, all reviewed and accepted: (1) dc-1 said the obligation's `verifies` edge targets an AC FRAGMENT, which conflicts with 02's ratified closedEdgeVocab (verifies may not target a fragment) — the reviewer directed a fix so obligations `verifies` the WHOLE story spec (AC in id/path), mirroring attestations and touching NO ratified invariant (the ValidateLinkForKind carve-out was fully reverted, common.go/vl003.go byte-identical to pre-build); VL-019 enforces story-AC-only from the obligation's own id. (2) The on-disk keying uses the SPEC NAME (`.verdi/obligations/<spec-name>/<ac>--<kind>.md`) rather than dc-2's story-ref-slug — a deliberate, self-consistent choice (backend + frontend agree, VL-011 path/id check passes) that AVOIDS the D6-18 story-ref-slug ambiguity the board would otherwise reintroduce; accepted, and Wave 2/3 will follow the same keying. (3) The board UX (a class-keyed obligation pushpin on story walls + a yarn-drop→for_kind picker, mirroring the scoping canvas) is Fable's disclosed design choice for the ac-3 authoring ambiguity — an invention-ledger candidate. make verify green (129 e2e). Judged coverage accepted as absent.
