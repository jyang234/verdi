---
id: spec/showcase-drift-gate
kind: spec
title: "Showcase Drift Gate"
owners: [platform-team]
class: story
status: accepted-pending-build
story: jira:VERDI-23
problem: { text: "make verify has no drift gate over the showcase: nothing in the repository enumerates what capabilities exist on any axis (CLI verb, MCP tool, workbench surface) or cross-checks that enumeration against e2e evidence, so a capability can ship, its own tests can stay green, and examples/showcase (spec/showcase-corpus-renovation's freshly vetted corpus) can silently stop demonstrating it — spec/public-showcase#ac-2 requires this to fail the build, by name, and nothing today does.", anchor: "#problem" }
outcome: { text: "a new internal/showcasealign package computes a three-axis capability-coverage inventory — CLI verbs parsed mechanically from dispatch.go, MCP tools queried live from tools/list, workbench surfaces hand-listed — checks every enumerated capability against a committed mapping to showcase-backed e2e evidence, and two new make targets (lint-showcase, showcase-coverage) wire this and the showcase's own lint-clean check into make verify, so an unshowcased capability is a named, red gate rather than a silent pass.", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "make verify fails with a named gap when a capability (CLI verb / MCP tool / workbench surface) has no showcase-backed e2e coverage", evidence: [static, behavioral], anchor: "#ac-1" }
links:
  - { type: implements, ref: "spec/public-showcase#ac-2" }
decisions:
  - { id: dc-1, text: "the capability inventory is derived mechanically wherever a mechanical source exists — CLI verbs parsed from cmd/verdi/dispatch.go's verbPhase composite literal (go/parser, every phase > 0 entry plus lint) and MCP tools from a live tools/list call against internal/mcpserve exactly as internal/specalign/mcptools_test.go already drives it — so new verbs and tools appear in the inventory automatically; only the workbench axis, which has no mechanical source of truth in the binary, is a single hand-maintained list (design doc §10's named risk and its own stated mitigation)", anchor: "#dc-1" }
  - { id: dc-2, text: "the coverage marker is one literal text a file's bytes must contain — SHOWCASE. for a Playwright spec under e2e/tests/, examples/showcase for a Go e2e test (ledger L-B) — checked by direct grep-equivalent match, not by import resolution or type analysis; fixtures.ts's header forbids re-aliasing the SHOWCASE export specifically because an alias (const S = SHOWCASE) would keep every spec passing while making the marker text disappear, defeating the gate by construction", anchor: "#dc-2" }
  - { id: dc-3, text: "the check is wired into make verify as two separately named targets, lint-showcase and showcase-coverage, rather than folded into the general test target — mirroring spec-align's existing precedent that a gate a CI failure must name gets its own target — positioned in the verify chain after the fast build/vet/lint/test/fixture gates and before e2e; showcase-coverage's go test -run pattern is authored to also select public-readme's forthcoming TestReadmeExamplesFresh, which does not yet exist at this story's own boundary and so is skipped vacuously until that sibling story lands it", anchor: "#dc-3" }
constraints:
  - { id: co-1, text: "no network in any test: the CLI axis is parsed from source text on disk, the MCP axis is driven through internal/mcpserve's in-process server exactly as the existing specalign harness drives it (loopback pipes, never a socket), and every evidence-file check reads the repository's own working tree — nothing in this gate ever reaches a live service", anchor: "#co-1" }
  - { id: co-2, text: "every gap is named, in both directions: an enumerated capability with no mapped evidence, and a mapped evidence file that no longer exists or no longer matches its marker, each produce their own finding naming the exact capability key (cli:<verb>, mcp:<tool>, or wb:<surface>) — the check never reports a bare pass/fail with no offending name (00 §Provenance discipline: silence is never a pass)", anchor: "#co-2" }
frozen: { at: 2026-07-15, commit: 47882ae8664a27e6e2d3769d5594d4e3a61b4cbc, stub_matched: true }
---
# Showcase Drift Gate

## Problem

`make verify` has no mechanism that keeps the showcase honest as the binary
grows. Nothing in the repository enumerates "every capability that exists"
on any of the three surfaces a reader could touch — a CLI verb, an MCP
tool, a workbench surface — and nothing cross-checks that enumeration
against what the e2e suite actually demonstrates against
`examples/showcase` (`spec/showcase-corpus-renovation`'s freshly vetted
corpus). A new verb can ship, `cmd/verdi`'s own unit tests can stay green,
`make verify` can stay green, and the showcase can silently stop being a
complete tour of the tool — coverage of *behavior* is gated, coverage of
*showcase* is not. `spec/public-showcase#ac-2` requires this gap to fail
the build with a named capability, not pass silently; nothing today
computes the inventory or wires a check into the gate.

## Outcome

