---
id: obligation/showcase-drift-gate--ac-1--static
kind: obligation
title: "The three-axis capability inventory is enumerated mechanically and the gate is wired into verify"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/showcase-drift-gate" }
frozen: { at: 2026-07-15, commit: 53fb2ac893d88c9538b1221ffbc30d14b6eb7bf8 }
---
# The three-axis capability inventory is enumerated mechanically and the gate is wired into verify

The static evidence must show `internal/showcasealign`'s capability
inventory is built from source, not hand-guessed: the CLI axis
(`cli:<verb>`) parsed from `cmd/verdi/dispatch.go`'s `verbPhase` map
literal via `go/parser` (every entry with phase > 0, plus `lint`), the MCP
axis (`mcp:<tool>`) queried from a live `tools/list` call against
`internal/mcpserve`'s server exactly as
`internal/specalign/mcptools_test.go` already drives it, and the workbench
axis (`wb:<surface>`) from the one committed, hand-maintained
`workbenchSurfaces` list (spec §10 mitigation — the one axis with no
mechanical source of truth). It must further show that the committed
`showcaseCoverage` map keys every enumerated capability to a
`coverageEvidence` entry (a repo-relative file path plus a marker regexp:
`SHOWCASE\.` for a Playwright spec under `e2e/tests/`, or
`examples/showcase` for a Go e2e test — ledger L-B's two evidence forms),
and that `make verify`'s target list includes `lint-showcase` and
`showcase-coverage`, each invoking a named `go test` pattern so a CI
failure names the gate, not just "tests failed" — the same legibility
rationale `spec-align` already established.
