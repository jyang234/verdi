---
id: spec/mutation-ratchet
kind: spec
title: "Mutation testing as a ratchet"
owners: [platform-team]
class: feature
problem: { text: "Nothing in the gate checks that a test would fail if the behaviour it names were broken: a test can run, pass, and constrain nothing, and every downstream claim treats it as proof of its criterion. Process-audit PA-017's witness is wave-1 Task 2 of spec/readiness-recovery, where the persistence check for constraint co-2 searched only for new files whose names contained the word readiness, so a loader writing any differently-named file passed it; a reviewer found it, no tool could have.", anchor: problem }
outcome: { text: "A gate step deliberately faults the production code a change touched and requires that some test fails; a change may add no new surviving mutants, existing survivors are grandfathered in a committed baseline that can only shrink, and the posture is hard where the specification's risk tier demands it and report-only elsewhere, so test load-bearingness is measured instead of assumed.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "A mutation step scoped to the Go files changed since the merge base with main runs a pinned, hermetic mutation tool and writes a canonical survivor report (file, line, operator, status) with sorted keys and a trailing newline; the same tree at the same merge base produces identical report bytes twice; no network is used at run time.", evidence: [behavioral, attestation], anchor: ac-1 }
  - { id: ac-2, text: "A committed baseline lists the grandfathered survivors; the step fails when a change leaves a surviving mutant that is not in the baseline, passes when it kills baseline survivors, and the baseline is regenerated only by a named, disclosed command whose diff a reviewer reads, the same posture the digest ratchets already take.", evidence: [behavioral, static, attestation], anchor: ac-2 }
  - { id: ac-3, text: "A committed regression fixture proves the step reaches PA-017's class: the co-2 persistence test as it stood before fix commit 410db101 lets a mutant that writes an arbitrary file survive, and the test as fixed kills it; the fixture is part of the step's own tests.", evidence: [behavioral, attestation], anchor: ac-3 }
  - { id: ac-4, text: "The hard-or-report posture is declared per package path in one committed manifest that names the spec risk tier motivating each entry; a package with no entry is report-only; the manifest is strict-decoded and a missing or unknown tier fails closed.", evidence: [static, behavioral, attestation], anchor: ac-4 }
constraints:
  - { id: co-1, text: "Zero survivors is never required: equivalent mutants exist and a threshold of zero would block work indefinitely; the requirement is always relative to the baseline.", anchor: co-1 }
  - { id: co-2, text: "The mutation tool is pinned by version exactly as golangci-lint is, installed by the workflow before make verify, and never fetched during the gate; a missing tool in CI fails the step, never skips it.", anchor: co-2 }
  - { id: co-3, text: "The step's wall clock is recorded by the gate's timing series and a budget is declared in this spec's decisions before the step joins make verify; the step never mutates files the change did not touch.", anchor: co-3 }
  - { id: co-4, text: "No test is weakened, deleted, or marked to make a mutant die; a mutant that cannot be killed is baselined with a one-line reason.", anchor: co-4 }
decisions:
  - { id: dc-1, text: "A ratchet, not a threshold: the requirement is 'no new survivors' relative to a committed baseline, mirroring the repository's digest ratchets, because zero is unreachable in the presence of equivalent mutants and any fixed percentage is arbitrary.", anchor: dc-1 }
  - { id: dc-2, text: "Scope the run, not the requirement: only files the change touched are mutated and only the tests covering them run, because a whole-tree run against a workbench package whose race tests alone take about 147 seconds does not fit a per-wave gate.", anchor: dc-2 }
open_questions:
  - { id: oq-1, text: "Which Go mutation tool runs on this module (Go 1.25, 105 packages, build tags, test binaries that exec cmd/verdi) hermetically and can be pinned like golangci-lint? Candidates: gremlins, go-mutesting, ooze.", anchor: oq-1 }
  - { id: oq-2, text: "What are the runtime, mutant count, and survivor count of a touched-files run over the wave-1 diff of spec/readiness-recovery (b810c302..e963f4d0), and does that fit inside a per-wave gate whose steps are now timed?", anchor: oq-2 }
  - { id: oq-3, text: "Does the technique reach the witness: with the co-2 persistence test reverted to its pre-410db101 form, does a mutant that writes an arbitrary file survive, and does the fixed test kill it?", anchor: oq-3 }
  - { id: oq-4, text: "What is the baseline record's shape and home (per-package survivor list under testdata, a digest, or the tool's own report), how are equivalent mutants marked, and how is a regeneration disclosed?", anchor: oq-4 }
  - { id: oq-5, text: "Can the hard-or-report posture be derived from the spec risk tiers (dc-6 of each feature assigns tiers per criterion, and verdi.bindings.yaml binds producers to criteria, but nothing maps files to criteria), or must the manifest be authored by hand?", anchor: oq-5 }
stubs:
  - { slug: mutation-tool-fit, spike: true, resolves: [oq-1, oq-2, oq-3, oq-4, oq-5] }
links:
  - { type: depends-on, ref: spec/readiness-recovery }
---
# Mutation testing as a ratchet

## Problem

A passing test is treated everywhere in this build as proof of its
criterion, and nothing verifies that inference. The gate can tell that a
test ran and passed; it cannot tell that the test would have failed had the
behaviour it names been broken. The one known witness (PA-017) was found by
a reviewer reading a constraint beside an assertion. That catch depends on a
second reader existing and paying attention, which is the single
load-bearing dependency the process audit names.

## Outcome

Fault the code on purpose and demand that a test notices. Surviving mutants
are direct, model-free proof that no test constrains a behaviour. Requiring
"no new survivors" per change turns test load-bearingness into a measured,
ratcheting quantity rather than an assumption.

## Context from the process audit

- PA-001 (parent): a wrong reading of a criterion freezes into a permanently
  green test. This feature closes its mechanical half, PA-017. The other
  half, PA-018, is spec/verification-rules; neither closes PA-001 alone.
- PA-017 witness: `internal/readinessload` persistence check before
  410db101 grepped new file names for "readiness" only. The corrected test
  snapshots the data tree.
- PA-025: gate duration is now recorded per step by `make verify`; this
  step's budget must be declared against that series (co-3).
- Remedy design 6.2 in the audit ledger: ratchet not threshold, scope to
  touched files, tier the hard requirement, report the rest.
- Highest expected yield: tests asserting that something does *not* happen.
  Lowest: code already pinned by golden files.

## ac-1

Determinism is what makes a survivor report a ratchet input: the same tree
must produce the same bytes so the baseline diff is meaningful. Canonical
JSON, sorted, trailing newline, as every other generated artifact here.

## ac-2

The baseline is the grandfather list. It shrinks when someone kills a
survivor; it grows only through a named regeneration whose diff is read,
exactly as `make fixture-regen` and the digest ratchets are handled.

## ac-3

The retro-witness is the proof that this step is not theatre: it must
reproduce the one defect the audit already attributes to this class.

## ac-4

Tiers already exist in every feature's dc-6. The manifest makes the mapping
from tier to gate posture explicit and countable, and fails closed on an
unknown tier.

## co-1

Equivalent mutants are permanent survivors by construction. A zero target
would make the gate unpassable.

## co-2

Same pin discipline as golangci-lint; the gate never fetches.

## co-3

The step joins the gate only with a declared budget, measured against the
timing series `make verify` now records.

## co-4

A ratchet that can be satisfied by weakening tests is worse than none.

## dc-1

Ratchet, not threshold.

## dc-2

Touched files only.

## oq-1

Tool fit. The module has test packages that build and exec `cmd/verdi`
(CROSS_BINARY_PKGS in the Makefile), race tests, and golden fixtures; a tool
that rewrites sources in place must not corrupt any of them. Answer needs an
install pin, a hermetic run, and a runtime on two representative packages.

## oq-2

Yield and cost on a real diff. The wave-1 range is the natural sample: it
contains the witness, five tasks, and about twenty commits across journey,
readinessload, policyconflict, workbench, mcpserve and cmd/verdi.

## oq-3

Reach. If the technique cannot reproduce the known witness the feature has
no evidence it addresses PA-017 and should not be built.

## oq-4

Baseline shape. The repository already has two ratchet idioms (golden
digests under testdata, `golden-digests.json` in policyartifact); the
baseline should reuse one, not invent a third.

## oq-5

Tier mapping. If files cannot be mapped to criteria mechanically the
manifest is authored by hand and ac-4's "names the tier" is a citation, not
a derivation.
