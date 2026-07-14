---
id: obligation/directory-home--ac-1--static
kind: obligation
title: "The home renderer consumes the directory-index seam and enumerates no refs itself"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/directory-home" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# The home renderer consumes the directory-index seam and enumerates no refs itself

The static evidence must show that internal/workbench's home-page renderer
(the `GET /` handler that replaced index.go's single-checkout renderHome)
takes the ref-index story's computed directory index as its input — one
call into that seam per render — and contains NO git ref enumeration of its
own: no gitx branch/ref listing and no second copy of the status-grouping
rules inside the page renderer. Grouping must be keyed off the index
entry's status field (the four feature-dc-2 groups), not off any on-disk
path or address. Build and vet clean over the package.
