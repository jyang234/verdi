# Mechanical import Task 1: owner findings and continuation

Status: **ACCEPTED as the Task 2 parser dependency at `1d6c16df`**.
M1–M10, N1/N2 and N4 are closed by distinct genuine Opus 5 re-reviews; N3's
same-target override is adjudicated and tested. Main independently reproduced the
corrected key paths. FABLE integration verifies containment and consumes this exact
accepted head before Task 2. This is no importer, release or MVP completion claim.

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

## Resumed independent review and owner adjudication

The same reviewer session `be44d409-ddde-4956-8fa1-ac552126e2a4` resumed as actual
`claude-opus-5` and completed with REVISE, exit 0, at the unchanged runtime range.
Full returned review: local `execution/task1-review-probes/review-received.md`.
The reviewer independently reproduced M1–M7, ran 11 additional observational
probes through a Go overlay, and passed package tests (0.368s) and vet. The probe
PASS means observations ran, not that the demonstrated behavior is correct.
No tracked source was changed. Shell temporary-file permission failures were
stopped; the same reviewer used the ordinary Write tool in its explicitly assigned
evidence directory. The completed invocation had zero permission denials.

Main adjudicates C2/I1/I3–I7 as duplicates/extensions of M3/M1/M2/M7/M5/M4/M6.
These do not create additional fix rounds. C1 and C2 carry the reviewer's Critical
classification; the lane already uses the full Tier 3 reviewer/fixer/re-reviewer
chain. Main accepts three additional concrete normalization defects:

| Owner ID | Review ID | Accepted correction |
| --- | --- | --- |
| M8 | C1 | With a thematic break first in Problem, leafStart returns zero and copies the title and field heading into the statement. Compute the exact section body from real heading boundaries, preserving its interior bytes; pin thematic-break and other unsegmented-first-block cases. The suggested symmetric end-boundary concern is root-cause coverage, not a separately proven defect. |
| M9 | I2 | Explicitly mapped nonblank statements still carry blocking missing/ambiguous findings from automatic extraction. Reconcile resolved field-value gaps in Normalize while preserving truthful source-structure disclosures as nonblocking where the explicit mapping actually resolves that target. Deferral remains Compose's responsibility. Never blanket-clear multiple-target, unsupported structure, source-ID, object-section ambiguity or unmapped-source findings. |
| M10 | I8 | Explicit empty/blank source mappings can yield empty fields and no findings. Emit target-specific blocking empty-field findings for every empty final mapped field, including statements and objects; do not mistake a present target for a resolved value. Task 2 still performs all canonical requiredness checks. |

Clarifications against reviewer suggestions: `Source.Data` and ReadSource MUST
remain the full supplied file so original digest/range validation is possible.
Only `Snapshot.Data` becomes the selected bytes (M5); do not change the correct
ReadSource no-preslicing test. Snapshot tests/comments encoding full-file retention
must change. Evidence origin can remain derivable as user-selected from the closed
request; no new field is authorized. Explicit override demotion to retained-only
is compatible with the user's RetainUnmapped disposition. Concurrent external file
rewrite hardening is not a new acceptance gate; ordinary bounded reads and shared
validation remain within M6. Native validation/deferral ownership stays in Task 2.
The empty-list-item ordinal observation adds no new ordinal rule, but existing
requirements to report empty objects remain binding.

Next: fresh genuine Opus 5 fixer owns only internal/specimport corrections and
regressions M1–M10, then a distinct genuine Opus 5 re-reviewer. Main/FABLE verify
the resulting dependency before Task 2. No new design or user adoption is needed.

## First correction and distinct re-review

Fresh genuine Opus 5 fixer session `175b0942-f7f9-4aac-a623-d144c98e6a88` produced
10 commits `0d55ea45..71d2001796f356a54bc9310e4a20e6f53bc6e98b`, contained to 12
files under internal/specimport (+1339/-161). Actual assistant records identify
`claude-opus-5`. Main verified clean tree and diff-check, independently passed
`go test -race ./internal/specimport -count=1` (2.596s), and reran the seven owner
probes: every original bad result now has the intended result. Evidence is in
`execution/owner-probes/task1-fixed-local-checks.json`. Fixer final package/vet/fmt/
race gates also passed. Its report's claim of no permission denials is inaccurate:
compound/unallowed commands were refused and ordinary allowed commands succeeded.
It also wrote personal Claude auto-memory; that material is not authority or
verification and was excluded from the subsequent review.

Distinct genuine Opus 5 re-review session `2a55e03a-2c65-4147-89cc-af59ce8819d4`
closed M1–M10 individually at that exact head. Full verdict and overlay probes:
local `execution/task1-rereview/review-received.md`. Its package run passed 122
top-level tests, F13 pins were unchanged, and tracked files remained read-only.
The reviewer nevertheless returned REVISE on adjacent probes. Main adjudicates:

- **N1 — accepted Important.** A loose flat bullet item with paragraphs `alpha`
  and `beta` becomes `alphabeta`; fenced and quoted blocks also lose syntax and
  closing-fence bytes. The shared list-item transform must preserve paragraph
  separators and every interior byte except the declared bullet/deindent transform.
  Retain the complete item span. Preserve supported flat multi-paragraph items;
  nested lists remain unsupported. Automatic and explicit transformations must
  agree. Do not fix this by silently narrowing ordinary flat-list input or by
  collapsing paragraphs. This is completion of existing source fidelity, not a
  new input format.
