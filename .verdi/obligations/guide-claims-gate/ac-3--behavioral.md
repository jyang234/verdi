---
id: obligation/guide-claims-gate--ac-3--behavioral
kind: obligation
title: "a PARTIAL row without caveat reds; a non-EXISTS row or downgrade without cite: reds; cite: presence gates in CI, resolution checks workspace-side with a loud skip"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/guide-claims-gate" }
frozen: { at: 2026-07-20, commit: 1b0976c1039e0aa95e2be207dad8256b6d3b509e }
---
# a PARTIAL row without caveat reds; a non-EXISTS row or downgrade without cite: reds; cite: presence gates in CI, resolution checks workspace-side with a loud skip

The behavioral evidence must show four cases in
`guideclaims_test.go`. First: a `PARTIAL` row with no caveat text reds.
Second: a non-`EXISTS` row (`PARTIAL` or `INVENTED`) with no `cite:`
field reds. Third: a row whose status is a DOWNGRADE from a prior
recorded value (e.g. a fixture pair simulating an `EXISTS` row flipping
to `PARTIAL` across two manifest versions) with no `cite:` reds — proving
the downgrade case is caught independently of the plain non-EXISTS case,
closing the red-condition asymmetry the design wave's refuters named (a
gate that only checks EXISTS-row completeness would make downgrading the
cheapest path to green). Fourth: `cite:`'s two-tier check — presence is
asserted in a CI-simulated environment (asserting the finding reds when
absent, regardless of whether a chronicle is reachable), while
RESOLUTION (does the cited entry actually exist) is asserted only in a
workspace-simulated environment with a chronicle path available, and a
separate case with the chronicle path UNAVAILABLE must show a loud,
visibly-logged skip rather than a silent pass. Green in CI's test step,
as part of `make verify`.
