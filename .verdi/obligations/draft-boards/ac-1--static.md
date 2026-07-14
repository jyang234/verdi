---
id: obligation/draft-boards--ac-1--static
kind: obligation
title: "The /b/ prefix mounts the existing board server over the seam-obtained worktree"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/draft-boards" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# The /b/ prefix mounts the existing board server over the seam-obtained worktree

The static evidence must show that internal/workbench's /b/{branch} prefix
route resolves its branch segment (percent-decoded, dc-1) and serves the
board by constructing the EXISTING board server (boardSpecServer and its
route set — page, fragment, api/{action}, peek, pinsearch) rooted at a
worktree path obtained from the worktree-manager story's seam — one call
into that seam, no `git worktree` invocation and no worktree lifecycle
code inside the routing layer, and no second copy of the board projection
or its renderers. Build and vet clean over the package.
