---
schema: verdi.deviation/v1
covers: 7d510c84b73e6d9d792cd1c8bb97057b75bc21cc
findings:
  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd=\"claude -p --output-format json\")", disposition: accepted-deviation, note: "Judge timed out at 2m on the 272-file mechanical rename diff (round6 D6-5); reviewer ran the undeclared-conflict sweep manually and found none — the diff is a module-path rename plus CI wiring, the four lint fixes, and forge/bundle correctness fixes, none touching a design decision or ADR (corpus empty). Judged coverage accepted as absent for this build." }
digest: sha256:fbe83f89a9e085ae703b994a52a87ed12ec9030f94348e45972250b5ed9d61dd
frozen: { at: 2026-07-13, commit: 7d510c84b73e6d9d792cd1c8bb97057b75bc21cc }
provenance: { generator: verdi-align, version: v0, inputs: [spec/remote-and-ci@7d510c84b73e6d9d792cd1c8bb97057b75bc21cc, spec/remote-and-ci@6b7b6afcf54b2fb6882076455a67a0fae99be435], digest: sha256:fbe83f89a9e085ae703b994a52a87ed12ec9030f94348e45972250b5ed9d61dd }
---
# Alignment report

## Computed

(none)

### Boundary diff vs acceptance baseline

(no impacted services)

## Judged

- **judged-coverage-absent** [accepted-deviation]: judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd="claude -p --output-format json") — Judge timed out at 2m on the 272-file mechanical rename diff (round6 D6-5); reviewer ran the undeclared-conflict sweep manually and found none — the diff is a module-path rename plus CI wiring, the four lint fixes, and forge/bundle correctness fixes, none touching a design decision or ADR (corpus empty). Judged coverage accepted as absent for this build.
