---
id: obligation/showcase-drift-gate--ac-1--behavioral
kind: obligation
title: "make verify fails naming the exact missing capability and passes once every capability is showcase-backed"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/showcase-drift-gate" }
frozen: { at: 2026-07-15, commit: 53fb2ac893d88c9538b1221ffbc30d14b6eb7bf8 }
---
# make verify fails naming the exact missing capability and passes once every capability is showcase-backed

The behavioral evidence must show `TestShowcaseCoverage` (run standalone
and via `make showcase-coverage`) actually fails, and names the specific
offending key (for example, "showcase-coverage gap: cli:close has no
showcase-backed e2e evidence"), when a real capability — a `verbPhase`
entry, an MCP tool from a live `tools/list`, or a hand-listed workbench
surface — has no matching entry in `showcaseCoverage`, or when a mapped
entry's evidence file is missing or no longer matches its marker regexp;
the check runs in both directions, so an unmapped capability and a stale
mapping pointing at dead evidence are both named, never silently passed.
It must also show the gate passing clean — zero gaps — once every
enumerated CLI verb, MCP tool, and workbench surface has a real Playwright
spec matching `SHOWCASE\.` or a Go e2e test matching `examples/showcase`,
and that this same result is reached through `make verify` end to end,
not only the standalone test binary.
