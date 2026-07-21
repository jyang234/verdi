---
id: spec/guide-claims-gate
kind: spec
title: "Guide Claims Gate"
owners: [platform-team]
class: story
status: closed
story: jira:VERDI-P2-5
problem: { text: "the owner-accepted Integration & Startup Guide makes capability claims (Appendix B) that nothing in make verify ties to the corpus, so a claim can go stale silently — a witness renamed, gutted, or simply never run again leaves the guide's claim exactly as confident as the day it was written; a name-existence-only check would repeat the ADJ-50 lying-gate class (a witness merely existing proves nothing about whether it actually runs or asserts anything), and a red-condition asymmetry that only checks EXISTS rows would let a downgrade to PARTIAL or INVENTED become the cheapest way to a green gate, teaching weakening as the path of least resistance", anchor: problem }
outcome: { text: "every capability claim lives in verdi/docs/guide-claims.yaml, a strict-decoded manifest of atomic capability rows — one capability, one status, one witness set per row, a bundled multi-capability row shape rejected at decode; every EXISTS or PARTIAL row's witness is bound three ways (the name exists in the corpus, it carries a // guide-claim: <row-id> anchor at its declaration, and it is PASS-coupled in make verify), closing the ADJ-50 lying-gate class a name-existence-only check would permit; every non-EXISTS row and every status downgrade carries a cite: naming a chronicle/ledger entry, gated in CI with workspace-side resolution and a loud skip; the whole check is wired into make verify and discloses its own scope honestly as inventory-only (row-to-witness, not yet guide-to-row completeness, which needs the guide itself in-repo first)", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "verdi/docs/guide-claims.yaml strict-decodes as atomic capability rows — one capability, one status (EXISTS/PARTIAL/INVENTED), one witness set per row; a bundled multi-capability row shape is rejected at decode (unknown key or shape fails closed); the manifest transcribes the guide's current Appendix B atomically at authoring, with bundled rows (7.2, 6.2, 8.4, 5.3) decomposed into their own sub-rows per the Task-0 adjudication", evidence: [static], anchor: ac-1 }
  - { id: ac-2, text: "every EXISTS or PARTIAL row's witness is bound three ways and each binding is independently red-tested: a witness name absent from the corpus reds naming both the row and the missing witness; a witness present but missing its // guide-claim: <row-id> anchor reds; a witness present and anchored but not PASS-coupled in make verify (a skip or build-tag case) reds via the require-pass.sh mechanism", evidence: [behavioral], anchor: ac-2 }
  - { id: ac-3, text: "a PARTIAL row without caveat text reds; a non-EXISTS row or any status downgrade without a cite: naming a chronicle/ledger entry reds; cite: presence is gated in CI, and its resolution (does the cited entry actually exist) is checked workspace-side only, with a loud skip when the chronicle is unavailable, since the chronicle lives out-of-repo", evidence: [behavioral], anchor: ac-3 }
  - { id: ac-4, text: "the gate is wired into make verify (internal/specalign grows to include it) and its own doc comment discloses the scope honestly as inventory-only: row-to-witness binding is proven, but guide-to-row completeness — that every claim the guide's prose makes has a corresponding row at all — is not, since that needs the guide itself in-repo (a later-phase, hard Task-18 requirement this story does not claim to satisfy)", evidence: [static], anchor: ac-4 }
links:
  - { type: implements, ref: "spec/ritual-integrity#ac-5" }
frozen: { at: 2026-07-20, commit: bd1bb1f92b7999e2aed857fe1ffb935db19249ba, stub_matched: true }
---
# Guide Claims Gate

## Problem

The owner-accepted Integration & Startup Guide makes capability claims in
its Appendix B that nothing in `make verify` ties to the corpus today. A
witness can be renamed, gutted down to an empty body, or simply never run
again, and the guide's claim about that capability stays exactly as
confident on disk as the day it was written — there is no mechanical
pressure keeping the claim honest. The design wave that pressure-tested
this story's own ledger seed (L-N4) found two ways a naive version of
this gate would fail on its own terms: a check that only proves a
witness's *name* exists in the corpus repeats the ADJ-50 lying-gate class
outright, since name existence proves nothing about whether the witness
actually runs, or asserts anything true, when `make verify` executes; and
a gate that only reds on an `EXISTS` row missing a witness, without also
reding a *downgrade* to `PARTIAL` or `INVENTED` that lacks its own
justification, would make weakening a claim the cheapest path to a green
gate — teaching exactly the wrong incentive.

