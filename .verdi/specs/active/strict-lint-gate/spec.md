---
id: spec/strict-lint-gate
kind: spec
title: "Strict lint gate"
owners: [platform-team]
class: story
story: jira:VERDI-PH-1
problem: { text: "The ground-rule linters are chosen, but nothing runs them: there is no strict configuration, no make target, no committed baseline, and no gate that fails a change adding a finding, so the rules in docs/ground-rules.md are still enforced only by a reviewer remembering them.", anchor: problem }
outcome: { text: "make lint-strict runs the five gated linters over the module for linux/amd64 and compares their findings with a committed baseline, failing a change that adds a finding, grows the baseline, or leaves a fixed finding's allowance behind; it is a step of make verify and runs in CI's static job, while the parity configuration stays byte-identical.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "A configuration file at the repository root, .golangci.strict.yml, enables exactly containedctx, contextcheck, noctx, errorlint with only its errorf check, and gochecknoglobals, each under a comment quoting the docs/ground-rules.md sentence it enforces, and caps no findings (max-issues-per-linter and max-same-issues both 0). make lint-strict runs the Makefile's pinned golangci-lint with --config .golangci.strict.yml for GOOS=linux GOARCH=amd64 and then the baseline check; it is a step of make verify's VERIFY_STEPS and runs in the static job of merge-gate.yml and verify.yml. A static witness pins .golangci.yml's bytes, which this story leaves unchanged. Over a committed fixture module holding one violation per gated linter, the strict configuration reports exactly those findings.", evidence: [static, behavioral], anchor: ac-1 }
  - { id: ac-2, text: "The committed baseline, .golangci.strict-baseline.json, holds each finding's key (linter, package, message, and the flagged source line) with its count, generated and checked for linux/amd64. The check exits 1 when the current run reports a key more times than the baseline allows, when the baseline allows a key more times than the current run reports, or when the baseline allows any key more times than the baseline at the merge base with the default branch, or, on the default branch itself, at HEAD's first parent, read with git show; it exits 0 otherwise. A golangci-lint failure other than reporting findings, a missing or malformed report or baseline, or a merge base or parent that cannot be resolved exits 2. A finding fixed in one change and reintroduced in a later change fails the later change.", evidence: [behavioral], anchor: ac-2 }
  - { id: ac-3, text: "The strict configuration disables no gated linter and excludes none wholesale; every exclusion names a path or a text pattern and carries a reason, and a static witness counts the exclusions and fails when the count differs from the number it pins, so every added exclusion is a visible change.", evidence: [static], anchor: ac-3 }
constraints:
  - { id: co-1, text: "The parent's constraints bind this story: .golangci.yml is not modified (co-1), both targets use the Makefile's one pinned version (co-2), and make lint-strict's time is recorded by the gate timing series against the declared 60-second budget (co-3, dc-5).", anchor: co-1 }
  - { id: co-2, text: "The committed baseline is generated at this story's own base; a finding this story's own change introduces is fixed, never added to the baseline.", anchor: co-2 }
decisions:
  - { id: dc-1, text: "The baseline check lives in one internal package, internal/lintratchet, which parses golangci-lint's JSON report, keys and counts findings, and compares them, with a thin command the Makefile runs. golangci-lint runs with --issues-exit-code=0 so a reported finding is data; the check alone decides exit 0, 1, or 2, and any nonzero golangci-lint exit is operational.", anchor: dc-1 }
  - { id: dc-2, text: "The merge base is taken against the default branch resolved as verdi resolves it elsewhere (CI_DEFAULT_BRANCH, then origin/HEAD, then origin/main or origin/master). On the default branch itself, where the merge base would be HEAD, the growth comparison reads the baseline at HEAD's first parent, the default branch before the change, so a change pushed straight to the default branch is checked too. A merge base or parent that cannot be resolved, such as in a shallow clone, exits 2, never a pass.", anchor: dc-2 }
  - { id: dc-3, text: "Tests that run the real linter skip, with a printed reason, when the Makefile's pinned golangci-lint is absent, as it is in CI's test jobs. CI job verify, which produces the evidence records, gains the static job's two golangci-lint steps, copied exactly (ledger SI-309): it runs after the static job and restores the binary the static job cached under the same key, so the install step runs only on a cache miss and CI still installs the pinned version once per version (parent co-2). A skip in verify would be recorded as abstain, never as a pass.", anchor: dc-3 }
links:
  - { type: implements, ref: "spec/strict-lint-target-v2#ac-1" }
  - { type: implements, ref: "spec/strict-lint-target-v2#ac-2" }
  - { type: implements, ref: "spec/strict-lint-target-v2#ac-4" }
---
# Strict lint gate

## Problem

The ground-rule linters are chosen, but nothing runs them: there is no strict configuration, no make target, no
committed baseline, and no gate that fails a change adding a finding, so the rules in docs/ground-rules.md are still
enforced only by a reviewer remembering them.

## Outcome

make lint-strict runs the five gated linters over the module for linux/amd64 and compares their findings with a
committed baseline, failing a change that adds a finding, grows the baseline, or leaves a fixed finding's allowance
behind; it is a step of make verify and runs in CI's static job, while the parity configuration stays byte-identical.

## ac-1

A configuration file at the repository root, .golangci.strict.yml, enables exactly containedctx, contextcheck, noctx,
errorlint with only its errorf check, and gochecknoglobals, each under a comment quoting the docs/ground-rules.md
sentence it enforces, and caps no findings (max-issues-per-linter and max-same-issues both 0). make lint-strict runs the
Makefile's pinned golangci-lint with --config .golangci.strict.yml for GOOS=linux GOARCH=amd64 and then the baseline
check; it is a step of make verify's VERIFY_STEPS and runs in the static job of merge-gate.yml and verify.yml. A static
witness pins .golangci.yml's bytes, which this story leaves unchanged. Over a committed fixture module holding one
violation per gated linter, the strict configuration reports exactly those findings.

## ac-2

The committed baseline, .golangci.strict-baseline.json, holds each finding's key (linter, package, message, and the
flagged source line) with its count, generated and checked for linux/amd64. The check exits 1 when the current run
reports a key more times than the baseline allows, when the baseline allows a key more times than the current run
reports, or when the baseline allows any key more times than the baseline at the merge base with the default branch, or,
on the default branch itself, at HEAD's first parent, read with git show; it exits 0 otherwise. A golangci-lint failure
other than reporting findings, a missing or malformed report or baseline, or a merge base or parent that cannot be
resolved exits 2. A finding fixed in one change and reintroduced in a later change fails the later change.

