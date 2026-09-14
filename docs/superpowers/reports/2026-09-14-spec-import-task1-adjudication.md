# Mechanical import Task 1: owner findings and continuation

Status: **REVISE — implementation present, not accepted**. The parser package is
committed and its existing tests pass, but main reproduced contract defects.
Genuine Opus 5 began the independent runtime review and stopped on HTTP 429 before
returning a verdict. Do not start Task 2 or label the importer/release complete.
The adopted design and its CLOSED contract review remain unchanged.

## Fixed runtime range and provenance

Base: `424933d276e96494cfd726a47c78b8ae31713e2f`.
Runtime head: `c79ff64f214ee319c320728df0ba34655590288c`.
Producer commits: `e4f51891`, `51f9287e`, `c79ff64f`; 37 files, all under
`internal/specimport/` (12 production Go files, 8 test/helper files, 17 fixtures).
No CLI, workbench, candidate composer, publication service or installed binary
was changed. The old release binary is not an importer candidate.

`/fable-orchestration` expanded successfully in genuine Claude Code session
`e41a85db-41cd-45a2-8356-a7e31c91a59b`, model `claude-fable-5-1`.
Its genuine Agent child `afebeeb27e02d95df` authored the code as `claude-sonnet-5`.
The model IDs are present in actual assistant/stream records, not just prompts.
Genuine independent review session `be44d409-ddde-4956-8fa1-ac552126e2a4` used
`claude-opus-5`; it read the contract/report but returned **no review verdict**.
Both controller and reviewer ultimately exited 1 with session-limit HTTP 429.
The reported reset is 2026-09-14 13:00 America/New_York (17:00 UTC).

Local evidence root (workspace-relative):
`.local/verdi-system/development/spec-import-f13-20260914/execution/`.
`task1-resume-scoped/` contains the final controller/child stream and process
metadata; `task1-review/` contains the unfinished Opus review. The child's harness
refused a report-file write and required a text return; main preserved the actual
returned text in `task1/producer/report-received.md`. Controller command outputs
exist, but it did not complete its final pre-review report before the limit.

Earlier launch attempts are retained: missing skill registration, sandbox-only
authentication access and omitted evidence-directory access were corrected.
The final invocation declares the assigned evidence directory explicitly. No new
user authorization is needed for the already-approved Verdi/Anthropic scope.

## Verification observed at the runtime head

Main independently ran:

- `go test -race ./internal/specimport -count=1` — exit 0, package passed (2.637s).
- `go vet ./internal/specimport` — exit 0, no output.
- `gofmt -l internal/specimport` — exit 0, no output.
- `git diff --check 424933d2..c79ff64f -- 'internal/specimport/*.go'` — exit 0.

Full command results are in `owner-probes/task1-local-checks.json`. Producer and
controller also passed the focused/package commands and reported 105 top-level
tests. Pinned F13 fixtures retain the expected hashes; a full-plan fixture exists
only to test lines 602–671 against the extracted 2409-byte primary.

TDD evidence has limits: the producer initially wrote some implementation first,
acknowledged it, restored stubs, then captured a compile-failing RED before
restoring implementation. The later mapping/F13/native batch was written together
with its implementation, not strictly red-first. Do not claim otherwise.
A real trailing-newline loop and next-heading-boundary defect were fixed during
that pass. Main's isolated corrected-helper probe returned the expected LF, CRLF
and multi-line results. Those fixes do not close the findings below.

No integrated `make verify`, full-repository race gate, new binary build, browser
journey or independent adoption journey has been completed for this change.

## Main's reproduced findings (all open)

These are owner findings, not an Opus verdict. Reproduction source and exact-head
outputs are `owner-probes/task1-api-probes.go` and `task1-api-probes.json`.
The observation program's exit 0 means it ran; the printed results demonstrate
incorrect behavior and are not a passing assertion suite.

