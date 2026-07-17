---
schema: verdi.deviation/v1
covers: 71c54f25b681291f256fadbcd575ced0eb2de1c3
findings:
  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: decoding inner findings JSON: artifact: strict json decode: invalid character 'A' looking for beginning of value (stage=inner-parse, exit=0, cmd=\"claude -p --output-format json\")" }
digest: sha256:9d50c3ed3406e9e8207aa1efc2ddf9269abefb5f81ec67bf8a6bb7c9699426a8
provenance: { generator: verdi-align, version: v0, inputs: [spec/model-digest@71c54f25b681291f256fadbcd575ced0eb2de1c3, spec/model-digest@b8773fb49d1fe29af68ffff0fe92868c873962c2], digest: sha256:9d50c3ed3406e9e8207aa1efc2ddf9269abefb5f81ec67bf8a6bb7c9699426a8, model: sha256:b4e5edd8798acdbc5ca1ca89a164f6b329fb067fe5a241448699ebd836eab489 }
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
