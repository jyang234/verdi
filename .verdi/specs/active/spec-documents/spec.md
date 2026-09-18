---
id: spec/spec-documents
kind: spec
title: "Spec Documents"
owners: [platform-team]
class: feature
problem: { text: "Verdi's accepted specs are machine-shaped: the authoritative object list is YAML frontmatter with ids and anchors, no surface renders a spec as a document a reviewer or an agent can read top to bottom, no export exists, the coding-agent workflow has no verdi-native entry points, adopting a project leaves every board with an unproven policy check, and the default vocabulary is Verdi's internal one.", anchor: problem }
outcome: { text: "A deterministic, stamped document projection of every spec, plan, and task list is available from the CLI, the board, the docs site, and MCP, byte-identical across all four; coding agents drive specify, clarify, plan, and tasks through verdi-generated, drift-checked skills over the existing MCP and import surfaces; a project adopts a starter constitution with one verb; new stores speak plain words by default and readiness cards lead with what to do next. Verdi matches spec-kit's speed to a readable draft and remains the only one of the two that proves the built thing matches the spec.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "A shared document core builds one canonical document model from a spec's decoded objects and body at an exact commit, the projection facts the workbench already computes (stub coverage per criterion, obligation and story state from the matrix fold, supersession classification), and the readiness snapshot when present; renders it as Markdown and HTML in three kinds (spec, plan, tasks); is byte-deterministic for identical inputs; stamps every render with ref, commit, engine digest, and the sentence that it is derived from accepted objects and not authority; states \"proposed, not accepted\" in the header for unmerged bytes; and never omits a section whose facts are unavailable, saying instead that they are unavailable.", evidence: [behavioral, attestation], anchor: ac-1 }
  - { id: ac-2, text: "`verdi spec doc <spec/ref> [--kind spec|plan|tasks] [--format md|html] [--at <commit>] [--proposed] [-o <path>]` renders the document from the ref's accepted bytes on the default branch by default, from a pinned commit with --at, or from the current design branch with --proposed; writes to stdout or -o; exits 0 on a render and 2 on an unresolvable ref, commit, or store; never exits 1.", evidence: [behavioral, attestation], anchor: ac-2 }
  - { id: ac-3, text: "Every spec board offers a Document tab at /board/spec/{name}/document with /board/spec/{name}/document/snapshot as its conditional projection route: a revision token over the rendered facts, two-second polling only while visible, 304 on an unchanged token, only the document region replaced on change, focus and scroll preserved, a copy-as-Markdown control, and a download control serving the same bytes with a content-disposition header; a design-branch board renders the proposed state and the sealed wall the accepted state; one accepted-HEAD resolution per page; no new JavaScript asset over 64 KiB.", evidence: [behavioral, attestation], anchor: ac-3 }
  - { id: ac-4, text: "The docs site's per-spec page gains a Document view produced by the core at the site's pinned commit beside the existing verbatim body, and writes the spec, plan, and tasks Markdown files beside the page; the verbatim body remains and is unchanged.", evidence: [behavioral, attestation], anchor: ac-4 }
  - { id: ac-5, text: "A read-only MCP tool get_document(ref, kind, commit?) returns the Markdown and its stamps; the MCP write surface remains mutate_draft and add_annotation plus the import tools of ac-9.", evidence: [behavioral, attestation], anchor: ac-5 }
  - { id: ac-6, text: "One parity test renders the same ref through the CLI, the board tab, the docs site, and get_document and asserts byte-identical Markdown after each consumer's shell wrapper is stripped; any divergence fails the gate.", evidence: [behavioral, attestation], anchor: ac-6 }
  - { id: ac-7, text: "`verdi harness render [--host claude|codex|all] [-o <repo root>]` writes the four skills for Claude Code (.claude/skills/verdi-{specify,clarify,plan,tasks}/SKILL.md) and their Codex prompt equivalents from templates embedded in the binary, each stamped with engine digest, template digest, and render commit and marked as generated; `verdi harness check` recomputes and exits 1 on drift; make verify runs the check for this repository.", evidence: [behavioral, attestation], anchor: ac-7 }
  - { id: ac-8, text: "The four skills behave as specified: specify runs the import source and preview steps through the delegated-agent actor, shows the human the preview and its digest, and applies only on confirmation; clarify lists unproven readiness concerns and unclaimed open questions for a draft and proposes decisions or spike stubs through mutate_draft one at a time, each shown before it is written; plan reads the plan document and proposes stubs for uncovered criteria through mutate_draft; tasks reads the tasks document and writes nothing; each skill's tool sequence is proven by a hermetic MCP or CLI transcript.", evidence: [behavioral, attestation], anchor: ac-8 }
  - { id: ac-9, text: "MCP tools import_preview and import_apply wrap the existing import service with the delegated-agent actor and the preview-digest handshake the CLI enforces; the import record names the harness and session that proposed the mapping; byte-offset provenance, coverage accounting, and user-edited-source marking are unchanged.", evidence: [behavioral, attestation], anchor: ac-9 }
  - { id: ac-10, text: "`verdi policy adopt --starter [--profile solo|team]` renders the constitution, one profile, one policy, and the consumers inventory through the existing scaffold renderer with template override, records each in the policy TemplateRecord seam, commits exactly those paths on a policy/adopt branch, adds no artifact kind, replaces the in-product guide's no-wizard disclaimer with the verb, and makes the context/policy board concern name the verb; every rendered rule is a real minimal rule, not a placeholder.", evidence: [behavioral, attestation], anchor: ac-10 }
  - { id: ac-11, text: "A vocabulary preset named plain (planned story, research task, revision, research spike) is written by verdi init for a new store by default, with --vocabulary canonical opting out and existing stores untouched; lint output leads with the sentence and trails the rule code in brackets; the renameable set and the vocabulary witness are unchanged.", evidence: [behavioral, attestation], anchor: ac-11 }
  - { id: ac-12, text: "On the readiness page and the wall shell each concern card shows its guidance sentence as the primary line with the concern id, timing, and blocking flag in the existing technical disclosure; the four area labels and the proven, needs-attention, not-enough-evidence triad are unchanged; no derivation changes.", evidence: [behavioral, attestation], anchor: ac-12 }
constraints:
  - { id: co-1, text: "No network in any test. CLI paths are exercised through the built binary; browser paths through Playwright under e2e/; MCP paths through hermetic transcripts; git and forge through fixtures.", anchor: co-1 }
  - { id: co-2, text: "The document is never authority. No verb reads a document back into objects, no rendered file is a store artifact, and every render carries the not-authority stamp of ac-1.", anchor: co-2 }
  - { id: co-3, text: "The write surface stays closed: mutate_draft, add_annotation, and the import tools of ac-9. The skills and the document core add no other mutation path.", anchor: co-3 }
  - { id: co-4, text: "All workbench, docs-site, and skill-visible markup (ac-3, ac-4, ac-12, and the Playwright paths) is implemented by Fable subagents; screenshot, trace, video, and screen recording stay disabled.", anchor: co-4 }
  - { id: co-5, text: "Every new production literal routes class and status words through the model display chain or is classified at the producing site; the vocabulary witness is never weakened.", anchor: co-5 }
  - { id: co-6, text: "Three-valued honesty holds in every document and card: a section with unavailable facts says so; a proposed spec is labeled proposed; a quieter card is still a disclosure.", anchor: co-6 }
decisions:
  - { id: dc-1, text: "Approach A: one document core in a new package with four thin consumers (CLI, board tab, docs site, MCP), rejecting a docs-site-only export and rejecting rendering the document inside the wall.", anchor: dc-1 }
  - { id: dc-2, text: "Binding presentation authority: 05 §Board as projection (R4), spec/scoping-canvas ac-7 (the feature document stays downward-blind; coverage is computed, never declared), and the Wave 6 workbench presentation design (shared shell, closed route grammar, conditional live refresh, performance budgets). The Document tab is a page route plus a /snapshot projection route under that grammar.", anchor: dc-2 }
  - { id: dc-3, text: "Document shape serves both readers in one document: narrative order (identity, problem, outcome, decisions with rationale, constraints, numbered criteria with evidence kinds and computed coverage, open questions with their claiming spike or 'unclaimed, blocks acceptance', plan as stubs with covered criteria, trailing evidence with obligation state) with ids as unobtrusive anchors. The tasks kind is plan plus evidence; the plan kind is decisions, constraints, and plan; spec is everything.", anchor: dc-3 }
  - { id: dc-4, text: "Skills are projections, not authority (spec/context-integrity dc-1 and dc-2): generated from templates embedded in the binary, stamped, drift-checked, re-rendered by verdi, never hand-edited. The Codex prompt format follows the parity established by spec/sealed-claude-vatc-events.", anchor: dc-4 }
  - { id: dc-5, text: "Brief-driven drafting reuses the frozen import contract (docs/superpowers/specs/2026-09-14-spec-import-contract.md) end to end; ac-9 adds only the MCP wrapping and the proposing-harness record.", anchor: dc-5 }
  - { id: dc-6, text: "The starter policy scaffold lands under spec/context-integrity dc-4's configurable-scaffold seam using policyartifact.TemplateRecord, before that spec's acceptance, with a PLAN.md §7 ledger note; it closes tracker UAT-018, which spec/uat-round-1 dc-1 deferred.", anchor: dc-6 }
  - { id: dc-7, text: "The plain vocabulary is a preset, not a kernel change: configuration in .verdi/model.yaml's Vocabulary block written by the init wizard; existing stores are never rewritten.", anchor: dc-7 }
  - { id: dc-8, text: "Risk tiers: Tier 3 for ac-1, ac-5, ac-9, ac-10 (authority and provenance); Tier 2 for the rest. Waves: 1 = ac-1, ac-2; 2 = ac-3, ac-4, ac-5, ac-6; 3 = ac-7, ac-8, ac-9; 4 = ac-10, ac-11, ac-12. Every wave ends with the full gate; wave 2 and wave 4 board work is Fable work under co-4.", anchor: dc-8 }
  - { id: dc-9, text: "Out of scope: PDF; prose-first authoring (facts remain the source; prose is generated from them); any new spec object kind; journey metrics; the experiment proof coordinator; presenting effective policy rules in the workbench (spec/context-integrity ac-6); editing the document in place.", anchor: dc-9 }
open_questions:
  - { id: oq-1, text: "Where Codex reads prompt files in a consuming repository and how a generated prompt is distinguished from a hand-written one on that host is not yet pinned by a ratified surface; the render location and the marker for the Codex output of ac-7 are open until the spike answers them.", anchor: oq-1 }
stubs:
  - { slug: document-core, acceptance_criteria: [ac-1] }
  - { slug: spec-doc-cli, acceptance_criteria: [ac-2] }
  - { slug: board-document-tab, acceptance_criteria: [ac-3] }
  - { slug: docs-site-document, acceptance_criteria: [ac-4] }
  - { slug: mcp-get-document, acceptance_criteria: [ac-5] }
  - { slug: document-parity, acceptance_criteria: [ac-6] }
  - { slug: harness-render, acceptance_criteria: [ac-7] }
  - { slug: agent-skills, acceptance_criteria: [ac-8] }
  - { slug: import-mcp-tools, acceptance_criteria: [ac-9] }
  - { slug: policy-adopt-starter, acceptance_criteria: [ac-10] }
  - { slug: plain-vocabulary, acceptance_criteria: [ac-11] }
  - { slug: readiness-guidance-first, acceptance_criteria: [ac-12] }
  - { slug: codex-prompt-conventions, spike: true, resolves: [oq-1] }
---
# Spec Documents

## Problem

Verdi's accepted spec is a verifiable object list: YAML frontmatter with ids,
anchors, evidence kinds, and typed links, followed by id-keyed body sections.
That shape is the reason lint, readiness, supersession fidelity, and
attestation can be proven, and it is the reason a product manager opening the
file cold sees YAML. No surface renders a spec as a document a reviewer or a
coding agent reads top to bottom; the docs site shows the authored body
verbatim, and no export exists. The coding-agent workflow has no verdi-native
entry points, so the fast path from a brief to a draft lives outside the tool.
Adopting a project creates no policy authority, so every board shows an
unproven "Check constraints" item with a manual four-file guide that disclaims
any wizard (tracker UAT-018, deferred by spec/uat-round-1 dc-1). The default
vocabulary is Verdi's internal one. Spec-kit wins the first look on all four
counts while offering no verification at all.

## Outcome

Every spec, plan, and task list has a deterministic, stamped document
projection available from the CLI, the board, the docs site, and MCP,
byte-identical across all four and never mistakable for the spec. Coding
agents drive specify, clarify, plan, and tasks through verdi-generated,
drift-checked skills over the existing write tools and the import contract. A
project adopts a starter constitution with one verb. New stores speak plain
words by default, and readiness cards lead with what to do next. Verdi matches
spec-kit's speed to a readable draft and stays the only one of the two that
proves the built thing matches the spec.

## ac-1

The core is a new package with one concern: turning facts into a document. It
computes nothing it can receive: coverage comes from the stub views, obligation
and story state from the matrix fold, classification from the supersession
manifest, readiness from the snapshot. Determinism is a golden-file property
over committed fixtures.

## ac-2

The verb is the canonical text form and the reference every other consumer is
compared against (ac-6). It never exits 1 because a document is not a verdict.

## ac-3

The tab is a page route plus a projection route under the Wave 6 grammar
(dc-2). Copy and download are the only new controls; download is the only new
response type. Fable work under co-4.

## ac-4

The verbatim body is the spec; the Document view is the reading of it. The
three Markdown files exist so a static site can be linked to for review.

## ac-5

Read-only. Adds nothing to the write surface (co-3).

## ac-6

Parity is asserted on bytes, not on style: one ref, four consumers, one
Markdown after wrapper stripping.

## ac-7

Projections, not authority (dc-4). The templates live in the binary so the
skills cannot drift from the engine that serves them; the check runs in this
repository's own gate.

## ac-8

Each skill writes only through mutate_draft, add_annotation, or the import
verb, and shows the human every proposal before it is written. The proof is a
transcript of tool calls, not prose.

## ac-9

Reuses the frozen import contract end to end (dc-5); the only additions are
the MCP wrapping and the proposing-harness record.

## ac-10

Closes UAT-018 under spec/context-integrity dc-4's scaffold seam (dc-6). Every
rendered rule is real; the guide's disclaimer becomes a pointer to the verb.

## ac-11

A preset (dc-7): configuration written at init, never applied to an existing
store. Lint codes stay; the sentence leads.

## ac-12

Presentation only, inside the Wave 6 shared shell: guidance first, formal
identifiers second. Fable work under co-4.

## co-1

No network in any test.

## co-2

The document is never authority.

## co-3

The write surface stays closed.

## co-4

Browser-facing work is Fable work with Playwright paths; recording stays off.

## co-5

Vocabulary discipline holds for every new literal.

## co-6

Three-valued honesty holds in every document and card.

## dc-1

Approach A over a docs-site-only export (fast, but no live preview, no agent
access, no skills) and over rendering inside the wall (serves neither reader).

## dc-2

The binding presentation authority for the tab and the cards.

## dc-3

One document for both readers.

## dc-4

Skills as projections.

## dc-5

Brief-driven drafting reuses the import contract.

## dc-6

The starter scaffold's authority and ledger note.

## dc-7

Plain vocabulary is configuration.

## dc-8

Risk tiers and waves.

## dc-9

Out of scope.

## oq-1

Answered by the codex-prompt-conventions spike before wave 3 dispatches ac-7's
Codex output; the Claude Code output is not blocked by it.
