---
schema: verdi.deviation/v1
covers: 0777013cdbb21c7e486da1f5c244b64de7c5e9cd
findings:
  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd=\"claude -p --output-format json\")", disposition: accepted-deviation, note: "Judge timed out at 2m on the 7-commit diff (close verb + self-hosted evidence producer + fake-tracker config + the ratified keying/provenance fix + mirror sync; D6-5). Reviewer ran the undeclared-conflict sweep manually and found none — no decision here conflicts with another decision or an ADR (the ADR corpus is empty). The status-flip and CI-topology deviations were separately adjudicated (zone-reading ratified, round6 D6-11; CI merge accepted, D6-12). Judged coverage accepted as absent for this build." }
digest: sha256:67241209efd32d2d69cb4641ed0fc566ea8d87945dc05173091c5698cc08c98e
provenance: { generator: verdi-align, version: v0, inputs: [spec/close-verb@0777013cdbb21c7e486da1f5c244b64de7c5e9cd, spec/close-verb@244f42bc9d3f7d76b3377a626bb492b787969941], digest: sha256:67241209efd32d2d69cb4641ed0fc566ea8d87945dc05173091c5698cc08c98e }
---
# Alignment report

## Computed

(none)

### Boundary diff vs acceptance baseline

(no impacted services)

## Judged

- **judged-coverage-absent** [UNDISPOSITIONED]: judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd="claude -p --output-format json")
