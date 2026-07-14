# testdata/violations/VL-019

VL-019 (an obligation's `verifies` edge must target a WHOLE STORY spec, and
the acceptance criterion named by the obligation's own id must be one that
story declares — spec/obligation-artifact AC-2/DC-3, spec/evidence-obligations
DC-3), implemented alongside the obligation artifact kind (evidence-
obligations wave 1). Obligations mirror attestations: the `verifies` edge
names the whole `spec/<story>` (no fragment), and the AC lives in the id and
on-disk path (`<story-slug>--<ac-id>--<for-kind>`).

- `.verdi/obligations/stale-decline/ac-1--static.md` — an obligation whose
  `verifies` edge targets the whole spec `spec/stale-decline`.
  `spec/stale-decline` is `class: feature` in the golden corpus, so it is
  not a STORY — VL-019's headline refusal case (obligations are a
  story-level concern only, 03 §The feature fold). The id's `ac-1` segment
  is never reached, because the class check refuses a non-STORY target first.

VL-019's other refusal shapes (a STORY spec whose declared ACs do not
include the id's `<ac-id>`; an unresolvable target spec; a fragment-bearing
verifies edge — the old, now-invalid form, which VL-003's closed edge
vocabulary also rejects) and its positive complement (a whole STORY spec
that declares the id's AC) are covered by ad hoc overlays in vl019_test.go
rather than additional testdata directories here, so no new corpus surface
is needed per scenario.
