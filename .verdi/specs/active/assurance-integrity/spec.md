---
id: spec/assurance-integrity
kind: spec
title: "Assurance Integrity"
owners: [platform-team]
class: feature
status: accepted-pending-build
problem: { text: "An external adversarial review (docs/design/external-assessment/, Priority 1 and its Wave A) found two integrity gaps in verdi's own repository: agent-facing instruction files (.claude/skills/*/SKILL.md, CLAUDE.md) can drift from the canonical CLI/lifecycle model with no gate ever noticing, and the closure ritual (verdi close, docs/design/specs/03-evidence-model.md §Closure ritual) leaves residue — specs whose active-zone status contradicts what git already shows happened, and stranded close/<name> branches — with no honest detection and no safe reclamation. The round accepted a story for each gap, spec/instruction-conformance and spec/closure-hygiene, but this store's own object model requires every non-spike story to carry a resolving implements edge to a feature acceptance criterion (internal/artifact/spec.go's validateStory/requireFragment, enforced unconditionally by VL-003) — and no active feature covered either gap. Authored independently, both stories hit the identical structural blocker and resolved it two different, incompatible ways: instruction-conformance disclosed a permanently dangling placeholder edge and accepted one standing VL-003 finding (R4-I-58); closure-hygiene self-authored its own bespoke parent feature, spec/closure-residue, because its own dispatch required an unconditionally green verdi lint (R4-I-67) — a parentless-story gap this feature exists to close with one convergent answer.", anchor: "#problem" }
outcome: { text: "A single small parent feature gives both stories a real, resolving implements target, sized to exactly what they already deliver: AC-1 is instruction-conformance's outcome (agent-facing instruction files mechanically checked against the canonical CLI verb set and a named retired-ritual tripwire, so drift fails make verify by naming the offending file and reference); AC-2 is closure-hygiene's outcome (verdi audit honestly detects and reports closure-ritual residue — status/git contradictions and stranded branches — with a concrete witness, never guessing and never mutating). Both ACs are fully delivered by their one stub story, so this feature closes the moment both stories close, carrying no aspirational AC neither story builds (DC-1) and none of the external-assessment round's other accepted items (CO-1).", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "internal/specalign gains a new, purely mechanical check that enumerates every agent-facing instruction file (a filesystem glob over .claude/skills/*/SKILL.md, plus the required repo-root CLAUDE.md), validates every `verdi <verb>` reference it makes against dispatch.go's own recognized-verb set by driving the real built binary, and independently tripwires the retired two-phase commit-to-design ritual by name — so a removed verb or a taught-as-current retired procedure fails `make verify`, naming the offending file and reference, proven to fire in both directions against a planted fixture before it is trusted against this repo's own tree. Delivered in full by this feature's one stub story, spec/instruction-conformance.", evidence: [static, behavioral, attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "verdi audit gains a third, additive report section that names every active-zone spec whose declared status contradicts what git already shows happened (a closure ritual that ran and never merged; a feature whose every stub is closed and merged yet the feature itself never closed) and every close/<name> branch left unmerged, plus a read-only survey of merged-but-undeleted branches and worktrees — each finding a concrete, named witness, three-valued honest (never guessing where git state cannot decide), and never performing a git-mutating call. Delivered in full by this feature's one stub story, spec/closure-hygiene; conservative reclamation of the provably-dead residue this AC surveys is explicitly out of this feature (CO-2).", evidence: [static, behavioral, attestation], anchor: "#ac-2" }
decisions:
  - { id: dc-1, text: "This feature exists solely to close the R4-I-58 parentless-story gap for these two, already-in-flight stories — not to re-open or re-scope the external-assessment round. Investigated the same four active features R4-I-58/R4-I-67 each independently checked (code-health, disclosure-legibility, public-showcase, scoping-canvas) plus the archived corpus; none covers either gap by subject. Its scope is fixed at exactly its two stub stories' outcomes: a future related item — the reclamation half of closure hygiene once ratified (OQ-1/CO-2), or any other assessment-round item — gets its own feature (or a ratified amendment to this one, if genuinely the same shape), never silently folded in here after acceptance.", anchor: "#dc-1" }
constraints:
  - { id: co-1, text: "Scope is exactly these two stories' outcomes — none of the external assessment's other accepted or surviving items enter this feature: safe gate enforcement and an aggregate required check (assessment 02 §Priority 2), the license/version boundary (§Priority 3), instrumented external pilots (§Priority 4), risk-scoped assurance profiles (§Priority 6), the evidence-adapter contract (§Priority 7), the architecture-specific threat model (§Priority 8), machine-assisted obligation authoring (§Priority 9), and the risk inbox (§Priority 10) are all out. `verdi doctor` and a conservative `verdi init` (§Priority 5) are sanctioned separately and already recorded with their own future story (R4-I-56) — not stubbed here, and not this feature's to build.", anchor: "#co-1" }
  - { id: co-2, text: "Conservative reclamation of provably-dead worktrees and branches — the gc-extension half of the closure-hygiene problem this feature's AC-2 only detects and reports — is explicitly out. Neither in-flight story stubs it: closure-hygiene's own finding is that reclaiming anything outside .verdi/data/ needs a ratification-flow amendment to verdi-store-layout's Garbage collection section first (R4-I-66), which has not happened. This feature carries no AC that AC-2's stub story does not itself deliver (DC-1) — reclamation gets its own future AC once, and only once, a story is ready to build it (see OQ-1).", anchor: "#co-2" }
open_questions:
  - { id: oq-1, text: "RESOLVED by controller adjudication under the owner's single-parent directive (2026-07-19). spec/closure-residue (class: feature, status: draft) was committed to the sibling verdi-wt/closure-hygiene worktree (18858ab) — authored independently by that stream before this feature existed, to resolve the identical R4-I-58-shaped blocker under a stricter lint-must-be-green tolerance than instruction-conformance's own dispatch carried (R4-I-67, which flagged both resolutions 'for the controller to reconcile if a single house convention is wanted going forward'). Its ac-1 (detection) duplicated this feature's AC-2 almost verbatim; the owner directed exactly one small parent feature, so the controller designated this feature that parent and WITHDREW spec/closure-residue: spec/closure-hygiene's implements edge was re-pointed from spec/closure-residue#ac-1 to spec/assurance-integrity#ac-2 and the closure-residue directory removed, and spec/instruction-conformance's placeholder edge was re-pointed to spec/assurance-integrity#ac-1 (clearing its standing R4-I-58 VL-003 finding). Its ac-2 (reclamation, deliberately unstubbed, blocked on the ratification question CO-2 and R4-I-66 name) has no counterpart here and needs none yet — its unique content, the candidate verdi-store-layout amendment language and the full conservative reclaim design, survives verbatim in spec/closure-hygiene dc-5 and ledger R4-I-66, and its future home is a new feature authored once, and only once, that ratification question resolves (CO-2). Recorded in ledger R4-I-70, which supersedes the R4-I-58-vs-R4-I-67 divergence.", anchor: "#oq-1" }
stubs:
  - { slug: instruction-conformance, acceptance_criteria: [ac-1] }
  - { slug: closure-hygiene, acceptance_criteria: [ac-2] }
frozen: { at: 2026-07-19, commit: 7f0143941b01f8a7daca8ef1f4513d4ee0efa868 }
---
# Assurance Integrity

## Problem

An external adversarial review
(`docs/design/external-assessment/`, Priority 1 and its Wave A) found two
integrity gaps in verdi's own repository:

1. **Agent-instruction drift.** A committed agent skill,
   `.claude/skills/commit-to-design/SKILL.md`, still instructs an agent to
   run `verdi board commit` and consume a frozen `board.json` as the
   *current* way to finish a design-branch spec, while the architecture and
   this store's own component specs say that ritual is retired. Nothing
   walked `.claude/skills/` or the repo-root `CLAUDE.md` to catch this —
   agent-facing instructions could drift from the canonical CLI/lifecycle
   model with no gate ever noticing.
2. **Closure residue.** `verdi close` (`docs/design/specs/03-evidence-model.md`
   §Closure ritual) commits its archival output to whatever branch it runs
   on and stops there — it never checks whether that output reached the
   default branch. On this repository's own main, this closure
   residue was live and unaddressed when the round was accepted. As of
   2026-07-19, a closure ritual had run and never landed —
   `close/showcase-corpus-renovation`'s tip (`24214fd`) moved
   `spec/showcase-corpus-renovation` to `archive/` while the branch stayed
   unmerged, so the spec still read `accepted-pending-build`; that canonical
   exemplar has since been resolved by PR #170 (the closure merged, the
   branch deleted), and the spec reads archived on main now. Several more
   `close/<name>` branches remain pure leftover, and a stub-complete feature
   (`spec/code-health`) still sits unclosed. `verdi gc` reclaims managed
   worktrees only, so none of this workspace-wide residue is even visible.

