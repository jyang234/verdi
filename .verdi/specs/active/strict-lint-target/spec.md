---
id: spec/strict-lint-target
kind: spec
title: "A second, stricter lint target beside the parity one"
owners: [platform-team]
class: feature
problem: { text: "The module runs golangci-lint's standard set only, five of the 114 linters the installed version offers, and holds that configuration in lockstep with the upstream dependency for a legitimate parity reason; the consequence is that six Go ground rules stated in CLAUDE.md have named, available enforcement sitting unused, and two wave-1 review minors in exactly that class (package-level mutable state, a helper duplicated across packages) were found by a human reviewer rather than a tool (process-audit PA-016).", anchor: problem }
outcome: { text: "A second golangci-lint target enumerates the linters that enforce written ground rules, each mapped to the rule it enforces, runs inside make verify, and is ratcheted so that pre-existing findings never block while new code must be clean; the parity configuration stays byte-identical to its reference.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "A second configuration file enumerates the ground-rule linters, each carrying a comment naming the CLAUDE.md rule it enforces; make lint-strict runs it at the same pinned golangci-lint version as the parity target and is a step of make verify; the parity configuration's bytes are pinned by a static witness and unchanged.", evidence: [static, behavioral, attestation], anchor: ac-1 }
  - { id: ac-2, text: "The strict target is a ratchet: findings present at the merge base with main never fail the step, a finding introduced by the change does, and the mechanism is deterministic in CI and locally for the same merge base.", evidence: [behavioral, attestation], anchor: ac-2 }
  - { id: ac-3, text: "A committed retro-witness proves the target reaches PA-016's class: at commit 1be75d01 the strict target reports the package-level readiness loader variables and the duplicated dot-dot path helper, and at 410db101 it reports neither.", evidence: [behavioral, attestation], anchor: ac-3 }
  - { id: ac-4, text: "No linter in the strict set is disabled or excluded wholesale to reach green; every exclusion names a path or a pattern and a reason, and a static witness counts the exclusions so the number is visible.", evidence: [static, attestation], anchor: ac-4 }
constraints:
  - { id: co-1, text: "The existing .golangci.yml is not modified: its parity with verdi-go's configuration is real and load-bearing; the strict target is an addition.", anchor: co-1 }
  - { id: co-2, text: "Both targets run the same pinned golangci-lint version and the Makefile's version pin remains the single source; CI installs it once.", anchor: co-2 }
  - { id: co-3, text: "The strict step's wall clock is recorded by the gate timing series and declared as a budget before the step joins make verify.", anchor: co-3 }
decisions:
  - { id: dc-1, text: "A second target, not a widened one (PA-016 remedy constraint): the parity reason is not disputed, so the ground rules get their own configuration whose findings are ratcheted rather than fixed retroactively across about 200,000 source lines.", anchor: dc-1 }
open_questions:
  - { id: oq-1, text: "What are the finding count and runtime per candidate linter (containedctx, noctx, contextcheck, errorlint, exhaustive, dupl, gochecknoglobals) on main today, and which other available linters map to a written ground rule?", anchor: oq-1 }
  - { id: oq-2, text: "What fraction of a sample of findings per linter is a true positive against the rule it is meant to enforce, and are the two wave-1 review minors reported at their pre-fix commit and silent at the fixed one?", anchor: oq-2 }
  - { id: oq-3, text: "Which ratchet mechanism is deterministic in CI and locally: golangci-lint's own --new-from-rev against the merge base with main (the merge-gate workflow checks out with fetch-depth 0), or a committed baseline of findings that only shrinks?", anchor: oq-3 }
  - { id: oq-4, text: "Can a second configuration coexist with the parity one without golangci-lint v2's configuration discovery picking the wrong file, and what does the Makefile invocation look like?", anchor: oq-4 }
stubs:
  - { slug: lint-yield, spike: true, resolves: [oq-1, oq-2, oq-3, oq-4] }
---
# A second, stricter lint target beside the parity one

## Problem

Six ground rules in CLAUDE.md are enforced only by an agent remembering
them, while the installed linter can enforce each of them today. The
minimal posture exists for a good reason (parity with the upstream
dependency's configuration) and this feature does not dispute it; it adds
enforcement beside it.

| Ground rule | Linter |
|---|---|
| Context is never stored in a struct | containedctx |
| Context is the first parameter of anything doing I/O | noctx, contextcheck |
| Errors wrap with `%w` | errorlint |
| Unknown enum values fail closed | exhaustive |
| Never copy-paste across packages | dupl |
| No package-level mutable state | gochecknoglobals |

## Outcome

New code meets the written rules by construction; old findings are visible
and shrink over time; the parity file is untouched.

## Context from the process audit

- PA-016 is the entry; PA-004 is its parent shape (a rule expressible as a
  command should be a hook or a gate, not a sentence).
- PA-009: most review attention goes to lint-grade findings. Moving this
  class to a tool frees reviewer attention for the semantic reading that
  only a reviewer can do.
- Remedy design 6.1: a rule with an analyzer available sits on rung two;
  today these six sit silently on rung five.

## ac-1

Two files, one pin, one gate. The parity file's digest is pinned so a
future "simplification" that merges them is caught.

## ac-2

Determinism matters more than mechanism: a ratchet that gives different
answers locally and in CI teaches people to ignore it.

## ac-3

The retro-witness is the reach proof: the two minors a reviewer found in
wave 1 must be what the tool would have found.

## ac-4

Exclusions are how strict targets quietly become weak ones. Count them.

## co-1

The parity file is not this feature's to change.

## co-2

One pin.

## co-3

Budget before adoption, measured.

## dc-1

Add, do not widen.

## oq-1

Counts first. `gochecknoglobals` over this tree may produce hundreds of
findings; whether that is tractable as a ratchet or needs per-path scoping
depends on the number.

## oq-2

True positives. A linter that is mostly noise against the rule it claims
to enforce belongs in a report, not the gate.

## oq-3

Mechanism. `--new-from-rev` needs a stable merge base; a baseline file
needs a regeneration protocol. Determinism decides.

## oq-4

Coexistence. golangci-lint v2 discovers `.golangci.yml` automatically; the
strict file must be passed explicitly and must not be discovered by
accident.