## Outcome

Every capability claim the guide makes is transcribed into
`verdi/docs/guide-claims.yaml`, a strict-decoded manifest of **atomic**
capability rows: one row names exactly one capability, carries exactly
one status, and binds exactly one witness set — a bundled,
multi-capability row shape is rejected at decode, so a row can never
quietly grow to cover more ground than its own status honestly describes.
Every `EXISTS` or `PARTIAL` row's witness is bound three separate ways,
closing the ADJ-50 lying-gate class a name-existence-only check would
otherwise permit: the witness's name must exist in the corpus; it must
carry a `// guide-claim: <row-id>` anchor at its own declaration, the
same discipline `vocab:identity` markers already use, so a rename or a
gutted test becomes a visible lie rather than a silent gap; and it must
be PASS-coupled inside `make verify`, the `require-pass.sh` precedent, so
a witness that is merely present but skipped, or never actually invoked,
cannot satisfy the row. Every row that is *not* `EXISTS`, and every row
whose status is *downgraded* from a prior value, carries a `cite:` field
naming a chronicle or ledger entry — its presence gated in CI, and its
resolution checked workspace-side only, with a loud skip when the
chronicle is unavailable, since the chronicle itself lives outside this
repository. The whole check is wired into `make verify`, and its own
documentation discloses its scope honestly: this story proves
row-to-witness binding only — it does not yet prove guide-to-row
completeness, which requires the guide to be in-repo first and is a
later-phase requirement this story does not claim to satisfy.

## Ac 1

`verdi/docs/guide-claims.yaml` strict-decodes through the single
`internal/artifact` seam as a list of atomic capability rows: one row
names exactly one capability, carries exactly one status drawn from the
closed `EXISTS`/`PARTIAL`/`INVENTED` enum, and binds exactly one witness
set. A bundled row shape — for example, an early draft's attempt to
describe several sub-claims under one row — is rejected at decode with an
unknown-shape error, never silently accepted as one merged claim. The
manifest transcribes the guide's *current* Appendix B atomically at
authoring time: bundled rows the guide's prose currently groups together
(7.2, 6.2, 8.4, 5.3) are decomposed into their own atomic sub-rows per
the Task-0 design wave's adjudication (the guide's own bundled prose
rows become display groupings over several atomic manifest rows, not one
manifest row each).

## Ac 2

Every `EXISTS` or `PARTIAL` row's witness is bound three independent
ways, each with its own red-tested case. First: a witness name absent
from the corpus reds, naming both the offending row and the missing
witness. Second: a witness that is present in the corpus but missing its
own `// guide-claim: <row-id>` anchor at its declaration reds — a rename
or a gutted test that leaves the corpus entry present but silently
disconnected from the row it once satisfied must become visible, never a
silent gap. Third: a witness that is present and correctly anchored but
is not PASS-coupled inside `make verify` — skipped, gated behind a build
tag that is not exercised, or simply never invoked as part of the real
gate — reds via the same `require-pass.sh` mechanism this repository
already uses elsewhere, so a witness that exists and is named correctly
but never actually runs cannot satisfy the row.

## Ac 3

A `PARTIAL` row without accompanying caveat text reds — a partial claim
must say, in the manifest itself, what part is missing, never leave a
reader to infer it. A non-`EXISTS` row, and any row whose status is
downgraded from a value it previously held, carries a `cite:` field
naming a chronicle or ledger entry; a row lacking `cite:` where one is
required reds. `cite:`'s mere *presence* is gated in CI (the chronicle
itself is outside this repository, so CI cannot read it), but its
*resolution* — does the cited entry genuinely exist — is checked
workspace-side only, with a loud, visible skip (never a silent pass) when
the chronicle is unavailable to the checking process, mirroring the
fidelity precedent this store already uses for other out-of-repo
citations.

## Ac 4

The whole check is wired into `make verify` — `internal/specalign` grows
to include it as one more gate the full build depends on, not a
side-channel script a developer must remember to run separately. Its own
doc comment discloses the scope honestly: this story proves
**row-to-witness** binding (every claimed capability's witness genuinely
exists, is anchored, and passes) but does **not** yet prove
**guide-to-row completeness** — that every capability claim the guide's
own prose makes has a corresponding manifest row at all. That
completeness check needs the guide itself in-repo to compare against,
which is a later-phase, hard requirement this story does not claim to
satisfy; claiming otherwise here would be exactly the kind of overclaim
the Task-0 design wave's refuters caught in the story's own ledger seed.