The round accepted one story for each gap — `spec/instruction-conformance`
and `spec/closure-hygiene` — but this store's object model requires every
non-spike story to carry a resolving `implements` edge to a **feature**
acceptance criterion (`internal/artifact/spec.go`'s `validateStory`/
`requireFragment`, enforced unconditionally by `VL-003`, no draft
exemption). No feature active at the time either story was authored covered
either gap.

Authored independently, in separate worktrees, both stories hit the
identical structural blocker and resolved it two different, incompatible
ways:

- `spec/instruction-conformance` disclosed a permanently dangling
  placeholder edge (`spec/todo-replace-feature-name#ac-1`, the scaffolder's
  own default) and accepted one standing `VL-003` finding, recorded in
  `PLAN-V1.md` as **R4-I-58**.
- `spec/closure-hygiene` self-authored its own bespoke parent feature,
  `spec/closure-residue`, because its own dispatch instruction required an
  unconditionally green `verdi lint` with no disclosed-finding tolerance —
  recorded as **R4-I-67**, which states explicitly that "the two dispatching
  tasks stated different tolerances, and both resolutions are disclosed
  here for the controller to reconcile if a single house convention is
  wanted going forward."

This is exactly the parentless-story gap R4-I-58 named. This feature exists
to close it with one convergent, shared answer instead of two divergent
ones.

