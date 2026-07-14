---
id: obligation/verification-extractor--ac-1--behavioral
kind: obligation
title: "A table-driven parser test covers every recognized form, an unrecognized construct, and an identity collision"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/verification-extractor" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# A table-driven parser test covers every recognized form, an unrecognized construct, and an identity collision

The behavioral evidence must show a table-driven test that: (1) parses a
fixture using every one of the four edge forms, both node-declaration
forms, `%%` comment lines, and a `classDef`/`:::classname` node-class
assignment, asserting `full` coverage and the correct extracted node/edge
set (proving a pristine, flowmap-generated-style diagram — which always
carries at least a header comment and its classDefs — can reach `full`);
(2) parses a fixture containing one construct outside the declared grammar
(e.g. a `subgraph` block), asserting the WHOLE artifact downgrades to
`partial` while the recognized lines still extract; (3) parses a fixture
where two distinct truth FQNs normalize to the same `shortName`, asserting
the affected proposal node is excluded from full classification and the
artifact downgrades to `partial` rather than guessing. A test that only
exercises the happy path does not satisfy this obligation.
