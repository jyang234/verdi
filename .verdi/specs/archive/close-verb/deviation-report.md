---
schema: verdi.deviation/v1
covers: d291d60c0608fdeee1b62a98e7155b0f28f24112
findings:
  - { id: judged-coverage-absent, kind: judged, text: "judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd=\"claude -p --output-format json\")", disposition: accepted-deviation, note: "Judge timed out at 2m on the 7-commit diff (close verb + self-hosted evidence producer + fake-tracker config + the ratified keying/provenance fix + mirror sync; D6-5). Reviewer ran the undeclared-conflict sweep manually and found none — no decision here conflicts with another decision or an ADR (the ADR corpus is empty). The status-flip and CI-topology deviations were separately adjudicated (zone-reading ratified, round6 D6-11; CI merge accepted, D6-12). Judged coverage accepted as absent for this build." }
digest: sha256:bad808335437527fe9e0989dabcc7d81c5fc8212be1fd53600dd7981e7fff750
frozen: { at: 2026-07-13, commit: d291d60c0608fdeee1b62a98e7155b0f28f24112 }
provenance: { generator: verdi-align, version: v0, inputs: [spec/close-verb@d291d60c0608fdeee1b62a98e7155b0f28f24112, spec/close-verb@244f42bc9d3f7d76b3377a626bb492b787969941], digest: sha256:bad808335437527fe9e0989dabcc7d81c5fc8212be1fd53600dd7981e7fff750 }
---
# Alignment report

## Computed

(none)

### Boundary diff vs acceptance baseline

(no impacted services)

## Judged

- **judged-coverage-absent** [accepted-deviation]: judged coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd="claude -p --output-format json") — Judge timed out at 2m on the 7-commit diff (close verb + self-hosted evidence producer + fake-tracker config + the ratified keying/provenance fix + mirror sync; D6-5). Reviewer ran the undeclared-conflict sweep manually and found none — no decision here conflicts with another decision or an ADR (the ADR corpus is empty). The status-flip and CI-topology deviations were separately adjudicated (zone-reading ratified, round6 D6-11; CI merge accepted, D6-12). Judged coverage accepted as absent for this build.
