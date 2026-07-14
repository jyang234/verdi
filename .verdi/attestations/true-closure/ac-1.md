---
id: attestation/true-closure--ac-1
kind: attestation
title: "AC-1 attested: a merged story reaches a true, archived closure — quartet and all — on authoritative CI-produced evidence alone"
owners: [platform-team]
links:
  - { type: verifies, ref: spec/true-closure }
frozen: { at: 2026-07-13, commit: 6185f58a6d34ca38059c317576b1da4c5c87e3fe }
---
# AC-1 outcome attestation

Operator attests (round 6, 2026-07-13): four merged verdi stories have each
reached a true, archived closure — the frozen quartet, `status: closed`,
`specs/archive/<name>/` — on authoritative `source: ci` evidence alone.
`spec/remote-and-ci` (jira:VERDI-1), `spec/close-verb` (jira:VERDI-2),
`spec/runtime-evidence` (jira:VERDI-3), and `spec/feature-supersession-state`
(jira:VERDI-4) all live in `specs/archive/` with `status: closed`, each closed
by `verdi close` after its closure gate passed 3/3 on `source: ci` records
fetched from the repo's own GitHub Actions `verify` runs via `verdi sync` — the
`verdi-evidence`/`verify` CI convention, never local regeneration (a
`--force-local` run stamps `source: local` and the closure gate refuses it).
Every archive re-lints clean (`verdi lint` + `make spec-align`). The central
promise — that what lands is in alignment with what was agreed — is proven past
the merge gate, end to end, for the first time.
