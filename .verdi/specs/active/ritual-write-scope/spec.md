---
id: spec/ritual-write-scope
kind: spec
title: "Declared write scope for every ritual that touches git"
owners: [platform-team]
class: feature
problem: { text: "The largest and most severe class of shipped product defects is lifecycle rituals branching, committing, staging, or switching in ways the operator did not ask for: six of thirty-two real findings from one day of use (UAT-021, UAT-023, UAT-031, UAT-033, UAT-034, UAT-036) and a seventh adjacent (UAT-019), costing operators their own uncommitted work swept into commits they did not author, by a tool whose premise is that claims are traceable to who made them; no gate check exists over what a verb does to the repository, and the one token witness that exists (internal/gitx's fast-forward test) covers one path (process-audit PA-024).", anchor: problem }
outcome: { text: "Every verb that mutates a repository declares its write scope as data in one registry; a gate witness runs each ritual on fixtures and fails when the observed effects exceed the declaration; the forbidden-token witness covers every verb; and the six findings become regression witnesses, so the whole class turns into a red gate instead of a product finding.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "Every verb that mutates a git repository declares a write scope in one strict-decoded registry: the refs it may create or move, whether HEAD may switch, the paths it may stage, whether the index may carry entries it did not stage, whether untracked files may enter a commit, and whether it may push; a static witness fails when a verb that calls gitx has no declaration, and an unknown scope field fails closed.", evidence: [static, attestation], anchor: ac-1 }
  - { id: ac-2, text: "A behavioral witness runs each declared ritual against fixturegit repositories seeded with an untracked file, a pre-staged unrelated index entry, and a dirty tracked file, observes the repository before and after, and fails when any effect lies outside the declared scope; an effect the sensor cannot attribute is reported as unproven, never as within scope.", evidence: [behavioral, attestation], anchor: ac-2 }
  - { id: ac-3, text: "The forbidden-token witness (reset, restore, clean, stash, --force, update-ref) covers every verb's git command log through one seam in internal/gitx, and spec/readiness-recovery ac-9's recovery witness is one consumer of that seam rather than a second mechanism.", evidence: [static, behavioral, attestation], anchor: ac-3 }
  - { id: ac-4, text: "UAT-033, UAT-034 and UAT-036 are pinned as regression witnesses: a ritual commit never includes an untracked file the operator did not name, never carries a pre-staged entry outside its declared paths, and the branch-cutting rituals check name collisions against the tree they cut from (UAT-031); UAT-021 and UAT-023's fixes remain covered by their existing tests.", evidence: [behavioral, attestation], anchor: ac-4 }
constraints:
  - { id: co-1, text: "No new git primitive is added; declarations describe what existing rituals do and the witness observes; making a ritual conform is a fix to the ritual, never a widening of its declaration without an owner-visible decision.", anchor: co-1 }
  - { id: co-2, text: "The declaration is data checked by the gate (enforcement rung analyzer or gate assertion), never a comment or a plan sentence an agent remembers.", anchor: co-2 }
  - { id: co-3, text: "No network in any test; every ritual runs against internal/fixturegit repositories; pushes are exercised against a local bare remote only.", anchor: co-3 }
  - { id: co-4, text: "spec/readiness-recovery's recovery semantics are not changed: whichever feature lands first owns the gitx seam and the other consumes it.", anchor: co-4 }
decisions:
  - { id: dc-1, text: "One declaration plus one witness for the class, not six point fixes: the findings share a cause (no check over what a verb does to the repository), and a per-bug fix leaves the next ritual free to repeat it.", anchor: dc-1 }
  - { id: dc-2, text: "Declared scope is compared against observed effects, not only against the command log: a command log proves what gitx ran, and an effect sensor proves what changed, and only the pair catches a mutation made outside gitx.", anchor: dc-2 }
open_questions:
  - { id: oq-1, text: "What git commands does each verb actually run today? An empirical per-verb inventory, captured by a throwaway recorder in gitx's exec seam while the existing tests run, is the ground truth the declarations must be written from.", anchor: oq-1 }
  - { id: oq-2, text: "Which repository mutations happen outside internal/gitx's run function (the plumbing and config-value paths inside gitx, upstream pinned CLIs run through internal/upstream, sealed execution workspaces, the public-release tool), and does any of them touch refs, the index, or the working tree? This decides whether the command log alone is a sufficient sensor or a before-and-after state diff is required.", anchor: oq-2 }
  - { id: oq-3, text: "What is the smallest scope grammar that fits design start, build start, close, the commit-to-design ritual, and policy adopt, tried by writing each declaration by hand against oq-1's inventory?", anchor: oq-3 }
  - { id: oq-4, text: "Where does the declaration live: a Go registry checked by a gate test, a policy payload in the store (tying to spec/self-governance), or an amendment to design specs 03 or 04 that would need the ratification flow?", anchor: oq-4 }
  - { id: oq-5, text: "Does a gitx command recorder seam built for spec/readiness-recovery ac-9 in its wave 3 serve this feature unchanged, and which feature should land first given wave 3's timing?", anchor: oq-5 }
stubs:
  - { slug: ritual-effect-inventory, spike: true, resolves: [oq-1, oq-2, oq-3, oq-4, oq-5] }
links:
  - { type: depends-on, ref: spec/readiness-recovery }
---
# Declared write scope for every ritual that touches git

## Problem

Grouped by theme, one class dominates the product findings log: rituals
that touch git state the operator did not ask them to touch.

| Finding | What the ritual does | Status at authoring |
|---|---|---|
| UAT-021 | `design start` branches from HEAD and switches the primary checkout | fixed, pending merge |
| UAT-023 | `build start` cuts the feature branch from HEAD | open |
| UAT-031 | Branch-cutting checks name collisions against the wrong tree | open |
| UAT-033 | `design start` commits every untracked file in the checkout | open |
| UAT-034 | The commit-to-design ritual sweeps the working tree the same way | open |
| UAT-036 | Ritual commits carry pre-staged index entries | open |
| UAT-019 | Adjacent: the commit dialog accepts any message, so a proposal commit can claim acceptance | fixed, pending merge |

Every other finding class costs confusion or rework. This one costs the
operator's own work.

## Outcome

A ritual's effect on the repository becomes a declared, checked contract.

## Context from the process audit

- PA-024 is the entry. It exists because the findings were clustered rather
  than read one at a time; the entry asks that the product log be
  re-clustered whenever the ledger is revisited.
- The existing token check is `internal/gitx/ffonly_test.go`, a unit test
  over the fast-forward path's calls (reset, --force, update-ref, apply,
  am). gitx's `run` in `exec.go` has no recorder seam today, and gitx has
  two other exec sites (`plumbing.go`, `configvalue.go`).