| ID | Finding and concrete incorrect result | Binding and required correction |
| --- | --- | --- |
| M1 | A user-added AC with `Text` and `Evidence:[attestation]` is rejected by `validateMappings`. A second evidence-only mapping cannot repair it because duplicate explicit targets are also rejected. Source-backed edits have the same restriction. | Contract's source-backed/user-added Mapping forms allow evidence on ACs; evidence-only is an additional form, not the exclusive home of evidence. Accept and preserve declared evidence on all AC mapping forms, while retaining existing enum/duplicate/non-AC checks. |
| M2 | A `list-item` mapping over ordinary prose succeeds as copied-source. Its implementation deliberately skips real list-span validation and continuation deindentation. | Contract requires an actual supported direct list-item span. Validate against the Markdown structure, refuse arbitrary/fenced/unsupported spans, and use the same declared transformation for automatic and explicit mapping. |
| M3 | A user-added mapping with target `ac-7` and no source span clears a source-ID blocker for an existing `ac-7:` item. | The accepted closure decision requires a mapping over the corresponding source item. Target equality alone is insufficient; bind resolution to that item and preserve neighboring ordinal behavior. |
| M4 | `Target.Slug = probe#ac-1` passes Validate because ParseRef accepts fragment refs. Pins need the same negative test. | Target.Slug is a bare spec name. Reuse the existing bare-name validation without accepting pins/fragments; every direct entry must reject it before path construction. |
| M5 | With a two-line source selecting only line 2, Snapshot.Data contains both lines while coverage counts only line 2. | Parent design requires retained selected bytes and original selection coordinates. Normalize Snapshot.Data to selected bytes, with Digest and spans in that same coordinate system; preserve OriginalDigest/ranges as metadata. Task 3 must distinguish the recorded original-input fingerprint from the selected bytes actually retained and verifiable in Git. |
| M6 | ReadSource returns empty and invalid UTF-8 data without error; a valid UTF-8 file named `需求.md` is refused because its derived ID is empty. | Source constraints require nonempty UTF-8 bounded input; filenames have no added ASCII-only rule. Reuse the source validation seam. Main's bounded choice for an unrepresentable basename is deterministic fallback ID `source`, keeping the original Label; duplicate IDs still fail Request validation. No new CLI flag is needed. |
| M7 | An ordered list is automatically promoted despite the bullet-list grammar. A nested Problem subheading also causes its body to be truncated to `p`, dropping `q` from the field with no finding once evidence is supplied. | Enforce the closed flat-bullet grammar. Preserve the entire statement section, including interior content, or explicitly block unsupported structure; silently truncating the recognized statement is not a supported transform. |

M1–M7 are Important within this Tier 3 lane: they affect supported authoring,
source interpretation, provenance or trusted target construction. They must be
corrected before the package is accepted as a dependency. Tests that currently
encode a narrower Mapping form or full-input Snapshot.Data must be corrected
against the contract, not retained as authority. The Opus review may add findings.

## Producer choices and next gate

Keep the specified DecodeExactJSON seam and top-level-null check; do not invent a
new nested-null ban. Wrapped error sentinels, explicit source-backed Transform,
unknown-source-ID rejection, and a whole-native mapped coverage interval are
compatible with this task. Compose owns actual statement deferral and candidate
requiredness; it must also reconcile resolved value gaps without erasing truthful
source disclosures. No acceptance or evidence rule is weakened.

The producer's evidence exclusivity, target-only ID resolution, and arbitrary
list-item transformation are rejected as contract deviations, not adopted as
new design decisions. Main adjudicates those against existing authority. The
fallback source ID above fills a narrow helper detail without changing request
identity validation or file-selection scope; this report and root I-127 record it.

When Claude is available: resume the same unfinished Opus 5 runtime review over
the fixed range (allow the subsequent report-only HEAD advance), consolidate its
verdict with these owner findings, then use a fresh genuine Opus 5 fixer and a
distinct Opus 5 re-reviewer for accepted Important defects. Preserve the fixed
review/fix ranges and add meaningful regressions. After acceptance, continue
Tasks 2–5 under the already-adopted plan. No third specification-review round or
new user design adoption is requested.

Hosted testing stays deferred. The paused independent journey does not count as
complete; local MVP acceptance still needs two complete journeys on the same new
release, with the second independent.
