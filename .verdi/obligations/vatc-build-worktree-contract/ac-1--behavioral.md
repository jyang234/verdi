---
id: obligation/vatc-build-worktree-contract--ac-1--behavioral
kind: obligation
title: "Build start stays inside the ATC runway"
owners: ["platform-team"]
for_kind: behavioral
quality:
  state: elaborated
  claim: "From .vatc/worktrees/<story-slug>/a<epoch>, built-binary build start checks out the expected feature branch in that runway while the primary checkout branch, HEAD, index, worktree, and untracked set remain byte-identical."
  falsifier: "Build start changes the primary checkout, creates or checks out the feature branch in the wrong worktree, selects a different base, or leaves an unrecorded mutation."
  scope: "A fixture Git primary checkout, one detached ATC runway, stable input commits, and before-and-after branch, HEAD, index, status, tree, and untracked snapshots."
  producer: { kind: test, ref: "go-test:cmd/verdi:TestBuildCommandsFromATCRunway_BuildStart" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun go-test:cmd/verdi:TestBuildCommandsFromATCRunway_BuildStart in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/vatc-build-worktree-contract" }
frozen: { at: 2026-08-25, commit: 0e8006b8b20270c9792ef6bf1a81ce165cbdcde9 }
---
# Build start stays inside the ATC runway

CI job `verify` must record producer `go-test:cmd/verdi:TestBuildCommandsFromATCRunway_BuildStart` at the exact candidate commit.

The evidence must prove: From .vatc/worktrees/<story-slug>/a<epoch>, built-binary build start checks out the expected feature branch in that runway while the primary checkout branch, HEAD, index, worktree, and untracked set remain byte-identical.

It is falsified when: Build start changes the primary checkout, creates or checks out the feature branch in the wrong worktree, selects a different base, or leaves an unrecorded mutation.

Scope: A fixture Git primary checkout, one detached ATC runway, stable input commits, and before-and-after branch, HEAD, index, status, tree, and untracked snapshots.
