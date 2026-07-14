---
id: obligation/file-topics--ac-4--static
kind: obligation
title: "The harness has one git path, bounded I/O, early signals"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/file-topics" }
frozen: { at: 2026-07-14, commit: 15d60efbe02636c1112907ded017f80eb4c46e94 }
---
# The harness has one git path, bounded I/O, early signals

The static evidence must show one run-git helper carrying the
deterministic-date env, context plus a bounded client on the exec/HTTP
surface, signal.Notify installed before build/provision, copyTree
splitting absent from unreadable, and provision_board named for what it
seeds.
