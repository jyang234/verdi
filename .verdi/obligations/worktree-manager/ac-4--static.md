---
id: obligation/worktree-manager--ac-4--static
kind: obligation
title: "The reclaim decision is a total, four-outcome map with no silent fifth path"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/worktree-manager" }
frozen: { at: 2026-07-14, commit: c464efb6133e449257c48738ab66ae93a0e071e5 }
---
# The reclaim decision is a total, four-outcome map with no silent fifth path

The static evidence must show the `gc` reclaim decision for a managed worktree is a single, total function returning exactly one of {reclaim, keep-dirty, keep-locked, keep-not-eligible} for every input combination of (merged-or-deleted, clean, unlocked) — no unreachable/undefined combination silently falls through to removal. It must also show `gitx.IsAncestor` and `gitx.StatusDirty` (both pre-existing, reused unchanged) are the merged and dirty checks respectively, and that removal is never called with `--force`.
