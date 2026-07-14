---
id: attestation/evidence-obligations--ac-1
kind: attestation
title: "AC-1 attested: an evidence obligation is a first-class artifact, graduated and frozen"
owners: [platform-team]
links:
  - { type: verifies, ref: spec/evidence-obligations }
frozen: { at: 2026-07-14, commit: 658e6ce3bb078692da8c638e1e5ace8d2936e127 }
---
# AC-1 attested: an evidence obligation is a first-class artifact, graduated and frozen

Operator attests (2026-07-14): `kind: obligation` is a real decodable artifact (internal/artifact/obligation.go), graduated from a board sticky (e2e 35-board-obligation-graduate), frozen at accept, carrying a `verifies` edge to its story spec — id `obligation/<story>--<ac>--<for-kind>`. Twelve real obligations for the feature's own three stories live at .verdi/obligations/, each decoding through the artifact seam. AC-1 satisfied.
