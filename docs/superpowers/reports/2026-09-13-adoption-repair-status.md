# Adoption repair status — 2026-09-13

Status: **VERIFIED — bounded repair complete**. The directory failure, policy-guide findings,
and vocabulary failure are repaired. Both independent Opus 5 reviews returned
no blocking findings. This closes the bounded repair; it does not accept the
local MVP, adopt ATC policy, or complete either adoption journey.

## Verified candidate

- Repair base: `06591bf661ef8573e61ef5c4a5a06013c03b2a35`.
- Final source: `ce51bb9abadaf0087fe999f36a0bd3496d2e7edf`.
- Binary: `.build/bin/verdi`, built from that clean source with
  `CGO_ENABLED=0 GOPROXY=off GOSUMDB=off go build -trimpath -o .build/bin/verdi ./cmd/verdi`.
- Binary SHA-256:
  `aef8f1a0b57f34f0851e20d51197c1be345852b2a37d438ef2023e6fd053a3c5`.

Directory tree and ancestry queries now reuse `specstate.ResolveDefaultBranch`.
The permitted local fallback is unchanged; a resolved ref name is not an
immutable Git snapshot. The board supplies expandable, read-only policy setup
guidance. No policy authority, acceptance rule, route, or adoption operation was
added. The installed Verdi/ATC pair remains unchanged.

## Genuine Claude execution and adjudication

The FABLE 5.1 producer, session `72697c76-58ed-42d7-bd10-97c523b3482f`,
genuinely invoked the registered `/fable-orchestration` skill. Initial Opus 5
review session `50ce5fdb-2047-4824-ab3f-3d4e1e78c053` found three defects;
main accepted all three. Genuine FABLE 5.1 fixer session
`b43d0655-aa46-46a5-af45-78d4780bd093` supplied their corrections and the
subsequent vocabulary correction.

| Finding | Final correction |
| --- | --- |
| F1 / live P04 | Escape static filename placeholders locally so complete profile and policy paths render. Assert actual visible text. |
| F2 | Scope the missing-policy refusal to the serving checkout. Inspect accepted/proposed snapshots and reasons first; do not infer why a branch differs or equate resolution with acceptance. |
| F3 | Make both policy concern rows respect authoring, review, and read-only modes. |
| Vocabulary gate | Use human-editing wording for generic prose; classify the three actual Git-operation homographs at their producing literals. Preserve the vocabulary witness and its assertions. |

Earlier capacity and automatic-approval blocks are historical. After the owner
explicitly authorized the private source, local test results, and task context
to Anthropic, the bounded genuine Claude Code calls completed successfully:

- Opus 5 re-review: `0022924f-3167-46ea-9428-8ac569706305`, **NO BLOCKERS**.
- Independent Opus 5 whole-wave review:
  `61faa1dc-055a-48bd-81fc-e997bb7f5e1d`, **NO BLOCKERS**.

Main checked the model transcripts, eight-file source coverage and hashes
against the frozen source, accepted both verdicts, and adjudicated the optional
notes as non-blocking. Runtime/test authorship remained with FABLE 5.1; the
reviews contain Opus 5 assistant messages without delegated review. The first
review's CLI usage also reports ancillary Haiku usage; this is retained in the
raw provenance and does not represent alternate-model review messages.

The existing unresolved-default posture remains unproven. Read-only wording
covers ordinary editing without changing existing new-artifact exceptions.
Policy inspection still requires reading reasons and identities, and resolving
one prerequisite can expose the next blocker. These limits do not create
acceptance authority.

## Real ATC regression

The same-operator comparison used the real-project checkout
`/Users/johnyang/code/verdi-system/verdi-atc-mvp-adoption-recheck-20260913`,
branch `design/f13-receipt-verification`, HEAD
`35c119bfca313d355ee646dbf1f33a64c4667c83`.
`origin/HEAD` names `origin/main`, at
`5d08b6a40dc476f708fb9019e6a266c5de63eb6f`; there is no local `main`.

