# F13 mechanical import content preview

Data-only reference mapping, specified by the main author. No import runtime or
model extraction ran. `mechanical-field-map.json` binds each proposed criterion
to exact source byte offsets and records its deterministic formatting transform.
This is not proof that a general Markdown parser can infer this mapping.

## Source-structure finding: missing labeled statements

**Problem: not explicitly mapped in the selected source.**

**Outcome: not explicitly mapped in the selected source.**

This is a finding against the selected F13 definition. It is not an importer
limitation for specifications that already contain labeled Problem and Outcome
statements: those must be mapped directly with their content preserved.

The importer does not invent either statement. The user can map an existing
passage or explicitly defer both statements under Verdi's existing draft rules.
A narrower slice's Goal is not automatically the whole F13 feature's Outcome.
The two inferred summaries from the previous prototype have been removed.

## Acceptance-criterion cards

Within each explicitly selected span, the source's words and punctuation are
preserved; only physical whitespace is collapsed for display. Selection itself
omits the introductory `Prove`, the imperative "Enumerate every allowed
edge..." test instruction, and separators from the cards, retaining them in
the source drawer. `primary_byte_dispositions` accounts for those bytes too. Card IDs are prototype-assigned; they are
not claimed to exist in the source. Evidence declarations remain separate:
source test commands do not constitute evidence kinds or proof. The feature's
existing attestation floor still applies.

Revised under spec/uat-round-1 ac-7/dc-9 (2026-09-16): each card now covers
exactly one complete claim, starting and ending at a clause or sentence
boundary; see `mechanical-field-map.json`'s `revision_note` for the full
rationale.

| Card | Source text with line wraps joined | Original lines |
|---|---|---|
| ac-1 | R0 cannot repeat | 650–651 |
| ac-2 | R1 count cannot exceed one | 651–651 |
| ac-3 | R1 candidate re-enters Aligning | 651–651 |
| ac-4 | R2 cannot produce another automatic correction | 652–652 |
| ac-5 | any tree change invalidates bound results | 652–653 |
| ac-6 | all stories Done moves the feature to AwaitingUAT | 653–653 |
| ac-7 | two consecutive operational exits from Align route only that flight to G2 | 653–654 |
| ac-8 | provider summaries never satisfy a gate | 654–655 |
| ac-9 | there is no `Failed` state. | 655–655 |
| ac-10 | A blocking finding requires nonempty binding-authority cite, reachable-state witness, concrete incorrect result, and threat-model fit. | 659–660 |
| ac-11 | The author lane adjudicates each finding. | 660–661 |
| ac-12 | Conflicting blocking findings route to G2. | 661–661 |

The prototype has **14 unsettled required values**: Problem, Outcome and the
evidence declaration for each of twelve criteria. Applicable model requirements
or explicit user selection must settle the latter; source test commands cannot.
This is not an exhaustive candidate validation result: target metadata, anchors,
links and project compatibility have not been validated. It is not one
confirmation away from a valid draft. The current twelve selectors define card
grouping; they do not demonstrate a generic rule for splitting prose.

## Retained structure and source material

Retained-only material stays in the source sidecar/drawer, outside the canonical
spec body and default context. Explicit mapping is required to promote any of
it into specification fields. Each support is one whole-document retained unit.

- The Interfaces section and complete 19-state catalog remain intact, ready for
  explicit field/constraint mapping. No summary constraint text is invented.
- File lists and test/Git commands remain implementation notes, not executable
  import instructions, proof records or automatic story stubs.
- All three supporting contracts remain attached in full. Their slice boundaries
  do not remove parent-feature requirements. In particular, feature AwaitingUAT
  remains in the primary F13 source despite its exclusion from the transition slice.
- External authority citations remain visible and explicitly unbundled; they are
  not automatically fetched or treated as already reviewed sources.
- No decisions, questions, relationships or story decomposition are inferred.

## Mapping review

The user reviews the mapping and source-selection boundary. Missing statement
fields remain unresolved until mapped or explicitly deferred; deferred values
must remain visible as incomplete. Other mandatory validity checks still apply.
The result is a proposed representation of existing requirements, not acceptance
or proof of implementation. The current prototype stops at this mapping; it has
not created a native draft or populated a running board.
