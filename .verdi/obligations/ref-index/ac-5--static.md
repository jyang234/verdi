---
id: obligation/ref-index--ac-5--static
kind: obligation
title: "The git-runner port's method set makes a checkout-mutating call impossible, not merely unused"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/ref-index" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# The git-runner port's method set makes a checkout-mutating call impossible, not merely unused

The static evidence must show, by reading the git-runner-port interface `internal/refindex.ComputeIndex` depends on (dc-2), that its full method set contains zero methods capable of moving HEAD or writing the working tree/index — no `Checkout`, no `Switch`, no generic `Run(args ...string)` escape hatch that could be handed arbitrary git subcommands. The guarantee must be at the interface's method signatures, not merely "the current implementation happens not to call such a method."
