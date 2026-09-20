---
id: spec/self-governance-spike
kind: spec
title: "Spike: adoption dry run, rule expressibility, and the exemption path"
owners: [platform-team]
class: story
spike: true
story: jira:VERDI-PA-5
problem: { text: "spec/self-governance cannot commit to adopting a profile, a rung field, or a disclosure index while nobody has run the starter adoption against a copy of this store, tried to express a ground rule in the kernel grammar, checked whether the rung is a schema change, exercised an exemption's review window end to end, or recorded a real deferred disclosure in any candidate home.", anchor: problem }
outcome: { text: "A timeboxed recommendation with the adoption diff and its blocker count, a fit table for five ground rules in the kernel grammar, a schema-or-attribute ruling for the enforcement rung with its ratification consequence, an end-to-end trace of one exemption through readiness, and a home for the disclosure index with wave 1's nine items recorded, precise enough for spec/self-governance's decisions and plan.", anchor: outcome }
links:
  - { type: resolves, ref: "spec/self-governance#oq-1" }
  - { type: resolves, ref: "spec/self-governance#oq-2" }
  - { type: resolves, ref: "spec/self-governance#oq-3" }
  - { type: resolves, ref: "spec/self-governance#oq-4" }
  - { type: resolves, ref: "spec/self-governance#oq-5" }
---
# Spike: adoption dry run, rule expressibility, and the exemption path

## Problem

Adopting governance on a store with 28 active specs is the kind of change
whose blast radius is easy to assert and cheap to measure. Measure it.

## Outcome

A recommendation. Everything built lives under
`docs/spikes/self-governance/` (VL-016 fence) or in a throwaway worktree;
the main checkout's store is never adopted by this spike.

## The questions (verbatim from spec/self-governance)

- oq-1: What does verdi policy adopt --starter produce in a copy of this
  repository's store, and what happens to store lint, model check, journey
  and readiness for the 28 active specs once a constitution and policy
  exist?
- oq-2: Can the CLAUDE.md ground rules be expressed as policy claims in the
  current kernel grammar? Try five. Which fit, which need a new payload
  kind?
- oq-3: Is the enforcement rung a new kernel field or expressible as an
  existing claim attribute, and which consumer counts rules per rung?
- oq-4: Does an exemption with a review window work end to end today for a
  build rule?
- oq-5: Where does the process-disclosure index live so readiness can read
  it, tried by recording wave 1's nine deferred items?

## Investigation plan

Throwaway worktree from main; `go build -o /tmp/verdi-spike ./cmd/verdi`.

1. **Adoption dry run (oq-1).** In the worktree run `verdi policy adopt
   --starter --profile solo --owner <handle>` (the verb commits; that is
   why the worktree is throwaway). Record `git show --stat` of the adoption
   commit, then run `verdi lint`, `verdi model check`, and `verdi journey
   --json` plus `verdi spec doc` for three specs (readiness-recovery,
   spec-documents, one component spec) before and after. Diff the blocker
   lists and count by reason code. Repeat with `--profile team`.
2. **Expressibility (oq-2).** Read `internal/policyartifact/payload.go`,
   `claim.go` and `grammar.go`. For each of the five rules write the claim
   as YAML in the grammar as it stands and run it through the decoder in a
   scratch test under the fence. Record: fits as-is, fits with a new
   payload kind, cannot be expressed (and why).
3. **Rung field (oq-3).** From the same files determine whether a claim
   already carries an attribute that can hold a closed-enum rung without a
   schema change; if not, draft the field and list every decoder and lint
   site it touches. State whether design spec 02 or the policy kernel's
   own spec must be amended first. Identify the consumer that would count
   rules per rung (a `verdi policy` subcommand, a lint rule, or readiness).
4. **Exemption path (oq-4).** Author an exemption for "golangci-lint runs
   the standard set only" with a review window that has already lapsed,
   run `verdi lint` and `verdi journey --json` and check whether an
   eventual blocker (exemption-ineffective or its reason code) appears; then
   with a future window and confirm it does not. Record every command and
   output.
5. **Disclosure index (oq-5).** Take the nine deferred, residual and parked
   items from the wave-1 SDD ledger. Record them in each candidate home:
   (a) obligations under a self-hosted spec in `.verdi/obligations`, (b) a
   new record file under `.verdi` in the blocker shape, (c) leave them in
   the ledger and parse with a scratch gate test. For each, run `verdi
   journey --json` and check whether the items surface. Judge by: appears
   in readiness, has an owner and clearing condition, survives the ledger
   folder's deletion.

Timebox: one working day.

## What "answered" means

- oq-1: the adoption commit stat, blocker counts by reason code for both
  profiles, and any lint or model-check failure verbatim.
- oq-2: a five-row fit table with the YAML that decoded, or the decoder
  error.
- oq-3: attribute or schema, the touched-sites list, the amendment order.
- oq-4: the two command transcripts and yes or no.
- oq-5: one home recommended with the nine items recorded in it.

## Spec seed

oq-1 fixes ac-1's expected blocker handling and co-3's exemptions list.
oq-2 and oq-3 fix ac-2's field and whether co-1's ratification precedes the
build. oq-4 confirms ac-4 verbatim or turns it into a defect against
spec/readiness-recovery ac-1. oq-5 fixes ac-5's index home and dc-3.

## Throwaway rule

The adopted store, the scratch claims, the exemption and the recorded
disclosures are evidence. The feature's plan performs the real adoption on
its own branch under the gate.
