---
id: obligation/evidence-slot--ac-1--static
kind: obligation
title: "Emptiness is the fold's own per-kind no-record state"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/evidence-slot" }
frozen: { at: 2026-07-14, commit: c464efb6133e449257c48738ab66ae93a0e071e5 }
---
# Emptiness is the fold's own per-kind no-record state

The static evidence must show the slot's emptiness computed from the
evidence package's own seams: records loaded by the fold's loader
(internal/evidence's derived-tree record loading), reduced through
evidence.Current, per-kind presence taken from the fold's no-record state,
and attestation-kind emptiness from AttestationExists — with NO wall-local
record parsing, no private latest-per-identity reduction, and no
duplicated derived-tree walking anywhere under internal/workbench (co-3).
