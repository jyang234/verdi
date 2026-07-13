---
schema: verdi.deviation/v1
covers: 9e26dd1051791f5b3bd6dd4efaf03bfccde59703
findings:
  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd=\"claude -p --output-format json\")", disposition: accepted-deviation, note: "Judge timed out at 2m on the 272-file mechanical rename diff (round6 D6-5); reviewer ran the undeclared-conflict sweep manually and found none — the diff is a module-path rename plus CI wiring, the four lint fixes, and forge/bundle correctness fixes, none touching a design decision or ADR (corpus empty). Judged coverage accepted as absent for this build." }
digest: sha256:1a2c87110f352be4ac607436e6d9a66a3c4b9c75b9cad954111633dead217977
provenance: { generator: verdi-align, version: v0, inputs: [spec/remote-and-ci@9e26dd1051791f5b3bd6dd4efaf03bfccde59703, spec/remote-and-ci@6b7b6afcf54b2fb6882076455a67a0fae99be435], digest: sha256:1a2c87110f352be4ac607436e6d9a66a3c4b9c75b9cad954111633dead217977 }
---
# Alignment report

## Computed

(none)

### Boundary diff vs acceptance baseline

(no impacted services)

## Judged

- **judged-coverage-absent** [UNDISPOSITIONED]: judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd="claude -p --output-format json")
