---
id: obligation/board-editor--ac-5--behavioral
kind: obligation
title: "e2e sees the rail render a canned extractor report, disclose unavailability without one, and never block a save"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/board-editor" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# e2e sees the rail render a canned extractor report, disclose unavailability without one, and never block a save

The behavioral evidence must show Playwright e2e coverage proving the rail
consumes rather than computes: (1) with a canned verification report supplied
through the dc-4 consumer port (a hermetic fixture — never a live flowmap
run), the rail renders the report's coverage tier (full / partial /
illustrative) and its per-element findings with their kinds (exists /
proposed-new / contradicted / stale-base) as given, verbatim from the canned
input; (2) with no extractor wired, the rail renders a visible, disclosed
verification-unavailable state — not an empty region, not a fabricated tier;
(3) in BOTH states, editing the pane and saving still succeeds — the rail
never blocks an edit or a save. Evidence in which the workbench computes any
tier or finding itself, or in which the unavailable state is merely an
absent element, does not satisfy this obligation.
