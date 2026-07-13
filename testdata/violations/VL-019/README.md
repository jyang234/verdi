# testdata/violations/VL-019

VL-019 (an obligation's `verifies` edge must target a STORY acceptance
criterion — spec/obligation-artifact AC-2/DC-3, spec/evidence-obligations
DC-3), implemented alongside the obligation artifact kind (evidence-
obligations wave 1).

- `.verdi/obligations/stale-decline/ac-1--static.md` — an obligation whose
  `verifies` edge targets `spec/stale-decline#ac-1`. `spec/stale-decline`
  is `class: feature` in the golden corpus, so this ac-1 is a FEATURE AC,
  not a STORY AC — VL-019's headline refusal case (obligations are a
  story-level concern only, 03 §The feature fold).

VL-019's other two refusal shapes (a non-AC fragment; a whole spec, no
fragment at all) and its positive complement (a real STORY AC) are covered
by ad hoc overlays in vl019_test.go rather than additional testdata
directories here, reusing this same golden-corpus fixture set
(`spec/stale-decline`'s constraints and `spec/borrower-update-api`'s own
acceptance criterion, and `spec/accepted-pending-build`'s `co-1`) so no new
corpus surface is needed per scenario.
