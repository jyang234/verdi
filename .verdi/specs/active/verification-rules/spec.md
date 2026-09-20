---
id: spec/verification-rules
kind: spec
title: "Verification rules per criterion"
owners: [platform-team]
class: feature
problem: { text: "A criterion declares what kind of proof it owes but never which claims it makes, so nothing can count whether every claim has a witness: one sentence can carry many prohibitions and read as satisfied on the strength of a test covering one (process-audit PA-018; witness: spec/readiness-recovery co-2 forbids nine distinct things and its first test covered two). Mutation testing cannot reach this, because a prohibition on adding something has no code to perturb. Downstream, nothing compares claims across the 28 active specifications (PA-021), and a named alignment gate does not check that fields the contract marks required are rejected by the decoder when absent (PA-006, witness UAT-008).", anchor: problem }
outcome: { text: "A criterion may enumerate its claims as individually addressable clauses with evidence kinds; witnesses cite clauses; the citation relation is checked as a set difference; a store lint flags criteria whose prose enumerates more than their clauses; enumerated clauses make cross-specification comparison possible; and every contract-required field is proven rejected-when-absent by its decoder, so the judgement layer's output becomes countable without pretending the enumeration is complete.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "An acceptance criterion or constraint may declare clauses, each with an id, text, and evidence kinds drawn from the existing closed set; a witness (a bindings entry or a test-side citation) may cite a clause fragment; a store lint reports every clause with no citing witness and every citation whose evidence kind mismatches the clause's declared kinds; a spec without clauses lints exactly as today.", evidence: [static, behavioral, attestation], anchor: ac-1 }
  - { id: ac-2, text: "An under-enumeration lint flags a criterion or constraint whose prose enumerates more items than its clause list declares, with a threshold calibrated on the active corpus and recorded as a decision; the readiness-recovery co-2 case (nine prohibitions, two witnessed) is its committed positive fixture.", evidence: [static, attestation], anchor: ac-2 }
  - { id: ac-3, text: "spec/readiness-recovery co-2 is the first enumerated constraint in this repository's store: each of its prohibitions is a clause with a cited witness, and the lint passes over it; the enumeration's completeness is a human attestation, never a lint claim.", evidence: [static, attestation], anchor: ac-3 }
  - { id: ac-4, text: "Every field the artifact contract marks required is proven rejected-when-absent by the corresponding strict decoder, or the contract names the exception and why; a gate test enumerates the contract's required fields and the decoder's behaviour for each, and UAT-008 is its first adjudicated row.", evidence: [static, behavioral, attestation], anchor: ac-4 }
constraints:
  - { id: co-1, text: "The artifact contract (design spec 02) changes only through the ratification flow with the change recorded in 08; no decoder accepts a clause field before the contract does.", anchor: co-1 }
  - { id: co-2, text: "Clauses are optional and additive: every existing spec, frozen or active, remains valid unchanged; no retroactive enumeration is mandated by this feature.", anchor: co-2 }
  - { id: co-3, text: "The lint checks a relation (cited or uncited, kind matches or not); it never judges whether a clause is true or whether an enumeration is complete, and its output says so.", anchor: co-3 }
  - { id: co-4, text: "No network in any test; fixtures under testdata; strict decoding with unknown fields rejected for the clause and citation records.", anchor: co-4 }
decisions:
  - { id: dc-1, text: "Relational, not semantic: the check is a set difference between declared clauses and cited witnesses, the same shape every mechanical check in this repository already takes, because equality and absence mechanize and meaning does not.", anchor: dc-1 }
  - { id: dc-2, text: "The residual is moved, not removed: an incomplete enumeration still passes; the feature's value is that the list is visible at specification time where a reviewer can count it against the sentence, which is strictly better than a test file nobody re-reads and not zero risk.", anchor: dc-2 }
open_questions:
  - { id: oq-1, text: "Across the 28 active specs, how many criteria and constraints enumerate more than three claims by a mechanical count (list separators, 'never', 'no ... or ...'), what is the size of the enumeration backlog if all were clause-listed, and what threshold for the under-enumeration lint yields a tolerable false-positive rate on the existing corpus?", anchor: oq-1 }
  - { id: oq-2, text: "Which citation seam works: extending verdi.bindings.yaml entries to clause fragments (spec/<name>#ac-2/c3) through the existing ResolveBindingAC and VL-003 path, or a test-side marker comment parsed by a gate test as the vocab:identity markers are, tried on readiness-recovery co-2's nine prohibitions?", anchor: oq-2 }
  - { id: oq-3, text: "What is the exact frontmatter delta in design spec 02 and its blast radius: which of the strict decoder, store lint, journey, readiness, spec-document renderer, dex site, and MCP get_document must change for clauses to appear, and how large is the 02 amendment?", anchor: oq-3 }
  - { id: oq-4, text: "Given enumerated clauses, is cross-specification comparison mechanical (same anchor and differing text across guided-lifecycle-governance v1, v2, v3 and comparative-spike-experiments v1, v2, v3) or a judge task, and are the older versions in the active directory actually marked superseded?", anchor: oq-4 }
  - { id: oq-5, text: "How many contract-required fields does the artifact contract mark today, and for how many does the decoder accept absence: the first count of the PA-006 class, with UAT-008 as one row?", anchor: oq-5 }
stubs:
  - { slug: clause-enumeration-measure, spike: true, resolves: [oq-1, oq-2, oq-3, oq-4, oq-5] }
links:
  - { type: depends-on, ref: spec/verdi-artifact-contract }
  - { type: depends-on, ref: spec/readiness-recovery }
---
# Verification rules per criterion

## Problem

The evidence list on a criterion is a bridge that says what kind of proof
is owed. It does not say which claims. So the set difference "clauses
without witnesses" cannot be computed, and a nine-clause constraint with
two clauses tested reads as satisfied.

## Outcome

Make the claims addressable, cite them, count the difference. Move the
residual risk from a test file nobody re-reads to a visible list at
specification time.

## Context from the process audit

- PA-018 (child of PA-001): the half of PA-001 that mutation testing
  cannot reach. spec/mutation-ratchet closes the other half.
- PA-021: every store lint rule is structural; none compares a claim in one
  specification against a claim in another. Enumerated clauses are the
  precondition for making that mechanical at all.
- PA-006: `spec-align` audits inventories and prose but not required-field
  agreement between contract and decoder. UAT-008 is the witness.
- Remedy design 6.3 in the ledger. Its cheap guard on the residual is the
  under-enumeration lint (ac-2).
- Section 4 of the ledger, the mechanical/judgement boundary: equality and
  absence mechanize; meaning does not. dc-1 is that rule applied.

## ac-1

Clauses, citations, and a relation check. A spec without clauses is
unchanged, so adoption is incremental.

## ac-2

The crude guard: prose that enumerates more than its clause list is flagged.
The threshold comes from oq-1, not from taste.

## ac-3

Dogfood on the case that motivated the feature. Completeness is attested by
a human, because the lint cannot know it.

## ac-4

The PA-006 class: required in the contract, optional in the decoder, gate
green. One enumerating test closes the class; UAT-008 is its first row and
gets adjudicated, not assumed.

## co-1

02 is authority; it changes through ratification, recorded in 08.

## co-2

Optional and additive. Nothing frozen is touched.

## co-3

The lint reports relations. Truth and completeness stay human.

## co-4

Hermetic and strict.

## dc-1

Relational restatement, or it does not transfer to the machine column.

## dc-2

The residual moves to where a reviewer can count it.

## oq-1

Measure the corpus before choosing a threshold or sizing the backlog.

## oq-2

Two seams already exist in spirit (bindings entries, marker comments);
pick by trying both on nine real prohibitions.

## oq-3

The 02 delta is the ratification cost; the blast radius is the build cost.
Both need a number.

## oq-4

Whether PA-021 is closable mechanically at all, and whether the versioned
duplicates in the active directory are already marked superseded (an
unverified nuance from the independent read of the audit).

## oq-5

First count of the PA-006 class.
