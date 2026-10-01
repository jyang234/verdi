---
id: spec/strict-lint-reach
kind: spec
title: "Strict lint reach"
owners: [platform-team]
class: story
story: jira:VERDI-PH-2
problem: { text: "Nothing shows that the strict target reaches the class of defect that motivated it: the wave-1 review minor, two package-level readiness loader variables, was found by a person, and no witness proves the gated linters would have reported it.", anchor: problem }
outcome: { text: "A committed witness runs the strict configuration over the commits just before and at that fix and shows the linter reports both variables before and neither after, and CI caches golangci-lint's analysis to shorten the static job.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "A retro-witness exports commits 410db101 and e4d5b141 from the repository's own history with git archive, runs the pinned golangci-lint with this repository's .golangci.strict.yml over the packages that held the variables (cmd/verdi, internal/readinessload, and internal/store) for linux/amd64, and asserts that gochecknoglobals reports serveReadinessLoader and serveReadinessDefaultSpec at 410db101 and reports neither at e4d5b141. A clone that lacks either commit fails the witness, never passes it.", evidence: [behavioral], anchor: ac-1 }
constraints:
  - { id: co-1, text: "The parent's constraints bind this story: one pinned version (co-2) and the declared budget (co-3, dc-5).", anchor: co-1 }
decisions:
  - { id: dc-1, text: "The static job of merge-gate.yml and verify.yml caches golangci-lint's analysis cache, keyed by the pinned version and go.sum, identically in both files, and internal/specalign's pinned-setup guard admits that step. It only shortens the job: the 60-second budget holds with a cold cache (parent dc-5), and the cache changes no finding.", anchor: dc-1 }
  - { id: dc-2, text: "Every full clone holds both commits (parent dc-4); the one other thing the witness needs is the pinned golangci-lint, which parent ac-3 already names. It skips, with a printed reason, where the linter is absent; CI job verify has it, by restoring the static job's cached binary (spec/strict-lint-gate dc-3, ledger SI-309, which this story depends on), so the evidence record is pass or fail, and a skip there would be recorded as abstain, never as a pass.", anchor: dc-2 }
links:
  - { type: implements, ref: "spec/strict-lint-target-v2#ac-3" }
  - { type: depends-on, ref: "spec/strict-lint-gate" }
---
# Strict lint reach

## Problem

Nothing shows that the strict target reaches the class of defect that motivated it: the wave-1 review minor, two
package-level readiness loader variables, was found by a person, and no witness proves the gated linters would have
reported it.

## Outcome

A committed witness runs the strict configuration over the commits just before and at that fix and shows the linter
reports both variables before and neither after, and CI caches golangci-lint's analysis to shorten the static job.

## ac-1

A retro-witness exports commits 410db101 and e4d5b141 from the repository's own history with git archive, runs the
pinned golangci-lint with this repository's .golangci.strict.yml over the packages that held the variables (cmd/verdi,
internal/readinessload, and internal/store) for linux/amd64, and asserts that gochecknoglobals reports
serveReadinessLoader and serveReadinessDefaultSpec at 410db101 and reports neither at e4d5b141. A clone that lacks
either commit fails the witness, never passes it.

## co-1

The parent's constraints bind this story: one pinned version (co-2) and the declared budget (co-3, dc-5).

## dc-1

The static job of merge-gate.yml and verify.yml caches golangci-lint's analysis cache, keyed by the pinned version and
go.sum, identically in both files, and internal/specalign's pinned-setup guard admits that step. It only shortens the
job: the 60-second budget holds with a cold cache (parent dc-5), and the cache changes no finding.

## dc-2

Every full clone holds both commits (parent dc-4); the one other thing the witness needs is the pinned golangci-lint,
which parent ac-3 already names. It skips, with a printed reason, where the linter is absent; CI job verify has it, by
restoring the static job's cached binary (spec/strict-lint-gate dc-3, ledger SI-309, which this story depends on), so
the evidence record is pass or fail, and a skip there would be recorded as abstain, never as a pass.
