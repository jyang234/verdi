---
id: spec/strict-lint-target-v2
kind: spec
title: "A second, stricter lint target beside the parity one"
owners: [platform-team]
class: feature
problem: { text: "The module runs golangci-lint's standard set only, five of the 111 linters the installed version offers, and holds that configuration in lockstep with the upstream dependency for a legitimate parity reason; the consequence is that Go ground rules stated in the repository's rules file (docs/ground-rules.md, moved there from CLAUDE.md) have named, available enforcement sitting unused, and a wave-1 review minor in exactly that class (package-level mutable state) was found by a human reviewer rather than a tool (process-audit PA-016).", anchor: problem }
outcome: { text: "A second golangci-lint target enumerates the linters that enforce written ground rules, each mapped to the rule it enforces, runs inside make verify, and is ratcheted so that pre-existing findings never block while new code must be clean; the parity configuration stays byte-identical to its reference.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "A second configuration file at the repository root, .golangci.strict.yml, enumerates the ground-rule linters (containedctx, noctx, contextcheck, errorlint, gochecknoglobals), each carrying a comment naming the docs/ground-rules.md rule it enforces, and caps no findings; make lint-strict runs it at the same pinned golangci-lint version as the parity target and is a step of make verify, run by the CI gate jobs that run make verify's steps; the parity configuration's bytes are pinned by a static witness and unchanged.", evidence: [static, behavioral, attestation], anchor: ac-1 }
  - { id: ac-2, text: "The strict target is a ratchet over a committed baseline of findings: a finding the baseline holds never fails the step, a finding it does not hold does, a baseline that allows more of a finding than the current run reports fails, so the change that fixes a finding also removes its allowance, and a change that adds to the baseline fails against the baseline at its merge base with main; findings are compared as counts per key (linter, package, message, flagged source line), both generation and check lint for GOOS=linux GOARCH=amd64, so the result is the same in CI and locally, and a golangci-lint failure other than reporting findings is an operational error (exit 2); a regression witness proves that a finding fixed in one change and reintroduced in a later one fails.", evidence: [behavioral, attestation], anchor: ac-2 }
  - { id: ac-3, text: "A committed retro-witness proves the target reaches PA-016's class: at commit 410db101 the strict target reports the package-level variables serveReadinessLoader and serveReadinessDefaultSpec, and at e4d5b141, the commit that removed them, it reports neither; the witness runs in CI where the pinned golangci-lint is installed and never passes by skipping there.", evidence: [behavioral, attestation], anchor: ac-3 }
  - { id: ac-4, text: "No linter in the strict set is disabled or excluded wholesale to reach green; every exclusion names a path or a pattern and a reason, and a static witness counts the exclusions so the number is visible.", evidence: [static, attestation], anchor: ac-4 }
constraints:
  - { id: co-1, text: "The existing .golangci.yml is not modified: its parity with verdi-go's configuration is real and load-bearing; the strict target is an addition.", anchor: co-1 }
  - { id: co-2, text: "Both targets run the same pinned golangci-lint version and the Makefile's version pin remains the single source; CI installs it once.", anchor: co-2 }
  - { id: co-3, text: "The strict step's wall clock is recorded by the gate timing series and declared as a budget before the step joins make verify.", anchor: co-3 }
decisions:
  - { id: dc-1, text: "A second target, not a widened one (PA-016 remedy constraint): the parity reason is not disputed, so the ground rules get their own configuration whose findings are ratcheted rather than fixed retroactively across about 200,000 source lines.", anchor: dc-1 }
  - { id: dc-2, text: "The gated set is containedctx and contextcheck (context is never stored in a struct; it is the first parameter of anything that blocks or does I/O), noctx (the same rule for standard-library calls that do I/O without a context), errorlint with only its errorf check (errors wrap with %w; its comparison and type-assertion checks enforce a rule the ground rules do not state), and gochecknoglobals (no package-level mutable state). Not gated: dupl, which golangci-lint runs per package and which therefore cannot see a copy across packages (owner decision 2026-09-21); exhaustive, which scored 0 of 20 true positives against 'unknown enum values fail closed' even with default-signifies-exhaustive, because this code refuses unknown values when it decodes them, not at each switch; and, by scope rather than for want of a rule, ireturn (return structs), wrapcheck (wrap errors with context) and gochecksumtype (unknown enum values fail closed, for interface sum types): the owner's 2026-09-21 decision fixed the gated set without them and their true-positive rates are unmeasured, so those three rules stay enforced by review until a later revision measures and gates them. Test files are linted like any other Go code.", anchor: dc-2 }
  - { id: dc-3, text: "The ratchet is a committed baseline, not golangci-lint's --new-from-rev (owner decision 2026-09-21). Findings are counted per key, because identical findings repeat within a file, and the key leaves out file and line so that a line shift or a move within a package does not turn an old finding into a new one; editing a flagged line or moving it to another package does, and that finding is then fixed. Output is byte-identical across cache states and invocation paths but not across operating systems, since build-constrained files differ, so generation and check both lint for linux/amd64. The committed baseline always equals the current findings, never more: an allowance left behind after its finding is fixed would let the finding return unseen. golangci-lint runs with --issues-exit-code=0 so a reported finding is data, and any nonzero exit is operational.", anchor: dc-3 }
  - { id: dc-4, text: "The retro-witness pair is 410db101 before and e4d5b141 after. The first revision's pair was wrong: both variables are still present at 410db101, and 5f60c76c does not descend from 1be75d01, so its silence shows only that the code did not exist yet. Both commits build and are in main's history, so any full clone reproduces the witness, and the lint diff between them is exactly the two variables. The duplicated dot-dot path helper leaves the witness, because no gated linter can see a copy across packages (dc-2).", anchor: dc-4 }
  - { id: dc-5, text: "The declared budget for make lint-strict is 60 seconds in any cache state (co-3): cold runs measured 17.6 to 23.4 seconds on the development machine on 2026-09-30, warm runs about 1.3 seconds, and the spike's worst cold run was 50.2 seconds under heavy load. The gate timing series records every make verify step, so the step's actual time is recorded beside the figure.", anchor: dc-5 }
  - { id: dc-6, text: "The strict configuration is passed explicitly with --config and sits at the repository root: golangci-lint then reads that file alone, never merging .golangci.yml, and reports paths relative to the repository root, which keeps the baseline portable; make lint is unchanged.", anchor: dc-6 }
stubs:
  - { slug: strict-lint-gate, acceptance_criteria: [ac-1, ac-2, ac-4] }
  - { slug: strict-lint-reach, acceptance_criteria: [ac-3] }
links:
  - { type: supersedes, ref: "spec/strict-lint-target" }
supersession:
  carried: [ac-4, co-1, co-2, co-3, dc-1]
  amended:
    - { id: ac-1, note: "names the gated linters (containedctx, noctx, contextcheck, errorlint, gochecknoglobals), the file and its root location, uncapped findings, the ground-rules file in place of CLAUDE.md (process-hardening A5), and the CI gate jobs that run make verify's steps; spike oq-1, oq-2, oq-4; dc-2, dc-6" }
    - { id: ac-2, note: "the ratchet is a committed, shrink-only baseline compared as counts per key that never allows more than the current run reports, with a fix-then-reintroduce regression witness, linted for linux/amd64, with operational failures exiting 2; spike oq-3 and the 2026-09-30 re-measurement; dc-3" }
    - { id: ac-3, note: "the retro-witness pair becomes 410db101 and e4d5b141 and names both variables; the dot-dot helper half is dropped because dupl cannot reach it (owner decision 2026-09-21); the witness runs where the pinned linter is installed; dc-4" }
  amended_advisory: []
  removed:
    - { id: oq-1, note: "answered by spec/strict-lint-target-spike (docs/spikes/strict-lint-target/README.md, oq-1) and re-measured at a303a71f on 2026-09-30: five gated linters, about 1,066 findings; absorbed into dc-2" }
    - { id: oq-2, note: "answered by spec/strict-lint-target-spike (oq-2) and the 2026-09-30 exhaustive re-measurement (0 of 20 true positives); absorbed into dc-2 and dc-4" }
    - { id: oq-3, note: "answered by spec/strict-lint-target-spike (oq-3), a committed baseline over --new-from-rev (owner decision 2026-09-21); absorbed into dc-3" }
    - { id: oq-4, note: "answered by spec/strict-lint-target-spike (oq-4) and re-proven at a303a71f: --config reads only the strict file; absorbed into dc-6" }
  added: [dc-2, dc-3, dc-4, dc-5, dc-6]
---
# A second, stricter lint target beside the parity one (v2)

## Problem

Ground rules the repository writes down in `docs/ground-rules.md` are
enforced only by an agent remembering them, while the installed linter can
enforce several of them today. The minimal posture exists for a good
reason (parity with the upstream dependency's configuration) and this
feature does not dispute it; it adds enforcement beside it.

| Ground rule | Linter |
|---|---|
| Context is never stored in a struct | containedctx |
| Context is the first parameter of anything that blocks or does I/O | contextcheck, noctx |
| Errors wrap with `%w` | errorlint (errorf check) |
| No package-level mutable state | gochecknoglobals |

Three more written rules have linters this revision leaves out of the gate
(dc-2): "return structs" (ireturn) and "wrap with context" (wrapcheck) by
scope, and "unknown enum values fail closed" because `exhaustive` measured
as noise against it and `gochecksumtype` has nothing annotated to check.
Review still enforces them.

## Outcome

New code meets the written rules by construction; old findings are visible
and shrink over time; the parity file is untouched.

## What changed from v1

spec/strict-lint-target was accepted with four open questions and one
spike stub. The spike (spec/strict-lint-target-spike, answer at
`docs/spikes/strict-lint-target/README.md`) answered all four, and the
owner decided on 2026-09-21: `dupl` leaves the gate, `exhaustive` is
re-measured with `default-signifies-exhaustive: true` before the list is
final, `gochecknoglobals` stays with a written rule for it, and the ratchet
is a committed baseline. The rule landed with the rules file
(`docs/ground-rules.md`, process-hardening unit A5, PR #382).

A re-measurement at main `a303a71f` on 2026-09-30 settled the rest:

- **exhaustive leaves the gate.** With the setting, it reports 73
  findings; two samples of ten scored no true positive against "unknown
  enum values fail closed", because the code refuses unknown values when it
  decodes them. Five of the twenty would silently take a permissive branch
  if a new member were added to the enum, which is a real risk the written
  rule does not state; a switch-level rule would bring the linter back.
- **The baseline would ship about 1,066 findings** across the five gated
  linters (containedctx 10, noctx 329, contextcheck 62, errorlint 140 with
  only the errorf check, gochecknoglobals 525; 1,183 with exhaustive and
  errorlint's other checks). It grew about a quarter since the spike, so
  the sooner it is committed, the smaller it is.
- **The retro-witness pair was wrong.** 410db101 still has both variables,
  and 5f60c76c does not descend from 1be75d01. The removal is e4d5b141.
- **Output differs by operating system** (1,183 findings on darwin, 1,188
  for linux, from build-constrained files), so both generation and check
  lint for linux/amd64.
- **The CLI's default caps hide findings** (171 of 1,183 shown), so a new
  finding identical to three old ones would pass unseen; the configuration
  caps nothing.
- The installed version offers 111 linters, not 114 (114 was the line
  count of its help output).

## Context from the process audit

- PA-016 is the entry; PA-004 is its parent shape (a rule expressible as a
  command should be a hook or a gate, not a sentence).
- PA-009: most review attention goes to lint-grade findings. Moving this
  class to a tool frees reviewer attention for the semantic reading that
  only a reviewer can do.
- Remedy design 6.1: a rule with an analyzer available sits on rung two;
  today these rules sit silently on rung five.

## ac-1

Two files, one pin, one gate. The parity file's digest is pinned so a
future "simplification" that merges them is caught. `make verify`'s steps
run in CI across the gate jobs, and `internal/specalign` fails unless every
step runs there, so the gate stub's lane adds `make lint-strict` to the
`static` job of both `merge-gate.yml` and `verify.yml`.

## ac-2

Determinism matters more than mechanism: a ratchet that gives different
answers locally and in CI teaches people to ignore it. The baseline is
compared three ways: the current findings against the committed baseline
(a new finding fails), the committed baseline against the current findings
(an allowance with no finding left fails, so a fixed finding cannot come
back later under an old allowance), and the committed baseline against
its copy at the merge base with main, read with `git show` (a baseline
that grew fails).

## ac-3

The retro-witness is the reach proof: the wave-1 minor a reviewer found
must be what the tool would have found. golangci-lint is installed only in
the CI `static` job, so the witness runs there; a local run without the
linter discloses the skip.

## ac-4

Exclusions are how strict targets quietly become weak ones. Count them.

## co-1

The parity file is not this feature's to change.

## co-2

One pin.

## co-3

Budget before adoption, measured (dc-5).

## dc-1

Add, do not widen.

## dc-2

Each gated linter enforces a sentence the rules file states, but not
every stated sentence has its linter gated: ireturn, wrapcheck and
gochecksumtype map to written rules and stay out by scope, with review
enforcing those rules meanwhile. A linter
that is mostly noise against the rule it claims to enforce belongs in a
report, not the gate (the spike's own test), which is where `exhaustive`
landed. Linting test files follows from the rules binding all Go code; most
of the `noctx` and `contextcheck` findings are in tests and sit in the
baseline.

## dc-3

The key: linter, package, message, and the flagged source line. A pure
file rename inside a package keeps every key; a move across packages or an
edit to a flagged line does not, and the finding is then fixed rather than
carried. `exhaustive` would have re-keyed every switch whenever an enum
gained a member, another reason it is not gated.

## dc-4

Both commits build (the fix's parent does not type-check, so it cannot
serve), and both are in main's history, so no tag is needed to reproduce
them.

## dc-5

Declared before the step joins `make verify`, as co-3 requires.

## dc-6

Proven by poisoning `.golangci.yml` to exclude every gated linter: the
strict run still reported every finding, and a plain run that read the
poisoned file reported none.
