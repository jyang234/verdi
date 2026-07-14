---
id: obligation/ref-index--ac-1--static
kind: obligation
title: "ComputeIndex's signature depends on a ref-scoped port, never a working-tree runner"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/ref-index" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# ComputeIndex's signature depends on a ref-scoped port, never a working-tree runner

The static evidence must show `internal/refindex.ComputeIndex`'s exported signature and the git-runner-port interface it accepts (dc-2): the port's method set contains only ref-scoped reads (listing local `refs/heads/design/*`, listing remote-tracking `refs/remotes/origin/design/*`, resolving the default branch, reading a path's content at a given ref, listing a tree at a given ref) and exposes no method named or shaped like `checkout`/`switch`/anything that takes a ref to move HEAD to. Both the default-branch walk and the design-branch walk (ac-1's "one entry per spec/draft, no ref read twice, none skipped") must be traceable to this same port — no direct `exec.Command("git", ...)` call inside `internal/refindex` itself.