The preserved old binary failed with `fatal: Not a valid object name main`.
The final binary renders the F13 directory entry and opens its board without
creating a local alias or moving the default ref. The missing-policy notice
reaches the guide; both filename placeholders render in full and all four
quoted here-document commands expand correctly.

Those exact command blocks ran from the real project root with the final
binary. All exited 0 while reporting absent policy, unproven consumer coverage,
and `ready_for_submission: false`. Semantic review still reports
`policy-forbidden: project has not adopted policy authority`.

`verdi journey --json spec/f13-receipt-verification` reports a proposed feature,
no adopted profile, unavailable forge facts, and unproven author-vouch evidence.
`verdi matrix --preview --json spec/f13-receipt-verification` reports four
`no-signal` criteria, absent attestation, and no implementing stories.
Its `violated: false` is not completion proof.

The ATC working tree stayed clean, the default ref did not move, and no
`.verdi/data` file is tracked. The temporary server and browser tab were closed.
No screenshots, browser traces, or video were recorded. This is a development
regression check, not a complete or independent adoption journey.

## Local verification

The full gate ran on clean source `ce51bb9a` and exited **0**:

```sh
GOPROXY=off GOSUMDB=off \
HTTP_PROXY=http://127.0.0.1:9 HTTPS_PROXY=http://127.0.0.1:9 \
NO_PROXY=localhost,127.0.0.1,::1 VERDI_E2E_PORT_BASE=4400 make verify
```

```text
279 passed (13.7m)
verify OK
```

Build, formatting, vet, lint, `go test -race ./...`, the forced fresh
cross-binary race checks, fixture/store checks, specification alignment,
showcase checks, and the full browser suite passed. No data-race warning or
specification-alignment skip was reported. The log contains 94 existing
`disclosed-unproven` lint notices for unavailable mutable evidence; these
remain disclosures, not proof. The browser result directories contain no
screenshots, video, or trace archives.

The focused guide browser check also passed: **1 test, 7.8 seconds**, with no
recording artifacts. FABLE recorded vocabulary RED then GREEN and passing
focused guide/mode tests. The earlier candidate's full gate failed at eight
vocabulary sites and the dependent guide-claims witness; that failure remains
in the historical evidence and is superseded by the final run above.

## Remaining acceptance work

Actual ATC policy/tracker setup, review and adoption remain open. The bounded
F13 adapter proposal still needs its governing design/acceptance and implementation
path. Complete the meaningful-change journey, including supported editing,
alignment and truthful evidence inspection, then have an independent operator
complete a second journey on the same final release. Earlier runs on different
binaries and this same-operator regression do not satisfy that pair.

Hosted forge/CI integration testing remains deferred. Local execution is not
authoritative CI evidence. Any step requiring external proof remains incomplete
until that proof exists. Authoritative closure is not a blanket prerequisite
for the narrower define-and-inspect MVP. The guide alone does not complete
initial policy adoption or P02.

## Evidence

Workspace-local evidence is retained under
`/Users/johnyang/code/verdi-system/.local/verdi-system/development/mvp-readiness-20260913/adoption-repair/`:

- `authorized-vocab-gate-fable-result.json` and corresponding stream.
- `authorized-opus-fix-rereview.md` and `authorized-opus-whole-wave.md`, with
  their result/stream records.
- `authorized-whole-wave-source-manifest.json`, source patch, and
  `authorized-final-source-model-provenance.json`.
- `main/whole-wave-adjudication.json` and `main/review-and-provenance-checks.json`.
- `main/release-candidate-validation.json`: exact CLI output and browser/Git observations.
- `main/make-verify-vocabulary-fixed.log` and final result record.
- `main/focused-browser-vocabulary-fixed-result.json` and its log.

These are local development witnesses, not forge approvals, authoritative CI,
independent adoption evidence, or installed paired-release validation.
