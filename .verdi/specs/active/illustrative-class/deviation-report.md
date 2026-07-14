---
schema: verdi.deviation/v1
covers: d5c328616bb33fdb1ad95813944825d99ffc0a5f
findings:
  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd=\"claude -p --output-format json\")", disposition: accepted-deviation, note: "Judge timed out at 2m (D6-21). Reviewer performed the alignment review manually against spec/illustrative-class and parent ac-4: dc-1's badge emitted once at the shared render seam (figure[data-diagram-tier] + figcaption chip, unknown-class fail-closed), dc-2's location/class tier decision as a pure function of bytes, dc-3 links-not-transclusion, ac-1's six render surfaces each proven under the one vendored asset with non-loopback aborts, ac-3's no-unbadged-mermaid sweep. Adjudicated and ACCEPTED (ADJ-14): (1) proposal-tier display computes grammar coverage via diagramverify.Parse(source, nil) — a pure parse, no flowmap exec, satisfying the obligation's never-runs-flowmap intent without a canned report; the truth-compared tier + findings remain the board-editor rail's surface; (2) board-editor unmerged, so ac-3's proposal surfaces = dex artifact/corpus/peek, all covered; (3) fixtures live only in the scratch e2e store, preserving golden SHAs. Salvage adopted whole after review (both commits + the dirty fixture file). No decision conflicts (ADR corpus empty). make verify green (150 e2e). Judged coverage accepted as absent for this build." }
digest: sha256:f2004256526f2a31f2e0046bfd602f3a739e5a2038d1bd8c6f999b5276ed54c9
provenance: { generator: verdi-align, version: v0, inputs: [spec/illustrative-class@d5c328616bb33fdb1ad95813944825d99ffc0a5f, spec/illustrative-class@941e68b442168a6c9c8e6832c7f3b6929b9cbe9b], digest: sha256:f2004256526f2a31f2e0046bfd602f3a739e5a2038d1bd8c6f999b5276ed54c9 }
---
# Alignment report

## Computed

(none)

### Boundary diff vs acceptance baseline

(no impacted services)

### Diagram alignment

- (no accepted proposals)
- (no illustrative diagrams in this spec's body)

## Judged

- **judged-coverage-absent** [accepted-deviation]: judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd="claude -p --output-format json") — manual reviewer alignment; ADJ-14 calls ratified; see frontmatter note.
