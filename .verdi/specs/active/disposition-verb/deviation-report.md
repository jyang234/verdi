---
schema: verdi.deviation/v1
covers: e2cc9be9335a9732b0232993362345fffce7927b
findings:
  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: decoding inner findings JSON: artifact: strict json decode: invalid character 'A' looking for beginning of value (stage=inner-parse, exit=0, cmd=\"claude -p --output-format json\")" }
digest: sha256:98db5d32715448560f09b896c30df28fecd11fa14b4289b467a510c6c6d831fb
provenance: { generator: verdi-align, version: v0, inputs: [spec/disposition-verb@e2cc9be9335a9732b0232993362345fffce7927b, spec/disposition-verb@d2ecf50f0e6f8a3163692abce22fe55de7adf3c2], digest: sha256:98db5d32715448560f09b896c30df28fecd11fa14b4289b467a510c6c6d831fb }
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

- **judged-coverage-absent** [UNDISPOSITIONED]: judged coverage absent: decoding inner findings JSON: artifact: strict json decode: invalid character 'A' looking for beginning of value (stage=inner-parse, exit=0, cmd="claude -p --output-format json")
