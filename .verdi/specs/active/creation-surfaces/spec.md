---
id: spec/creation-surfaces
kind: spec
title: "Creation Surfaces"
owners: [platform-team]
class: feature
status: accepted-pending-build
problem: { text: "Creation is the store's least-served ritual: there is no verdi init at all, so a team starting fresh must hand-assemble a store by hand though the owner-accepted guide's Part II already describes a configuring wizard and R4-I-56 already named the narrower scaffold-wrapper baseline it should default to; story creation from a stub exists only on the board's stub-instantiate action, with no CLI equivalent, and every board- or CLI-created spec still carries generic TODO placeholders instead of fields generated from its own class template (the ADJ-65 asymmetry); the evidence-obligation renderer already exists in the workbench but is unsurfaced at accept, so a story can freeze with declared evidence kinds and zero obligations stating what that evidence must specifically show, exactly the gap X-9 named; the guide's own 8.4 documents a waive workflow — --expires, reaffirmation, audit counting — over waivers/ and reaffirmations/ kinds the model already carries, but v0 left the verb itself unbuilt; and DisplayVerb is wired and ready while TestVocabProseWitness's word list has never grown to cover verb words, because no surface has ever spoken one in production prose — the vocabulary category exists unconsumed.", anchor: problem }
outcome: { text: "A team reaches a working, tailored store through verdi init, either the bare scaffold wrapper or the --wizard interview, whose complete candidate store is validated by the full model-check core over a staged sibling-temp directory before a single rename ever promotes it into place, so an interview can never leave a half-configured store behind; specs are created from board or CLI alike, both generating their fields from the same class-template placeholders through the one shared designscaffold producer commit-to-design now uses too, and --from-stub reaches the CLI through the identical shared stub-instantiate seam the board's own action calls, so the two surfaces can never drift apart; every story accepted from this point on is born with its declared evidence kinds' obligations already in hand, because accept's freeze-moment backstop scaffolds exactly what is missing before its own lint gate ever runs, and verdi obligation author gives the design branch a pre-freeze surface for authoring or regenerating them through that identical renderer; verdi waive lands per the guide's own letter, and every verb-speaking surface this feature creates routes through DisplayVerb with the prose witness extended in the same story, so the vocabulary category is born enforced rather than merely possible; and every one of these surfaces is honestly bounded by the v1 frontier, claiming nothing about declared verbs, a flat hierarchy, per-transition enforcement, or live-store migration, which stay explicit, disclosed roadmap.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "verdi init offers both paths at once: a bare, non-interactive scaffold wrapper (R4-I-56's canonical baseline) and a --wizard interview (guide Part II) that walks vocabulary renames, template-set selection, and display conventions with live validation preview; the wizard writes nothing to the real store while interviewing — it assembles the complete candidate store in a same-filesystem sibling temp directory and gates promotion on the full model-check core run over that staged root rather than a decode-only check, promoting by exactly one rename only once the staged model.yaml both passes that check and decode-compares equal to the interview's own intent (W-1/W-2/W-4); both paths refuse on any existing .verdi/ directory at all, not merely an existing manifest, because the rename itself requires .verdi to be absent (W-3/W-3b), exiting 2 and naming what already exists; a mid-interview abort, or a simulated crash mid-write, leaves nothing at the real root, since the temp directory is discarded on any pre-rename error and no real-store write ever happens before that single rename", evidence: [behavioral, attestation], anchor: ac-1 }
  - { id: ac-2, text: "the board's creation form generates its fields directly from the target class template's own placeholders (guide 5.3's D-1 contract), rendering the submitted spec through the same shared designscaffold producer every other creation surface now shares — inheriting CheckClass's post-render validation, so a form-submitted spec can never round-trip as the wrong class; commit-to-design is switched to the identical producer, so a store's own template overrides are honored there for the first time too, discharging L-M12's third-producer divergence; a spec created from a vocabulary-renamed store's board renders its form labels in that store's own display words and lands TODO-free wherever a field was actually filled in", evidence: [behavioral, attestation], anchor: ac-2 }
  - { id: ac-3, text: "design start gains a --problem and --outcome pair of flags that produce a TODO-free scaffold section-for-section, a --defer-statements flag that commits deliberate TODOs together with an explicit disclosure line, and a TTY interview — driven from the same placeholder-enumeration descriptors the board's creation form already reuses — when no creation flags are given at all; --from-stub creates a story from a feature's declared stub exactly as the board's own stub-instantiate action already does, because both now call one shared stub-instantiate core extracted out of the workbench rather than reimplementing it a second time, an extraction proven behavior-preserving by the board's own existing tests passing unmodified", evidence: [behavioral, attestation], anchor: ac-3 }
  - { id: ac-4, text: "accept cannot complete while any of a story's declared (ac, kind) pairs is missing its obligation: the freeze-moment backstop scaffolds exactly the missing pairs to disk before the in-ritual lint gate ever runs (O-1), stamps every scaffolded obligation preFlipHead — identical to the spec's own flip stamp (O-4) — stages the newly-scaffolded paths into the accept commit itself so the pairing can never be replayed away (O-2), skips any pair an already-decodable obligation already covers rather than clobbering board- or CLI-authored content (O-3/O-3b), and unlinks exactly the obligations it newly created, and none it merely skipped, on any refusal or error after scaffolding, so an unrelated gate refusal leaves a pristine tree instead of orphaned stubs that would themselves block a retry (O-1b); verdi obligation author is the separate design-branch, pre-freeze authoring and regeneration surface, sharing the identical renderer seam the backstop itself calls (O-5) so the two paths can never diverge, and refusing outright on any obligation a merge to main has already frozen. None of this reverses evidence-obligations co-2's draft-tolerance: the design wave's re-attack proved that closure unimplementable (VL-006 refuses a zero-kind AC unconditionally, drafts included) and withdrew it, so VL-020's existing draft-tolerance stands exactly as ratified — the gap this AC closes was always the freeze moment itself, by construction, never authoring-time tolerance", evidence: [behavioral, attestation], anchor: ac-4 }
  - { id: ac-5, text: "verdi waive lands per the guide's own 8.4 specification — --expires, a reaffirmation flow, and audit counting — over the waivers/ and reaffirmations/ kinds the model already carries; every verb-speaking surface this feature creates, the wizard's prompts, the creation form's actions, and waive's own output among them, routes its verb words through DisplayVerb rather than hand-writing bare verb prose, and TestVocabProseWitness's word list extends to cover verb words in this same story, so a bare, unrouted verb word introduced anywhere this feature touches reds the witness by construction rather than passing unnoticed", evidence: [behavioral, static, attestation], anchor: ac-5 }
stubs:
  - { slug: init-wizard, acceptance_criteria: [ac-1] }
  - { slug: creation-form, acceptance_criteria: [ac-2] }
  - { slug: cli-creation, acceptance_criteria: [ac-3] }
  - { slug: obligation-seam, acceptance_criteria: [ac-4] }
  - { slug: verb-surfaces, acceptance_criteria: [ac-5] }
frozen: { at: 2026-07-21, commit: 5a59a33555b9d09bfd6179d9f12ae6f14ca70873 }
---
# Creation Surfaces

## Problem

Ritual integrity fixed how the process repeats; creation surfaces is what
the process is actually *for*, and today it is the store's least-served
ritual — every path into a working, tailored store is either missing,
board-only, or honestly incomplete.

**No init.** There is no `verdi init` at all. A team starting fresh must
hand-assemble a store by hand: no scaffold wrapper (R4-I-56 already named
this narrow, non-interactive baseline as owed) and no configuring wizard,
though the owner-accepted guide's Part II already describes one in detail
— vocabulary renames, template-set selection, display conventions, all
with live validation. The gap between what the guide promises and what
`verdi` can do is total.

**Board-only stub-instantiate; TODO-placeholder scaffolds.** Creating a
story from a feature's declared stub exists only as a board action
(`internal/workbench`'s stub-instantiate); the CLI has no equivalent at
all, an asymmetry ADJ-65 already harvested seven frictions from. Worse,
every board- or CLI-created spec — however it is created — still carries
generic TODO placeholders instead of fields generated from its own class
template, because no creation surface reads the template's placeholders
as a field contract at all.

**Obligation authoring unsurfaced at accept (X-9).** The evidence-
obligation renderer already exists (`internal/workbench/
obligationauthor.go`'s `actionObligationGraduate`), but only the board
can reach it, and nothing at `accept` time enforces that a story's
declared evidence kinds actually have obligations behind them. A story
can freeze — cross into `main`, permanently — with `evidence: [behavioral]`
declared and no obligation anywhere stating what that behavioral proof
must specifically show. X-9 named this gap directly: the mechanism to
close it exists; nothing wires it to the one moment that matters.

**No waive verb; verb vocabulary unconsumed.** The guide's 8.4 documents
a complete waive workflow — `--expires`, a reaffirmation flow, audit
counting — and the `waivers/`/`reaffirmations/` kinds already exist in
the model. v0 built everything except the verb itself. And `DisplayVerb`
sits wired and ready in the model, but no production surface has ever
spoken a true verb word, so `TestVocabProseWitness`'s word list has never
grown to cover them — the vocabulary category exists in the machinery
without a single enforced instance.

## Outcome

A team reaches a working, tailored store through `verdi init` — the bare
scaffold wrapper or the `--wizard` interview — whose complete candidate
store is validated by the full model-check core over a staged
sibling-temp directory before a single rename ever promotes it into
place, so an interview can never leave a half-configured store behind and
a refusal, wherever it happens, leaves nothing at the real root.

Specs are created from board or CLI alike, and the two can no longer
drift: both generate their fields from the same class-template
placeholders through the one shared `designscaffold` producer that
`commit-to-design` now uses too, and `--from-stub` reaches the CLI
through the identical shared stub-instantiate seam the board's own action
calls.

Every story accepted from this point on is born with its declared
evidence kinds' obligations already in hand: accept's freeze-moment
backstop scaffolds exactly what is missing before its own lint gate ever
runs, and `verdi obligation author` gives the design branch a
pre-freeze surface for authoring or regenerating them through that same
renderer — two paths, one seam, never two implementations to diverge.

`verdi waive` lands per the guide's own letter, and every verb-speaking
surface this feature creates routes through `DisplayVerb`, with the
prose witness extended to verb words in the same story that first
exercises them — the vocabulary category is born enforced, not merely
possible.

Every one of these surfaces is honestly bounded by the v1 frontier:
nothing here claims declared verbs, a flat hierarchy, per-transition
obligation enforcement, or live-store migration — those stay explicit,
disclosed roadmap, and a wizard asked for any of them says so rather than
degrading silently.

## Ac 1

`verdi init` offers both paths the guide promises, at once. The bare form
is non-interactive — R4-I-56's canonical baseline, the store skeleton and
nothing more. `--wizard` opts explicitly into a guided interview (guide
Part II): vocabulary renames, template-set selection, and display
conventions, each previewed with live validation, refusing a structural
request outside the v1 frontier by explaining the frontier rather than
pretending to honor it. The wizard writes nothing to the real store while
interviewing at all: it assembles the complete candidate store — every
file a finished store would have, `model.yaml` included wherever the
interview diverges from canonical — in a same-filesystem sibling temp
directory, and gates promotion on the *full* model-check core
(`runModelCheck`) run over that staged root, never a decode-only check
that would leave a wizard-written template override unvalidated (W-1).
Promotion is exactly one `os.Rename` from the staged temp directory onto
`.verdi`, and it fires only once the staged `model.yaml` both passes that
check and decode-compares equal to the interview's own in-memory intent
(W-2/W-4) — a sibling temp directory, never `os.TempDir`, so the rename
can never cross a filesystem boundary. Both paths refuse on *any* existing
`.verdi/` directory, not merely an existing manifest, because the rename
itself requires `.verdi` to be absent or it fails with `ENOTEMPTY` (W-3
create-only posture, unified by W-3b's predicate) — the refusal exits 2,
naming exactly what already exists, and points at hand-editing
`.verdi/model.yaml` (validated by `verdi model check`) as the
reconfigure-an-existing-store path, since that is explicitly out of v1
scope. A mid-interview abort, or a simulated crash partway through
writing the staged store, leaves nothing whatsoever at the real root: no
real-store write ever happens before the single rename, and the temp
directory is discarded on any pre-rename error.

## Ac 2

The board's creation form generates its fields directly from the target
class template's own placeholders — guide 5.3's D-1 contract made real —
rather than presenting a generic, one-size form. The submitted spec
renders through the same shared `designscaffold` producer every other
creation surface in this feature now shares, inheriting `CheckClass`'s
post-render validation so a form-submitted spec can never round-trip
decoded as the wrong class from the one it was created as. Enumerating a
store's own *override* template (not just the embedded canonical one)
yields that override's own fields — the property that makes vocabulary
renames and custom template sets actually reach the form a user fills
in. `commit-to-design` is switched to the identical producer as part of
this same story, so a store's own template overrides are honored on that
path for the first time too, discharging L-M12's long-standing
third-producer divergence; the switch is required to be byte-stable for
every input the old producer already handled, a parity pin, not merely a
behavior change. A story created from the board of a vocabulary-renamed
store renders its form labels in that store's own display words and
lands on the correct branch, TODO-free wherever a field was actually
filled in.

## Ac 3

`design start` gains creation ergonomics on the CLI to match the board.
A `--problem` and `--outcome` pair of flags produces a TODO-free scaffold
section-for-section; a `--defer-statements` flag commits deliberate TODOs
anyway, but only together with an explicit disclosure line naming them as
deferred, never silently. Given no creation flags at all on a TTY, an
interview prompts from the *same* placeholder-enumeration descriptors
`ac-2`'s board form already reuses — one field contract, two front ends,
never a second one hand-rolled to drift from the first. `--from-stub
<feature> <stub>` creates a story from a feature's declared stub exactly
as the board's own stub-instantiate action already does, because both
now call one shared stub-instantiate core extracted out of
`internal/workbench` rather than reimplementing it a second time on the
CLI side — closing the ADJ-65 asymmetry at the mechanism, not just at the
surface. The extraction is required to be behavior-preserving for the
board's existing path: its own tests must keep passing unmodified, the
proof that nothing about board creation changed underneath this
refactor. `--owners` deliberately stays out of scope, the same posture
I-10/X-4 already ratified, disclosed rather than silently reconsidered.

## Ac 4

Accept cannot complete while any of a story's declared (ac, kind) pairs
is missing its obligation. The freeze-moment backstop scaffolds exactly
the missing pairs to disk *before* the in-ritual lint gate ever runs
(O-1) — ordering that matters on its own terms, independent of any other
rule's status, because a scaffolded stub must already exist on disk by
the time anything downstream inspects the working tree. Every scaffolded
obligation is stamped `preFlipHead`, identical to the spec's own flip
stamp (O-4) — never the not-yet-created accept commit, so there is no
chicken-and-egg. The newly-scaffolded paths join accept's own scoped
`addPaths` set and land inside the accept commit itself (O-2), the
mechanism that makes "the gap cannot be replayed away" literally true
rather than aspirational. The backstop skips, never overwrites: only
*missing* pairs are scaffolded, keyed on the same decode-based coverage
predicate the existing obligation gate already uses, never a bare
`os.Stat` (O-3/O-3b), so board- or CLI-authored obligations are never
clobbered. Exactly the obligations newly created this invocation — never
one merely skipped as already-covered — are unlinked on any refusal or
error after scaffolding (O-1b), so an unrelated quartet-gate refusal
leaves a pristine tree rather than orphaned stubs that would themselves
block a retry through the very authoring surface they defer to.
`verdi obligation author` is that authoring surface: a separate,
design-branch, pre-freeze mechanism for authoring or regenerating an
obligation ahead of accept, sharing the identical renderer seam the
backstop itself calls (O-5) — one shared seam, never a second
reimplementation in `cmd/verdi` — and refusing outright on any obligation
a merge to `main` has already frozen, since a frozen obligation is not
"refined," it is superseded through the normal ladder like any other
frozen artifact.

None of this reverses evidence-obligations co-2's draft-tolerance. The
design wave's re-attack on that proposed reversal (O-7's original
framing) proved it unimplementable: `VL-006` unconditionally refuses an
acceptance criterion declaring zero evidence kinds, drafts included, so
"no kinds by default on a fresh scaffold" cannot be encoded without
simply relocating the born-red from one lint rule to another the backstop
cannot cure (it writes obligation files, never spec AC lines). That
reversal was withdrawn at adjudication; `VL-020`'s existing
draft-tolerance stands exactly as already ratified. The gap this AC
closes was always the freeze moment itself, by construction — never
authoring-time tolerance, which this feature leaves untouched.

## Ac 5

`verdi waive` lands per the guide's own 8.4 specification: `--expires`,
a reaffirmation flow, and audit counting, over the `waivers/` and
`reaffirmations/` kinds the model already carries — v0's one deliberately
unbuilt piece of an otherwise-complete mechanism. Every verb-speaking
surface this feature creates — the wizard's prompts, the creation form's
actions, `waive`'s own output among them — routes its verb words through
`DisplayVerb` rather than hand-writing bare verb prose, the same
class/state display discipline every other production surface already
owes the model's vocabulary. `TestVocabProseWitness`'s word list extends
to cover verb words in this same story, the same mutation-witness
discipline the class and state word lists already use: a deliberately
bare, unrouted verb word introduced anywhere this feature touches reds
the witness by construction, so the category is born enforced rather than
merely possible the way an unexercised vocabulary axis would otherwise
stay.