A new `internal/showcasealign` package computes a three-axis capability
inventory: CLI verbs parsed mechanically from `cmd/verdi/dispatch.go`'s
`verbPhase` map (every verb with phase > 0, plus `lint`, which dispatches
before the phase check), MCP tools queried live from a real `tools/list`
call against `internal/mcpserve`'s server, and workbench surfaces from one
committed, hand-maintained list — the only axis with no mechanical source
of truth in the binary. Every enumerated capability is checked against a
committed `showcaseCoverage` mapping naming one piece of showcase-backed
e2e evidence per capability: a Playwright spec whose text matches
`SHOWCASE\.`, or a Go e2e test whose text matches `examples/showcase`
(ledger L-B's two evidence forms). Two new `make` targets, `lint-showcase`
and `showcase-coverage`, wire this inventory check and the showcase's own
`verdi lint`-clean check into `make verify`, and `e2e/tests/fixtures.ts` is
zoned into `SHOWCASE`/`EDGE` constant groups so the literal marker text
`SHOWCASE.` is one honest, ungameable coverage signal. A capability that
ships without showcase-backed evidence turns `make verify` red and names
exactly which capability is missing — the drift this story closes.

## AC-1

`make verify` fails with a named gap — which capability, on which axis,
lacking which evidence — when a capability has no showcase-backed e2e
coverage. The three axes are enumerated without hand-maintaining the two
that have a mechanical source: `cliVerbs` parses `cmd/verdi/dispatch.go`
with `go/parser`, walking the `verbPhase` composite literal for every key
whose value is greater than zero and appending `lint`; `mcpTools` drives a
live `tools/list` call against `internal/mcpserve.NewServer` exactly as
`internal/specalign/mcptools_test.go` already does. Workbench surfaces are
the one hand-listed axis (board, board-review-mode, board-scoping-canvas,
obligation-wall, wall-badges, wall-receipts, evidence-slot, diagram-editor,
diagram-tier, derivation-drawer, directory-home, draft-boards, dex,
dex-by-story, disclosures, presentation, ref-peek). Each enumerated
capability key (`cli:<verb>`, `mcp:<tool>`, `wb:<surface>`) must resolve to
a committed `coverageEvidence` entry whose named file exists and whose
bytes match its marker regexp; the check runs in both directions — an
unmapped capability and a mapping pointing at missing or non-matching
evidence are both named findings, never a silent pass and never a bare
failure with no offending key. `make lint-showcase` (the showcase's own
`verdi lint`-clean check) and `make showcase-coverage` (this inventory
check) are both wired into the `verify:` target so either failure is a
`make verify` failure, and both are separately named targets so a CI
failure names the gate directly, the same legibility `spec-align` already
established.

Evidence: **static** (the `verbPhase` parse, the live `tools/list` query,
the hand-listed workbench surfaces, and the committed `showcaseCoverage`
mapping are all inspectable directly from source — the inventory is
declared, not inferred, and the two new `make` targets are present in
`verify:`'s dependency chain) + **behavioral** (`go test
./internal/showcasealign/ -run TestShowcaseCoverage`, and `make verify`
end to end, fail naming the exact missing capability when a real
capability's mapping is removed or a new capability is added without one,
and pass clean once every enumerated capability has matching
showcase-backed evidence).

## DC-1

Mechanical wherever mechanical is possible. `dispatch.go`'s `verbPhase` and
a live `tools/list` call are both sources of truth already read elsewhere
in the repository (`internal/specalign`); parsing them here means a new
CLI verb or MCP tool appears in the inventory automatically. The workbench
axis has no such source in the binary (surfaces are workbench pages and
board affordances, not registered anywhere machine-readable), so it stays
one small, named, hand-maintained list — the design doc's own risk section
names exactly this axis as the one where the inventory can go stale, and
names exactly this mitigation.

## DC-2

One literal marker, not import resolution. The coverage signal is the
literal text a file's bytes contain — `SHOWCASE\.` in a Playwright spec
under `e2e/tests/`, `examples/showcase` in a Go e2e test (ledger L-B) —
checked the same way a human skimming a diff would notice it, not by
following imports or types. `fixtures.ts`'s header forbids re-aliasing the
`SHOWCASE` export (`const S = SHOWCASE` and similar) precisely because an
alias keeps every existing spec passing while making the marker text
disappear from the file that uses it — a mechanical way to defeat the gate
that the header calls out by name so nobody reaches for it by accident.

## DC-3

Two named targets, not one absorbed into `test`. `lint-showcase` and
`showcase-coverage` are separate `make` targets, added to `verify:`'s
chain after the fast build/vet/lint/test/fixture gates and before `e2e`,
mirroring `spec-align`'s existing precedent that a gate whose failure
needs to be named in CI output earns its own target rather than
disappearing into a general `test` run. `showcase-coverage`'s `go test
-run` pattern is written to select `public-readme`'s forthcoming
`TestReadmeExamplesFresh` as well as this story's own
`TestShowcaseCoverage`; at this story's own boundary the former does not
exist yet, so the pattern selects nothing for it and passes vacuously —
honest because `public-readme` is the next story in the same feature, not
a permanent gap.

## CO-1

No network, ever. The CLI axis is parsed from source text already on
disk; the MCP axis is driven entirely in-process through
`internal/mcpserve`'s server, over loopback pipes exactly as
`internal/specalign/mcptools_test.go` already drives it, never a real
socket; every evidence-file check reads the repository's own working
tree. Nothing in this gate reaches, or needs, a live service.

## CO-2

Every gap named, in both directions. An enumerated capability with no
mapped evidence is a finding; a mapped evidence file that no longer
exists, or no longer matches its marker regexp, is also a finding — each
names the exact capability key (`cli:<verb>`, `mcp:<tool>`, or
`wb:<surface>`) responsible. The check never reports a bare pass/fail with
no offending name: silence is never a pass (00 §Provenance discipline).
