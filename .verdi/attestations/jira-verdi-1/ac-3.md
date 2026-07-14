---
id: attestation/jira-verdi-1--ac-3
kind: attestation
title: "AC-3 attested: verdi sync fetches the authoritative bundle by (ref, commit); the fold consumes only source:ci"
owners: [platform-team]
links:
  - { type: verifies, ref: spec/remote-and-ci }
frozen: { at: 2026-07-13, commit: 79d9e4ce6b3dc978809f3582aa1074c5e485aa20 }
---
# AC-3 outcome attestation

Operator attests (round 6 real-remote proof, 2026-07-13): `verdi sync`
fetched the authoritative evidence bundle for the current ref by (ref,
commit) through the forge port from the real remote, and the fold consumed
only `source: ci` records — `verdi matrix spec/remote-and-ci` shows ac-1
evidenced (static:pass; behavioral:pass) from those CI-produced records, with
no local or advisory record load-bearing. This is the first gate in the
system's history to consume forge-fetched CI evidence (round6-divergences
D6-VICTORY).