- **N2 — accepted Important.** Two source items both declare ac-7; mapping the
  first removes both source-ID blockers because resolution is keyed by target.
  Resolve the corresponding occurrence only. The second declaration must stay
  explicitly blocking with correction guidance; duplicate explicit Mapping targets
  remain invalid. No auto-renumbering or new conflict-resolution UI is authorized.
- **N3 — nonblocking under the explicit override contract.** The source's first
  item declares ac-3; its third ordinary item automatically receives ordinal ac-3.
  An explicit ac-3 mapping over the first item replaces the automatic ac-3 field.
  The contract explicitly says text/span Mappings override automatic mappings for
  the same target; residual 4 also preserves positional numbering. This request
  exercises that authorized replacement. Its origin/span identify the selected
  first item, and the displaced item's bytes remain covered as retained-only under
  the user's RetainUnmapped selection. This is the same override demotion accepted
  in the initial review's nonblocking note, not loss of retained source or an
  accepted canonical criterion. Do not invent a namespace gate, auto-renumbering,
  or a second mapping shape here. Add a regression pinning explicit override,
  preserved source bytes, truthful spans and balanced coverage so the limitation
  remains reviewable. Later preview must show the actual fields and coverage.

Duplicate empty-field wording and unusual marker-inclusive selection variants are
nonblocking. Preserve the already accepted content-span path and source honesty.
Next correction is only N1/N2 plus the N3 contract regression, through a fresh
Opus 5 fixer and distinct re-review. No additional design/adoption is needed.

## Adjacent correction accepted; empty-item diagnostic

Fresh genuine Opus 5 fixer `168e49d8-5ec8-4f58-bec1-c729fdb302bb` produced
`8aabcb18..714b16ae` (four commits). Main caught an incomplete first-block boundary
at intermediate f7de2285; the same fixer corrected it before independent review.
Main's first-block probe at 714b16ae now preserves opening fence and quote syntax.
Distinct genuine Opus 5 reviewer `16013938-9aac-4a56-9495-729ee5b8354e` returned
ACCEPT on the exact range, closed N1/N2/N3, passed package tests (0.402s), race
(2.576s), vet/fmt, all 128 top-level tests and seven F13 pins. Its independent
shape probes and six deliberately regressed code overlays showed the tests detect
wrong starts, deindent, emptiness, occurrence resolution and spans. Evidence:
`execution/task1-rereview2/review-received.md`; no tracked reviewer edits.

**N4 — owner-reproduced, required empty-field diagnostic.** At 714b16ae a complete
labeled source with Acceptance Criteria body `-` then `- done`, plus user-selected
attestation on ac-2 and RetainUnmapped=true, yields fields problem/outcome/ac-2 and
`findings=[]`. Probe: `TestOwnerEmptyBullet` in the local owner first-block overlay;
it is observational, exit 0 is not a correctness assertion. The source's empty
first item consumes ordinal 1 but receives no unresolved/empty disclosure.

Contract mapping says one direct item is one object, empty fields are reported,
and empty objects cannot be made valid merely by RetainUnmapped. Earlier ordinal
notes left numbering open only; they did not waive empty-object reporting. Emit
blocking empty-field for the corresponding would-be ordinal object when a direct
flat-list item is empty. Preserve ordinal continuity, no fabricated Field/text/span.
A later explicit nonblank mapping to that target may resolve the value through the
existing M9 demotion; preserve the source-empty disclosure, evidence requirements
and coverage. Do not classify syntax-bearing empty fenced/quote blocks as empty,
or change nested-list refusal. Fresh Opus correction and distinct narrow re-review
remain required; do not reopen the closed source-fidelity changes or add a new API.

## Final Task 1 owner gate

Fresh N4 fixer `e251881c-094f-4bbf-9765-b266a62110d7` authored
`293fb773..1d6c16dff59b9686a70f185fc84dac3e0fc9bf9d` (one commit, three package
files). Distinct reviewer `a154acd1-79b8-41bf-8e7c-280977dc7d1d` returned ACCEPT:
N4 closed, package test 0.485s, focused 13-subtest regression 0.216s, vet/fmt clean,
tracked tree unchanged. Main reran its empty-bullet probe and observed the required
blocking empty-field/ac-1 while ac-2 stayed intact. The sandbox could not read a
Go cache entry; the same authorized local test passed with host cache access.
Evidence: `execution/task1-empty-item-review/review-received.md` and associated
probe/mutant artifacts. The reviewer proved multiple-empty behavior via overlay;
its absence as a committed multi-empty regression is nonblocking coverage debt,
not an incorrect result. No runtime correction remains open.

Main accepts the parser API at 1d6c16df for Task 2, retaining the original
producer's disclosed partial TDD process history. Required behavior is now covered
by package tests plus independent adversarial evidence; that does not retroactively
make the original production sequence strictly red-first. Whole-wave review, full
gates, release identity and both same-release adoption journeys remain ahead.
