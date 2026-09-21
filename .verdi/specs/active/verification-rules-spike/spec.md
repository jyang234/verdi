---
id: spec/verification-rules-spike
kind: spec
title: "Spike: clause enumeration measure, citation seam, and contract delta"
owners: [platform-team]
class: story
spike: true
story: jira:VERDI-PA-4
problem: { text: "spec/verification-rules cannot fix its lint threshold, citation seam, or the size of the design-spec-02 amendment while the enumeration backlog is unmeasured, neither citation seam has been tried on a real constraint, the blast radius of a clause field is unknown, cross-specification comparison is untested, and the PA-006 class has never been counted.", anchor: problem }
outcome: { text: "A timeboxed recommendation with the enumeration counts and a calibrated threshold, the citation seam that survived a trial on readiness-recovery co-2's nine prohibitions, the drafted 02 frontmatter delta with a per-consumer blast-radius table, a mechanical-or-judge ruling on cross-specification comparison over the versioned specs, and the first required-field contradiction count, precise enough for spec/verification-rules' decisions and the 02 ratification request.", anchor: outcome }
links:
  - { type: resolves, ref: "spec/verification-rules#oq-1" }
  - { type: resolves, ref: "spec/verification-rules#oq-2" }
  - { type: resolves, ref: "spec/verification-rules#oq-3" }
  - { type: resolves, ref: "spec/verification-rules#oq-4" }
  - { type: resolves, ref: "spec/verification-rules#oq-5" }
---
# Spike: clause enumeration measure, citation seam, and contract delta

## Problem

This is the feature with the largest authority footprint of the five: it
amends the artifact contract. Its ratification request must carry numbers,
not intent.

## Outcome

A recommendation. Throwaway scripts and drafts live under
`docs/spikes/verification-rules/` (VL-016 fence). Nothing touches
`docs/design/specs/` directly; the 02 delta is a draft for the ratification
flow.

## The questions (verbatim from spec/verification-rules)

- oq-1: Across the 28 active specs, how many criteria and constraints
  enumerate more than three claims by a mechanical count, what is the size
  of the enumeration backlog, and what threshold for the under-enumeration
  lint yields a tolerable false-positive rate on the existing corpus?
- oq-2: Which citation seam works: bindings entries extended to clause
  fragments, or a test-side marker comment parsed by a gate test, tried on
  readiness-recovery co-2's nine prohibitions?
- oq-3: What is the exact frontmatter delta in design spec 02 and its
  blast radius, and how large is the 02 amendment?
- oq-4: Given enumerated clauses, is cross-specification comparison
  mechanical or a judge task, and are the older versions in the active
  directory actually marked superseded?
- oq-5: How many contract-required fields does the artifact contract mark
  today, and for how many does the decoder accept absence?

## Investigation plan

Read-only over main plus scratch files under the spike fence.

1. **Measure (oq-1).** Script: decode every active spec's criteria and
   constraints (the built binary's own decoder through a small Go program
   under the fence, or `yq`), count claims per sentence by three mechanical
   proxies (comma-or-semicolon list items, occurrences of "never" and "no",
   "and"-joined verb phrases), and produce a histogram. Hand-read the top
   twenty to label true multi-claim versus false positive. Choose the
   threshold where hand-labelled false positives fall under one in five.
   Report the count above threshold as the backlog.
2. **Seam trial (oq-2).** Enumerate co-2's nine prohibitions as clauses in a
   scratch copy of the readiness-recovery spec. Seam A: add fragment
   entries `spec/readiness-recovery#co-2/c1`..`c9` to a scratch
   `verdi.bindings.yaml` and check what `artifact.ResolveBindingAC` and
   VL-003 do with a two-level fragment (accept, reject, misparse). Seam B:
   add `// verdi:clause spec/readiness-recovery#co-2/c1` markers to the
   tests in `internal/readinessload` that witness each prohibition and
   write a 30-line gate test that collects them. Judge by: which clauses
   actually have a witness today (expect fewer than nine); how a missing
   witness reads in each seam's output; blast radius on existing code.
3. **Contract delta (oq-3).** Draft the 02 frontmatter addition (a
   `clauses:` list under a criterion or constraint object) as a diff
   against `docs/design/specs/02-*.md` kept under the fence, then grep the
   consumers: `internal/artifact` (decoder), `internal/lint`, `internal/
   journey`, `internal/readinesspilot`, `internal/specdoc*`, `internal/dex`,
   `internal/mcpserve`. For each, state must-change, may-change, or
   untouched, with the file that decides it. Count the 02 diff lines.
4. **Cross-spec comparison (oq-4).** Check the status field of guided-
   lifecycle-governance v1, v2 and comparative-spike-experiments v1, v2 in
   `.verdi/specs/active/` (superseded or not). Then align v2 and v3 of each
   by anchor id and diff criterion text; classify each pair as identical,
   reworded-same-claim, or different-claim by hand. If most differences
   are rewordings, comparison is a judge task; if anchors line up and text
   differs rarely, a mechanical same-anchor-different-text lint is useful.
5. **PA-006 count (oq-5).** List every `# required` in 02's frontmatter
   tables; for each, find the decoder field in `internal/artifact` and
   check whether absence is rejected (a validator error) or accepted.
   Produce the table; UAT-008's `status` row is one line of it.

Timebox: one working day.

## What "answered" means

- oq-1: histogram, threshold, false-positive rate on twenty samples,
  backlog count.
- oq-2: one seam recommended, the nine clauses with their witnessed-or-not
  status, and the output of the missing-witness case.
- oq-3: the drafted 02 diff (line count) and the per-consumer table.
- oq-4: status of the versioned specs; classification counts; a ruling.
- oq-5: the required-field table with accept-or-reject per row.

## Spec seed

oq-1 fixes ac-2's threshold as a decision. oq-2 fixes ac-1's citation seam.
oq-3 becomes the ratification request under co-1 and sizes the plan. oq-4
decides whether PA-021 gets a criterion in this feature or is closed as a
judge task. oq-5 seeds ac-4's enumerating test and its adjudicated rows.

## Throwaway rule

Scratch bindings, markers and the 02 draft are evidence. The real 02
amendment goes through ratification; the real markers and lint are built
under test in the feature's plan.