## Outcome

A single small parent feature gives both stories a real, resolving
`implements` target, sized to **exactly** what they already deliver — no
more:

- **AC-1** is `spec/instruction-conformance`'s outcome: agent-facing
  instruction files mechanically checked against the canonical CLI verb set
  and a named retired-ritual tripwire, so drift fails `make verify` by
  naming the offending file and reference.
- **AC-2** is `spec/closure-hygiene`'s outcome: `verdi audit` honestly
  detects and reports closure-ritual residue — status/git contradictions
  and stranded branches — with a concrete witness, never guessing and never
  mutating.

Both ACs are fully delivered by their one stub story, so this feature can
close the moment both stories close. It carries no aspirational AC that
neither story builds (DC-1), and none of the external-assessment round's
other accepted or surviving items — safe gate enforcement, `verdi doctor`/
`init`, the license/version boundary, risk-scoped profiles, the
evidence-adapter contract, the threat model, obligation authoring, the risk
inbox, or external pilots (CO-1). Conservative reclamation of the residue
AC-2 surveys is also out (CO-2) — it is not licensed yet, and neither story
builds it.

## AC-1

`internal/specalign` gains a new, purely mechanical check — no semantic or
natural-language drift detection — built from three parts: enumeration of
every agent-facing instruction file by filesystem glob
(`.claude/skills/*/SKILL.md`, plus the required repo-root `CLAUDE.md`, so a
newly added skill is picked up with no code change); verb validation, which
extracts every `verdi <verb>` reference an enumerated file makes and checks
it against `dispatch.go`'s own recognized-verb set by driving the real
built binary; and a retired-ritual tripwire that catches what verb
validation structurally cannot — a still-dispatched verb (`board`) used to
teach a retired procedure as current.

A removed verb, or a retired-ritual phrase with no retirement disclosure,
fails `make verify`, naming the offending instruction file and the exact
reference. Both directions are proven to actually fire against a planted,
committed fixture before the check is trusted against this repo's own
tree — a fixture-free check whose red direction is unexercised is itself a
silent pass.

Delivered in full by this feature's one stub story, `spec/instruction-conformance`.

## AC-2

`verdi audit` gains a third report section, additive to its existing
exemption and spec-stale audits, that names:

- every active-zone spec whose declared status contradicts what git already
  shows happened — a stranded closure ritual (a `close/<name>` branch whose
  tip already moved the spec to `archive/`, unmerged), and a stub-complete
  feature (every declared stub realized by a closed, merged story) that
  never itself closed;
- every `close/<name>` branch left unmerged into the default branch,
  classified by whether its own archive move already landed some other way;
- a read-only survey of merged-but-undeleted branches and worktrees,
  workspace-wide, not limited to managed worktrees.

Every finding names a concrete witness — the branch, its tip commit, the
exact contradiction — never a guess where git state cannot decide. A
stranded closure ritual flags the run (an operator-actionable defect,
`exit 1`); the survey and superseded-elsewhere findings are reported but
never flip the exit code, since they are ordinary git housekeeping, not
defects. No git-mutating call is ever performed.

Delivered in full by this feature's one stub story, `spec/closure-hygiene`.
Conservative reclamation of the residue this AC surveys is explicitly out
of this feature (CO-2) — see OQ-1 for the related, unresolved
`spec/closure-residue` overlap.

## DC-1

