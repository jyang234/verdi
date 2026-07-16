---
id: spec/closure-ergonomics
kind: spec
title: "Closure Ergonomics"
owners: [platform-team]
class: feature
status: accepted-pending-build
problem: { text: "closing a story is the most manual, error-prone stretch of the lifecycle, and every unit of that weight is accidental, not essential. The operator hand-edits `verdi.bindings.yaml`, hand-authors attestation markdown at exact slugged paths where one wrong slug silently folds as `absent` (D6-16/D6-18), exports CI-only env vars to make `verdi sync` resolve the forge repo in a local checkout (D6-14), fights sync's HEAD-exact bundle demand though the fold's own rule is ancestor-based (D6-32), and records review dispositions by hand-editing deviation reports because no verb exists (D6-25). Round 6 closed seventeen specs this way; real usage reports the loop 'really heavy and feels unnecessary.' The closure GATE is sound — the round proved it load-bearing — but a failed close is routinely the FIRST disclosure of what was missing, and the artifacts the gate demands are the least tool-assisted in the system.", anchor: "#problem" }
outcome: { text: "an operator takes a built story from merged to closed through guided, honest tooling: a non-mutating preflight disclosing every unmet closure condition with the exact artifact and path needed, so close's refusal is never the first disclosure; an attestation helper that scaffolds the correctly-slugged, correctly-placed skeleton while the human authors every word of the claim; a disposition verb so recording a reviewer's decision is a command, not a hand-edit of a report; and a `verdi sync` that works in a plain local checkout — forge repo derived from the git origin, bundle resolution honoring the fold's own ancestor rule. Closure-gate semantics are byte-for-byte unchanged: no condition weakened, no new pass path — the weight removed is only the accidental kind.", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a non-mutating closure preflight reports, for a named story, every condition `verdi close` would refuse on — unbound ACs, missing or mis-slugged attestations, absent or stale evidence records, unresolved flags — each with the exact artifact required and the exact path where the fold will look for it, exiting with the verdict discipline (0 ready / 1 unmet / 2 operational) and mutating nothing", evidence: [behavioral, attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "attestation authoring is scaffolded and mis-slug-proofed: a helper verb, given a (story, AC), writes a valid attestation skeleton at the correct slugged path with correct frontmatter and `verifies` edge, leaving the claim body for the operator to author (a helper never fabricates a human record); and an attestation whose path or slug does not resolve to the (story, AC) it claims is a lint refusal — never a silent `absent` at fold time", evidence: [behavioral, attestation], anchor: "#ac-2" }
  - { id: ac-3, text: "a reviewer's disposition on a deviation-report finding is recorded by a verb, not a hand-edit: the disposition names the finding, the decision, and the rationale; it is preserved verbatim across `align --freeze`; and the previously de-facto flow of editing report files by hand is retired from the documented lifecycle", evidence: [behavioral, attestation], anchor: "#ac-3" }
  - { id: ac-4, text: "`verdi sync` works in a plain local checkout with no CI-only environment: the forge owner/repo is derived from the git `origin` remote when the explicit env is absent (CI env, when present, still wins), and bundle resolution accepts the nearest-ancestor bundle by the same ancestor rule the fold already applies — never demanding a HEAD-exact bundle the fold itself would not require", evidence: [behavioral, attestation], anchor: "#ac-4" }
decisions:
  - { id: dc-1, text: "closure-gate semantics are UNCHANGED — this feature removes accidental toil only. Every closure condition (all-ACs-evidenced over authoritative records, no unresolved spec-stale, no pending-supersession, and for features the five-condition AND) stands byte-for-byte; no verb here introduces a new way to pass, waive, or skip a condition. The round-6 verdict was that the gate is the load-bearing control and the heaviness around it is accidental (D6-14/16/18/25/32) — the answer to 'the loop feels unnecessary' is better tooling, never a laxer gate", anchor: "#dc-1" }
  - { id: dc-2, text: "helpers scaffold, never fabricate: verdi writes structure — paths, slugs, frontmatter, edges — and the human writes every word of the claim. An attestation body is never generated, defaulted, or templated with claim-shaped prose; the scaffold is not foldable until the operator has authored the claim. This carries the three-valued-honesty discipline (a machine must not manufacture a human oracle's record) into the tooling that makes human records cheap to author", anchor: "#dc-2" }
  - { id: dc-3, text: "sync's local flow adds NO new resolution semantics: origin-derivation is a fallback ordered strictly after the explicit CI env (existing CI behavior is untouched), and the ancestor rule for bundle acceptance is the fold's existing rule applied verbatim at fetch time — closing the D6-32 asymmetry where sync demanded more than the fold it feeds. Where the two surfaces would disagree, the fold's rule is authoritative and sync conforms to it", anchor: "#dc-3" }
  - { id: dc-4, text: "dispositions are recorded IN PLACE in the living deviation report's disposition layer — the layer deliberately outside the integrity digest. Freeze preserves the adjudicated report verbatim (align.FreezeInPlace, ratified via PR #99): a fully-dispositioned living report covering the freeze commit is stamped byte-for-byte, never re-judged. The remaining destruction vector — an ordinary align re-run regenerating over a genuine judged exchange — is closed by the D6-24 keep-best fix sequenced before any build of this feature. A sidecar artifact was rejected: it would add a second artifact kind and fight the freeze-in-place architecture. Settles oq-1 (ADJ-22)", anchor: "#dc-4" }
  - { id: dc-5, text: "the preflight is a mode of `verdi close` — one verb owns the ritual and its rehearsal, so the preflight report and close's refusal are one code path and their required agreement (co-3) is structural. `verdi gate` stays commit-scoped. No new verb is added for ac-1; the story contract settles the exact flag spelling. Settles oq-2 (ADJ-23)", anchor: "#dc-5" }
constraints:
  - { id: co-1, text: "no network in any test: forge interactions (origin-derivation, bundle fetch, ancestor resolution) against hermetic fakes (httptest, fixturegit with stable SHAs); attest/disposition/preflight exercised entirely on fixture stores", anchor: "#co-1" }
  - { id: co-2, text: "every verb keeps the exit discipline — 0 clean, 1 verdict, 2 operational. Preflight's unmet conditions are a VERDICT (exit 1), not an error; only genuinely operational failures (unreadable store, unreachable fake forge) exit 2. Preflight and the attest scaffold mutate nothing beyond the files they exist to write", anchor: "#co-2" }
  - { id: co-3, text: "the operative property: after this feature, a failed `verdi close` is never the FIRST disclosure of a missing artifact. Everything close would refuse on is disclosed by the preflight, with the exact path and slug, before close is attempted. The feature satisfies this or it is not done", anchor: "#co-3" }
stubs:
  - { slug: close-preflight, acceptance_criteria: [ac-1] }
  - { slug: attest-helper, acceptance_criteria: [ac-2] }
  - { slug: disposition-verb, acceptance_criteria: [ac-3] }
  - { slug: sync-local-flow, acceptance_criteria: [ac-4] }
frozen: { at: 2026-07-16, commit: 865c51ac0163198d98c4690caaff0b85dd5e0cf1 }
---
# Closure Ergonomics

## Problem

Closing a story is the most manual, error-prone stretch of the lifecycle, and
the round-6 dogfood plus real usage agree on the diagnosis: every unit of that
weight is **accidental**, not essential.

The operator hand-edits `verdi.bindings.yaml`. They hand-author attestation
markdown at exact slugged paths, where one wrong slug silently folds as
`absent` — the fold cannot distinguish "evidence missing" from "evidence
misfiled" (D6-16, D6-18). They export CI-only environment variables so that
`verdi sync` can resolve the forge repository in a local checkout at all
(D6-14). They fight sync's demand for a HEAD-exact bundle even though the fold
it feeds applies an ancestor rule (D6-32; worked around in round 6 by cutting
closure branches from the verified ancestor, ADJ-19). And they record review
dispositions by hand-editing deviation reports, because no verb exists (D6-25).

Round 6 closed seventeen specs this way — proving the gate works and the toil
is real. Real usage reports the loop "really heavy and feels unnecessary." The
closure **gate** is sound; what is broken is that a failed close is routinely
the *first* disclosure of what was missing, and the artifacts the gate demands
are the least tool-assisted in the system.

## Outcome

An operator takes a built story from merged to closed through guided, honest
tooling. A non-mutating **preflight** discloses every unmet closure condition
with the exact artifact and path needed, so close's refusal is never the first
disclosure. An **attestation helper** scaffolds the correctly-slugged,
correctly-placed skeleton while the human authors every word of the claim. A
**disposition verb** makes recording a reviewer's decision a command, not a
hand-edit of a report. And **`verdi sync` works in a plain local checkout** —
forge repo derived from the git origin, bundle resolution honoring the fold's
own ancestor rule.

Closure-gate semantics are byte-for-byte unchanged: no condition weakened, no
new pass path. The weight removed is only the accidental kind.

## AC-1

A non-mutating closure preflight reports, for a named story, every condition
`verdi close` would refuse on — unbound ACs, missing or mis-slugged
attestations, absent or stale evidence records, unresolved flags — each with
the exact artifact required and the exact path where the fold will look for
it. It follows the verdict discipline (0 ready / 1 unmet / 2 operational) and
mutates nothing. This is co-3 made checkable: the preflight's report and
close's refusal must agree, and the preflight comes first. Evidence:
behavioral (a fixture store with each defect class produces the matching
disclosure; a ready store exits 0 and close then succeeds) + attestation.

## AC-2

Attestation authoring is scaffolded and mis-slug-proofed. A helper verb, given
a (story, AC), writes a valid attestation skeleton at the correct slugged path
with correct frontmatter and `verifies` edge — and leaves the claim body for
the operator, because a helper never fabricates a human record (dc-2). The
enforcement half: an attestation whose path or slug does not resolve to the
(story, AC) it claims is a **lint refusal**, never a silent `absent` at fold
time — the D6-18 failure class becomes a named violation instead of a
mystery. Evidence: behavioral (scaffold round-trips decode/validate at the
path the fold reads; the mis-slug lint refuses a misplaced fixture) +
attestation.

## AC-3

A reviewer's disposition on a deviation-report finding is recorded by a verb,
not a hand-edit. The disposition names the finding, the decision, and the
rationale; it is preserved verbatim across `align --freeze`; and the
previously de-facto flow of editing report files by hand (D6-25 — every
round-6 disposition was recorded this way) is retired from the documented
lifecycle. The storage question is settled by dc-4: dispositions live in
place, in the living report's disposition layer.
Evidence: behavioral (disposition recorded by verb, then surviving a freeze,
proven on fixtures) + attestation.

## AC-4

`verdi sync` works in a plain local checkout with no CI-only environment.
The forge owner/repo is derived from the git `origin` remote when the explicit
env is absent — CI env, when present, still wins (dc-3) — closing D6-14, where
the local developer flow the whole model assumes 404'd until CI variables were
exported by hand. And bundle resolution accepts the nearest-ancestor bundle by
the same ancestor rule the fold already applies, never demanding a HEAD-exact
bundle the fold itself would not require (D6-32). Evidence: behavioral
(origin-derivation and ancestor-walk against hermetic fakes and fixturegit) +
attestation.

## DC-1

Closure-gate semantics are **unchanged** — this feature removes accidental
toil only. Every closure condition (all ACs evidenced over authoritative
records, no unresolved spec-stale, no pending-supersession, and for features
the five-condition AND) stands byte-for-byte; no verb here introduces a new
way to pass, waive, or skip a condition. The round-6 verdict was that the gate
is the load-bearing control and the heaviness around it is accidental. The
answer to "the loop feels unnecessary" is better tooling, never a laxer gate.

## DC-2

Helpers **scaffold, never fabricate**: verdi writes structure — paths, slugs,
frontmatter, edges — and the human writes every word of the claim. An
attestation body is never generated, defaulted, or templated with claim-shaped
prose, and the scaffold is not foldable until the operator has authored the
claim. This carries the three-valued-honesty discipline — a machine must not
manufacture a human oracle's record — into the tooling that makes human
records cheap to author. The helper's value is that the *mechanical* parts
(where the file goes, what its slug is, which edge it carries) stop being the
operator's problem; the *epistemic* part remains entirely theirs.

## DC-3

Sync's local flow adds **no new resolution semantics**. Origin-derivation is a
fallback ordered strictly after the explicit CI env, so existing CI behavior
is untouched. The ancestor rule for bundle acceptance is the fold's existing
rule applied verbatim at fetch time — closing the D6-32 asymmetry where sync
demanded more (HEAD-exact) than the fold it feeds. Where the two surfaces
would disagree, the fold's rule is authoritative and sync conforms to it.

## DC-4

Dispositions are recorded **in place** in the living deviation report's
disposition layer — the layer deliberately outside the integrity digest.
Freeze preserves the adjudicated report verbatim (`align.FreezeInPlace`,
ratified via PR #99): a fully-dispositioned living report covering the freeze
commit is stamped byte-for-byte, never re-judged. The remaining destruction
vector — an ordinary align re-run regenerating over a genuine judged exchange
— is closed by the D6-24 keep-best fix, sequenced before any build of this
feature. A sidecar artifact was rejected: it would add a second artifact kind
and fight the freeze-in-place architecture. This settles oq-1 (ADJ-22).

## DC-5

The preflight is a mode of `verdi close` — one verb owns the ritual and its
rehearsal, so the preflight report and close's refusal are one code path and
their required agreement (co-3) is structural. `verdi gate` stays
commit-scoped. No new verb is added for ac-1; the story contract settles the
exact flag spelling. This settles oq-2 (ADJ-23).

## CO-1

No network in any test. Forge interactions (origin-derivation, bundle fetch,
ancestor resolution) run against hermetic fakes — `httptest`, `fixturegit`
with stable SHAs; attest, disposition, and preflight are exercised entirely on
fixture stores.

## CO-2

Every verb keeps the exit discipline — 0 clean, 1 verdict, 2 operational.
Preflight's unmet conditions are a **verdict** (exit 1), not an error; only
genuinely operational failures (unreadable store, unreachable fake forge) exit
2. Preflight and the attest scaffold mutate nothing beyond the files they
exist to write.

## CO-3

The operative property: after this feature, a failed `verdi close` is never
the **first** disclosure of a missing artifact. Everything close would refuse
on is disclosed by the preflight, with the exact path and slug, before close
is attempted. The feature satisfies this or it is not done.
