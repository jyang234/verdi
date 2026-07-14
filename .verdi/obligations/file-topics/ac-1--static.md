---
id: obligation/file-topics--ac-1--static
kind: obligation
title: "The bootstrap helpers live in a named topic file"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/file-topics" }
frozen: { at: 2026-07-14, commit: 15d60efbe02636c1112907ded017f80eb4c46e94 }
---
# The bootstrap helpers live in a named topic file

The static evidence must show the four helpers relocated verbatim to a
cmd/verdi topic file whose doc header names the store/forge-bootstrap
topic, sync.go reduced to sync's own verb logic, and gate_threads.go's
stale home-pointer comment corrected.
