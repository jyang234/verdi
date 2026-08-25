---
id: spec/vatc-build-worktree-contract
kind: spec
title: "ATC runway worktree contract"
owners: [platform-team]
class: story
story: jira:VERDI-ATC-3
problem: { text: "Verdi's build commands have not been exercised as one lifecycle transcript from an ATC-owned non-primary worktree, so an orchestrator cannot prove that repository discovery, branch checkout, mutations, and closure preparation stay in the runway rather than affecting the primary checkout", anchor: problem }
outcome: { text: "a hermetic built-binary contract proves build start, align, disposition, gate, and close preparation inside a detached ATC runway, and runtime code changes only if the test exhibits a specific violation", anchor: outcome }
acceptance_criteria:
  - id: ac-1
    text: "from .vatc/worktrees/<story-slug>/a<epoch>, verdi build start checks out the expected feature/<feature-name> branch in that same worktree and leaves the primary checkout branch, HEAD, index, worktree, and untracked set unchanged"
    evidence: [behavioral]
    anchor: ac-1
  - id: ac-2
    text: "align, disposition, gate, and close --prepare run from and apply every permitted mutation to the runway worktree, with the transcript recording exit class, current branch, HEAD, tree, status, and changed paths after each command"
    evidence: [behavioral]
    anchor: ac-2
  - id: ac-3
    text: "an existing feature branch whose base differs from the expected base and an operational Git failure are refused with deterministic witnesses, without redirecting work or partially mutating the primary checkout"
    evidence: [behavioral]
    anchor: ac-3
links:
  - { type: implements, ref: "spec/verdi-atc-prerequisites#ac-3" }
decisions:
  - id: dc-1
    text: "the canonical runway path is .vatc/worktrees/<story-slug>/a<epoch> relative to the primary repository, but all invoked Verdi commands discover and operate on the current worktree root rather than resolving mutations through the primary checkout"
    anchor: dc-1
  - id: dc-2
    text: "the first delivery artifact is a fixture-Git built-binary transcript; runtime edits are permitted only for a failure that the transcript reproduces and must be the smallest repair of that witnessed gap"
    anchor: dc-2
  - id: dc-3
    text: "the story does not make Verdi own runway creation, epochs, leases, quarantine, or branch reconciliation policy; those remain Verdi-ATC responsibilities"
    anchor: dc-3
constraints:
  - id: co-1
    text: "the test uses explicit fixture paths and stable commits, never the developer's live checkout, environment, credentials, or network"
    anchor: co-1
  - id: co-2
    text: "no test or repair may weaken existing branch, accepted-story, alignment, index, closure, or frozen-spec gates to make the runway transcript pass"
    anchor: co-2
---
# ATC runway worktree contract

## Problem

ATC deliberately creates a detached non-primary worktree before BuildStart.
Individual Verdi command tests do not prove that the complete sequence stays
inside it.

## Outcome

A single hermetic transcript becomes the executable compatibility contract.
If current Verdi behavior passes, this story lands proof rather than speculative
runtime changes.

## AC-1

The transcript snapshots both primary and runway repositories before and after
BuildStart and proves the expected branch exists only where intended.

## AC-2

Each remaining build-phase command runs through the built binary. Every change
is attributed to the runway and the primary checkout is compared byte-for-byte
at the tracked/index/status boundary.

## AC-3

Negative fixtures prove fail-closed behavior for a branch-base mismatch and a
Git operational error. No fallback silently selects another checkout.

## Decisions

### DC-1

ATC chooses the runway path; each Verdi command operates from the invoking
worktree root.

### DC-2

The transcript lands first and runtime changes require its concrete witness.

### DC-3

Runway creation, leases, epochs, quarantine, and reconciliation remain ATC
responsibilities.

## Constraints

### CO-1

All Git fixtures use explicit temporary paths and stable commits.

### CO-2

No repair may weaken an existing lifecycle, Git, evidence, or spec gate.
