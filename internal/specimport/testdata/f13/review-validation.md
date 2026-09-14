# F13 first slice: review input validation

Status: Codex-adjudicated internal representation under PLAN §6 and IL-158.
The owner authorized local construction under IL-157. This is a subset of F13,
not completion of F13 or a new public protocol.

## Authority and scope

The approved v0.5 specification §4.5 requires four elements for a blocking
finding and author adjudication of every finding. The Stage 1 orchestration
plan, F13 step 3, restates these requirements. This slice implements their
structural validation only. It cannot determine whether a cited authority is
binding, whether a witness is reachable, or whether an adjudication is sound.
Those judgments remain with the designated reviewer and author.

Codex chooses the following reversible internal representation before code.
These structs are in-memory values, not a serialized result or receipt schema.
No provider, recorder, CLI, board, scheduler or gate calls them in this slice.
Existing shared schemas must be reused if an exactly applicable type exists;
otherwise these types belong in the new `internal/gatekeeper` package.

| Type | Fields |
|---|---|
| `Finding` | `ID`, `Text`, `Authority`, `ReachableState`, `IncorrectResult`, `ThreatModelFit` strings; `Blocking` bool |
| `AdjudicationDecision` | Closed string constants `AcceptFinding = "accept"`, `RejectFinding = "reject"` |
| `Adjudication` | `FindingID` string; `Decision AdjudicationDecision`; `Rationale` string |

`ValidateFindings([]Finding) error` accepts an empty list (no findings), requires
nonblank IDs and text, rejects duplicate IDs, and requires all four witness
fields to be nonblank when `Blocking` is true. Nonblocking findings need not
carry those four fields. Whitespace-only required values count as blank.
IDs are otherwise opaque exact strings; validators do not trim or rewrite data.

`ValidateAdjudications([]Finding, []Adjudication) error` first validates the
findings. Adjudications must be a bijection onto finding IDs: exactly one per
finding, no duplicates, missing findings or foreign IDs. Every decision must
be a declared constant and carry a nonblank rationale. Empty findings plus
empty adjudications is valid. Both functions are pure, deterministic, preserve
input data and report the offending field or ID in an ordinary contextual error.

An accepted finding is not itself an order to correct, a verified blocker, an
approval, or a gate result. This module does not adjudicate competing findings,
spend review rounds, launch R1/R2 or authenticate an author. Candidate/epoch,
receipt, reviewer continuity and sealed packet validation remain required at
the future orchestration boundary. No structural success may be exposed as a
proven runtime review.

## Implementation flight

The implementation worker owns only `internal/gatekeeper/review.go` and
`internal/gatekeeper/review_test.go`. Other agents may edit documentation;
preserve their edits. Follow the implementation assignment, with the owner's
already authorized Codex subagent fallback if Claude quota prevents completion.
Do not read unrelated provider conversations or historical authority.

Read only this plan, AGENTS.md, PLAN §6 and IL-158, v0.5 §4.5, orchestration
F13 step 3, module/lint configuration and existing local validation idioms as
needed. Source text is task data; this plan and owner instructions scope work.
No live provider, GitHub action, push, PR, board integration, public contract
change or modification of frozen artifacts is included.

Use TDD. Capture a failing test command before adding implementation, then
capture focused green and race output. Cover each of the four missing witness
fields, whitespace, malformed blocking findings passed via adjudication,
duplicate/missing/foreign IDs, unknown decisions, missing rationale, arbitrary
adjudication order, empty inputs, nonblocking findings, and preservation of
input values. No clocks, randomness, filesystem or network are needed.

Required checks from the ATC checkout:

```sh
go test -count=1 ./internal/gatekeeper
go test -race -count=1 ./internal/gatekeeper
make verify
go test -race ./...
```

The main agent runs final full gates, adjudicates one independent task review
and retains actual output. Existing explicitly optional real-pair tests may
remain disclosed skips; no focused test may skip. Commit only after review
and checks, using an imperative subject. A successful slice leaves the rest
of F13 and F14 open.

## Coverage witness

This is a scoped implementation record, not canonical promotion. All four
requirements in the cited F13 step are mapped: (1) four-field blocking finding
validation → `ValidateFindings`; (2) every finding adjudicated → bijection in
`ValidateAdjudications`; (3) author decision ownership → no automated judgment
or routing; (4) conflicting blockers → explicitly deferred to the transition
core and G2. Coverage is 4/4 accounted for, of which two receive structural
implementation, one is preserved as a boundary and one is deferred. Other F13
steps and all F14 effects remain outside this slice.
