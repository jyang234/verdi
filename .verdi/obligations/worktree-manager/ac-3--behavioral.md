---
id: obligation/worktree-manager--ac-3--behavioral
kind: obligation
title: "Concurrent EnsureWorktree calls produce exactly one cut; a live-locked worktree survives a concurrent gc run"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/worktree-manager" }
frozen: { at: 2026-07-14, commit: c464efb6133e449257c48738ab66ae93a0e071e5 }
---
# Concurrent EnsureWorktree calls produce exactly one cut; a live-locked worktree survives a concurrent gc run

The behavioral evidence must show a Go test that fires two (or more) concurrent `EnsureWorktree` calls for the SAME not-yet-cut branch (goroutines, or two invocations of the built binary against the same fixture repo) and asserts: exactly one `git worktree add` occurred (via `git worktree list` or an instrumented count), every caller received the identical path, and no error or corrupted worktree state resulted. A second test must hold a live lock (a fake/simulated pid, or a real held file handle) over a merged, clean worktree and assert a concurrent `gc` run skips it, reporting it kept/in-use, rather than removing it.
