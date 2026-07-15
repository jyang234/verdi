---
id: spec/escrow-notify-v2
kind: spec
title: "Escrow notify v2 (fixture, supersedes escrow-notify)"
owners: [platform-team]
class: story
status: accepted-pending-build
story: jira:ESCROW-2
problem: { text: "a borrower learns about an escrow shortfall only on their next statement", anchor: "#problem" }
outcome: { text: "a borrower is notified within an hour of an escrow shortfall", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "an escrow shortfall notifies the borrower within one hour", evidence: [behavioral], anchor: "#ac-1" }
links:
  - { type: implements, ref: "spec/stale-decline#ac-4" }
  - { type: supersedes, ref: "spec/escrow-notify" }
frozen: { at: 2026-07-12, commit: 30c5ff945413930879823be6db0ccc07d5abd6b9 }
---
# Escrow notify v2 (fixture, supersedes escrow-notify)

**Story-rung supersession fixture, v2** (spec/feature-supersession-state
dc-4). Supersedes `spec/escrow-notify`; its acceptance is what flips the
predecessor story's `status` to `superseded` (the rung-3 flip D-12 shipped).
It is the source of the predecessor's computed `superseded-by` backlink on
dex, exactly as the feature-rung `rate-lock-v2` pair is. Unlike that
feature-rung pair, this story-rung one carries no `supersession:` block —
`02 §Object model`'s `supersession:` field is feature-only
(`artifact.SpecFrontmatter.Validate`: "story spec must not carry
feature-only fields"), so VL-015's carried/amended/removed manifest and
fidelity check apply only at rung 4; a story-rung supersession is fully
expressed by the `supersedes` link plus the terminal `status: superseded`
flip on the predecessor (03 §rung 3).

## Problem

A borrower learns about an escrow shortfall only on their next statement.

## Outcome

A borrower is notified within an hour of an escrow shortfall.

## AC-1

An escrow shortfall notifies the borrower within one hour, down from the
predecessor's 24-hour window.
