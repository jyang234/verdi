---
id: obligation/verb-surfaces--ac-3--behavioral
kind: obligation
title: "scaffolded obligation: ac-3 behavioral evidence"
owners: ["johnyang"]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/verb-surfaces" }
frozen: { at: 2026-07-22, commit: fc77362bd91d5e77b199e22ddd22bd272132e4f7 }
---
# scaffolded obligation: ac-3 behavioral evidence

This obligation was scaffolded at accept; not elaborated. It is a placeholder for ac-3's behavioral evidence, written by accept's
freeze-moment backstop because no obligation existed for this pair
when spec/verb-surfaces was accepted (spec/creation-surfaces#ac-4). Replace this body
with a first-person statement of what that evidence must specifically
show before relying on it — by hand, or via `verdi obligation author
spec/verb-surfaces ac-3 behavioral` on a design branch before the replacement itself freezes.
The acceptance criterion's own declared text, for reference:

verdi audit gains a waiver-audit section wired into decisionsweep.Audit (the X-18 counterweight's own counting site, beside its existing exemption and spec-stale sections): for every story spec with at least one waiver file, it lists each waiver's AC, status, and expiry, discloses whether an active-status waiver's recorded expiry has already lapsed by wall-clock at the audit invocation (never baked into a generated/frozen artifact — an ephemeral stdout read exactly like the existing closure-hygiene section's own git-state reads), excludes a lapsed waiver from the counted-active total, and flags a story whose active count exceeds a configured threshold (verdi.yaml audit.waivers_stale_threshold, decoded by internal/store.AuditConfig alongside the two existing thresholds, defaulting to 3 exactly as deviations_stale_threshold already does when absent or non-positive) — contributing to the same FLAGGED/exit-1 outcome the existing sections already produce, as its own clearly-labeled count, never merged into the accepted-deviations budget; a lifecycle test proves an active waiver under threshold passes clean, crossing the threshold flags by name, and an expired-status or lapsed-by-date waiver is excluded from the count while still disclosed in the listing