- spec/readiness-recovery ac-9 requires that no recovery run's git command
  log contains the forbidden tokens; that witness is wave 3 of that feature
  and unbuilt at authoring. co-4 and oq-5 govern the shared seam.
- PA-015 and spec/self-governance: if declarations become policy payloads,
  the scope registry is governance data with an enforcement rung.

## ac-1

The registry is the contract; the static witness makes a missing
declaration impossible to ship, and strict decoding makes an unknown scope
field fail closed like every other artifact here.

## ac-2

The seeded fixture is the whole point: an untracked file, an unrelated
staged entry, and a dirty tracked file are exactly what the six findings
swept up. Unattributable effects are unproven, never within scope (three-
valued honesty).

## ac-3

One seam, two consumers.

## ac-4

The findings become pins, so the class stays closed.

## co-1

Describe, observe, fix the ritual; never widen the declaration to make the
gate green without an owner-visible decision.

## co-2

Data, not prose.

## co-3

Hermetic, as always.

## co-4

The seam is shared; recovery semantics are not this feature's to change.

## dc-1

Class remedy, not point fixes.

## dc-2

Command log and effect diff together.

## oq-1

Inventory first. Declarations written from memory would be the same
prose-only rule PA-004 warns about.

## oq-2

Blind spots decide the sensor.

## oq-3

Grammar by construction, from five real rituals.

## oq-4

Home of the declaration, and whether ratification is needed.

## oq-5

Sequencing against wave 3.
