---
id: obligation/borrower-update-mobile--ac-1--behavioral
kind: obligation
title: "A mobile update lands and reads back correctly, including after the client's own retry loop fires"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/borrower-update-mobile" }
frozen: { at: 2026-07-12, commit: faf8d8c412c9df35b5a445146a5fe0e8309caa71 }
---
# A mobile update lands and reads back correctly, including after the client's own retry loop fires

The behavioral evidence must show a mobile client update reaching 200
with the new state under two conditions: a clean single request (the
happy path ac-1's text describes), and a request that fails transiently
and is retried by the client's own backoff loop
(deviation-report.md's accepted-deviation finding against this same ac-1)
— the retried request must land exactly once server-side, not create a
duplicate write, since the whole point of accepting the client-side retry
was that it stays behind the same idempotent contract the single-request
case already has. Evidence that only exercises the clean-request path
does not cover what was actually accepted as a deviation from ac-1's
literal text.