This feature exists solely to close the R4-I-58 parentless-story gap for
these two, already-in-flight stories — not to re-open or re-scope the
external-assessment round.

Investigated the same four active features R4-I-58 and R4-I-67 each
independently checked — `code-health`, `disclosure-legibility`,
`public-showcase`, `scoping-canvas` — plus the archived corpus, for a
genuine fit. None covers either gap by subject (`code-health`'s own `co-3`
excludes it explicitly; the rest are unrelated by subject).

Its scope is fixed at exactly its two stub stories' outcomes. A future
related item — the reclamation half of closure hygiene once ratified
(OQ-1/CO-2), or any other item from the external-assessment round — gets
its own feature, or a ratified amendment to this one if it is genuinely the
same shape, never silently folded in here after acceptance. Keeping this
feature small and closeable is the explicit point (owner directive,
2026-07-19): it should close when its two stories close, not stay open for
months absorbing every adjacent finding the same review raised.

## CO-1

Scope is exactly these two stories' outcomes — none of the external
assessment's other accepted or surviving items enter this feature:

- safe gate enforcement and an aggregate required check
  (`docs/design/external-assessment/02-surviving-priorities-and-feature-gap-analysis.md`
  §Priority 2);
- the license/version boundary (§Priority 3);
- instrumented external comparative pilots (§Priority 4);
- risk-scoped assurance profiles (§Priority 6);
- the evidence-adapter contract and reference integrations (§Priority 7);
- the architecture-specific threat model (§Priority 8);
- machine-assisted obligation authoring (§Priority 9, itself Conditional in
  the source review);
- the risk inbox (§Priority 10, itself Deferred in the source review).

`verdi doctor` and a conservative `verdi init` (§Priority 5) are sanctioned
separately and already recorded with their own future story — **R4-I-58's
sibling entry R4-I-56** — not stubbed here, and not this feature's to
build.

## CO-2

Conservative reclamation of provably-dead worktrees and branches — the
gc-extension half of the closure-hygiene problem that this feature's AC-2
only detects and reports — is explicitly out.

Neither in-flight story stubs it: `spec/closure-hygiene`'s own finding is
that reclaiming anything outside `.verdi/data/` needs a ratification-flow
amendment to `verdi-store-layout`'s Garbage collection section first
(**R4-I-66**), which has not happened. This feature carries no AC that
AC-2's stub story does not itself deliver (DC-1) — reclamation gets its own
future AC once, and only once, a story is ready to build it. See OQ-1,
which records the controller's withdrawal of `spec/closure-residue` and the
future home of its unstubbed `ac-2` (reclamation).

## OQ-1

**Resolved** by controller adjudication under the owner's single-parent
directive (2026-07-19).

`spec/closure-residue` (`class: feature`, `status: draft`) was committed to
the sibling `verdi-wt/closure-hygiene` worktree (`18858ab`) — authored
independently by that stream, before this feature existed, to resolve the
identical R4-I-58-shaped blocker under a stricter lint-must-be-green
tolerance than `spec/instruction-conformance`'s own dispatch carried
(**R4-I-67**, which flagged both resolutions "for the controller to
reconcile if a single house convention is wanted going forward"). Its own
`ac-1` (detection) duplicated this feature's AC-2 almost verbatim, and
`spec/closure-hygiene`'s `links:` edge then resolved against
`spec/closure-residue#ac-1`, not this feature.

The owner directed exactly one small parent feature (DC-1). The controller
designated **this feature** that parent and **withdrew** `spec/closure-residue`:

- `spec/closure-hygiene`'s `implements` edge was re-pointed from
  `spec/closure-residue#ac-1` to `spec/assurance-integrity#ac-2`, and the
  `closure-residue/` spec directory was removed from that worktree.
- `spec/instruction-conformance`'s scaffold placeholder edge was re-pointed
  to `spec/assurance-integrity#ac-1`, clearing the standing R4-I-58 `VL-003`
  finding it had disclosed.

`spec/closure-residue`'s own `ac-2` (reclamation, deliberately left
unstubbed, blocked on the ratification question CO-2 and **R4-I-66** name)
has no counterpart in this feature, and needs none yet. Its unique
content — the candidate `verdi-store-layout` Garbage-collection amendment
language and the full conservative reclaim design — survives verbatim in
`spec/closure-hygiene`'s DC-5 and in ledger **R4-I-66**. Its future home is
a new feature, authored once — and only once — that ratification question
resolves (CO-2), never silently folded into this small, closeable feature
after acceptance (DC-1).

Recorded in ledger **R4-I-70**, which supersedes the R4-I-58-vs-R4-I-67
divergence this open question named.