## ac-3

The strict configuration disables no gated linter and excludes none wholesale; every exclusion names a path or a text
pattern and carries a reason, and a static witness counts the exclusions and fails when the count differs from the
number it pins, so every added exclusion is a visible change.

## co-1

The parent's constraints bind this story: .golangci.yml is not modified (co-1), both targets use the Makefile's one
pinned version (co-2), and make lint-strict's time is recorded by the gate timing series against the declared 60-second
budget (co-3, dc-5).

## co-2

The committed baseline is generated at this story's own base; a finding this story's own change introduces is fixed,
never added to the baseline.

## dc-1

The baseline check lives in one internal package, internal/lintratchet, which parses golangci-lint's JSON report, keys
and counts findings, and compares them, with a thin command the Makefile runs. golangci-lint runs with
--issues-exit-code=0 so a reported finding is data; the check alone decides exit 0, 1, or 2, and any nonzero
golangci-lint exit is operational.

## dc-2

The merge base is taken against the default branch resolved as verdi resolves it elsewhere (CI_DEFAULT_BRANCH, then
origin/HEAD, then origin/main or origin/master). On the default branch itself, where the merge base would be HEAD, the
growth comparison reads the baseline at HEAD's first parent, the default branch before the change, so a change pushed
straight to the default branch is checked too. A merge base or parent that cannot be resolved, such as in a shallow
clone, exits 2, never a pass.

## dc-3

Tests that run the real linter skip, with a printed reason, when the Makefile's pinned golangci-lint is absent, as it is
in CI's test jobs. CI job verify, which produces the evidence records, gains the static job's two golangci-lint steps,
copied exactly (ledger SI-309): it runs after the static job and restores the binary the static job cached under the
same key, so the install step runs only on a cache miss and CI still installs the pinned version once per version
(parent co-2). A skip in verify would be recorded as abstain, never as a pass.
