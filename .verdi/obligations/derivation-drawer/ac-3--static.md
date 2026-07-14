---
id: obligation/derivation-drawer--ac-3--static
kind: obligation
title: "Sweep provenance comes from the decoded report, compared not verdicted"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/derivation-drawer" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# Sweep provenance comes from the decoded report, compared not verdicted

The static evidence must show the judged-findings surface reading the
spec's decision-conflict-report.md through artifact.DecodeDecisionConflict
(the one strict decoder — never a local YAML parse), surfacing Covers,
SweepProvenance.ADRCorpusDigest, and DecisionsScanned into the drawer
content, and computing only equality/set comparisons against the current
spec digest and declared decision ids (dc-3) — no new staleness verdict
type, no rule that blocks on the comparison's outcome, and no silent drop
of an undispositioned finding.
