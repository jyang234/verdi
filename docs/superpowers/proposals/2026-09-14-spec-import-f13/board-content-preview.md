# F13 import content preview

Data-only, main-authored prototype. These are proposed board contents, not the
output of a working importer, a visual UI implementation, or an accepted spec.
Source identity and exact selected bytes are in `source-inventory.json`.

## Proposed statements

**Problem — inferred summary, needs review:** Candidate changes, excessive
review cycles, conflicting findings and unsupported evidence need explicit
handling so implementation review cannot advance on invalid grounds.

**Outcome — inferred summary, needs review:** ATC determines the next state and
required action through bounded review and alignment, invalidates results when
the candidate changes, and exposes owner-decision conditions while preserving
F14's separate responsibility for landing and closure effects.

Neither statement occurs verbatim as a Problem/Outcome pair in the selected
source. Both derive from primary F13 lines 614–661 and the transition core's
explicit scope boundary. The importer must not label them copied or approved.

## Proposed acceptance-criterion cards

All rows are source-derived draft requirements. The expected evidence kinds
must be reviewed separately; no test recipe or source checkbox supplies proof.
A feature's existing attestation floor remains mandatory. The prototype does
not manufacture attestations or implementation stories.

| Card | Proposed text | Source |
|---|---|---|
| ac-1 | Enumerate allowed transitions and representative forbidden transitions. | primary F13 650 |
| ac-2 | R0 cannot repeat; R1 occurs at most once and re-enters Aligning; R2 cannot trigger another automatic correction. | primary F13 650–652 |
| ac-3 | Any tree change invalidates results bound to the previous candidate. | primary F13 652–653 |
| ac-4 | All stories Done moves the feature to AwaitingUAT. | primary F13 653 |
| ac-5 | Two consecutive operational exits from Align route only that flight to G2. | primary F13 653–654 |
| ac-6 | Provider summaries never satisfy a gate, and there is no Failed state. | primary F13 654–655 |
| ac-7 | A blocking finding requires a binding-authority cite, reachable-state witness, concrete incorrect result and threat-model fit. | primary F13 659–660 |
| ac-8 | The author adjudicates every finding; conflicting blocking findings route to G2. | primary F13 660–661 |

## Proposed constraints and retained material

- `co-inputs`: committed candidate, align, review, adjudication, receipt and G2
  facts are consumed; pure next-state decisions and required effects are produced.
- `co-state-catalog`: preserve the full 19-state catalog in the source drawer/body.
  State vocabulary includes later phases; it does not transfer F14 effect ownership.
- File lists and test/Git commands remain attached implementation notes.
- Each supporting slice contract remains available in full. The review-validation
  slice provides structural checks only; the transition core excludes production
  wiring and feature aggregation; the journal excludes production shared-ledger
  routing and effect dispatch. These exclusions describe slice boundaries, not
  removal of the parent feature's requirements (including `ac-4`).
- Cited upstream authority outside the selected bundle is explicitly unbundled.
  A user may add it; the importer does not fetch or follow it automatically.
- No story-stub card is proposed: the primary file list is not an explicit story
  decomposition. Supporting implementation slices are not automatically stories.
- No accepted decision or open-question card is invented from absent data.

## Preview questions

The user reviews the two inferred statements, all mapped requirements, evidence
kind proposals, and source selection. Missing required values remain unresolved
in preview and block final creation until supplied through the supported flow.
The existing F13 source remains authoritative in its own context; this transfer
creates only a proposed Verdi representation and supplies no lifecycle proof.
