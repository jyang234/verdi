---
schema: verdi.deviation/v1
covers: 855d29436a7c0463f31ce28927f35da4824db0a5
findings:
  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd=\"claude -p --output-format json\")", disposition: accepted-deviation, note: "Judge timed out at 2m (D6-21). Reviewer performed the alignment review + conflict sweep manually; no decision conflicts with another decision or an ADR. ac-1 (VL-020): the activation gate mirrors VL-006's structure and requires an obligation per (story-AC, kind) at .verdi/obligations/<spec-name>/<ac-id>--<kind>.md (spec-name keying, consistent with wave 1). ac-2: feature/component ACs exempt (class-resolved), record + fold UNCHANGED (oq-1 resolution honored). Two disclosed judgment calls, both reviewed and accepted: (1) VL-006 timing — vl006.go has NO draft branch (it fires unconditionally); VL-020 instead honors spec/obligation-gate co-2 directly (draft-tolerant, gates non-draft), the more correct reading, disclosed in the doc comment. (2) obligationGateBaseline — a named, documented one-time grandfather list exempting the ~37 pre-existing (ac,kind) pairs that predate obligations (the standard lint-rollout pattern; rejected fabricating 37 obligations and a dormant flag). Accepted as the smallest reversible option; consequence disclosed: the feature’s own stories are grandfathered, so no real spec uses obligations yet (mechanism proven by fixtures) — authoring obligations for the feature’s own stories (removing them from the baseline) is a dogfood-completeness follow-on. Separately, the reviewer re-synced the verdi-artifact-contract mirror after the owner ratified VL-019 into 02 externally (a separate commit). make verify green (129 e2e). Judged coverage accepted as absent." }
digest: sha256:85e92aaa7c4c7b165c38a96e939d17be10d93b9d08b532e820ced3e8ad218ed2
provenance: { generator: verdi-align, version: v0, inputs: [spec/obligation-gate@855d29436a7c0463f31ce28927f35da4824db0a5, spec/obligation-gate@f877ff019cda7d7271aeea9f4fb1d36a3449c4dd], digest: sha256:85e92aaa7c4c7b165c38a96e939d17be10d93b9d08b532e820ced3e8ad218ed2 }
---
# Alignment report

## Computed

(none)

### Boundary diff vs acceptance baseline

(no impacted services)

## Judged

- **judged-coverage-absent** [UNDISPOSITIONED]: judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd="claude -p --output-format json")
