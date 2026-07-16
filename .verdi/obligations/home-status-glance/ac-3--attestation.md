---
id: obligation/home-status-glance--ac-3--attestation
kind: obligation
title: "An operator confirms an empty bucket reads as an honest fact, not a broken page"
owners: [platform-team]
for_kind: attestation
links:
  - { type: verifies, ref: "spec/home-status-glance" }
frozen: { at: 2026-07-16, commit: d11cd50bf4840109ef8834b16e97a1920805c178 }
---
# An operator confirms an empty bucket reads as an honest fact, not a broken page

The attestation must record a named operator's affirmation, against a real
checkout genuinely lacking any spec in one of the three buckets, that the
corresponding glance bucket renders visibly with its zero count and
empty-state text, and that the operator did not need to view source or
consult the store on disk to confirm the absence was real rather than a
rendering failure.
