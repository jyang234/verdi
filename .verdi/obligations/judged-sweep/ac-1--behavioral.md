---
id: obligation/judged-sweep--ac-1--behavioral
kind: obligation
title: "A CLI test proves the sweep writes its report and a gate test proves an undispositioned finding never blocks"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/judged-sweep" }
frozen: { at: 2026-07-14, commit: 1d78c2776983c7c08ae4a065727a828b2fd28825 }
---
# A CLI test proves the sweep writes its report and a gate test proves an undispositioned finding never blocks

The behavioral evidence must show a test invoking
`verdi align --diagram-sweep diagram/<name>` (or its exported Go entry
point) over a fixture `class: proposal` diagram, asserting
`.verdi/diagrams/<name>.sweep-report.md` is created with a well-formed
`verdi.diagramsweep/v1` frontmatter. It must also show a SEPARATE test
that runs `verdi gate` (or `runGate`) over a build whose corpus contains a
sweep-report.md carrying an undispositioned finding, asserting the gate's
pass/fail outcome is completely unaffected by that finding's presence or
disposition state — proving co-1's "never in any gate's deterministic
path" behaviorally, not just by source absence.
