# Wave 3.5 readiness pilot evaluation

Status: in progress

This report records only observed participant behavior and the closed pilot
facts required by the Wave 3.5 plan. It does not record source text, prompts,
hidden reasoning, secrets, or individual productivity judgments.

## Scenario 1 — feature

### Fixed identity

- Runtime branch: `design/refi-decline-flow`
- Runtime HEAD and snapshot HEAD:
  `33f8fd8038bd9a665e87a01d5f8c2deaac9e2537`
- Target: `spec/refi-decline-flow`
- Target class: `feature`
- Request digest:
  `sha256:fbaf575ab56ff275abe1a139dc7d10f62270148db1e0bec5ce7ba062611eb522`
- Browser scenario: local in-app browser at
  `http://127.0.0.1:4173/readiness`
- CLI scenario: not attempted
- Exact browser version: unproven

### First-use observation

Before completing the prompted navigation tasks, the participant reported
that the view's purpose was unclear and that its wording, layout, and state
designations were not intuitive. No favorable interpretation is applied: the
pilot's first-use comprehension goal is violated with this participant
witness.

When asked what the page should answer first, the participant supplied the
exact desired decision frame: “Where am I in the process and what should I
focus on next?” This becomes the correction's information-hierarchy anchor;
the existing terminology and evidence remain subordinate to those two
questions.

For multiple concerns, the participant requested a short ranked preview of
three to five items, with an explicit remaining-count affordance such as
“x more items” that expands to the full list. The preview must never imply
completeness or make the remaining concerns undiscoverable.

The participant selected inline expansion: the remaining ranked concerns
appear in the same sequence and location rather than navigating to a separate
section.

For state language, the participant accepted plain primary labels — `Ready`,
`Needs attention`, and `Not enough evidence yet` — with the exact formal state
(`proven`, `violated-with-witness`, or `unproven`) retained as subordinate
supporting text. This changes presentation only and preserves three-valued
honesty.

The participant accepted plain stage labels mapped losslessly onto the four
existing area identities: `Define the work`, `Define success`,
`Check constraints`, and `Get approval`. The current stage should also state
its ordinal position as `Step N of 4`.

The participant asked that technical identifiers and digests not dominate the
primary cards. Prefer an exact human-readable display name already supplied by
the source (for example, a story or object title). When no trustworthy display
name exists, use a basic generic description rather than inventing a label.
Keep the exact identifier, digest, witness, and formal state readily available
under an expandable `Technical details` affordance.

The participant selected a fixed preview of three ranked concerns on every
viewport. The remaining-count disclosure and inline expansion therefore have
stable meaning across desktop and narrow layouts.

After reviewing the interactive correction mockup, the participant reported
that the revised hierarchy “looks better.” This is evidence of directional
improvement, not yet proof of task completion; the corrected runtime must still
receive the feature and story pilot scenarios.

The participant selected current-step-first attention ordering. Unresolved
items in the current process stage must precede downstream issues. Downstream
violations remain explicitly counted and available, but they must not displace
the work required to advance the current stage.

### Corrected-runtime rerun

The corrected runtime was rerun after the consolidated correction closed at
implementation head `8fbef0958f43c7df13bd4ab64ac0672a5a16d7af` with the
same fixed scenario identity above. Without opening technical details or the
inline remainder, the participant correctly identified the exact target title
as “Refinancing decline flow,” located the current stage as “Define the work,”
and named both current-stage priorities: the unresolved declared open question
and the design-provenance posture for seven declared objects. This proves that
the corrected orientation and current-stage ordering materially improved the
first-use model.

The participant could not interpret the third visible priority, “forge facts
become available to the projection,” or its relationship to “Get approval”
without opening technical details. After reading the witness, the participant
understood it as eventual work but found its placement in the top three
distracting while the current stage is “Define the work.” This is recorded as a
presentation finding, not a failure by the participant: filling the fixed
three-item preview with a downstream approval concern weakens the meaning of
“Focus next” when the current stage has fewer than three unresolved concerns.

The participant reported that “Not enough evidence yet” makes sense for the
current stage, but the cockpit does not tell a newcomer what must be done to
obtain that evidence. The plain state label is therefore comprehensible while
the corrective route remains a barrier to entry. Exact technical facts and a
destination are present, but presence alone did not communicate the next
action.

The remaining navigation, work-class, board-edit, restart, and event-sequence
steps are still in progress. No favorable interpretation is applied to them.

After expanding the complete queue, the participant described every additional
item as actionable later and asked whether the four stages are intended to be
completed out of order. The cockpit does not answer that sequencing question.
This reinforces F-02: exact ordering is visible, but the relationship between
current-stage action and downstream awareness is not yet explicit.

