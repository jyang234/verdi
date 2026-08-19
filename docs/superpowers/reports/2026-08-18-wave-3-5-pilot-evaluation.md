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

### Success-criterion state

| Criterion | State | Witness |
|---|---|---|
| Understand what the view is trying to communicate | violated-with-witness | Participant could not form a clear purpose from the rendered view. |
| Identify the target | unproven | The participant had not supplied an answer before reporting the comprehension failure. |
| Locate the current area | unproven | No participant answer yet. |
| Explain readiness | violated-with-witness | The participant reported that the wording and designations did not communicate a usable meaning. |
| Name the top concern | unproven | No participant answer yet. |
| Find another concern | unproven | No participant answer yet. |
| Distinguish mechanical work from human judgment | unproven | No participant answer yet. |
| Follow a board link or CLI fallback | unproven | Not attempted before the comprehension failure. |
| Make one supported board edit | unproven | Not attempted before the comprehension failure. |
| Explain why restart is required | unproven | No participant answer yet. |
| Preserve and record the closed event sequence | unproven | Event capture remains pending. |

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
- Correction posture: pending one bounded design diagnosis. Do not add a new
  workflow engine, scoring model, persisted readiness record, cockpit
  mutation, or favorable handling of unknown facts.

### Pending participant evidence

The next observation should identify the earliest comprehension break: page
purpose, process position, status terminology, or corrective action. Remaining
task steps should not be forced until the interface establishes that basic
model.

## Scenario 2 — story

Not started. All evidence remains unproven.

## Verification evidence

- Consolidated Task 4 implementation review: approved at `3580cb62`.
- Focused readiness Playwright: 10/10 passed before the human pilot.
- Full browser suite: 234/234 passed before the human pilot.
- Human usability is not inferred from automated browser success.

## Wave 3B re-entry

No decision requested or recorded. Wave 3B remains paused.
