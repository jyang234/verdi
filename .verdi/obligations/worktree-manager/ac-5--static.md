---
id: obligation/worktree-manager--ac-5--static
kind: obligation
title: "dispatch.go's gc entry routes to a real implementation with a non-zero phase"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/worktree-manager" }
frozen: { at: 2026-07-14, commit: c464efb6133e449257c48738ab66ae93a0e071e5 }
---
# dispatch.go's gc entry routes to a real implementation with a non-zero phase

The static evidence must show `cmd/verdi/dispatch.go`'s `verbPhase["gc"]` is changed from `0` to a real, non-zero phase, and that `run()` dispatches `"gc"` to a real `cmdGc`-shaped function rather than falling through to the generic `"not implemented"` path. It must also show that function's own output includes a literal, unconditional scope-disclosure string naming derived-cache and layout-cache pruning as out of scope, printed on every invocation, not merely in documentation.
