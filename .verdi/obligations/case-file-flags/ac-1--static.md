---
id: obligation/case-file-flags--ac-1--static
kind: obligation
title: "Case-file ladder stamps come from the dex lens's entry points"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/case-file-flags" }
frozen: { at: 2026-07-14, commit: c464efb6133e449257c48738ab66ae93a0e071e5 }
---
# Case-file ladder stamps come from the dex lens's entry points

The static evidence must witness the case file's spec-stale and
pending-supersession values flowing from decisionsweep.ScanSpecStale
(over lint.BuildSnapshot) and evidence.PendingSupersession fed by
evidence.LoadPendingSupersessionCandidates and
evidence.ImplementsByFeature — the exact functions internal/dex/lens.go
and ladder.go call — through the badge compute layer's attachment point,
with NO local re-derivation of either flag (no second accepted-deviation
counter, no second open-MR fold) anywhere under internal/workbench.
Matching outputs alone are insufficient; the shared call sites are the
claim (co-3).
