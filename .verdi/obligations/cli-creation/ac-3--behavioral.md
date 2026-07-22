---
id: obligation/cli-creation--ac-3--behavioral
kind: obligation
title: "scaffolded obligation: ac-3 behavioral evidence"
owners: ["johnyang"]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/cli-creation" }
frozen: { at: 2026-07-22, commit: e1cd2d1f957a200804b97a78829482d1ca8b57f9 }
---
# scaffolded obligation: ac-3 behavioral evidence

This obligation was scaffolded at accept; not elaborated. It is a placeholder for ac-3's behavioral evidence, written by accept's
freeze-moment backstop because no obligation existed for this pair
when spec/cli-creation was accepted (spec/creation-surfaces#ac-4). Replace this body
with a first-person statement of what that evidence must specifically
show before relying on it — by hand, or via `verdi obligation author
spec/cli-creation ac-3 behavioral` on a design branch before the replacement itself freezes.
The acceptance criterion's own declared text, for reference:

design start --from-stub <feature> <stub> creates a story from a declared feature stub from the CLI for the first time, exactly as the board's own stub-instantiate action already does, because both now call one shared stub-instantiate core extracted out of internal/workbench/boardspecapi.go into its own package rather than a second CLI-side reimplementation drifting from the board's. Given the identical feature and stub, the two surfaces' rendered spec content is asserted equal — the parity proof that closes the ADJ-65 asymmetry at the mechanism, not merely at the surface — and the board's own existing stub-instantiate and creation-form handler tests pass completely unmodified, the proof that extracting the shared core changed no board behavior underneath it

