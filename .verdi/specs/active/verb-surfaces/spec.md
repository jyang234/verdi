---
id: spec/verb-surfaces
kind: spec
title: "jira:VERDI-P2-10"
owners: [platform-team]
class: story
status: draft
story: jira:VERDI-P2-10
problem: { text: "spec/creation-surfaces#ac-5 names the feature's last unbuilt gap (ledger L-N9, design doc §4 C-5): the owner-accepted guide's 8.4 documents a complete waive workflow — --expires, a reaffirmation flow, audit counting — over the waivers/ and reaffirmations/ kinds the model already carries (internal/artifact/waiver.go, internal/artifact/reaffirmation.go), but v0 left the verb itself unbuilt (dispatch.go's existing \"waivers\" entry is a DIFFERENT, still-out-of-scope audit verb 03 names; this story ships the new verb the guide names, verdi waive). Worse, model.DisplayVerb has sat wired and ready since round four, and the merged creation-form story already began routing a verb word through it (boardspecrender.go's p.words.verb(\"accept\") call), but TestVocabProseWitness has never scanned for verb words at all — only class and state words — so nothing has ever mechanically checked that a verb-speaking surface actually routes one. Every verb-speaking surface the four merged sibling creation-surface stories shipped — the init wizard's interview prompts, the creation form's own actions, the cli-creation interview, obligation author's output — landed with zero mechanical pressure on its verb prose, because the category has never been enforced.", anchor: problem }
outcome: { text: "verdi waive <story-ref> <ac-id> --rationale <text> [--expires YYYY-MM-DD] creates the waiver record at its existing convention path (waivers/<story-slug>/<ac-id>.md, the WaiverFrontmatter schema already carries unmodified), so the fold immediately reads that AC as waived through the existing, unmodified evidence.WaiverActive; --reaffirm extends an existing waiver in place with a fresh rationale and a mechanically-accumulating, dated reaffirmation log in its own body — a new committed record each time, exactly as the guide describes — and both the verb's own output and verdi audit's new waiver section surface a waiver's configured expiry and whether it has lapsed. verdi audit counts active (unexpired) waivers per story against a configured threshold using the same threshold-and-flag machinery evidence.SpecStale already established for accepted-deviations (internal/decisionsweep, the X-18 counterweight's home) — a parallel, clearly-disclosed count, never folded into the deviations count itself. TestVocabProseWitness's word list extends to every canonical verb id (today: accept, close), and every bare verb-word hit the extended witness finds — across this story's own waive.go and every verb-speaking surface the sibling stories already shipped — is routed through model.DisplayVerb or marked // vocab:identity, proven by a mutation witness exactly like the class/state word lists already carry, so the vocabulary category is born enforced rather than merely possible.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "verdi waive <story-ref> <ac-id> --rationale <text> [--expires YYYY-MM-DD] resolves the (story, AC) pair through the same classifyPair seam verdi attest and verdi obligation author already share, refuses (exit 2) on a missing --rationale or a malformed --expires value, and otherwise writes a create-only WaiverFrontmatter record (status: active, reason, expiry when given, owners copied verbatim from the resolved story spec, a frozen stamp) at waivers/<story-slug>/<ac-id>.md, self-validated by decoding the exact rendered bytes before the first write — never overwriting a waiver already present at that path (that refusal names --reaffirm as the extension path); a lifecycle test proves the AC folds waived immediately after (verdi matrix's own STATUS column, evidence.Fold unmodified) and that the verb's own stdout surfaces the configured expiry (or discloses none was given)", evidence: [behavioral], anchor: ac-1 }
  - { id: ac-2, text: "verdi waive <story-ref> <ac-id> --reaffirm --rationale <text> [--expires YYYY-MM-DD] refuses (exit 1, naming the plain create form) when no waiver yet exists at the convention path, and otherwise rewrites that SAME file in place: frontmatter reason/expiry/status(reset to active)/frozen are all refreshed to the fresh invocation, and the body's mechanically-owned reaffirmation log — delimited by a fixed marker so it is appended-to, never reparsed as prose — gains one new dated entry naming the fresh rationale and expiry, so a waiver reaffirmed more than once carries its full history legibly in one committed file; a lifecycle test proves a reaffirm round-trips (the file's frozen stamp and reason change, the prior log entry survives verbatim, a new one is appended) and that both the verb's own output and a lapsed prior expiry are disclosed when reaffirming after the recorded expiry has passed", evidence: [behavioral], anchor: ac-2 }
  - { id: ac-3, text: "verdi audit gains a waiver-audit section wired into decisionsweep.Audit (the X-18 counterweight's own counting site, beside its existing exemption and spec-stale sections): for every story spec with at least one waiver file, it lists each waiver's AC, status, and expiry, discloses whether an active-status waiver's recorded expiry has already lapsed by wall-clock at the audit invocation (never baked into a generated/frozen artifact — an ephemeral stdout read exactly like the existing closure-hygiene section's own git-state reads), excludes a lapsed waiver from the counted-active total, and flags a story whose active count exceeds a configured threshold (verdi.yaml audit.waivers_stale_threshold, decoded by internal/store.AuditConfig alongside the two existing thresholds, defaulting to 3 exactly as deviations_stale_threshold already does when absent or non-positive) — contributing to the same FLAGGED/exit-1 outcome the existing sections already produce, as its own clearly-labeled count, never merged into the accepted-deviations budget; a lifecycle test proves an active waiver under threshold passes clean, crossing the threshold flags by name, and an expired-status or lapsed-by-date waiver is excluded from the count while still disclosed in the listing", evidence: [behavioral], anchor: ac-3 }
  - { id: ac-4, text: "internal/specalign/vocabprose_test.go's TestVocabProseWitness word list extends to every canonical model verb id (mdl.Lifecycle[*].Transitions[*].Verb — today accept and close, derived from model.Canonical() exactly as the existing class/state derivation already is, never hand-maintained) alongside its existing class and state words; run at head after seeding, the witness is green — every unrouted, unmarked bare hit of a verb word the extended list newly catches across the whole cmd/ and internal/ production tree, including this story's own waive.go and every verb-speaking surface the merged sibling stories already shipped, is either routed through model.DisplayVerb (or an equivalent already-routed local per the witness's own ROUTED heuristic) or marked // vocab:identity at its producing site with a stated reason; a dedicated mutation-witness test — mirroring the existing class/state witness's own convention — proves the extended scanner is RED against a synthetic, deliberately-bare, unrouted verb word and GREEN once seeded, so the new category is proven to bite rather than merely present", evidence: [behavioral, static], anchor: ac-4 }
links:
  - { type: implements, ref: "spec/creation-surfaces#ac-5" }
---
# Verb Surfaces

## Problem

`spec/creation-surfaces#ac-5` names the feature's last unbuilt gap (ledger
L-N9, design doc §4 C-5): the owner-accepted guide's 8.4 documents a
complete waive workflow — `--expires`, a reaffirmation flow, audit
counting — over the `waivers/` and `reaffirmations/` kinds the model
already carries (`internal/artifact/waiver.go`,
`internal/artifact/reaffirmation.go`), but v0 left the verb itself unbuilt.
`dispatch.go`'s existing `"waivers"` entry is a *different*, still-out-of-
scope audit verb 03 §Attestations and waivers names (`verdi waivers`,
list/audit-shaped); this story ships the *new* verb the guide names,
`verdi waive` (create/reaffirm-shaped).

Worse, `model.DisplayVerb` has sat wired and ready since round four, and
the merged creation-form story already began routing a verb word through
it (`boardspecrender.go`'s `p.words.verb("accept")` call, spec/creation-form
ac-3), but `TestVocabProseWitness` has never scanned for verb words at
all — only class and state words — so nothing has ever mechanically
checked that a verb-speaking surface actually routes one. Every
verb-speaking surface the four merged sibling creation-surface stories
shipped — the init wizard's interview prompts, the creation form's own
actions, the cli-creation interview, obligation author's output — landed
with zero mechanical pressure on its verb prose, because the category has
never been enforced.

## Outcome

`verdi waive <story-ref> <ac-id> --rationale <text> [--expires
YYYY-MM-DD]` creates the waiver record at its existing convention path
(`waivers/<story-slug>/<ac-id>.md`, the `WaiverFrontmatter` schema already
carries unmodified), so the fold immediately reads that AC as `waived`
through the existing, unmodified `evidence.WaiverActive`. `--reaffirm`
extends an existing waiver in place with a fresh rationale and a
mechanically-accumulating, dated reaffirmation log in its own body — a new
committed record each time, exactly as the guide describes — and both the
verb's own output and `verdi audit`'s new waiver section surface a
waiver's configured expiry and whether it has lapsed.

`verdi audit` counts active (unexpired) waivers per story against a
configured threshold using the same threshold-and-flag machinery
`evidence.SpecStale` already established for accepted-deviations
(`internal/decisionsweep`, the X-18 counterweight's home) — a parallel,
clearly-disclosed count, never folded into the deviations count itself.

`TestVocabProseWitness`'s word list extends to every canonical verb id
(today: `accept`, `close`), and every bare verb-word hit the extended
witness finds — across this story's own `waive.go` and every
verb-speaking surface the sibling stories already shipped — is routed
through `model.DisplayVerb` or marked `// vocab:identity`, proven by a
mutation witness exactly like the class/state word lists already carry,
so the vocabulary category is born enforced rather than merely possible.

## Disclosed reading: the reaffirmation flow does not mint a new `reaffirmations/` file

`spec/creation-surfaces#ac-5`'s own frozen text says the waive workflow
lands "over the `waivers/` and `reaffirmations/` kinds the model already
carries." Read literally this could suggest `--reaffirm` should write a
*new* `reaffirmations/<story-slug>/<object-id>.md` file through the
existing `internal/artifact.KindReaffirmation` / `ReaffirmationFrontmatter`
schema. Concretely checked against that schema and its lint enforcement,
this is not available honestly:

- `ReaffirmationFrontmatter.Object` must be a **pinned, fragment-qualified
  ref** (`ref.Pinned() && ref.Fragment()`), and `internal/lint/vl003.go`'s
  fragment-resolution rule (`target.Spec == nil ⇒ red`) requires that ref's
  unpinned half to resolve to a **spec** whose declared objects (its own
  ACs/stubs) contain the fragment. A waiver artifact is not a spec and
  declares no such objects, so `object:` can only ever validly name
  `spec/<story>@<commit>#<ac-id>` — never the waiver itself.
- `HashPair.Validate` requires `Old != New`: the field's own documented
  meaning is the `(kind, id, text)` content hash of the *amended object*
  (the AC's own declared text) before and after the supersession that
  triggered the reaffirmation (03 §The amendment ladder rung 4, R4-I-4).
  A waive-driven reaffirmation does not change the AC's own declared
  text — nothing about the spec changed — so `Old` would equal `New`,
  which `Validate` rejects, and fabricating a distinct pair whose
  documented meaning does not match what actually happened would be
  dishonest content in a frozen artifact.

`docs/guide-claims.yaml`'s own `8.4-reaffirmations-kind` row already
independently confirms this reading: it is EXISTS, witnessed by
`TestDecodeReaffirmation_Happy` — the amendment-ladder test — decoupled
from whether `verdi waive` uses that kind at all. The manifest's own
authors (Task 6, `ritual-integrity`, merged before this story) already
scoped "the guide's 8.4 `reaffirmations/` kind claim" as satisfied by the
kind's mere existence, not by `verdi waive` minting instances of it.

Smallest-reversible choice taken: `verdi waive --reaffirm` never writes
under `reaffirmations/` — the existing `internal/artifact.KindReaffirmation`
mechanism stays exactly as `spec/ritual-integrity`'s R4-I-4 predecessor
built it, untouched by this story. "A reaffirmation flow" is realized
entirely within the `waivers/` kind: the SAME committed file is rewritten
with a fresh, committed `frozen` stamp and rationale each time (guide
8.4's "a new committed record with a fresh rationale," honestly read as a
fresh *commit* of the *same* record), and its body accumulates a dated,
append-only reaffirmation log (guide 8.4's "reaffirmations accumulate in
the record" — "the record" being the waiver's own). This is disclosed here
rather than resolved silently, per CLAUDE.md's provenance discipline; the
alignment judge is free to test this reading.

## Ac 1

`verdi waive <story-ref> <ac-id> --rationale <text> [--expires
YYYY-MM-DD]` resolves the `(story, AC)` pair through the same
`classifyPair` seam `verdi attest` and `verdi obligation author` already
share — never a second resolution implementation. It refuses, exit 2, on
a missing `--rationale` or a malformed `--expires` value (not
`YYYY-MM-DD`), and otherwise writes a create-only `WaiverFrontmatter`
record — `status: active`, the given reason, the given expiry when
present, `owners` copied verbatim from the resolved story spec exactly as
`attest.go` already does, and a frozen stamp (today's date, HEAD) — at
`waivers/<story-slug>/<ac-id>.md`, self-validated by decoding the exact
rendered bytes before the file is ever written (mirroring `attest.go`'s
own pre-write posture). It never overwrites a waiver already present at
that path; that refusal (exit 1) names `--reaffirm` as the extension path
rather than silently clobbering or silently no-op-ing.

A built-binary lifecycle test proves the AC folds `waived` immediately
after (`verdi matrix`'s own STATUS column, `evidence.Fold` unmodified —
the existing `evidence.WaiverActive` already reads the `status: active`
field this verb writes) and that the verb's own stdout surfaces the
configured expiry (or discloses plainly that none was given).

## Ac 2

`verdi waive <story-ref> <ac-id> --reaffirm --rationale <text> [--expires
YYYY-MM-DD]` refuses, exit 1, naming the plain create form, when no
waiver yet exists at the convention path for that pair. Otherwise it
rewrites that SAME file in place: frontmatter `reason`/`expiry`/`status`
(reset to `active` — a reaffirm un-lapses a waiver whose status had
drifted) and `frozen` are all refreshed to the fresh invocation, and the
body's mechanically-owned reaffirmation log — delimited by one fixed
marker line so it is appended to, never reparsed as free prose — gains
exactly one new dated entry naming the fresh rationale and expiry. A
waiver reaffirmed more than once therefore carries its full history
legibly in one committed file, never scattered across files whose
ordering would have to be reconstructed from git log.

A built-binary lifecycle test proves a reaffirm round-trips: the file's
`frozen` stamp and `reason` change, the PRIOR log entry survives verbatim
(byte-for-byte), and exactly one new entry is appended. A second test
proves that both the verb's own output and a lapsed prior expiry (wall-
clock past the previously-recorded `--expires` date, at invocation time)
are disclosed plainly when reaffirming after the recorded expiry has
already passed.

## Ac 3

`verdi audit` gains a waiver-audit section wired into
`decisionsweep.Audit` (the X-18 counterweight's own counting site, beside
its existing exemption and spec-stale sections — never a parallel,
disconnected counting path). For every story spec with at least one
waiver file on disk, it lists each waiver's AC, status, and expiry;
discloses whether an active-status waiver's recorded expiry has already
lapsed by wall-clock AT the audit invocation (never baked into a
generated/frozen artifact — an ephemeral stdout read at run time, exactly
like the existing closure-hygiene section's own live git-state reads);
excludes a lapsed waiver from the counted-active total (guide 8.4: "past
expiry the waiver lapses... reverts to pending"); and flags a story whose
active count exceeds a configured threshold
(`verdi.yaml` `audit.waivers_stale_threshold`, decoded by
`internal/store.AuditConfig` alongside the two existing thresholds,
defaulting to 3 exactly as `deviations_stale_threshold` already does when
absent or non-positive). A flagged story contributes to the same
`FLAGGED`/exit-1 outcome the existing sections already produce, as its
own clearly-labeled count — never merged into, or confused with, the
accepted-deviations budget, which counts a structurally different thing
(align-finding dispositions, not waiver artifacts).

A built-binary lifecycle test proves: an active waiver under threshold
passes clean; crossing the threshold flags the story by name; and an
expired-status or wall-clock-lapsed waiver is excluded from the active
count while still disclosed in the listing (never silently dropped from
the report).

## Ac 4

`internal/specalign/vocabprose_test.go`'s `TestVocabProseWitness` word
list extends to every canonical model verb id
(`mdl.Lifecycle[*].Transitions[*].Verb` — today `accept` and `close` —
derived from `model.Canonical()` exactly as the existing class/state
derivation already is, never hand-maintained) alongside its existing
class and state words.

Run at head after seeding, the witness is green: every unrouted, unmarked
bare hit of a verb word the extended list newly catches across the whole
`cmd/` and `internal/` production tree — including this story's own
`waive.go` and every verb-speaking surface the merged sibling stories
already shipped (the init wizard's prompts, the creation form's actions,
the cli-creation interview, obligation author's output) — is either
routed through `model.DisplayVerb` (or an equivalent already-routed local
per the witness's own ROUTED heuristic) or marked `// vocab:identity` at
its producing site with a stated reason.

A dedicated mutation-witness test — mirroring the existing class/state
witness's own convention (`TestScanVocabProse_Classifier`) — proves the
extended scanner is RED against a synthetic, deliberately-bare, unrouted
verb word and GREEN once seeded, so the new category is proven to bite
rather than merely present.
