---
id: obligation/badge-computes--ac-2--behavioral
kind: obligation
title: "Object findings badge cards; spec findings badge the case file; plumbing stays off"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/badge-computes" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# Object findings badge cards; spec findings badge the case file; plumbing stays off

The behavioral evidence must render fixture stores exercising all three
buckets: (a) an object-anchored finding (e.g. VL-006's AC-with-no-evidence-
kind) badges exactly the card of the object it names; (b) a spec-level
finding with no single object anchor badges the case file and no card;
(c) a store-structural finding (e.g. a .gitattributes or data-tracking
violation) present in `verdi lint` output produces NO badge anywhere on
the wall. Findings scoped to a DIFFERENT spec's directory must not badge
this wall.
