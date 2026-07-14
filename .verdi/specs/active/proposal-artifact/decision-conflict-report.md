---
schema: verdi.decisionconflict/v1
covers: 90aba9d98d76adc6fa75ea24607b572c3c346206
findings:
  - { id: judged-decision-coverage-absent, kind: judged, text: "judged decision-conflict coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd=\"claude -p --output-format json\")", disposition: exempt, note: "the configured judge_cmd timed out (2m0s) in this authoring environment rather than producing a result; this instance of the sweep is excused from requiring judged coverage rather than fabricated. Excusing this ONE absent run is not a claim that no undeclared conflict exists — a human re-run of `verdi align` with a working judge remains the honest path to real coverage before this story is built." }
sweep_provenance: { adr_corpus_digest: sha256:37517e5f3dc66819f61f5a7bb8ace1921282415f10551d2defa5c3eb0985b570, decisions_scanned: [spec/diagram-proposals#dc-1, spec/diagram-proposals#dc-2, spec/diagram-proposals#dc-3, spec/diagram-proposals#dc-4, spec/diagram-proposals#dc-5, spec/diagram-proposals#dc-6, spec/diagram-proposals#dc-7, spec/diagram-proposals#dc-8, spec/proposal-artifact#dc-1, spec/proposal-artifact#dc-2, spec/proposal-artifact#dc-3, spec/proposal-artifact#dc-4] }
digest: sha256:56dc08c8021c2e9f8e111248b4548c24e2fb538676af4f9703ccba66125b6cd1
provenance: { generator: verdi-align, version: v1, inputs: [spec/proposal-artifact@90aba9d98d76adc6fa75ea24607b572c3c346206], digest: sha256:56dc08c8021c2e9f8e111248b4548c24e2fb538676af4f9703ccba66125b6cd1 }
---
# Decision-conflict report

## Computed (declared-edge completeness)

(none)

## Judged (undeclared-conflict sweep)

- **judged-decision-coverage-absent** [exempt]: judged decision-conflict coverage absent: no result within 2m0s (stage=timeout, exit=0, cmd="claude -p --output-format json") — the configured judge_cmd timed out (2m0s) in this authoring environment rather than producing a result; this instance of the sweep is excused from requiring judged coverage rather than fabricated. Excusing this ONE absent run is not a claim that no undeclared conflict exists — a human re-run of `verdi align` with a working judge remains the honest path to real coverage before this story is built.
