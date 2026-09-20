---
id: spec/self-governance
kind: spec
title: "Self-governance: the build's rules as policy with declared enforcement"
owners: [platform-team]
class: feature
problem: { text: "Verdi ships a policy-authority system (kernel, typed payloads, constitution, exemptions with review windows, and a starter adoption verb), yet this repository's own store carries specs, attestations, obligations and conflicts and no policy or constitution at all; the rules that govern this build live as prose in CLAUDE.md, are enforced by an agent remembering them, and the product's exception-and-amendment path is never exercised by the build that produced it (process-audit PA-015). Every ground rule sits silently on the weakest enforcement rung, nobody can count which could move to an analyzer, and process disclosures accrue in wave ledgers with no owner or clearing condition (PA-020).", anchor: problem }
outcome: { text: "This repository's store carries an adopted governance profile; each ground rule that governs the build exists as a policy record with an enforcement rung from a closed enum and, on the review or prose rungs, a sentence saying what a violation looks like; rule counts per rung are projected so the question 'why is this rule not on rung two' is askable; overrides go through the existing exemption and amendment path; and deferred process disclosures are recorded as owned, clearable records in one index.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "This repository's .verdi store carries an adopted constitution, profile and policy set produced by the existing adoption verb and hand-completed, and make verify's store lint, model check and readiness derivation pass over the 28 active specs with them present; any new blocker the adoption introduces is either cleared or recorded as an exemption with a review window before merge.", evidence: [behavioral, attestation], anchor: ac-1 }
  - { id: ac-2, text: "Every ground rule stated in CLAUDE.md's Go style and testing sections exists as a policy claim carrying an enforcement rung from the closed enum {unrepresentable, analyzer, gate-assertion, review, prose}; a claim on the review or prose rung also carries a violation-looks-like sentence; an unknown rung fails closed and a missing sentence on those rungs is a lint finding.", evidence: [static, attestation], anchor: ac-2 }
  - { id: ac-3, text: "A read-only projection (a verdi policy subcommand and the MCP policy tools) lists rules per rung with their enforcing witness where one exists, and a drift witness fails when a rule exists in CLAUDE.md without a policy claim or in the policy set without a CLAUDE.md sentence.", evidence: [behavioral, static, attestation], anchor: ac-3 }
  - { id: ac-4, text: "The golangci-lint parity exception is recorded as an exemption with a review window, and the readiness derivation surfaces it as an eventual blocker when the window lapses, proving the exception-and-amendment path end to end on a real build rule.", evidence: [behavioral, attestation], anchor: ac-4 }
  - { id: ac-5, text: "Every item a wave ledger or report marks deferred, residual or parked is recorded as an owned record with a clearing condition in one index the readiness derivation can read, and the wave-1 and wave-4 items that exist at authoring (nine and ten) are its first entries.", evidence: [behavioral, attestation], anchor: ac-5 }
constraints:
  - { id: co-1, text: "The policy kernel's grammar changes only through the ratification flow; if the enforcement rung needs a schema field, the artifact contract amendment lands first.", anchor: co-1 }
  - { id: co-2, text: "CLAUDE.md remains the human-readable statement of the rules and the policy set is its enforceable mirror; neither is generated from the other, and the drift witness keeps them aligned.", anchor: co-2 }
  - { id: co-3, text: "Adoption never weakens an existing gate to pass: a blocker the new policy set raises against this repository is fixed or exempted with a review window, never suppressed.", anchor: co-3 }
  - { id: co-4, text: "No network in any test; the adopted store is exercised through the built binary over fixture copies.", anchor: co-4 }
decisions:
  - { id: dc-1, text: "Enforcement rung is a required field on a rule, strongest first: unrepresentable, analyzer, gate-assertion, review, prose. Lower rungs are permitted; the point is that a rule on rung four or five is declared and therefore countable (process-audit remedy design 6.1).", anchor: dc-1 }
  - { id: dc-2, text: "A rule that stays on the prose rung must state what a violation looks like, or a human reader cannot hunt for it either; if that sentence cannot be written the rule belongs on the analyzer rung or should not exist (owner amendment to 6.1, 2026-09-20).", anchor: dc-2 }
  - { id: dc-3, text: "Process disclosures reuse the product's own blocker record shape (reason, owner, witness, clearing condition) rather than a new artifact kind; the mechanism spec/readiness-recovery ac-1 derives for closure debt is applied to the build's own debt.", anchor: dc-3 }
open_questions:
  - { id: oq-1, text: "What does verdi policy adopt --starter produce in a copy of this repository's store, and what happens to store lint, model check, journey and readiness for the 28 active specs once a constitution and policy exist: how many new blockers, of which reason codes?", anchor: oq-1 }
  - { id: oq-2, text: "Can the CLAUDE.md ground rules be expressed as policy claims in the current kernel grammar? Try five: context is the first parameter of anything doing I/O; errors wrap with %w; no god packages with the owner's restatement ('a package you would need two sentences to describe is two packages'); every commit builds; never bare git stash. Which fit, which need a new payload kind?", anchor: oq-2 }
  - { id: oq-3, text: "Is the enforcement rung a new kernel field (artifact contract change through ratification) or expressible as an existing claim attribute, and which consumer (lint, readiness, a policy subcommand) counts rules per rung?", anchor: oq-3 }
  - { id: oq-4, text: "Does an exemption with a review window work end to end today for a build rule (the golangci-lint parity exception): authored, linted, surfaced by readiness when the window lapses, and closed by amendment?", anchor: oq-4 }
  - { id: oq-5, text: "Where does the process-disclosure index live so readiness can read it: obligations in the store keyed to a self-hosted spec, a dedicated record under .verdi, or the wave ledgers themselves parsed by a gate test; tried by recording wave 1's nine deferred items?", anchor: oq-5 }
stubs:
  - { slug: adopt-dry-run, spike: true, resolves: [oq-1, oq-2, oq-3, oq-4, oq-5] }
links:
  - { type: depends-on, ref: spec/spec-documents }
  - { type: depends-on, ref: spec/guided-lifecycle-governance-v3 }
  - { type: depends-on, ref: spec/readiness-recovery }
---
# Self-governance: the build's rules as policy with declared enforcement

## Problem

The mechanism that would fix PA-004 and PA-016 already exists in this tree
and is not applied to this tree. The store at the wave-1 head lists
`attestations`, `bin`, `conflicts`, `obligations`, `specs`, `verdi.yaml`,
and no policy directory, while `internal/policyartifact` carries the
kernel, exemptions and a constitution. Product finding UAT-018 made the
same observation from the user side.

## Outcome

Dogfood the governance layer on the build that produced it, and make every
rule's enforcement rung a declared, countable fact.

## Context from the process audit

- PA-015 is the entry. PA-004 and PA-016 are the rules that would move
  rungs first; spec/strict-lint-target and the pre-commit build hook
  (shipped 2026-09-20) are their analyzers and gate assertions.
- Remedy design 6.1 with the owner's amendment (dc-1, dc-2).
- PA-020: nine wave-1 and ten wave-4 disclosures with no cross-wave index.
  dc-3 reuses the blocker shape the product already has.
- The starter adoption verb is spec/spec-documents ac-10 (`verdi policy
  adopt --starter`, profiles solo and team).

## ac-1

Adoption is real only if the gate still passes with the policy present.
Anything it raises is fixed or exempted with a window, never suppressed.

## ac-2

Rungs as data, and the prose rung earns its place with a sentence a human
can hunt with.

## ac-3

Countable, and mirrored: a rule in one place and not the other is drift.

## ac-4

The exception path exercised on the one exception the build actually has.

## ac-5

Disclosure debt gets owners and clearing conditions, like every other debt
the product tracks.

## co-1

Kernel grammar through ratification.

## co-2

Two artefacts, one meaning, one witness.

## co-3

Never weaken a gate to adopt.

## co-4

Hermetic.

## dc-1

Rungs, strongest first.

## dc-2

The prose rung must be huntable.

## dc-3

Reuse the blocker shape.

## oq-1

What adoption does to this store, measured.

## oq-2

Expressibility of five real rules.

## oq-3

Whether rungs need a schema change.

## oq-4

Exemptions end to end, today.

## oq-5

Home of the disclosure index.
