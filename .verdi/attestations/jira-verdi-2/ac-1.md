---
id: attestation/jira-verdi-2--ac-1
kind: attestation
title: "AC-1 attested: verdi close folds source:ci only, verifies eligibility, freezes align, generates a valid rollup, and archives the frozen quartet — proven on real merged stories"
owners: [platform-team]
links:
  - { type: verifies, ref: spec/close-verb }
frozen: { at: 2026-07-13, commit: 9fce5b8cced19879330daa1009fd29cf628a5db2 }
---
# AC-1 outcome attestation

Operator attests (round 6, 2026-07-13): `verdi close <story>` was run end to
end against **two** real merged verdi stories and produced a valid archived
closure each time. For `spec/remote-and-ci` (jira:VERDI-1) and
`spec/runtime-evidence` (jira:VERDI-3): the closure gate passed 3/3 (story
eligible on authoritative `source: ci` evidence fetched via `verdi sync`, no
spec-stale flag, no pending-supersession flag); `align --freeze` wrote the
frozen deviation report at head; a valid `rollup.json` was generated; and the
frozen quartet (spec, rollup.json, deviation-report.md) was moved
active → `specs/archive/<name>/` with `status: closed`. Both archives re-lint
clean (`verdi lint` + `make spec-align`) — the D6-20 discipline: the verb's
output is validated, not just produced. Only `source: ci` records were folded;
no local record was load-bearing (a `--force-local` run stamps `source: local`
and is refused by the closure gate). The ritual works exactly as ac-1 requires.
