---
id: obligation/badge-computes--ac-3--static
kind: obligation
title: "Ladder flags come from the dex lens's own entry points"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/badge-computes" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# Ladder flags come from the dex lens's own entry points

The static evidence must witness the wall's compute layer calling
decisionsweep.ScanSpecStale (over lint.BuildSnapshot) and
evidence.PendingSupersession fed by
evidence.LoadPendingSupersessionCandidates and
evidence.ImplementsByFeature — the exact exported functions
internal/dex/lens.go's computeLensData calls — and must show NO local
reimplementation of either computation (no second accepted-deviation
counter, no second open-MR supersession fold) anywhere under
internal/workbench. Evidence that only shows matching outputs is
insufficient; the shared call sites are the claim (co-3).
