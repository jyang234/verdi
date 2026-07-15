---
id: obligation/borrower-update-api--ac-1--behavioral
kind: obligation
title: "A submitted application actually updates end to end through the API route"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/borrower-update-api" }
frozen: { at: 2026-07-12, commit: 16219044c9d6d41de9a0de9464ed24d49283b40c }
---
# A submitted application actually updates end to end through the API route

The behavioral evidence must show a real submitted-state application
driven through `PUT /applications/:id/update` with a changed field, the
response asserted as HTTP 200 carrying the new state (not just a 2xx
status — the response body's new-state claim is part of ac-1's own text),
and the change confirmed durable by re-reading the application afterward
rather than trusting the mutation response alone. A test that only checks
the status code without confirming the read-back state does not satisfy
this obligation — matching status codes without matching state is exactly
the kind of evidence-shaped-but-hollow proof this obligation exists to
rule out.
