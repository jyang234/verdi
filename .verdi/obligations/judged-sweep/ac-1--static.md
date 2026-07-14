---
id: obligation/judged-sweep--ac-1--static
kind: obligation
title: "--diagram-sweep is a new align mode with zero references from gate/lint's own source"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/judged-sweep" }
frozen: { at: 2026-07-14, commit: 1d78c2776983c7c08ae4a065727a828b2fd28825 }
---
# --diagram-sweep is a new align mode with zero references from gate/lint's own source

The static evidence must show `cmd/verdi/align.go` (or its equivalent
dispatch point) declares a `--diagram-sweep <diagram-ref>` flag that
routes to a new code path writing `.verdi/diagrams/<name>.sweep-report.md`,
distinct from the existing build-branch and design-branch decision-conflict
modes. The evidence must also show, by naming the files and grepping their
content, that `cmd/verdi/gate.go` (`runGate`) and `internal/lint`'s source
contain no reference to `sweep-report.md`, `DiagramSweepFrontmatter`, or
`DecodeDiagramSweep` anywhere — an absence demonstrated, not merely
asserted.
