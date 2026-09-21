---
id: spec/strict-lint-target-spike
kind: spec
title: "Spike: strict lint yield, true-positive rate, and ratchet mode"
owners: [platform-team]
class: story
spike: true
story: jira:VERDI-PA-2
problem: { text: "spec/strict-lint-target cannot declare its linter set, ratchet mechanism, or gate budget while the finding counts, true-positive rates, ratchet determinism, and configuration coexistence are unmeasured.", anchor: problem }
outcome: { text: "A timeboxed recommendation with a per-linter count and runtime table on main, a sampled true-positive rate per linter with the wave-1 retro-witness result, a ruling between --new-from-rev and a committed baseline with the determinism evidence, and a working two-file Makefile invocation, precise enough for spec/strict-lint-target's decisions and plan.", anchor: outcome }
links:
  - { type: resolves, ref: "spec/strict-lint-target#oq-1" }
  - { type: resolves, ref: "spec/strict-lint-target#oq-2" }
  - { type: resolves, ref: "spec/strict-lint-target#oq-3" }
  - { type: resolves, ref: "spec/strict-lint-target#oq-4" }
---
# Spike: strict lint yield, true-positive rate, and ratchet mode

## Problem

Adding a gate step whose finding count is unknown is how gates become
things people route around. Measure before adopting.

## Outcome

A recommendation. Throwaway artifacts live under
`docs/spikes/strict-lint-target/` (VL-016 fence).

## The questions (verbatim from spec/strict-lint-target)

- oq-1: What are the finding count and runtime per candidate linter
  (containedctx, noctx, contextcheck, errorlint, exhaustive, dupl,
  gochecknoglobals) on main today, and which other available linters map
  to a written ground rule?
- oq-2: What fraction of a sample of findings per linter is a true positive
  against the rule it is meant to enforce, and are the two wave-1 review
  minors reported at their pre-fix commit and silent at the fixed one?
- oq-3: Which ratchet mechanism is deterministic in CI and locally:
  golangci-lint's own --new-from-rev against the merge base with main, or
  a committed baseline of findings that only shrinks?
- oq-4: Can a second configuration coexist with the parity one without
  golangci-lint v2's configuration discovery picking the wrong file, and
  what does the Makefile invocation look like?

## Investigation plan

Throwaway worktree from main. golangci-lint v2.5.0, the Makefile's pin.

1. **Counts (oq-1).** Write `docs/spikes/strict-lint-target/golangci.strict.yml`
   enabling exactly the seven candidates (`linters: default: none`, then
   `enable:`). Run `golangci-lint run --config <file> ./...` once per linter
   (`--enable-only` or one config per linter) and record findings and wall
   clock. Then read `golangci-lint help linters` in full and list every
   other linter that maps to a sentence in CLAUDE.md's Go style or testing
   rules, with the sentence quoted.
2. **True positives (oq-2).** For each linter take the first ten findings in
   path order and judge each against the ground rule it enforces: true
   positive, false positive, or a rule the CLAUDE.md sentence does not
   actually state. Then run the strict config at `1be75d01` and at
   `410db101` (on the wave-1 branch) restricted to `./internal/readinessload/
   ./cmd/verdi/ ./internal/store/` and check that gochecknoglobals reports
   the package-level `serveReadinessLoader` variables and dupl reports the
   duplicated `hasDotDotElement` at the first head and neither at the second.
3. **Ratchet mode (oq-3).** Run `--new-from-rev=$(git merge-base HEAD origin/main)`
   twice locally and compare outputs byte for byte; introduce one deliberate
   new finding in a scratch commit and confirm exactly one finding is
   reported; check the merge-gate workflow's checkout (`fetch-depth: 0` is
   present) provides the merge base. Separately, draft a baseline file from
   step 1's output and describe its regeneration protocol. Judge on
   determinism first, then on how a killed finding is disclosed.
4. **Coexistence (oq-4).** With both files present, run plain
   `golangci-lint run` and confirm it picks `.golangci.yml` only; run with
   `--config` and confirm the strict set runs; check whether a `.golangci.strict.yml`
   name is ever auto-discovered. Write the Makefile recipe.

Timebox: half a working day.

## What "answered" means

- oq-1: a table (linter, findings, seconds) plus a list of additional
  candidates each with its CLAUDE.md sentence.
- oq-2: per-linter true-positive fraction from ten samples, and yes or no on
  both retro-witness findings.
- oq-3: the chosen mechanism with the two-run byte comparison and the
  one-new-finding result recorded.
- oq-4: a recipe that runs both targets and a statement of which file each
  invocation loaded.

## Spec seed

oq-1 and oq-2 fix ac-1's linter list (a linter with a low true-positive
rate is dropped or moved to report-only, recorded as a decision). oq-3
fixes ac-2's mechanism and the plan's ratchet ruling. oq-4 fixes ac-1's
invocation and the file name. oq-1's runtime fills co-3's budget. ac-3 and
ac-4 stand regardless.

## Throwaway rule

The strict config drafted here is evidence, not the deliverable; the
feature's plan authors the real file under test with the digest witness.
