---
schema: verdi.deviation/v1
covers: da822c245134feb70b3616b6bed094cef610a2f0
findings:
  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd=\"claude -p --output-format json\")", disposition: accepted-deviation, note: "Judge timed out at 2m (D6-21). Reviewer performed the alignment review + conflict sweep manually; no decision conflicts with another decision or an ADR. ac-1 (VL-020): the activation gate mirrors VL-006's structure and requires an obligation per (story-AC, kind) at .verdi/obligations/\u003cspec-name\u003e/\u003cac-id\u003e--\u003ckind\u003e.md (spec-name keying, consistent with wave 1). ac-2: feature/component ACs exempt (class-resolved), record + fold UNCHANGED (oq-1 resolution honored). Two disclosed judgment calls, both reviewed and accepted: (1) VL-006 timing — vl006.go has NO draft branch (it fires unconditionally); VL-020 instead honors spec/obligation-gate co-2 directly (draft-tolerant, gates non-draft), the more correct reading, disclosed in the doc comment. (2) obligationGateBaseline — a named, documented one-time grandfather list exempting the ~37 pre-existing (ac,kind) pairs that predate obligations (the standard lint-rollout pattern; rejected fabricating 37 obligations and a dormant flag). Accepted as the smallest reversible option; consequence disclosed: the feature’s own stories are grandfathered, so no real spec uses obligations yet (mechanism proven by fixtures) — authoring obligations for the feature’s own stories (removing them from the baseline) is a dogfood-completeness follow-on. Separately, the reviewer re-synced the verdi-artifact-contract mirror after the owner ratified VL-019 into 02 externally (a separate commit). make verify green (129 e2e). Judged coverage accepted as absent." }
digest: sha256:072a811a2c233b7c3de88e2411e72be0cc0edc1a4dfe3159f1c231c875b869db
frozen: { at: 2026-07-13, commit: da822c245134feb70b3616b6bed094cef610a2f0 }
provenance: { generator: verdi-align, version: v0, inputs: [spec/obligation-gate@da822c245134feb70b3616b6bed094cef610a2f0, spec/obligation-gate@f877ff019cda7d7271aeea9f4fb1d36a3449c4dd], digest: sha256:072a811a2c233b7c3de88e2411e72be0cc0edc1a4dfe3159f1c231c875b869db }
---
# Alignment report

## Computed

(none)

### Boundary diff vs acceptance baseline

(no impacted services)

## Judged

- **judged-coverage-absent** [accepted-deviation]: judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd="claude -p --output-format json") — Judge timed out at 2m (D6-21). Reviewer performed the alignment review + conflict sweep manually; no decision conflicts with another decision or an ADR. ac-1 (VL-020): the activation gate mirrors VL-006's structure and requires an obligation per (story-AC, kind) at .verdi/obligations/<spec-name>/<ac-id>--<kind>.md (spec-name keying, consistent with wave 1). ac-2: feature/component ACs exempt (class-resolved), record + fold UNCHANGED (oq-1 resolution honored). Two disclosed judgment calls, both reviewed and accepted: (1) VL-006 timing — vl006.go has NO draft branch (it fires unconditionally); VL-020 instead honors spec/obligation-gate co-2 directly (draft-tolerant, gates non-draft), the more correct reading, disclosed in the doc comment. (2) obligationGateBaseline — a named, documented one-time grandfather list exempting the ~37 pre-existing (ac,kind) pairs that predate obligations (the standard lint-rollout pattern; rejected fabricating 37 obligations and a dormant flag). Accepted as the smallest reversible option; consequence disclosed: the feature’s own stories are grandfathered, so no real spec uses obligations yet (mechanism proven by fixtures) — authoring obligations for the feature’s own stories (removing them from the baseline) is a dogfood-completeness follow-on. Separately, the reviewer re-synced the verdi-artifact-contract mirror after the owner ratified VL-019 into 02 externally (a separate commit). make verify green (129 e2e). Judged coverage accepted as absent.
