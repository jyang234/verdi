---
id: conflict/vatc-forge-countersign-solo-approval-unreachable
kind: conflict
title: "vatc-forge-countersign leaves a solo repository no approval evidence"
status: open
owners: [platform-team]
links:
  - { type: challenges, ref: spec/vatc-forge-countersign }
---
# Conflict: a solo repository cannot produce countersign approval evidence

## What is disputed

`spec/vatc-forge-countersign` accepts only forge change approvals (AC-1,
AC-2), refuses self-approval unconditionally (AC-3), and bars author
claims, comments, and reactions as substitutes (DC-3). GLG v3 AC-3 defines
a solo governance profile in which "one authenticated principal may fill
author and approver roles where configured, with the collapsed separation
visibly disclosed". Under that profile the story's own contract leaves no
approval evidence at all, so no story or feature in a solo repository can
close. The dispute is wrong-for-this-story (03 §The amendment ladder, rung
3): `spec/verdi-atc-prerequisites#ac-2` still stands, because a GitHub
environment review is an authenticated forge review approval that binds an
approver principal and an exact candidate commit.

## Witness

1. GitHub refuses a pull request author's approving review of their own
   pull request, so a solo owner can never produce the change approval
   AC-1 and AC-2 require.
2. AC-3 lists "self-approved" among the refused cases with no profile
   qualifier, and DC-3 bars "author claims, comments, reactions" as
   substitutes, so no other forge act qualifies either.
3. `internal/lifecyclecountersign/resolve.go` hard-wires
   `SeparationDifferentFromAuthor` for every close, so even GLG v3's solo
   profile cannot reach the kernel's role-collapse path.
4. The closing-machinery plan (PR #345) first proposed treating the
   author's own open change as the approval. Codex review F1 refuted it:
   GLG v3 AC-3 says a profile "changes requirements, not the meaning of
   evidence", so any solo approval act must be ratified into this story's
   contract, not read into it.
5. Nothing in this repository has closed since 2026-07-30. The owner has
   declared the repository solo (closing-machinery decision D1).

## Resolution sought

Supersede with `spec/vatc-forge-countersign-v2` (rung 3): qualify AC-3's
self-approval refusal by the selected profile's separation requirement,
and add the owner's GitHub environment review of the dispatch-only close
run as the solo approval act (v2 AC-4, DC-5), with its derived identity
and time disclosed. Team and high-assurance profiles are unchanged.
