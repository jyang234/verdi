# Spec importer usability correction: local validation

This correction addresses the owner's failed F13 import attempt on the earlier
candidate. It supersedes that candidate's readiness assessment; its historical
[validation report](2026-09-15-spec-import-local-validation.md) remains unchanged.
Local MVP acceptance still requires the owner's independent journey on this
release. Hosted forge/CI validation remains deferred.

## Release identity

- Source: `48f8dc7fd25ba860531b353c6560ce89bafcb0ca`; tree `8c862392a71383afe081370f6cc233b6486bc305`.
- Candidate: `.build/candidates/import-usability-48f8dc7f/verdi`.
- SHA-256: `6a5797dde08b991c2e972c1535add3940dd46079ef1a981f20c85511fd8d6f69`.
- Build: `go build -o <candidate> ./cmd/verdi`, go version go1.25.5 darwin/arm64, exit 0.

The final binary is byte-for-byte identical to the candidate built at `d5f9cbea`
and used for the completed F13 rehearsal. The intervening changes are three test
comparisons and documentation. The rehearsal's `engine_digest` equals the final
binary hash. No installed or earlier candidate binary was replaced.

## What changed

Missing evidence now produces the relevant blocking findings without the bogus
extra `unsupported-structure` error. The remaining criteria stay visible;
creation remains blocked until their requirements are explicitly supplied.
Unrelated composition failures and the existing attestation rule are preserved.

The browser explains expected evidence before showing its choices, offers direct
actions for missing statements and retained source text, uses readable labels,
and places byte mappings and links in Advanced. Changed inputs replace the old
readiness count and mark previous findings and coverage as earlier results.
Native supporting files are described as references rather than candidate spec
content. No evidence, statement deferral, retention or confirmation is selected
automatically. Import still performs no AI inference.

The [guide](../../import-existing-spec.md) now gives the exact F13 bundle and line
selection, explains choices and stale previews, and explicitly names the source
retention step. Labeled Problem and Outcome sections remain supported.

## Verification

`make verify` passed at `48f8dc7fd25ba860531b353c6560ce89bafcb0ca`, exit 0, in 23m 42s. Whole-repository race tests passed 93 packages (3 have no tests), followed by the seven required cache-disabled package reruns. All **297 browser cases passed** (11.1m). The wrapper confirmed the unchanged source head, clean tracked tree, protected binary identity and an empty recording-artifact scan. The gate also passed build, formatting, vet, lint, fixtures, store/model, specification-alignment and showcase checks. Existing VL-017 disclosures remain unproven rather than being counted as proven facts.

Focused FABLE tests passed all **18 importer browser cases**, with immediate
empty recording scans. Independent Opus additionally repeated the native-primary
plus supporting-source regression against a real served binary. Backend tests
cover duplicate suppression and deliberately incomplete plans without permitting
creation. The exact updated guide CLI example passed on the final candidate in a
clean synthetic project, with matching engine digest and unchanged project HEAD.

The first integrated gate failed at lint on three new test comparisons (QF1001),
before the test suite. A fresh FABLE call rewrote exactly `!(a < b)` to `a >= b`;
focused test and lint passed. No runtime or gate rule was changed. Earlier failures
remain recorded rather than being relabeled as passes.

## Genuine Claude review and correction

All calls below were actual Claude Code calls. Actual assistant model identifiers
and non-error terminal results were checked; FABLE turns invoked the actual
`fable-orchestration` skill.

| Work | Model / session | Outcome |
| --- | --- | --- |
| Backend finding | Opus 5 / `ab66c2d3-25ba-4eb9-9769-609e3950f7d9` | Important duplicate/false structure error |
| Fresh backend fix | Opus 5 / `28685caa-4e04-49d9-aded-aaafc19abc8e` | `a03dd661` |
| Fresh backend closure | Opus 5 / `0a264d94-8f89-4031-907e-493e8158c414` | Accepted |
| UI implementation and pre-review correction | FABLE 5.1 / `9c8e7d93-439f-4653-8113-2d7801395d65` | UI through `5bfb1b18` |
| Independent UI review | Opus 5 / `922ad984-a56d-4b3f-86b3-a04811540e34` | Accepted |
| Whole-wave review | Opus 5 / `f5447f2b-3520-4adb-b95b-441f08eb047a` | Accepted; documentation correction identified |
| Fresh UI-test lint correction | FABLE 5.1 / `14d137af-a52b-4ab9-9b22-e658572a6633` | `4cd70140` |
| Fresh final correction closure | Opus 5 / `02f88cde-b832-4a3e-b955-4726ceb1979a` | Accepted through `48f8dc7f` |

