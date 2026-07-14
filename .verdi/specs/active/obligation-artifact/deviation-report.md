---
schema: verdi.deviation/v1
covers: 2f78d219817f1e88530f0fcc48e68d3f7cf21400
findings:
  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd=\"claude -p --output-format json\")", disposition: accepted-deviation, note: "Judge timed out at 2m (D6-21). Reviewer performed the alignment review + conflict sweep manually; no decision conflicts with another decision or an ADR (ADR corpus empty). Three disclosed deviations, all reviewed and accepted: (1) dc-1 said the obligation's `verifies` edge targets an AC FRAGMENT, which conflicts with 02's ratified closedEdgeVocab (verifies may not target a fragment) — the reviewer directed a fix so obligations `verifies` the WHOLE story spec (AC in id/path), mirroring attestations and touching NO ratified invariant (the ValidateLinkForKind carve-out was fully reverted, common.go/vl003.go byte-identical to pre-build); VL-019 enforces story-AC-only from the obligation's own id. (2) The on-disk keying uses the SPEC NAME (`.verdi/obligations/<spec-name>/<ac>--<kind>.md`) rather than dc-2's story-ref-slug — a deliberate, self-consistent choice (backend + frontend agree, VL-011 path/id check passes) that AVOIDS the D6-18 story-ref-slug ambiguity the board would otherwise reintroduce; accepted, and Wave 2/3 will follow the same keying. (3) The board UX (a class-keyed obligation pushpin on story walls + a yarn-drop→for_kind picker, mirroring the scoping canvas) is Fable's disclosed design choice for the ac-3 authoring ambiguity — an invention-ledger candidate. make verify green (129 e2e). Judged coverage accepted as absent." }
digest: sha256:86629e350e017d5fa53d9e5baee78f47a9c45a7100f42eaf42bbc91244c09d5e
provenance: { generator: verdi-align, version: v0, inputs: [spec/obligation-artifact@2f78d219817f1e88530f0fcc48e68d3f7cf21400, spec/obligation-artifact@800094b6d688dfaa2a9063078065fc75d7858a72], digest: sha256:86629e350e017d5fa53d9e5baee78f47a9c45a7100f42eaf42bbc91244c09d5e }
---
# Alignment report

## Computed

(none)

### Boundary diff vs acceptance baseline

(no impacted services)

## Judged

- **judged-coverage-absent** [UNDISPOSITIONED]: judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd="claude -p --output-format json")
