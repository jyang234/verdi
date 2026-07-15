---
schema: verdi.deviation/v1
covers: 30c5ff945413930879823be6db0ccc07d5abd6b9
findings:
  - { id: ac-1, kind: judged, text: "the mobile PUT route retries a failed update up to 3 times with client-side exponential backoff before surfacing an error to the borrower; ac-1's own wording ('returns 200 with the new state') describes a single request/response contract and never mentions a client-driven retry loop", disposition: accepted-deviation, note: "accepted by platform-team, 2026-07-12: the retry loop measurably reduces borrower-visible failures on flaky mobile networks and does not touch loansvc's server-side outbox retry budget (adr/0002), so it is orthogonal to the outbox path rather than a substitute for it; recorded against the AC's own text, so spec-stale counter-pressure applies (03 §The amendment ladder)" }
  - { id: f-2, kind: computed, text: "declared implements edges resolve at build head", disposition: fixed }
---
# Alignment report: borrower-update-mobile (living)

A living, mid-build alignment report whose first finding is an
`accepted-deviation` disposition targeting the story's OWN declared ac-1 —
`evidence.SpecStale` trigger (a) (finding id equals an own AC id, R4-I-18).
The story page's `spec-stale` ladder badge renders this exact computation
(05 §Lenses, story lens).

## Judged

Mid-build, the mobile client grew its own retry loop for the update call —
three attempts with client-side backoff — before the story was accepted.
That's a real behavior the spec's ac-1 text never described: ac-1 says the
route "returns 200 with the new state," which reads as one request and one
response, not a client that silently retries before the borrower ever
sees a failure. The team judged this a reasonable divergence rather than a
defect: it's a client-side reliability improvement over a shaky mobile
network, it never touches loansvc's own outbox retry budget on the server
side, and rewriting ac-1's text mid-build to describe retry counts felt
like over-specifying an implementation detail into an acceptance
criterion. platform-team accepted the deviation on 2026-07-12, the same
day the story's stories froze, which is precisely what accumulates
rung-arbitrage pressure (03 §The amendment ladder) — the spec-stale badge
exists so this doesn't quietly become permanent drift.

## Computed

Edge resolution holds at the covered head.