The participant identified “Get approval — 1 principal role required for
review” as human approval work. Most other rows appeared mechanically
checkable, but only after opening each row's technical details. The distinction
can therefore be recovered, but it is not comprehensible from the primary
presentation.

The participant followed the first concern's board link, created a spike
sticky, and attempted to graduate it to resolve `oq-1`. Graduation failed with
the exact operational witness:

`splice: validate-before-write: artifact: stubs[0]: artifact: stub slug
"find-out-which-decline-reasons-can-legally-be-shown-verbatim----follow-up-with-legal."
must be kebab-case.`

The participant had not been told that the sticky content would become a slug
or had to satisfy the kebab-case grammar, and the existing board offered no way
to edit the sticky after creation. Sticky creation proves one supported board
edit; the intended graduation remains violated with the witness above.

The participant expected the readiness page to reflect the resolution
automatically and inferred that the capability does not yet exist. The current
startup-snapshot notice did not establish the stronger operational model: the
page will stay byte-stable until the human manually restarts `verdi serve`.
Live refresh is intentionally outside the pilot, but the expectation mismatch
is recorded rather than treated as successful stale-state understanding.

The closed page-memory event sequence could not be read from the preserved
in-app source tab because the browser-control session exposed no claimable tab.
No alternate or reconstructed sequence is substituted; that evidence remains
unproven.

### Success-criterion state

| Criterion | State | Witness |
|---|---|---|
| Understand what the view is trying to communicate | proven | On the corrected runtime, the participant identified the target, process stage, and immediate-focus purpose without opening subordinate details. |
| Identify the target | proven | Participant answered “Refinancing decline flow,” the exact source title. |
| Locate the current area | proven | Participant answered “Define the work,” the exact current-stage label. |
| Explain readiness | violated-with-witness | The participant understood the unknown-state label but could not determine how a newcomer obtains the missing evidence; the third downstream card also weakened the immediate-focus model. |
| Name the top concern | proven | Participant named the unresolved declared open question first. |
| Find another concern | proven | Participant found the design-provenance concern and the downstream forge-facts concern, while reporting the latter's placement as distracting. |
| Distinguish mechanical work from human judgment | proven | Participant identified the principal-role review row as human approval and most remaining work as mechanically checkable, while reporting that the distinction required opening technical details. |
| Follow a board link or CLI fallback | proven | Participant followed the first concern's board link into the existing editable board. |
| Make one supported board edit | proven | Participant created a spike sticky. The subsequent graduation attempt failed on an undisclosed slug constraint and is separately recorded as violated. |
| Explain why restart is required | violated-with-witness | Participant expected automatic reflection and inferred only that the capability was absent; the page did not communicate that the immutable startup snapshot changes only after a manual serve restart. |
| Preserve and record the closed event sequence | unproven | The in-app browser exposed no claimable tab to the read-only event-capture session; no sequence was reconstructed. |

### Findings

#### F-01 — First-use comprehension failure

- State: `accepted-wave-3.5`
- Observation: the page did not establish a clear purpose, process position,
  meaning for its designations, or intuitive reading order before asking the
  participant to act.
- Affected pilot dimensions: task orientation, terminology, layout hierarchy,
  and readiness-state explanation.
- Participant-defined success frame: first identify the current process
  position, then identify the next focus.
- Lossless presentation constraint: show a ranked three-to-five-item preview,
  disclose the exact remaining count, and provide direct expansion to every
  remaining concern.
- Expansion behavior: inline, preserving the original ranking and context.
- State-language hierarchy: plain-language primary label plus exact formal
  state in secondary text; unknown facts remain explicitly unknown.
- Process-language hierarchy: four plain stage labels and explicit ordinal
  position, while the existing internal area identities remain unchanged.
- Identity-language hierarchy: exact source display names when available;
  otherwise basic descriptions, with all verbose identity and evidence facts
  preserved in accessible technical details.
- Preview size: exactly three concerns on every viewport before inline
  expansion.
- Attention ordering: current-stage unresolved items first; remaining slots and
  the expanded tail retain deterministic downstream severity/order. Known
  downstream violations remain disclosed with an exact count.
- Correction posture: corrected and closure-reviewed at implementation head
  `8fbef0958f43c7df13bd4ab64ac0672a5a16d7af`. No workflow engine,
  scoring model, persisted readiness record, cockpit mutation, or favorable
  handling of unknown facts was added.

#### F-02 — Distant downstream card dilutes immediate focus

- State: `deferred-original-wave` — Wave 6 GLG workbench journeys.
- Observation: with only two unresolved concerns in the current stage, the
  fixed three-card preview fills its final slot with a “Get approval” concern.
  The participant correctly inferred that it matters eventually but found it
  distracting and could not tell why it belonged under “Focus next.” In the
  story scenario, an adjacent `Define success` concern was understood as work
  to do soon and did not cause the same failure.
