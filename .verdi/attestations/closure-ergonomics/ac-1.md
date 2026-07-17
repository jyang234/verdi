---
id: attestation/closure-ergonomics--ac-1
kind: attestation
title: "AC-1 attested: a non-mutating closure preflight discloses every condition a real close would refuse on, before any close is attempted"
owners: [platform-team]
links:
  - { type: verifies, ref: spec/closure-ergonomics }
frozen: { at: 2026-07-17, commit: b32afdb39c1474e2c8b79f0af664fa28752d7824 }
---
I reviewed the preflight outcome across the family: every closure condition close would refuse on is surfaced pre-close with its path — operatively proven in Phase 4 itself, where the work lists for both families came from nine preflight runs, not from failed closes; co-3's property (a failed close is never the first disclosure) held in anger across every run. One disclosed limitation (D6-37, adjudicated ADJ-66): an AC declared but not yet bound in verdi.bindings.yaml is surfaced as 'no current passing record' rather than naming the missing binding as the exact artifact — the condition is disclosed pre-close, but the root artifact for the unbound-AC edge is named coarsely, because the story contract that built the preflight narrowed the feature's 'unbound ACs' enumeration; recorded here rather than papered over. The AC holds in its operative property (co-3), with that granularity refinement filed as a follow-up.
