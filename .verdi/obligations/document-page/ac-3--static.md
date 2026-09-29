---
id: obligation/document-page--ac-3--static
kind: obligation
title: "The document body stays byte-identical across its four renders"
owners: [platform-team]
for_kind: static
quality:
  state: elaborated
  claim: "For every fixture spec, the body bytes inside the board's Document page equal the docs site's, verdi spec doc's, and MCP get_document's bytes for that spec."
  falsifier: "Any byte of the shared body differs between the Document page and another render."
  scope: "The four renders over the committed fixture specs, including specs with closed-spec supersession lines."
  producer: { kind: test, ref: "go-test:internal/workbench:TestDocumentPage_BodyByteParity" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:internal/workbench:TestDocumentPage_BodyByteParity in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/document-page" }
frozen: { at: 2026-09-29, commit: 88ebe32308f48b37711592d4fc21bcde2d166bbe }
---
# The document body stays byte-identical across its four renders

CI job `verify` must record producer `go-test:internal/workbench:TestDocumentPage_BodyByteParity` at the exact candidate
commit. A parity test extracting the shared body from the Document page and comparing it with the other renders.
