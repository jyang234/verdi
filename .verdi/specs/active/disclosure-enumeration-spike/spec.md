---
id: spec/disclosure-enumeration-spike
kind: spec
title: "Disclosure enumeration spike"
owners: [platform-team]
class: story
status: draft
spike: true
story: jira:VERDI-R5-1
problem: { text: "spec/disclosure-legibility's oq-1 is open: whether disclosures should be machine-enumerable (MCP/audit surface) and what an enumeration item should carry is unanswered, and story-2 (disclosures-panel) cannot be specced responsibly without a concrete recommendation", anchor: "#problem" }
outcome: { text: "a concrete, timeboxed recommendation on MCP-enumerability of disclosures and a specified enumeration item shape, precise enough for story-2's spec to consume directly", anchor: "#outcome" }
links:
  - { type: resolves, ref: "spec/disclosure-legibility#oq-1" }
---
# Disclosure enumeration spike

## Problem

`spec/disclosure-legibility` froze with an open question, oq-1: whether
disclosures should be machine-enumerable and, if so, what belongs in the
enumeration. The feature spec is frozen and downward-blind — it cannot
carry the answer itself — but story-2 (`disclosures-panel`, ac-1 + ac-2)
needs a concrete shape to build against, not an open question. This spike
exists to close that gap before story-2 is designed.

## Outcome

A single design answer document, timeboxed and scoped to reading (not
changing) the existing disclosure call sites, that:

- recommends whether disclosures should be MCP-enumerable, with reasons
  grounded in what the codebase already does today (not what similar tools
  do — CLAUDE.md's provenance discipline);
- if the recommendation is yes, specifies the enumeration item shape
  concretely enough to consume: source, text, severity (if any), stable id
  (if any), and how the shape maps onto the disclosure mechanisms that
  already exist (lint's `Finding.Severity`, `gateCondition.Disclosed`,
  mcpserve's ad hoc `review_unavailable`-style fields, workbench's
  `proj.Notices`).

## The question (oq-1, verbatim)

"Should disclosures be machine-enumerable (MCP/audit surface), and what
belongs in the enumeration?"

## Investigation plan

1. Read every existing disclosure call site and its shape: `internal/lint`'s
   `Finding`/`Severity` (VL-017's disclosed-unproven notices), `cmd/verdi`'s
   `gateCondition.Disclosed` (the closure gate's pending-supersession-on-a-
   nil-forge case), `internal/mcpserve`'s `review_unavailable` field
   (`list_annotations`, `get_board`) and its `internal/workbench` origin
   (`boardspec.go`'s `proj.Notices`), and `internal/align`'s judged-section
   disclosed-unproven labeling.
2. Read `internal/mcpserve`'s existing read-tool patterns (`tooldefs.go`,
   `tool_get_board.go`, `tool_list_annotations.go`) to see how a new
   enumerable surface would fit the server's existing shape and safety
   conventions (the data-never-instructions note, read-only tool
   discipline).
3. Read `internal/workbench`'s board projection to see whether disclosures
   already have a natural collection point (`proj.Notices`) that an
   enumeration could reuse or would have to generalize.
4. Write the recommendation and shape as
   `docs/spikes/v1/disclosure-enumeration-spike.md`, inside the
   `spike_paths:` fence — the spike's sole deliverable.

## What "answered" means

The spike is answered when the recommendation document exists, is
concrete enough that story-2's author needs to make no further design
judgment calls about the enumeration item shape (only implementation
ones), and states plainly whether the recommendation is "build it" or
"not yet, because X" — either is a valid answer; open-ended deferral
without a reason is not. Per 03 §Ceremony pricing, the spike closes when
its question is resolved — this document, plus the merged answer, is that
resolution. It never amends the frozen feature spec (oq-1 stays on the
record there, verbatim); the answer lands here and in story-2's own spec.