- Constraint: the exact downstream violation count and the complete inline
  queue must remain available; no fact may be hidden or reclassified.
- Scope ruling: complete lifecycle presentation and authoritative action
  guidance remain with the original Wave 6 GLG workbench-journeys unit. The
  single Wave 3.5 correction pass and closure review are complete.

#### F-03 — State label lacks newcomer-oriented corrective explanation

- State: `deferred-original-wave` — Wave 6 GLG workbench journeys.
- Observation: “Not enough evidence yet” was understandable, but neither the
  primary card nor its subordinate facts made clear what a newcomer should do
  to obtain the required evidence. In the story scenario, descriptive source
  prose sometimes supplied enough context, but the state label itself still
  did not identify the action.
- Constraint: any future explanation must remain source-derived, must not
  synthesize a favorable proof posture, and must preserve the exact CLI or
  board destination.
- Scope ruling: authoritative action and recovery guidance remain with the
  original Wave 6 GLG workbench-journeys unit; no second automatic Wave 3.5
  correction pass is authorized.

#### F-04 — Stage sequencing is not explained

- State: `deferred-original-wave` — Wave 6 GLG workbench journeys.
- Observation: after expanding the queue, the participant considered every
  non-current-stage concern actionable later and could not tell whether the
  four displayed stages may or should be completed out of order.
- Original-wave owner: Wave 6 GLG workbench journeys, whose deferred scope
  includes complete lifecycle presentation and authoritative actions.

#### F-05 — Work type is recoverable only through technical detail

- State: `deferred-original-wave` — Wave 6 GLG workbench journeys.
- Observation: the participant correctly distinguished a principal-role row as
  human approval and the policy-conflict verdict as mechanical after expanding
  the queue. The primary cards do not explain the distinction; the participant
  suggested a direct `Human review requested` flag.
- Original-wave owner: Wave 6 GLG workbench journeys, including broader roles
  and authoritative-action presentation.

#### F-06 — Board graduation exposes slug grammar after authoring

- State: `deferred-original-wave` — Wave 6 ASD workbench.
- Observation: graduating a newly created spike sticky failed because the
  sticky-derived slug was not kebab-case; the author had no advance indication
  of that constraint and could not edit the sticky in place after creation.
- Witness: the exact `splice: validate-before-write` error recorded above.
- Scope ruling: the Wave 3.5 cockpit only deep-links to the existing board and
  may not change board mutation semantics. In-place edit protection and
  authoring validation belong to the original ASD workbench delivery.

#### F-07 — Startup immutability is disclosed but not operationally understood

- State: `deferred-original-wave` — Wave 6 GLG workbench journeys.
- Observation: after a board edit, the participant expected readiness to
  update automatically and inferred that live reflection was simply missing.
  The stale notice did not convey that the present contract intentionally
  requires a manual `verdi serve` restart.
- Scope ruling: live refresh is explicitly deferred in the Wave 3.5 carve-out
  matrix; no cockpit refresh or mutation may be added in this pilot.

### Remaining evidence limitation

The closed in-memory event sequence remains unproven because the preserved
in-app source tab could not be claimed by the read-only browser-control
session. No event sequence is reconstructed from other evidence.

## Scenario 2 — story

### Fixed identity

- Scenario posture: hermetic synthetic story linked to a hermetic accepted
  parent feature; neither artifact is committed to the delivery branch.
- Runtime branch: `design/pilot-story`
- Runtime HEAD and snapshot HEAD:
  `e67f17e920b83eb00841d907ba77bcb55e34901b`
- Target: `spec/pilot-story`
- Target title: `Borrower appeal notice`
- Target class: `story`
- Accepted parent: `spec/pilot-parent#ac-1` on the fixture's main branch
- Request digest:
  `sha256:f25ca885edb4f5a427e3b47e789ae37dad608d70b789f69ef8a20fdf2ec7e924`
- Browser scenario: local in-app browser at
  `http://127.0.0.1:4173/readiness`
- CLI scenario: not attempted
- Exact browser version: unproven

The first attempted existing story fixtures could not produce a conforming
startup snapshot: stories under `stale-decline`/`escrow-autopay` required a
pinned ADR object absent from the hermetic e2e history, while the archived-parent
fixture was not accepted. No favorable workaround was applied. The synthetic
story uses the same strict spec decoder, accepted-parent requirement, policy
resolver, journey projection, and production workbench handler, with its exact
identity disclosed above.

Without opening technical details or the inline remainder, the participant
identified the exact title `Borrower appeal notice` and the current stage
`Define the work`. The participant treated the first two `Define the work`
priorities as work to complete now and the third `Define success` priority as
work to complete soon afterward. Unlike the feature scenario's third-card jump
to `Get approval`, this adjacent-stage card did not break the participant's
process model.

