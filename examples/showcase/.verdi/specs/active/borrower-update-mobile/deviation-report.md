---
schema: verdi.deviation/v1
covers: 5507c6d963bd78d9eabed2324c3d380e678f891e
findings:
  - { id: ac-1, kind: judged, text: "mobile PUT route commits direct writes for offline support — the story's own ac-1 wording assumes the outbox path", disposition: accepted-deviation, note: "accepted for offline support; recorded against the AC's own text, so spec-stale counter-pressure applies (03 §The amendment ladder)" }
  - { id: f-2, kind: computed, text: "declared implements edges resolve at build head", disposition: fixed }
---
# Alignment report: borrower-update-mobile (living)

The V1-P8 dex-overlay fixture: a living, mid-build alignment report whose
first finding is an `accepted-deviation` disposition targeting the story's
OWN declared `ac-1` — `evidence.SpecStale` trigger (a) (finding id equals
an own AC id, R4-I-18). The story page's `spec-stale` ladder badge renders
this exact computation (05 §Lenses, story lens).

## Judged

The mobile update route deviates from ac-1's own wording; accepted with a
note, which is precisely what accumulates rung-arbitrage pressure.

## Computed

Edge resolution holds at the covered head.
