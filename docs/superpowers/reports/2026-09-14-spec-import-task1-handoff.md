# Mechanical spec import Task 1 — accepted parser dependency

Status: **ACCEPTED by main**, ready for FABLE integration and Task 2.
Runtime head: `1d6c16dff59b9686a70f185fc84dac3e0fc9bf9d`.
Original implementation base: `424933d276e96494cfd726a47c78b8ae31713e2f`.
The commit containing this report changes documentation only.

## Delivered and frozen for consumers

`internal/specimport` owns the contract Request/Target/Source/Mapping/Link,
Plan/Snapshot/Field/Span/Coverage/Finding shapes, strict DecodeRequest,
Request.Validate, Normalize and ReadSource. No other package imports it yet.
No Compose, publication/record service, CLI or import UI is delivered.

- ReadSource/Source.Data contains the full input; Snapshot.Data contains only the
  selected bytes. Digest hashes Snapshot.Data; OriginalDigest/ranges record the
  original-input fingerprint and selection. Retained selected bytes, spans and
  coverage share coordinates. Task 3 must not claim original full-file verification
  from only a selected sidecar.
- Native, markdown-v1, manual-v1 and pinned f13-reference-v1 remain closed formats.
  Mechanical labeled statements, exact list-item deindent, source-ID occurrence
  blockers, explicit corrections/evidence and complete byte partitions are tested.
  Empty source items remain blocking until explicitly supplied, with no fake text.
- Explicit mappings may replace the same-target automatic field. The displaced
  source remains retained under RetainUnmapped; preview must expose actual fields
  and coverage. No inferred AI content, evidence or authority exists.
- Resolved explicit nonblank statement/value gaps remain truthful nonblocking
  source disclosures. Deferral, canonical candidate requiredness and model/lint
  checks remain Task 2's responsibility. No other finding may be blanket-cleared.

## Provenance and review

Actual genuine Claude Code sessions/models, recorded in local stream metadata:

| Role | Session / child | Actual model |
| --- | --- | --- |
| FABLE controller; actual /fable-orchestration invocation | e41a85db-41cd-45a2-8356-a7e31c91a59b | claude-fable-5-1 |
| Task 1 producer child | afebeeb27e02d95df | claude-sonnet-5 |
| Initial runtime reviewer | be44d409-ddde-4956-8fa1-ac552126e2a4 | claude-opus-5 |
| M1–M10 fixer | 175b0942-f7f9-4aac-a623-d144c98e6a88 | claude-opus-5 |
| M1–M10 closure / adjacent reviewer | 2a55e03a-2c65-4147-89cc-af59ce8819d4 | claude-opus-5 |
| N1/N2 fixer (including owner boundary follow-up) | 168e49d8-5ec8-4f58-bec1-c729fdb302bb | claude-opus-5 |
| N1/N2/N3 closure | 16013938-9aac-4a56-9495-729ee5b8354e | claude-opus-5 |
| N4 fixer | e251881c-094f-4bbf-9765-b266a62110d7 | claude-opus-5 |
| N4 closure | a154acd1-79b8-41bf-8e7c-280977dc7d1d | claude-opus-5 |

All runtime changes are confined to internal/specimport. Report-only commits
separate owner adjudications; no reviewed history was amended. Detailed ruling,
fixed ranges and meaningful probes are preserved in the sibling Task 1
adjudication report; no new design adoption occurred during runtime corrections.

## Verification and limits

At 1d6c16df: `go test ./internal/specimport -count=1` passed (independent review,
0.485s); `go vet ./internal/specimport` and `gofmt -l internal/specimport/` clean.
There are 129 top-level package tests. Focused N4 has 13 subtests; independent
multi-empty/correction probes pass. Latest package race at 714b16ae passed 2.576s;
its successor N4 adds only a pure diagnostic and was package-verified. Main race
at 71d20017 passed 2.596s; key owner observations were rerun after corrections.
No full repository or browser gate is claimed here.

Pinned F13 fixture bytes remain unchanged: primary SHA-256
`7c6d95d4aa516cf682e6d4a33d1c260f1c861d938d46559241a7c764bffca577`, eight selectors,
2409 total / 642 mapped / 1767 retained bytes, 17 intervals (8 mapped). Full stage
plan selection has identical selected digest. These are byte-accounting claims,
not canonical acceptance/evidence or semantic-completeness proof.

Residuals: original producer was only partly red-first; later defect corrections
captured real RED/GREEN. Multiple empty items work and have independent overlay
proof but no committed same-section multi-empty regression. Duplicate empty-field
wording is truthful and nonblocking. Tab/surplus indentation stays preserved under
the declared ASCII-space deindent. Personal Claude auto-memory is not authority.

## Next authorized action

FABLE verifies this exact accepted runtime plus report-only child, then dispatches
one genuine Sonnet Task 2 lane for Compose/shared candidate lint and the narrow
creation-only splice helper if needed. Follow the adopted implementation plan;
no Task 3 starts until Task 2's fixed-range review closes. No source/profile or
specification review is reopened. Preserve every acceptance/evidence requirement.
Hosted testing, installation replacement and user ATC checkout writes stay deferred.
The local MVP still requires two complete journeys on the same new release, the
second independently run by the user.
