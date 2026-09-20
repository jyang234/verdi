---
id: spec/mutation-ratchet-spike
kind: spec
title: "Spike: mutation tool fit, yield, and reach"
owners: [platform-team]
class: story
spike: true
story: jira:VERDI-PA-1
problem: { text: "spec/mutation-ratchet cannot be designed with confidence while its five open questions stand: no Go mutation tool has been run on this module, the cost of a touched-files run is unmeasured, it is unproven that mutation reaches the PA-017 witness, the baseline shape is undecided, and it is unknown whether gate posture can be derived from spec risk tiers.", anchor: problem }
outcome: { text: "A timeboxed recommendation naming the tool and pin, a runtime and survivor table for the wave-1 diff, a yes-or-no on the retro-witness with the mutant that decides it, a baseline record shape reusing an existing ratchet idiom, and a ruling on tier mapping, precise enough for spec/mutation-ratchet's decisions and implementation plan to consume directly.", anchor: outcome }
links:
  - { type: resolves, ref: "spec/mutation-ratchet#oq-1" }
  - { type: resolves, ref: "spec/mutation-ratchet#oq-2" }
  - { type: resolves, ref: "spec/mutation-ratchet#oq-3" }
  - { type: resolves, ref: "spec/mutation-ratchet#oq-4" }
  - { type: resolves, ref: "spec/mutation-ratchet#oq-5" }
---
# Spike: mutation tool fit, yield, and reach

## Problem

Five unknowns stand between the audit's remedy design 6.2 and a spec whose
criteria can be written without guessing. Each is cheap to answer by
running a tool, and expensive to get wrong in a gate that every wave pays.

## Outcome

A recommendation, not code. Anything built here is throwaway and lives
under `docs/spikes/mutation-ratchet/` (the VL-016 spike fence,
`spike_paths` in `.verdi/verdi.yaml`).

## The questions (verbatim from spec/mutation-ratchet)

- oq-1: Which Go mutation tool runs on this module (Go 1.25, 105 packages,
  build tags, test binaries that exec cmd/verdi) hermetically and can be
  pinned like golangci-lint? Candidates: gremlins, go-mutesting, ooze.
- oq-2: What are the runtime, mutant count, and survivor count of a
  touched-files run over the wave-1 diff of spec/readiness-recovery
  (b810c302..e963f4d0), and does that fit inside a per-wave gate whose
  steps are now timed?
- oq-3: Does the technique reach the witness: with the co-2 persistence
  test reverted to its pre-410db101 form, does a mutant that writes an
  arbitrary file survive, and does the fixed test kill it?
- oq-4: What is the baseline record's shape and home, how are equivalent
  mutants marked, and how is a regeneration disclosed?
- oq-5: Can the hard-or-report posture be derived from the spec risk tiers,
  or must the manifest be authored by hand?

## Investigation plan

Work in a throwaway worktree cut from `agent/readiness-recovery-wave-1` at
`e963f4d0` so the witness range is present. Never run in the main checkout.

1. **Tool fit (oq-1).** For each candidate, `go install <module>@<version>`
   with the exact version recorded, then run against
   `./internal/readinessload/...` and `./internal/journey/...` only. Record:
   installs cleanly on Go 1.25; runs without network after install; leaves
   the working tree byte-identical afterwards (`git status --porcelain`
   empty, `git diff --stat` empty); handles a package whose tests exec a
   built binary without hanging; runtime per package. Disqualify any tool
   that fails the clean-tree check.
2. **Yield and cost (oq-2).** With the surviving tool, compute the touched
   Go files of `b810c302..e963f4d0` (`git diff --name-only -- '*.go' |
   grep -v _test.go`), run mutation over exactly those files with only their
   packages' tests, and record wall clock, mutants generated, killed,
   survived, timed out, per package. Compare against the gate timing series
   in `.verdi/data/gate/timings.tsv` from the tooling PR's first full run.
3. **Reach (oq-3).** Check out `1be75d01` (the pre-fix head), run mutation
   over `internal/readinessload/load.go` only, and list survivors whose
   operator touches the file-write path. Then check out `410db101` and rerun.
   The answer is yes only if a survivor at the first head is killed at the
   second. Record the mutant's file, line and operator verbatim.
4. **Baseline shape (oq-4).** Draft one record with the survivors from step
   2 in each of two existing idioms: a `testdata/mutation-baseline.json`
   canonical file beside a digest, and the policyartifact
   `golden-digests.json` style. Judge by: diff readability when one survivor
   is killed; how an equivalent mutant carries its one-line reason; how a
   regeneration command discloses itself. Recommend one.
5. **Tier mapping (oq-5).** For the touched files of step 2, attempt to
   derive a tier: file → test files in the same package → `verdi.bindings.yaml`
   producers → criteria → dc-6 tier of the owning spec. Count how many files
   resolve to exactly one tier, to several, to none. If most resolve to none
   the manifest is hand-authored.

Timebox: one working day. If step 1 disqualifies every candidate, stop and
report that; the spec is then blocked on tooling, not on design.

## What "answered" means

- oq-1: one tool named with a pinned version, or a finding that none fits,
  each candidate's disqualification reason recorded.
- oq-2: a table of package, files, mutants, killed, survived, seconds; a
  total; and the ratio of that total to the last full gate's total.
- oq-3: yes or no, with the deciding mutant quoted.
- oq-4: one record shape recommended with a filled example.
- oq-5: counts for one, several, none; a recommendation.

## Spec seed

The answers land in spec/mutation-ratchet as follows. oq-1 and oq-2 fill
co-2's pin and co-3's budget as a new decision. oq-3 either confirms ac-3
verbatim or closes the feature as not-doing. oq-4 names ac-2's baseline
file and regeneration command. oq-5 decides whether ac-4's manifest is
derived or authored. None of the acceptance criteria as written depend on
which tool wins.

## Throwaway rule

Everything produced here is evidence for a recommendation. No tool, config,
baseline or Makefile target from this spike is kept; the feature's
implementation plan re-creates what it needs under test.
