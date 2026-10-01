---
id: spec/gitx-recorder-seam
kind: spec
title: "Git recorder seam"
owners: [platform-team]
class: story
story: jira:VERDI-PH-5
problem: { text: "The forbidden-token check runs over one verb's command log only (recover's), the token list lives inside internal/recovery, three read-only git calls bypass the seam, and one verb family emits a forbidden token today (update-ref, from design-branch creation).", anchor: problem }
outcome: { text: "Every verb's git command log passes through the one gitx seam and is checked against one shared forbidden-token list, design-branch creation no longer spells update-ref, and a verb that emits a forbidden token turns the gate red.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "The forbidden tokens are one list in a shared package, imported by recovery and by this witness, holding at least reset, restore, clean, stash, --force, update-ref, and recovery's -f (ledger SI-224), with recovery's semantics unchanged; no production code executes git except through internal/gitx; and design-branch creation uses the create-only git branch at a commit instead of update-ref (parent dc-8).", evidence: [static], anchor: ac-1 }
  - { id: ac-2, text: "Every declared verb, run through its real entry point, records its whole git command log through the gitx seam, the built binary into the file named by the test-only VERDI_GITLOG (parent dc-10); the forbidden-token witness fails when any verb's log carries a forbidden token, proven by a verb made to emit one; and recovery's runtime refusal stays one consumer of the same seam.", evidence: [behavioral], anchor: ac-2 }
constraints:
  - { id: co-1, text: "The parent's constraints bind this story unchanged: no new git primitive and no widened declaration without an owner-visible decision (co-1), the declaration is data checked by the gate (co-2), every test is hermetic over fixturegit repositories with pushes only to a local bare remote (co-3), and recovery's semantics do not change (co-4).", anchor: co-1 }
decisions:
  - { id: dc-1, text: "The shared list moves out of internal/recovery into its own small package, and recovery imports it; recovery's refusal behavior and its VERDI_RECOVERY_GITLOG hook keep working (co-4, ledger SI-305).", anchor: dc-1 }
  - { id: dc-2, text: "VERDI_GITLOG names a file the binary appends one record per git execution to, in execution order; unset, nothing is recorded. It is a test hook, documented as such, and never set by the product.", anchor: dc-2 }
  - { id: dc-3, text: "The three read-only git calls outside gitx at the base (the local actor's top-level lookup, the draft identity's, and the disclosure cache key's reads) move behind read-only gitx functions; the disclosure cache key's guard over lint's git reads is updated in the same change if its read shape changes.", anchor: dc-3 }
links:
  - { type: implements, ref: "spec/ritual-write-scope-v3#ac-3" }
---
# Git recorder seam

## Problem

The forbidden-token check runs over one verb's command log only (recover's), the token list lives inside
internal/recovery, three read-only git calls bypass the seam, and one verb family emits a forbidden token today
(update-ref, from design-branch creation).

## Outcome

Every verb's git command log passes through the one gitx seam and is checked against one shared forbidden-token list,
design-branch creation no longer spells update-ref, and a verb that emits a forbidden token turns the gate red.

## ac-1

The forbidden tokens are one list in a shared package, imported by recovery and by this witness, holding at least reset,
restore, clean, stash, --force, update-ref, and recovery's -f (ledger SI-224), with recovery's semantics unchanged; no
production code executes git except through internal/gitx; and design-branch creation uses the create-only git branch at
a commit instead of update-ref (parent dc-8).

## ac-2

Every declared verb, run through its real entry point, records its whole git command log through the gitx seam, the
built binary into the file named by the test-only VERDI_GITLOG (parent dc-10); the forbidden-token witness fails when
any verb's log carries a forbidden token, proven by a verb made to emit one; and recovery's runtime refusal stays one
consumer of the same seam.

## co-1

The parent's constraints bind this story unchanged: no new git primitive and no widened declaration without an
owner-visible decision (co-1), the declaration is data checked by the gate (co-2), every test is hermetic over
fixturegit repositories with pushes only to a local bare remote (co-3), and recovery's semantics do not change (co-4).

## dc-1

The shared list moves out of internal/recovery into its own small package, and recovery imports it; recovery's refusal
behavior and its VERDI_RECOVERY_GITLOG hook keep working (co-4, ledger SI-305).

## dc-2

VERDI_GITLOG names a file the binary appends one record per git execution to, in execution order; unset, nothing is
recorded. It is a test hook, documented as such, and never set by the product.

## dc-3

The three read-only git calls outside gitx at the base (the local actor's top-level lookup, the draft identity's, and
the disclosure cache key's reads) move behind read-only gitx functions; the disclosure cache key's guard over lint's git
reads is updated in the same change if its read shape changes.
