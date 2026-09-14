# Adoption repair status — 2026-09-13

Status: **REVISE; vocabulary gate failed, and automatic approval review blocks
further Claude calls.** FABLE capacity is restored. The directory repair and
three reviewed guide corrections are committed and locally exercised. This
candidate is not a verified release or an MVP acceptance.

## Current candidate

- Original repair base: `06591bf661ef8573e61ef5c4a5a06013c03b2a35`.
- Initial runtime repair: `f5bd42c975373066261c02d2ff8a59d3e5dd082c`.
- Corrected runtime candidate: `11831b632928f7428e139b96a69f6d53e53370f3`.
- Candidate binary SHA-256:
  `4e7a0165733b65053b2c755bda741443a4410f4cc058e1b9c0a148794852bb32`.

Directory reads now reuse `specstate.ResolveDefaultBranch` for both tree and
ancestry queries. Its permitted local fallback is unchanged; selecting a ref
name does not create an immutable Git snapshot. The existing board now has
expandable, read-only policy guidance. No policy authority, acceptance rule,
route, or adoption operation was added. The installed Verdi/ATC pair is unchanged.

## Genuine Claude execution and findings

The FABLE 5.1 producer session `72697c76-58ed-42d7-bd10-97c523b3482f`
genuinely invoked the registered `/fable-orchestration` skill. Independent
Opus 5 session `50ce5fdb-2047-4824-ab3f-3d4e1e78c053` reviewed the initial
candidate and returned three findings, all accepted by main:

| Finding | Correction in the current candidate |
| --- | --- |
| F1 / live P04 | Escape static filename placeholders so the browser shows complete profile and policy paths. Add actual visible-text assertions. |
| F2 | Scope the missing-policy refusal to the serving checkout. Inspect accepted and proposed snapshots and their reasons before choosing initial setup; do not infer why a branch differs or equate resolution with acceptance. |
| F3 | Make both policy concern rows respect authoring, review, and read-only modes. |

Fresh FABLE 5.1 fixer session `b43d0655-aa46-46a5-af45-78d4780bd093`
initially stopped on exhausted usage credits without edits. After the owner
reported restored capacity, the same genuine session completed the four-file
correction with semantic RED/GREEN evidence. Actual model records identify
`claude-fable-5-1` and the first-party provider. No substitute model authored
runtime code or tests.

A fresh Opus 5 re-review was then rejected **before execution** by automatic
approval review, which required explicit permission for the private source and
review context sent to Anthropic. A later, distinct FABLE vocabulary-gate
repair call was also rejected before execution for the same payload/destination
permission reason. Neither rejected transfer was retried or rerouted. The
independent re-review and whole-wave review have not run.

## Real ATC validation

The same-operator comparison used
`/Users/johnyang/code/verdi-system/verdi-atc-mvp-adoption-recheck-20260913`,
branch `design/f13-receipt-verification`, HEAD
`35c119bfca313d355ee646dbf1f33a64c4667c83`.
`origin/HEAD` names `origin/main`, at
`5d08b6a40dc476f708fb9019e6a266c5de63eb6f`; there is no local `main`.

The preserved old binary failed with `fatal: Not a valid object name main`.
The current binary renders the F13 directory entry and its board link without
creating a local alias or moving the default ref. The missing-policy notice
reaches the guide; both filename placeholders now render in full, and its four
complete quoted here-document commands expand correctly.

Those exact command blocks ran from the real project root. All exited 0 while
reporting absent policy, unproven consumer coverage, and
`ready_for_submission: false`. Semantic review still reports
`policy-forbidden: project has not adopted policy authority`.

`verdi journey --json spec/f13-receipt-verification` reports a proposed feature,
no adopted profile, unavailable forge facts, and unproven author-vouch evidence.
`verdi matrix --preview --json spec/f13-receipt-verification` reports four
`no-signal` criteria, each with absent attestation and no implementing stories.
Its `violated: false` is not proof of completion.

The ATC working tree remains clean, the default ref is unchanged, and no
`.verdi/data` file is tracked. The temporary server and browser tab were closed.
No screenshots, browser traces, or video were recorded. This is a development
regression check, not an independent adoption journey.

## Verification and remaining work

Focused Go tests for the guide, concern modes, and design shell passed. The
corrected Playwright guide test passed: **1 test, 11.3 seconds**, with an empty
recording-artifact scan.

`make verify` on the current candidate exited **2**. Build, formatting, vet,
and lint passed; 91 Go packages reported success. No data race was reported.
The test phase failed at `TestVocabProseWitness`: eight new production text
sites use bare vocabulary words (`draft` or `merge`) without the required
display resolution or justified identity/homograph classification. The
`TestGuideClaimsManifest_RowToWitnessBinding` failure is its dependent witness.
The later complete-gate stages, including the full browser suite, did not run.

Resume after explicit authorization for the bounded private Verdi source,
test results, and task/review context to Anthropic through genuine Claude Code:

1. FABLE 5.1 repairs the eight vocabulary sites. Preserve the witness and its
   assertions; classify actual Git operations accurately and keep human editing
   guidance clear. Run the failing witness and focused guide tests.
2. Freeze the corrected candidate and run its focused browser check and full
   `make verify`, recording the final binary identity and any material retest.
3. Obtain fresh independent Opus 5 re-review and bounded whole-wave review;
   main adjudicates the results. Neither review has occurred yet.

Actual policy adoption, the complete meaningful-change journey, and the
independent second journey remain open. Both complete journeys must use the
same final verified release. Hosted integration testing remains deferred;
a step requiring external proof stays incomplete. A guide does not close
initial policy adoption or P02 by itself.

Evidence is retained under
`/Users/johnyang/code/verdi-system/.local/verdi-system/development/mvp-readiness-20260913/adoption-repair/`:
`f2-precision-fable-result.json`, `f2-precision-command-evidence.json`,
`opus-fix-rereview-approval-block.json`, `vocab-gate-fable-approval-block.json`,
and `main/` browser observations, CLI results, focused test results,
`make-verify-fixed.log`, and `make-verify-fixed-result.json`.
