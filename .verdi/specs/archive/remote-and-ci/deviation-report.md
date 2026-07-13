---
schema: verdi.deviation/v1
covers: 971c229cfe7bdfe115e1fdf71bb7e74247741264
findings:
  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd=\"claude -p --output-format json\")", disposition: accepted-deviation, note: "Judge timed out at 2m on the 272-file mechanical rename diff (round6 D6-5); reviewer ran the undeclared-conflict sweep manually and found none — the diff is a module-path rename plus CI wiring, the four lint fixes, and forge/bundle correctness fixes, none touching a design decision or ADR (corpus empty). Judged coverage accepted as absent for this build." }
digest: sha256:60abdc9cbe8583e833e346616868eb14ef249cd177454e84f9b0531f345e7892
frozen: { at: 2026-07-13, commit: 971c229cfe7bdfe115e1fdf71bb7e74247741264 }
provenance: { generator: verdi-align, version: v0, inputs: [spec/remote-and-ci@971c229cfe7bdfe115e1fdf71bb7e74247741264, spec/remote-and-ci@6b7b6afcf54b2fb6882076455a67a0fae99be435], digest: sha256:60abdc9cbe8583e833e346616868eb14ef249cd177454e84f9b0531f345e7892 }
---
# Alignment report

## Computed

(none)

### Boundary diff vs acceptance baseline

(no impacted services)

## Judged

- **judged-coverage-absent** [accepted-deviation]: judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd="claude -p --output-format json") — Judge timed out at 2m on the 272-file mechanical rename diff (round6 D6-5); reviewer ran the undeclared-conflict sweep manually and found none — the diff is a module-path rename plus CI wiring, the four lint fixes, and forge/bundle correctness fixes, none touching a design decision or ADR (corpus empty). Judged coverage accepted as absent for this build.
