---
id: spec/lifecycle-hygiene
kind: spec
title: "Lifecycle Hygiene"
owners: [platform-team]
class: feature
status: draft
problem: { text: "the lifecycle's remaining silent edges: an accepted spec with declared evidence kinds but no producing binding folds pending forever with no finding; seam conformance (disclosure vocabulary, route completeness, digest format) is enforced only by non-exhaustive judged sweeps; a test can couple to a spec's lifecycle position and break main invisibly on a spec-only merge; adjudicated follow-ups live only as prose in archived disposition notes; and the authoritative publish path has never once run", anchor: "#problem" }
outcome: { text: "the loop's silent edges fail loud or become legible: unbound declared evidence is a named finding with a scaffolding helper; seam conformance is machine-checked in make verify; live-store path literals in tests are a lint refusal; named follow-ups are enumerable in one view; and the CI-run authoritative close+publish path is exercised for real once", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "an accepted spec whose AC declares an evidence kind with no producing verdi.bindings.yaml entry is a named finding (which spec, which AC, which kind), never a silent forever-pending fold; and a scaffolding helper writes the correctly-shaped binding entry the way verdi attest scaffolds attestations", evidence: [static, behavioral], anchor: "#ac-1" }
  - { id: ac-2, text: "seam conformance is machine-checked under make verify: a disclosed-unproven-shaped string authored outside internal/disclosure's Render, a workbench route absent from the coverage inventory, and a digest-format copy diverging from the shared helper are each a named failure — detectors, not judged sweeps, own these classes", evidence: [static, behavioral], anchor: "#ac-2" }
  - { id: ac-3, text: "a test referencing the live store's specs/ tree by path literal is a named lint refusal — testdata/ snapshots are the only sanctioned fixture home, mechanically enforced, so a spec-only closure merge can never again break code invisibly", evidence: [static, behavioral], anchor: "#ac-3" }
  - { id: ac-4, text: "every named follow-up recorded in an accepted-deviation disposition is enumerable in one view — an audit section or workbench page listing the follow-up, its source finding, and the archived report it lives in — so adjudication promises stop evaporating into frozen prose", evidence: [behavioral, attestation], anchor: "#ac-4" }
  - { id: ac-5, text: "the authoritative closure path runs end-to-end for real once: a close executed inside CI (no --force-local), its rollup published and read back through the configured provider authoritatively, with the run's evidence attached — retiring the standing every-close-was-local disclosure", evidence: [behavioral, attestation], anchor: "#ac-5" }
open_questions:
  - { id: oq-1, text: "should spikes and superseded specs get a terminal exit from the active zone (disclosure-enumeration-spike sits accepted-pending-build forever; disclosure-seam lingers superseded) — a resolve/retire ritual, or is active-forever the intended record?", anchor: "#oq-1" }
  - { id: oq-2, text: "what makes a live agent's worktree visible to reclamation as claimed (R4-I-86: git-dead is not process-dead) — a declared claim file the gc predicate treats as kept, or a coordination convention outside verdi?", anchor: "#oq-2" }
  - { id: oq-3, text: "does the waivers verb enter scope now that v0 is closed out, or does the phase-0 stub stand — waiver artifacts exist and audit counts their lapses, but no verb manages them?", anchor: "#oq-3" }
stubs:
  - { slug: bind-gate, acceptance_criteria: [ac-1] }
  - { slug: seam-detectors, acceptance_criteria: [ac-2] }
  - { slug: vocabulary-residues, acceptance_criteria: [ac-2] }
  - { slug: store-path-lint, acceptance_criteria: [ac-3] }
  - { slug: followup-ledger, acceptance_criteria: [ac-4] }
  - { slug: publish-rehearsal, acceptance_criteria: [ac-5] }
  - { slug: active-zone-exits, spike: true, resolves: [oq-1] }
  - { slug: residue-claims, spike: true, resolves: [oq-2] }
---
# Lifecycle Hygiene

## Problem

The 2026-07-28/29 four-feature closure session finished the v0 lifecycle
loop and, in doing so, witnessed its remaining silent edges — each with a
live witness, recorded here so the scoping wall carries the evidence, not
just the claims:

- **Unbound evidence is silently pending forever.** disclosure-seam-v2 and
  disclosures-panel sat "built and merged" for weeks folding pending, and
  code-health stalled since mid-July, because no verdi.bindings.yaml entry
  existed and nothing flags that state. Four bindings PRs (#229, #230,
  #244, plus the scoping set) were hand-authored into a 1,700-line YAML.
  Obligations gate what evidence must show; attest-helper mis-slug-proofed
  attestations; bindings — the same silent-absent class — have neither.
- **Seam conformance is judge-enforced.** disclosure-legibility ac-1 took
  three fix loops (#232, #236, #237) because each non-exhaustive judged
  sweep surfaced one more hand-authored disclosure dialect; the
  boarddiagramrender.go residue is the ledgered leftover. The workbench
  route inventory and the sha256-prefix digest format have the same shape:
  a seam exists, no detector guards it.
- **Spec lifecycle × code coupling.** PR #243's spec-only closure merge
  broke main's make verify invisibly (spec-gate-only CI; closure moves
  directories); #245 fixed the two witnessed couplings and established
  the testdata/ snapshot discipline, but no rule prevents the next one.
- **Follow-ups evaporate.** The session minted ~ten named reversible
  follow-ups (workbench completeness detector, canonjson.DigestBytes,
  list_disclosures, the diagram-editor migration, a real exit=1 README
  block, min-block-count assertion…) now living only inside archived
  deviation-report disposition notes that nobody re-reads.
- **The authoritative publish path has never run.** Every close ever
  executed used --force-local against the fake tracker — disclosed each
  time, unproven always.

## Outcome

The loop's silent edges fail loud or become legible: unbound declared
evidence is a named finding with a scaffolding helper; seam conformance
is machine-checked in make verify; live-store path literals in tests are
a lint refusal; named follow-ups are enumerable in one view; and the
CI-run authoritative close+publish path is exercised for real once.

## AC-1

An accepted spec whose AC declares an evidence kind with no producing
verdi.bindings.yaml entry is a named finding — which spec, which AC,
which kind — never a silent forever-pending fold. A scaffolding helper
writes the correctly-shaped binding entry, the attest-helper pattern
applied to the lifecycle's last un-tooled hand-edit.

## AC-2

Seam conformance is machine-checked under make verify: a
disclosed-unproven-shaped string authored outside internal/disclosure's
Render, a workbench route absent from the coverage inventory, and a
digest-format copy diverging from the shared helper are each a named
failure. Detectors own these classes; judged sweeps return to finding
what detectors cannot.

## AC-3

A test referencing the live store's specs/ tree by path literal is a
named lint refusal — testdata/ snapshots are the only sanctioned fixture
home, mechanically enforced, so a spec-only closure merge can never
again break code invisibly.

## AC-4

Every named follow-up recorded in an accepted-deviation disposition is
enumerable in one view — an audit section or workbench page listing the
follow-up, its source finding, and the archived report it lives in.
Adjudication promises stop evaporating into frozen prose; this is the
disclosure-enumeration principle applied one level up, to the system's
own commitments.

## AC-5

The authoritative closure path runs end-to-end for real once: a close
executed inside CI (no --force-local), its rollup published and read
back through the configured provider authoritatively, with the run's
evidence attached — retiring the standing every-close-was-local
disclosure.

## OQ-1

Should spikes and superseded specs get a terminal exit from the active
zone? disclosure-enumeration-spike sits accepted-pending-build forever
and disclosure-seam lingers superseded; neither artifact class has an
exit today. A resolve/retire ritual — or is active-forever the intended
record? The active-zone-exits spike owns this.

## OQ-2

What makes a live agent's worktree visible to reclamation as claimed?
R4-I-86's lesson: git-dead is not process-dead — a mid-build agent's
clean worktree on a merged base is indistinguishable from residue, and
this session's own scratch worktrees read reclaim-eligible while in use.
A declared claim file the gc predicate treats as kept, or a coordination
convention outside verdi? The residue-claims spike owns this.

## OQ-3

Does the waivers verb enter scope now that v0 is closed out? Waiver
artifacts exist and audit counts their lapses, but the verb is the
phase-0 stub. In, out, or deferred again — an owner scoping call, parked
here rather than silently assumed.

## Scoping notes

Drafted 2026-07-29 from the closure session's witnessed gap list (owner
directive: "make sure the remaining gaps are documented as
scoping-canvas stickies"). The declared stubs above are the durable,
graduated form of that parking — stickies proper are scratch-tier
mutable-zone annotations, so the wall renders these as first-class
scoping cards with coverage chips instead. Every AC is currently covered
by at least one stub; vocabulary-residues carries the ledgered
diagram-editor migration and canonjson.DigestBytes sweep under ac-2's
detector outcome so the fixes and their guards land together. This spec
is deliberately draft: reshape it on the wall.
