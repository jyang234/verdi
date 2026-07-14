---
id: obligation/case-file-flags--ac-1--behavioral
kind: obligation
title: "Ladder stamps render three-valued on the case file"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/case-file-flags" }
frozen: { at: 2026-07-14, commit: c464efb6133e449257c48738ab66ae93a0e071e5 }
---
# Ladder stamps render three-valued on the case file

The behavioral evidence must drive all three outcomes on fixtures: (a) a
story with a deviation report crossing a spec-stale trigger shows the
spec-stale stamp on the case-file lockup, its drawer naming the finding
ids/count; (b) a story whose implemented objects are touched by an open
supersession MR (hermetic fake forge) shows the pending-supersession stamp
naming MR ids and touched objects; (c) the same story with NO forge
available shows a disclosure line in the board's notice vocabulary — no
stamp, no silence. An unflagged fixture must show no stamp at all.
