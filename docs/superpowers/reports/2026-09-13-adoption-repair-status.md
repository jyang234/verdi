# Adoption repair status — 2026-09-13

Status: **REVISE; required FABLE repair blocked by Claude Code usage credits.**
The directory repair is implemented and reproduced successfully in the real
ATC checkout. The policy guide is implemented but has three accepted review
findings. This candidate is not a verified release or an MVP acceptance.

## Exact candidate and workflow

- Base: `06591bf661ef8573e61ef5c4a5a06013c03b2a35`.
- Runtime candidate: `f5bd42c975373066261c02d2ff8a59d3e5dd082c`.
- Candidate binary SHA-256:
  `0e1412c96fd1cfd6c70a5122a75b10be3dda16edaf70c09e81284786eb0bdef9`.
- Genuine Claude Code FABLE 5.1 producer session:
  `72697c76-58ed-42d7-bd10-97c523b3482f`; the registered
  `/fable-orchestration` invocation is recorded.
- Independent genuine Opus 5 lane review session:
  `50ce5fdb-2047-4824-ab3f-3d4e1e78c053`; verdict **REVISE**.
- Fresh FABLE 5.1 fixer session:
  `b43d0655-aa46-46a5-af45-78d4780bd093`; Claude Code reported exhausted
  usage credits before implementation. No substitute model was used.

The changes reuse `specstate.ResolveDefaultBranch` for directory tree and
ancestry reads, and add an inline, read-only policy guide to the existing
board. The shared resolver's permitted local fallback remains unchanged.
Selecting a resolvable ref name does not create an immutable Git snapshot.
No policy authority, acceptance rule, route, or adoption operation was added.

## Real-project comparison

The comparison used a fresh local clone at
`/Users/johnyang/code/verdi-system/verdi-atc-mvp-adoption-recheck-20260913`,
on `design/f13-receipt-verification`, HEAD
`35c119bfca313d355ee646dbf1f33a64c4667c83`.
Its genuine `origin/HEAD` names `origin/main`, at
`5d08b6a40dc476f708fb9019e6a266c5de63eb6f`; there is no local `main`.

The preserved old binary failed to render the directory with
`fatal: Not a valid object name main`. The new candidate rendered the F13
draft and its working board link in the same checkout. No local alias or
default-ref movement was needed. The policy notice reached the expandable
guide, and its CLI commands were visible. This was a development-agent
regression check, not an independent adoption journey.

The four real policy inspections all exited 0 while reporting missing policy,
unproven consumer coverage, and `ready_for_submission: false`. Semantic review
still returned `policy-forbidden: project has not adopted policy authority`.
The ATC checkout remained clean, with no tracked `.verdi/data` files. The
installed Verdi/ATC pair was not changed. No screenshots, traces, or video
were recorded.

## Accepted findings still requiring repair

| Finding | Required correction |
| --- | --- |
| F1 / live P04 | Escape the static filename labels: the browser currently drops `<profile-id>` and `<name>`, showing `profiles/.md` and `policies/.md`. Assert the complete visible paths. |
| F2 | Scope missing-policy statements to the serving checkout. That refusal does not prove absence on the default branch. Guide users to inspect accepted and proposed snapshots before choosing initial setup or updating an older branch. |
| F3 | Make policy concern rows respect authoring, review, and read-only modes. Do not claim that editing proceeds on a wall that refuses edits. |

Main accepted all three findings. F1 was independently reproduced through
browser accessibility and visible DOM text. F2 follows the production
`ConstitutionPolicySource`, which loads the serving checkout filesystem.
F3 concerns a new unconditional editing statement in the guide's notice row.
The frontend owner exception requires genuine FABLE 5.1 for these repairs.

## Verification and resumption

Focused refindex and workbench tests passed after their recorded behavioral
RED results. The focused Playwright guide test passed: **1 test, 11.2 seconds**;
its recording-artifact scan was empty. That test missed the rendered filename
defect and must be strengthened.

`make verify` passed build, formatting, vet, and lint. Main deliberately
interrupted it during race tests, exit 130, before authorizing reviewed source
fixes; race and the complete gate remain **unproven** for this candidate.
No source changes occurred after that interruption because the fixer then
hit the usage-credit limit.

Resume from the runtime candidate above when genuine FABLE 5.1 capacity is
available: repair F1–F3 with focused RED/GREEN and browser checks; obtain a
fresh independent Opus 5 re-review and the bounded whole-wave review; run
`make verify` against the fixed, unchanged candidate; repeat the real-ATC
browser comparison and record the final binary identity.

Actual policy adoption, the complete real-project journey, and the independent
second journey remain open. Both complete journeys must use the same final
verified release. Hosted integration testing remains deferred; a step requiring
actual external proof stays incomplete. The guide does not close P02 by itself.

Evidence is retained under
`/Users/johnyang/code/verdi-system/.local/verdi-system/development/mvp-readiness-20260913/adoption-repair/`,
including model/session records, the fixed-range review, and the `main/`
browser observations, CLI results, adjudication, and interrupted gate log.