The participant reported that the status labels still do not tell them what
action to take, although the story cards' source descriptions were descriptive
enough to infer what is needed. This corroborates F-03 while showing that
source-specific summary quality materially affects comprehension; the renderer
must not fabricate that quality where the source does not supply it.

After expanding the complete inline queue, the participant identified the
policy-conflict verdict as mechanically checkable and the principal role
required for review as human judgment. They suggested an explicit
`Human review requested` flag. This proves that the work types are
distinguishable from the available facts while reinforcing F-05: the primary
presentation does not expose that distinction directly.

The participant returned to the board and again found that the sticky could not
be edited. They correctly inferred that the pilot correction had not changed
the board's mutation capability. This independently corroborates F-06; no
Wave 3.5 cockpit change is inferred from the board limitation.

The participant reasoned that readiness should not change for every
unconfirmed sticky edit and preferred successful graduation as the default
confirmation boundary. They also identified the opposite risk: discovering
only after a permanent graduation that the edit did not change readiness would
feel wasteful. This is a sound desired model, but it is not the current complete
operational model. A successful graduation may change readiness only when it
changes the exact source fact underlying the concern, and the startup-only
pilot still requires a manual `verdi serve` restart before the page can expose
the recomputed snapshot.

The participant asked what the evaluator meant by “expanded later-stage
list.” That wording was imprecise and is not attributed to the cockpit. It
meant the complete inline queue revealed by `10 more items`, which can contain
current-stage and later-stage concerns. This report uses `complete inline
queue` from this point forward.

### Success-criterion state

| Criterion | State | Witness |
|---|---|---|
| Understand what the view is trying to communicate | proven | Participant described current work and the adjacent next stage without subordinate details. |
| Identify the target | proven | Participant answered `Borrower appeal notice`, the exact source title. |
| Locate the current area | proven | Participant answered `Define the work`. |
| Explain readiness | proven | Participant distinguished the two current-stage priorities from the next-stage priority, while reporting that statuses alone do not specify corrective action. |
| Name the top concern | proven | Participant recognized both `Define the work` cards as current priorities. |
| Find another concern | proven | Participant identified the third `Define success` card as work to do soon afterward. |
| Distinguish mechanical work from human judgment | proven | Participant identified the policy-conflict verdict as mechanical and principal-role review as human judgment, while recommending a visible human-review flag. |
| Follow a board link or CLI fallback | proven | Participant returned to the existing editable board from the cockpit flow. |
| Make one supported board edit | proven | Participant created or revisited the prompted sticky and confirmed that in-place editing remains unavailable. |
| Explain why restart is required | violated-with-witness | Participant chose graduation rather than draft editing as the desired confirmation boundary but did not identify the current manual serve restart required to recompute the startup snapshot. |
| Preserve and record the closed event sequence | unproven | Pending. |

### Story-scenario finding disposition

The story scenario narrows rather than reverses the feature findings:

- F-02 applies when a distant stage such as `Get approval` occupies a
  `Focus next` slot; an adjacent `Define success` item was understood as work
  to do soon and did not cause the same distraction.
- F-03 remains: descriptive source prose can make a card actionable, but the
  state label itself does not explain the corrective route.
- F-04 remains: the participant inferred a sensible current-then-next order,
  but the cockpit does not state whether stages are sequential or may be
  completed out of order.
- F-05 remains: the participant recovered the mechanical-versus-human
  distinction and proposed a direct `Human review requested` presentation
  flag.
- F-06 and F-07 were independently corroborated by the board and restart
  discussion.

No second automatic Wave 3.5 runtime correction is authorized. These findings
return to their original delivery units.

#### F-08 — Graduation impact is not previewable

- State: `deferred-original-wave` — Wave 6 ASD workbench.
- Observation: the participant preferred graduation as the boundary after
  which readiness may change, but identified the cost of learning only after
  that durable action that the edit did not resolve the cited readiness fact.
- Scope ruling: previewing the semantic effect of a proposed board mutation
  belongs to the ASD workbench's deferred semantic-review and provenance
  presentation scope. The readiness cockpit remains read-only and may not add
  speculative or favorable pre-graduation state.

## Verification evidence

- Consolidated Task 4 implementation review: approved at `3580cb62`.
- Corrected-runtime closure review: approved at `8fbef095`.
- Focused readiness Playwright: 15/15 passed before the corrected human rerun.
- Full browser suite: 239/239 passed before the corrected human rerun.
- Human usability is not inferred from automated browser success.

## Wave 3B re-entry

No decision requested or recorded. Wave 3B remains paused.
