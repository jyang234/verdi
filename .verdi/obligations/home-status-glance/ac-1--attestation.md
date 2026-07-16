---
id: obligation/home-status-glance--ac-1--attestation
kind: obligation
title: "An operator confirms the glance reads correctly against a real multi-status store"
owners: [platform-team]
for_kind: attestation
links:
  - { type: verifies, ref: "spec/home-status-glance" }
frozen: { at: 2026-07-16, commit: d11cd50bf4840109ef8834b16e97a1920805c178 }
---
# An operator confirms the glance reads correctly against a real multi-status store

The attestation must record a named operator's affirmation, against a real
checkout carrying at least one spec of each status this store's schema
legalizes (not solely the fixture store), that `GET /` leads with the
three-bucket glance above the existing Directory section, that every
entry's badge matches the spec's own real status, and that clicking each
rendered link (board; matrix/verdict where present) opens the real page it
names — never a 404, never a mismatched target. The attestation must name
the checkout and at least one real spec per populated bucket; a generic
"it works" statement without these specifics does not satisfy this
obligation.
