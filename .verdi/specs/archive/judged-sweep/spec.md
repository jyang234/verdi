---
id: spec/judged-sweep
kind: spec
title: "Judged Sweep"
owners: [platform-team]
class: story
status: closed
story: jira:VERDI-13
problem: { text: "a diagram proposal has no scrutiny-predictor: nothing reads a proposed change against the corpus of ADRs, constraints, and decisions and warns a designer what review will catch before it catches it. Every other judged surface this codebase has (the build-branch deviation report, the design-branch decision-conflict sweep) is EITHER mandatory machinery in a gate's own path or scoped to specs, never diagrams; a proposal author has no on-demand, disposable way to ask 'does this collide with something we already decided?' without waiting for a human reviewer to notice.", anchor: problem }
outcome: { text: "an on-demand verdi align --diagram-sweep <diagram-ref> mode reads a class: proposal diagram's mermaid body against the ADR/constraint/decision corpus through the SAME judge exec seam the design-branch decision-conflict sweep already proved, reusing the existing four-value ConflictFinding disposition machinery unchanged; findings are provenance-stamped (the existing judged-content integrity contract) and persisted to a sibling sweep-report file, never consulted by any gate, never run except on demand, and never phrased as a completeness guarantee — the AI reads and reports, a human disposes, and nothing here ever edits the diagram it just read.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "verdi align --diagram-sweep <diagram-ref> is a new on-demand mode of the existing align verb (not a new verb, not an MCP write tool) that writes a sibling .verdi/diagrams/<name>.sweep-report.md and is never read, invoked, or required by verdi gate, verdi lint, or any CI-run path", evidence: [static, behavioral], anchor: ac-1 }
  - { id: ac-2, text: "the sweep's judged findings reuse the existing judge exec seam (execJudgeEnvelope/JudgeRunner) and the existing four-value ConflictFinding/ConflictDisposition machinery unchanged, reading the proposal's mermaid body against the ADR corpus and the corpus's declared constraints/decisions", evidence: [static, behavioral], anchor: ac-2 }
  - { id: ac-3, text: "the sweep report is provenance-stamped exactly like existing judged content: an integrity hash of stdin+raw result (computeIntegrity, reused verbatim) plus the persisted judge_integrity exchange, both independently recomputable/verifiable and never claimed as reproducible", evidence: [static, behavioral], anchor: ac-3 }
  - { id: ac-4, text: "the sweep is provably read-only against the diagram it reads (a byte-identity test), and its rendered output never phrases a finding or an absent finding as a completeness guarantee", evidence: [static, behavioral], anchor: ac-4 }
links:
  - { type: implements, ref: "spec/diagram-proposals#ac-8" }
decisions:
  - { id: dc-1, text: "invocation surface: a new --diagram-sweep <diagram-ref> flag on the existing verdi align verb, not a new CLI verb and not an MCP write tool. 05 §CLI already grows align a design-branch decision-conflict MODE (R4-I-7) alongside its build-branch mode; this is the same pattern's third mode, the smallest reversible addition — no new verb to teach, no new dispatch table entry. An MCP tool was considered and rejected: 05 §MCP server's write surface is deliberately narrow (add_annotation is the only write tool; get_board's read-only growth is explicit precedent for extending the READ surface, never the write one) and a sweep-triggering tool would be a second, competing write path outside that narrow surface — a CLI flag keeps the human (or an agent shelling out, same as any other verb) in the same on-demand, disposable posture every other align mode already has", anchor: dc-1 }
  - { id: dc-2, text: "disposition vocabulary: reuse artifact.ConflictFinding and its existing four-value ConflictDisposition (superseded/exempt/rejected/no-conflict) verbatim rather than minting a fifth fix/rebut/carry enum the feature's own prose (ac-8) suggests. The mapping: 'fix' is the human's own out-of-band act of editing the proposal, which naturally supersedes the finding on the NEXT sweep (not a disposition value itself — no new value needed, mirroring how a decision-conflict finding's underlying decision changing makes it stale, not superseded-as-a-disposition); 'rebut' is rejected; 'carry' is exempt (a knowingly accepted tradeoff, the same 'stays valid, excused with a reason' semantics the closed edge vocabulary's exempts type already carries). One disposition vocabulary across every judged-finding surface, never a diagram-specific copy (CLAUDE.md: shared code, one source of truth)", anchor: dc-2 }
  - { id: dc-3, text: "package/file layout: internal/align gains diagram_judge.go, mirroring decision_judge.go's shape exactly (prompt builder, inner-result decode, RunDiagramSweep entry point) and reusing judge.go's execJudgeEnvelope/JudgeRunner/computeIntegrity unchanged; internal/artifact gains diagramsweep.go declaring DiagramSweepFrontmatter (schema verdi.diagramsweep/v1, covers, findings: []ConflictFinding, sweep_provenance, integrity, judge_integrity, provenance) — a sibling shape to DecisionConflictFrontmatter, reusing ConflictFinding/ConflictDisposition/SweepProvenance types directly rather than redeclaring them", anchor: dc-3 }
  - { id: dc-4, text: "persistence home: .verdi/diagrams/<name>.sweep-report.md, a sibling file to <name>.mermaid (mirroring how decision-conflict-report.md sits beside spec.md in a spec's own directory) — the smallest-invention path, since a diagram artifact is a single file with no directory of its own to nest a report inside. VL-002's singleFileKindDir mapping is untouched: a sweep-report.md is not itself a kind: diagram artifact, so it is decoded by its own schema seam (DecodeDiagramSweep), not routed through DecodeDiagram", anchor: dc-4 }
  - { id: dc-5, text: "read-only enforcement and the no-completeness-claim: RunDiagramSweep's signature takes the diagram's ALREADY-READ body bytes as an input parameter (never a path it could write to) — the type system itself makes an edit-back impossible from within this function, and a behavioral test confirms the diagram file's bytes are unchanged after a real sweep run. The rendered report's header (a fixed disclosure line BuildDiagramSweepPrompt's own render carries) states the sweep is advisory and non-exhaustive verbatim, mirroring how RunJudged's absence finding already discloses failure rather than implying success — never letting silence, or a clean sweep, read as 'nothing to find here'", anchor: dc-5 }
constraints:
  - { id: co-1, text: "never in any gate's deterministic path (parent co-1): verdi gate's source contains no reference to sweep-report.md or DiagramSweepFrontmatter, verified by a static grep-style check in this story's own test suite, not merely by convention", anchor: co-1 }
  - { id: co-2, text: "no network in any test (parent co-2): the judge exchange is exercised over the SAME fake JudgeRunner seam decision_judge_test.go already establishes; the corpus read (ADRs, a spec's declared decisions/constraints) is exercised over a fixture corpus, never a live judge binary or live corpus fetch", anchor: co-2 }
frozen: { at: 2026-07-14, commit: 1f7f9fc4b769bd20f47bd4620ef6ad3c3cec043e, stub_matched: true }
---
# Judged Sweep

## Problem

A diagram proposal has no scrutiny-predictor. Nothing reads a proposed
change against the corpus of ADRs, constraints, and decisions and warns a
designer what review will catch before it catches it. Every other judged
surface this codebase has — the build-branch deviation report, the
design-branch decision-conflict sweep — is either mandatory machinery in a
gate's own path or scoped to specs, never diagrams. A proposal author has
no on-demand, disposable way to ask "does this collide with something we
already decided?" without waiting for a human reviewer to notice.

## Outcome

An on-demand `verdi align --diagram-sweep <diagram-ref>` mode reads a
`class: proposal` diagram's mermaid body against the ADR/constraint/
decision corpus through the SAME judge exec seam the design-branch
decision-conflict sweep already proved, reusing the existing four-value
`ConflictFinding` disposition machinery unchanged. Findings are
provenance-stamped (the existing judged-content integrity contract) and
persisted to a sibling sweep-report file, never consulted by any gate,
never run except on demand, and never phrased as a completeness
guarantee. The AI reads and reports; a human disposes; nothing here ever
edits the diagram it just read.

## AC-1

`verdi align --diagram-sweep <diagram-ref>` is a new on-demand mode of the
existing `align` verb (not a new verb, not an MCP write tool). It writes a
sibling `.verdi/diagrams/<name>.sweep-report.md` and is never read,
invoked, or required by `verdi gate`, `verdi lint`, or any CI-run path.
Evidence: static (the flag's dispatch in `cmd/verdi/align.go` and an
exhaustive grep-style check that `runGate`/`lint`'s source has zero
references to the sweep report or its frontmatter type) + behavioral (a
CLI test invoking the new flag over a fixture proposal and asserting the
report file's existence and shape, plus a gate-run test over the same
fixture confirming an undispositioned sweep finding never blocks it).

## AC-2

The sweep's judged findings reuse the existing judge exec seam
(`execJudgeEnvelope`/`JudgeRunner`) and the existing four-value
`ConflictFinding`/`ConflictDisposition` machinery unchanged, reading the
proposal's mermaid body against the ADR corpus and the corpus's declared
constraints/decisions. Evidence: static (the prompt builder and inner
decode reuse `judge.go`'s exported plumbing, with no second exec path) +
behavioral (a fixture judge response producing a finding, decoded into a
`ConflictFinding`, and a fixture judge-absent case degrading to the
synthetic absence finding exactly like the decision-conflict sweep's own
does).

## AC-3

The sweep report is provenance-stamped exactly like existing judged
content: an integrity hash of stdin+raw result (`computeIntegrity`, reused
verbatim) plus the persisted `judge_integrity` exchange, both
independently recomputable/verifiable and never claimed as reproducible.
Evidence: static (the frontmatter's `integrity`/`judge_integrity` fields
and their `Validate` rules, mirroring `DecisionConflictFrontmatter`
exactly) + behavioral (a round-trip test: generate a report, recompute
`computeIntegrity` from its own persisted `judge_integrity` fields, and
assert it matches the stored `integrity` value).

## AC-4

The sweep is provably read-only against the diagram it reads (a
byte-identity test), and its rendered output never phrases a finding or an
absent finding as a completeness guarantee. Evidence: static
(`RunDiagramSweep`'s signature takes the diagram's already-read body bytes
as a parameter, never a writable path, and the rendered report's header
carries a fixed, non-empty advisory/non-exhaustive disclosure line) +
behavioral (a test that SHA-256s the target diagram file before and after
a real sweep run and asserts byte-identity, plus a test asserting the
disclosure line is present verbatim on both a findings-present and a
findings-absent report).

## DC-1

Invocation surface: a new `--diagram-sweep <diagram-ref>` flag on the
existing `verdi align` verb, not a new CLI verb and not an MCP write
tool. 05 §CLI already grows `align` a design-branch decision-conflict MODE
(R4-I-7) alongside its build-branch mode; this is the same pattern's third
mode, the smallest reversible addition — no new verb to teach, no new
dispatch table entry. An MCP tool was considered and rejected: 05 §MCP
server's write surface is deliberately narrow (`add_annotation` is the
only write tool; `get_board`'s read-only growth is the explicit
precedent for extending the READ surface, never the write one) and a
sweep-triggering tool would be a second, competing write path outside
that narrow surface — a CLI flag keeps the human (or an agent shelling
out, same as any other verb) in the same on-demand, disposable posture
every other `align` mode already has.

## DC-2

Disposition vocabulary: reuse `artifact.ConflictFinding` and its existing
four-value `ConflictDisposition` (`superseded`/`exempt`/`rejected`/
`no-conflict`) verbatim rather than minting a fifth fix/rebut/carry enum
the feature's own prose (`ac-8`) suggests. The mapping: "fix" is the
human's own out-of-band act of editing the proposal, which naturally
supersedes the finding on the NEXT sweep (not a disposition value itself
— no new value needed, mirroring how a decision-conflict finding's
underlying decision changing makes it stale, not superseded-as-a-
disposition); "rebut" is `rejected`; "carry" is `exempt` (a knowingly
accepted tradeoff, the same "stays valid, excused with a reason" semantics
the closed edge vocabulary's `exempts` type already carries). One
disposition vocabulary across every judged-finding surface, never a
diagram-specific copy (CLAUDE.md: shared code, one source of truth).

## DC-3

Package/file layout: `internal/align` gains `diagram_judge.go`, mirroring
`decision_judge.go`'s shape exactly (prompt builder, inner-result decode,
`RunDiagramSweep` entry point) and reusing `judge.go`'s
`execJudgeEnvelope`/`JudgeRunner`/`computeIntegrity` unchanged;
`internal/artifact` gains `diagramsweep.go` declaring
`DiagramSweepFrontmatter` (schema `verdi.diagramsweep/v1`, `covers`,
`findings: []ConflictFinding`, `sweep_provenance`, `integrity`,
`judge_integrity`, `provenance`) — a sibling shape to
`DecisionConflictFrontmatter`, reusing `ConflictFinding`/
`ConflictDisposition`/`SweepProvenance` types directly rather than
redeclaring them.

## DC-4

Persistence home: `.verdi/diagrams/<name>.sweep-report.md`, a sibling
file to `<name>.mermaid` (mirroring how `decision-conflict-report.md` sits
beside `spec.md` in a spec's own directory) — the smallest-invention path,
since a diagram artifact is a single file with no directory of its own to
nest a report inside. `VL-002`'s `singleFileKindDir` mapping is untouched:
a sweep-report.md is not itself a `kind: diagram` artifact, so it is
decoded by its own schema seam (`DecodeDiagramSweep`), not routed through
`DecodeDiagram`.

## DC-5

Read-only enforcement and the no-completeness-claim. `RunDiagramSweep`'s
signature takes the diagram's ALREADY-READ body bytes as an input
parameter (never a path it could write to) — the type system itself makes
an edit-back impossible from within this function, and a behavioral test
confirms the diagram file's bytes are unchanged after a real sweep run.
The rendered report's header (a fixed disclosure line
`BuildDiagramSweepPrompt`'s own render carries) states the sweep is
advisory and non-exhaustive verbatim, mirroring how `RunJudged`'s absence
finding already discloses failure rather than implying success — never
letting silence, or a clean sweep, read as "nothing to find here."

## CO-1

Never in any gate's deterministic path (parent `co-1`): `verdi gate`'s
source contains no reference to `sweep-report.md` or
`DiagramSweepFrontmatter`, verified by a static grep-style check in this
story's own test suite, not merely by convention.

## CO-2

No network in any test (parent `co-2`): the judge exchange is exercised
over the SAME fake `JudgeRunner` seam `decision_judge_test.go` already
establishes; the corpus read (ADRs, a spec's declared decisions/
constraints) is exercised over a fixture corpus, never a live judge
binary or live corpus fetch.
