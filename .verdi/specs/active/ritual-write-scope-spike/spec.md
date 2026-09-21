---
id: spec/ritual-write-scope-spike
kind: spec
title: "Spike: ritual effect inventory, sensor choice, and scope grammar"
owners: [platform-team]
class: story
spike: true
story: jira:VERDI-PA-3
problem: { text: "spec/ritual-write-scope cannot declare scopes, choose a sensor, or place the registry while nobody has recorded what each verb actually does to a repository, which mutations bypass internal/gitx, whether one grammar fits five rituals, and whether the seam can be shared with spec/readiness-recovery's wave 3.", anchor: problem }
outcome: { text: "A timeboxed recommendation carrying the empirical per-verb git command inventory, the list of repository mutations outside gitx with a sensor ruling, five hand-written scope declarations in one drafted grammar, a home for the registry with its ratification consequence, and a sequencing ruling against readiness-recovery wave 3, precise enough for spec/ritual-write-scope's decisions and the wave 3 plan.", anchor: outcome }
links:
  - { type: resolves, ref: "spec/ritual-write-scope#oq-1" }
  - { type: resolves, ref: "spec/ritual-write-scope#oq-2" }
  - { type: resolves, ref: "spec/ritual-write-scope#oq-3" }
  - { type: resolves, ref: "spec/ritual-write-scope#oq-4" }
  - { type: resolves, ref: "spec/ritual-write-scope#oq-5" }
---
# Spike: ritual effect inventory, sensor choice, and scope grammar

## Problem

The remedy in PA-024 reads as "extend the witness that already exists".
Independent review found that the existing witness is a unit test over one
path and that gitx has no recorder seam, so the cost was understated. Before
the spec commits to a sensor and a grammar, the ground truth has to be
recorded.

## Outcome

A recommendation. Throwaway code lives under
`docs/spikes/ritual-write-scope/` (VL-016 fence); the recorder patch to
gitx is never committed.

## The questions (verbatim from spec/ritual-write-scope)

- oq-1: What git commands does each verb actually run today?
- oq-2: Which repository mutations happen outside internal/gitx's run
  function, and does any of them touch refs, the index, or the working
  tree? This decides whether the command log alone is a sufficient sensor
  or a before-and-after state diff is required.
- oq-3: What is the smallest scope grammar that fits design start, build
  start, close, the commit-to-design ritual, and policy adopt?
- oq-4: Where does the declaration live: a Go registry checked by a gate
  test, a policy payload in the store, or an amendment to design specs 03
  or 04 that would need the ratification flow?
- oq-5: Does a gitx command recorder seam built for spec/readiness-recovery
  ac-9 in its wave 3 serve this feature unchanged, and which feature should
  land first?

## Investigation plan

Throwaway worktree from main. Never run rituals in the main checkout.

1. **Inventory (oq-1).** Patch `internal/gitx/exec.go`'s `run`, and the exec
   sites in `plumbing.go` and `configvalue.go`, to append `dir\targs` to a
   file named by an environment variable when set. Run `go test ./cmd/verdi/
   ./internal/designapp/ ./internal/wtmanager/ ./internal/closeapp/...`
   (every package that exercises a ritual) with the variable set, then
   reduce the log to a per-verb set of git subcommands with their
   mutating flags. Attribute to verbs by test name where the package is
   `cmd/verdi`, by package otherwise. Record the 29 `cmd/verdi` files that
   import gitx and which of them appear in the log.
2. **Blind spots (oq-2).** Grep every `exec.Command` and `exec.CommandContext`
   outside gitx (known sites: `internal/upstream/runner_real.go`,
   `internal/execworkspace/isolation.go`, `internal/publicrelease/*`,
   `internal/align/judge.go`, `internal/sealedexec/*`) and for each answer:
   can the child mutate refs, index, or working tree of the operator's
   repository? Then run one ritual (design start on a fixture) under a
   before-and-after snapshot (`git for-each-ref`, `git ls-files -s`, `git
   status --porcelain=v2`) and diff it against the command log's implied
   effects. If the diff shows an effect the log does not explain, the state
   diff is a required sensor.
3. **Grammar (oq-3).** Write five declarations by hand in one YAML shape
   (`refs_create`, `refs_move`, `head_may_switch`, `stage_paths`,
   `index_carry_foreign`, `untracked_may_enter`, `may_push`) from step 1's
   inventory for design start, build start, close, the commit-to-design
   ritual, and policy adopt. Note every field a ritual needed that the
   shape lacked, and every field no ritual used.
4. **Home (oq-4).** For each candidate home write down: who reads it (gate
   test, lint rule, readiness), whether it needs a schema change in
   `internal/artifact`, whether design specs 03 or 04 state anything about
   verb effects today (grep them), and therefore whether ratification is
   needed. Check whether the policy kernel's payload grammar
   (`internal/policyartifact/payload.go`) admits a scope record as-is.
5. **Seam and sequencing (oq-5).** Read the wave 3 plan if written, else
   ac-9's text; sketch the smallest recorder seam (an optional `Recorder`
   on a gitx runner, or a package-level hook, judged against the
   gochecknoglobals rule) that serves both a forbidden-token check and a
   per-verb log, and state which feature should land it.

Timebox: one working day.

## What "answered" means

- oq-1: a table verb → git subcommands and flags, with the recorder's log
  attached as evidence.
- oq-2: a list of non-gitx exec sites each marked can or cannot mutate the
  operator's repository, and a sensor ruling: log only, or log plus state
  diff.
- oq-3: five filled declarations and the final field list.
- oq-4: one home recommended with its ratification consequence stated.
- oq-5: a seam sketch and a sequencing ruling.

## Spec seed

oq-1 and oq-3 fill ac-1's field list as a decision. oq-2 confirms or
simplifies dc-2 and fixes ac-2's sensor. oq-4 fixes where ac-1's registry
lives and whether a spec 03/04 amendment precedes the build. oq-5 fixes
co-4's ownership and the wave 3 plan ruling. ac-3 and ac-4 stand
regardless.

## Throwaway rule

The recorder patch, the snapshot script, and the drafted declarations are
evidence. The feature's plan re-creates the seam under test.
