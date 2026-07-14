---
id: attestation/true-closure--ac-4
kind: attestation
title: "AC-4 attested: a superseded spec's terminal state is legible at both levels, story and feature predecessors alike"
owners: [platform-team]
links:
  - { type: verifies, ref: spec/true-closure }
frozen: { at: 2026-07-13, commit: 6185f58a6d34ca38059c317576b1da4c5c87e3fe }
---
# AC-4 outcome attestation

Operator attests (round 6, 2026-07-13): a superseded spec's terminal state is
legible at both rungs, and finding it no longer requires reading raw frontmatter
or chasing a `superseded-by` backlink. The story `spec/feature-supersession-
state` (jira:VERDI-4) delivered it: at the **story** rung the existing D-12
`status → superseded` flip is now rendered on `verdi matrix` (a `status:` line
where there was none), on the board (a `superseded` head badge), and on dex (the
proven `.badge-superseded`); at the **feature** rung `verdi accept` now performs
the equivalent flip on a superseded feature predecessor (closing 02 §Kind
registry's round-6 deferral), and the same three surfaces render it — the
feature's own `status:` on `verdi matrix`, and the badge on board/dex.
Confirmed against the real superseded story `spec/disclosure-seam` and hermetic
superseded-feature fixtures; e2e proves the browser surfaces. The terminal state
is read from the spec's own status, on every surface that renders it.
