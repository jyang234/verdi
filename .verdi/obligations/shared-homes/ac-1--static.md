---
id: obligation/shared-homes--ac-1--static
kind: obligation
title: "One atomic-write helper, no surviving copies"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/shared-homes" }
frozen: { at: 2026-07-13, commit: b52051b1058e17bb26f1f54c79bdaa8d2dbec71d }
---
# One atomic-write helper, no surviving copies

The static evidence must show internal/atomicfile owning
MkdirAll+temp+write+close+fsync+rename and the four former hand copies
(boardio boardstate/graduate inlines, boardio's writeFileAtomic,
boardlayout/file.go) reduced to calls into it — the fsync durability gap
closed once, as dc-1 disclosed.