FABLE controller `7141295b-9d22-44c6-abeb-c086fdf522fd` reconciled the passing gate,
review chain and binary/rehearsal evidence and returned `READY_FOR_OWNER_RISK_GATE`.
Main accepted the implementation correction, adjudicated each finding, authored
the guide changes and checked integration.
The integrated Tier 3 classification remains. No Critical or Important behavioral
finding remains open. A pre-review native supporting-source labeling defect was
fixed before independent UI review, with a failing then passing regression.

Advisory limits remain: a generic Advanced jump can be unhelpful for a missing
story tracker; Advanced's mapping count includes explicit choices made on cards;
the static source hint mentions mapping although the native option expressly
refuses it. Evidence guidance appears after initial parsing and before any
evidence selection, as adjudicated before implementation. These are not evidence
or acceptance exemptions.

## Same-release assisted F13 journey

Project: `/Users/johnyang/code/verdi-system/verdi-atc-import-usability-rehearsal-20260915`,
a disposable real Verdi-ATC copy at `c346c005d117dfb090e1dedd5a89f45080875a5e`.
Initialization commit: `b73e2285821b45e843ea0aadbdf4b26aba9f546d`.
The user checkout and its server were not modified.

Main supplied the real four-file F13 bundle, corrected an intentional line-range
error, used the visible retention and pair-TODO actions, explicitly chose
static evidence and attestation, and created one ordinary feature proposal.
The eight missing evidence declarations were shown without a false structure
finding. Choices invalidated readiness until a fresh preview.

Coverage: **40,285 selected bytes = 642 mapped + 39,643 retained + 0 unresolved**.
The primary selection is 2,409 bytes (642 mapped and 1,767 retained). Supporting
files are retained in full; the primary's 28,906 unselected bytes are not retained.
Source content was never executed.

Import commit: `577ba0dd7ba58111178aa820cc505c3d5c519a32`. Main used the supported
board edit to put the existing Produces phrase into the Outcome attribute and
verified persistence after reload. Local-only commit
`6b8ac69bdf5db5e6c342465ad0dac7da2f9c26e8` records that edit and its authoring
provenance. The browser's Commit & push control was not used. The source record
verifies the original import and correctly reports `current_spec_matches: false`.
The Outcome section body and remaining Problem TODO are not claimed complete.

All six CLI inspections exited 0: lint disclosed VL-017 as unproven; state was
proposed/new; journey disclosed missing forge facts and author-vouch; all eight
matrix criteria had no signal; the import record distinguished historical from
current content; alignment reported its limited declared-edge computation with
judged proof absent. Exit 0 is not proof of F13 implementation or authoritative CI.

This was assisted, including setup, evidence selections, statement editing and a
local Git commit. The first main browser probe stopped before creation because
its expected stale-message wording was wrong; the corrected probe passed without
a product change. All immediate browser artifact scans were empty. A second
final-named clone was initialized while checking release identity but no extra
adoption journey is counted: binary equality already ties the completed rehearsal
to the final candidate.

## Remaining acceptance

The owner already volunteered to run the [independent journey](2026-09-15-spec-import-usability-independent-task.md). The new task selects this release; the older task remains a historical record. The earlier failed
attempt and this assisted rehearsal do not satisfy that independent milestone.
Use the new candidate consistently with the updated product documentation, record
friction, and assess the result against the adopted local MVP boundary.

No tracker is needed for this feature workflow. Story tracker and link requirements
remain. No authoritative acceptance, closure, F13 implementation, forge approval,
CI evidence production or retrieval is asserted. Any actual required external
proof stays incomplete until it exists. No hosted testing was performed.

## Evidence coverage

Detailed evidence remains under
`/Users/johnyang/code/verdi-system/.local/verdi-system/development/spec-import-f13-20260914/execution/`.
This report adds no authority. All seven evidence groups are summarized (7/7);
full transcripts and raw outputs remain in their originals instead of being copied.

| Source group | Report destination |
| --- | --- |
| `usability-final-candidate.json`, candidate equivalence witness | Release identity |
| `usability-make-verify-*`, focused lane logs and scans | Verification, including failed gate |
| Backend/UI/whole-wave/closure reports and model proof files | Genuine Claude review and correction |
| `usability-doc-coverage.md`, exact guide-example result | Guide change and verification |
| `usability-rehearsal-sources.json`, browser import/inspect evidence | Selected byte coverage and supported edit |
| Local edit commit, CLI inspection and rehearsal summary | Historical/current distinction and missing proof |
| Prior failed owner attempt, advisory adjudications, new independent-task coverage witness | Remaining acceptance and limitations |
