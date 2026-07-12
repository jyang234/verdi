---
id: spec/true-closure
kind: spec
title: "True Closure"
owners: [platform-team]
class: feature
status: draft
problem: { text: "verdi's closure ritual is ratified text the binary declines to run: `verdi close` is recognized by dispatch only to answer \"not implemented (out of v0 scope)\" (I-23), so no story built against this system has ever reached a true, archived closure. Every gate proven to date — merge, lint, align — has consumed local/advisory evidence or fixture bundles; authoritative evidence has never once come from this repo's own CI, so constitution 4 (\"artifacts that gate come from trusted CI, never from the author under review\") has been asserted in prose and never actually exercised end to end. Runtime evidence, the third leg of the evidence model, has no producing mechanism at all — OQ-2 is deferred with only a decoder-level placeholder (`kind: runtime` parses; nothing ever emits or queries one). And two supersession-legibility gaps carry forward from round 5's D-12 fix pass: the story-predecessor terminal-state flip now has a scoped mechanism but no proven legibility at the surfaces that render it, and the feature-predecessor terminal-state question was explicitly deferred to this round (02 §Kind registry: \"its terminal-state question is carried to round 6\"). Taken together, the system's central promise — that what lands is in alignment with what was agreed — has never been proven past the merge gate. A merged story is not a closed story, and nothing has ever tested whether it could become one.", anchor: "#problem" }
outcome: { text: "a story travels from accepted spec to true, archived closure on authoritative CI evidence alone, its rollup published — and every evidence kind a spec can declare has a real producing mechanism.", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a merged story reaches a true, archived closure — quartet and all — on authoritative CI-produced evidence alone", evidence: [behavioral, attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "the closure's rollup is published to the configured tracker and is readable there", evidence: [behavioral, attestation], anchor: "#ac-2" }
  - { id: ac-3, text: "every evidence kind a spec can declare — runtime included — has a producing mechanism queryable by (story, AC) at close time", evidence: [behavioral, attestation], anchor: "#ac-3" }
  - { id: ac-4, text: "a superseded spec's terminal state is legible at both levels, story and feature predecessors alike", evidence: [behavioral, attestation], anchor: "#ac-4" }
decisions:
  - { id: dc-1, text: "authoritative evidence comes only from the repo's own CI (the verdi-evidence job), never local regeneration — for verdi itself exactly as for any consumer", anchor: "#dc-1" }
  - { id: dc-2, text: "the round-6 tracker is the hermetic fake provider; real Jira is a config change, not a code change", anchor: "#dc-2" }
open_questions:
  - { id: oq-1, text: "which runtime-evidence mechanism — scheduled probe, OTel-derived check, or trace ingest — and what does its (story, AC)-queryable store look like?", anchor: "#oq-1" }
stubs:
  - { slug: remote-and-ci, acceptance_criteria: [ac-1] }
  - { slug: close-verb, acceptance_criteria: [ac-1, ac-2] }
  - { slug: runtime-evidence, acceptance_criteria: [ac-3] }
  - { slug: feature-supersession-state, acceptance_criteria: [ac-4] }
---
# True Closure

## Problem

verdi's closure ritual is ratified text the binary declines to run. 05
§CLI names `verdi close <story|feature>` in full: fetch runtime records,
verify eligibility, run `align --freeze`, generate the frozen rollup, open
the closure MR — for a feature, additionally require every AC `evidenced`
(including the outcome floor) and stub reconciliation passed. None of it
runs. I-23 records the honest v0 posture: `close` is *recognized* by
dispatch and answers "not implemented (out of v0 scope)", exit 2, rather
than lying about the verb surface. That posture was correct for v0 — but
it means no story ever built against this system, including this system's
own self-hosted specs, has reached a true, archived closure. The quartet
(spec, board.json, rollup.json, deviation-report.md) exists only as a
described shape, never as a produced artifact.

Worse, the gates that *do* run have never proven the thing constitution 4
requires: "artifacts that gate come from trusted CI, never from the author
under review; local regeneration is advisory." Every merge gate exercised
so far — in this repo and in the round-4/round-5 protocol runs before it —
has consumed local/advisory evidence or fixture bundles assembled by hand.
This repo's own CI workflow is named `verify`, and it runs `make verify`:
build, fmt, vet, lint, tests, fixture gates. There is no `verdi-evidence`
job. 03 §Evidence records names the convention precisely — "CI publishes
this bundle under one fixed convention: the job (GitLab) or workflow
(GitHub) named `verdi-evidence` uploads the `data/derived/<ref-slug>/
<commit>/` tree as its artifact" — and this repo has never had a remote to
run it on: there is no `origin`, no forge, no pull request has ever been
opened for this store. `source: ci` evidence, the only kind a gate is
permitted to consume authoritatively, has never existed for any story
here. `provenance.source: local` is all this system has ever proven
anything on.

Runtime evidence fares worse still: it has no mechanism at all. OQ-2 is
deferred "with teeth kept" (PLAN.md §8) — the `verdi.evidence/v1` decoder
accepts `kind: runtime`, and the fold's `pending` rendering is locked by a
fixture — but nothing has ever emitted a runtime record, and nothing
queries one. 03 §Runtime evidence residence states the hard constraint
this round inherits verbatim: runtime records must be queryable by
(story, AC) at close time, with `rollup.json` as their first durable
residence in the corpus. A `verdi close` that cannot query runtime
evidence cannot close a story that declared any — and no spec here has
ever been forced to find out, because `close` has never run.

Finally, two supersession-legibility gaps carry forward from round 5's
D-12 fix pass (landed on this branch's parent commit, f694edb). The
ritual flip that marks a superseded predecessor's `status:` was
deliberately scoped to rung-3 **story** predecessors only; the rung-4
**feature** predecessor case was carried forward in 02's own words: "its
terminal-state question is carried to round 6." That is one gap — no
feature-predecessor terminal state exists at all yet. The other is
subtler: even the story-predecessor flip that *does* exist has never been
proven legible where an operator would actually look for it — the board,
`verdi matrix`, dex — only that the frontmatter status field itself
changes. A status nobody can see is not meaningfully different from no
status. Both gaps mean the system's supersession chain, load-bearing for
"what is currently authoritative," cannot yet be read with confidence at
either rung.

Put together: verdi's central promise — that what lands is in alignment
with what was agreed — has been proven at the merge gate and nowhere
past it. A merged story has never been closed. Closure has never touched
real CI evidence. Runtime evidence has never been produced. And a reader
trying to determine what is current in the face of supersession cannot
yet do so reliably at both levels the system defines. This feature exists
to close all four gaps together, because they are one gap seen from four
angles: the system has never proven, past the merge gate, that it keeps
its own promise.

## Outcome

A story travels from accepted spec to true, archived closure on
authoritative CI evidence alone, its rollup published — and every
evidence kind a spec can declare has a real producing mechanism.

Concretely: a story merged under the existing merge gate can be run
through `verdi close`, which fetches its evidence — static, behavioral,
attestation, and now runtime — exclusively from records whose provenance
is `source: ci`, produced by this repo's own `verdi-evidence` CI job;
freezes its alignment report; generates a real `rollup.json`; publishes
that rollup to the story's configured tracker where it is actually
readable; and archives the quartet. No step in that chain falls back to
local regeneration to make the gate pass. A feature that has evidenced
every AC (outcome floor included) and reconciled every stub can close the
same way. And a reader asking "is this spec still current, or has it been
superseded — at the story rung, or the feature rung?" gets a legible
answer from the surfaces that render specs, not just from a frontmatter
field.

This feature binds to two concrete infrastructure facts rather than
inventing them. First: the remote this round wires up is
`github.com/jyang234/verdi` (private) — the `github.com/OWNER/verdi`
placeholder that has stood in `go.mod`, `verdi.yaml`'s toolchain pin
target, and every PLAN.md reference since OQ-5 was resolved settles to
`jyang234` for real in the remote-and-ci story; nothing about the
module's identity is invented here, it is simply made concrete. Second:
this feature deliberately does NOT reach for a live tracker. The round-6
tracker of record is the hermetic fake provider (dc-2) — real Jira
integration is already contract-suite proven by 04's adapter and stays a
config change away, out of scope for what this round needs to prove about
publish mechanics.

One piece of round-6 work is intentionally scoped OUTSIDE this feature
and is called out here so its absence from the stub list below is legible
rather than silent: `list-disclosures`, realizing `spec/
disclosure-legibility#ac-2` (the still-open "enumerate every current
disclosure in one view" AC from round 5's accepted-pending-build
feature), attaches this round as a **late story** to that already-frozen,
downward-blind feature — not as a stub or an AC of *this* spec. Round 5
observed that a feature is never amended when stories are added after
acceptance (02 §Object model: "the feature is downward-blind"); this
round deliberately exercises that path for real, adding a story against
an accepted feature that has no stub reserving it, rather than folding
the work into true-closure for convenience. It is round-6 work; it is not
this feature's work.

## AC-1

A merged story reaches a true, archived closure — quartet and all — on
authoritative CI-produced evidence alone. "True" means the closure was
never assembled from local or advisory records to make it happen: every
evidence record `verdi close` folds into the decision to close carries
`provenance.source: ci`, sourced from a real `verdi-evidence` run on this
repo's own remote, fetched the way `verdi sync` already fetches evidence
bundles — by (ref, commit) through the forge port. "Archived" means the
quartet — spec, `board.json`/`layout.json`, `rollup.json`, and
`deviation-report.md` — lands in `specs/archive/` as real, frozen,
git-committed artifacts, not a described shape. This is the load-bearing
AC of the feature: every other AC exists to make this one possible
without inventing a shortcut.

Evidence: behavioral (an exerciser drives a real story from acceptance
through `verdi close` against real CI-produced evidence and confirms the
archived quartet exists, is frozen, and traces to `source: ci` records
only) and attestation (an operator affirms the closed story's evidence
trail holds up to inspection — no local-only record is load-bearing in
the decision to close).

## AC-2

The closure's rollup is published to the configured tracker and is
readable there. `rollup --publish` already exists and is contract-suite
proven (00 §delivered: "`rollup --publish` with the Jira adapter (field +
change-only comment)") — this AC is about closure actually reaching that
publish step for real, with a real (if hermetic-fake) tracker on the
other end, and the published rollup being something a reader of that
tracker can actually read, not merely something the CLI claims to have
sent.

Evidence: behavioral (an exerciser confirms a closed story's rollup lands
on the configured provider and can be read back through it) and
attestation (an operator affirms the published rollup, read from the
tracker's own surface, accurately reflects the story's final fold).

## AC-3

Every evidence kind a spec can declare — runtime included — has a
producing mechanism queryable by (story, AC) at close time. Static,
behavioral, and attestation evidence already have real producers; runtime
is the one kind that has only ever had a decoder and a fixture. This AC
closes that gap: whichever mechanism OQ-1 resolves to (oq-1), it must
answer a (story, AC) query the way 03 §Runtime evidence residence
requires, so that `verdi close` (ac-1) can fetch runtime records for a
story that declared them exactly as it fetches every other kind, rather
than being permanently unable to close such a story.

Evidence: behavioral (an exerciser confirms the runtime mechanism, once
built, actually answers a (story, AC) query with a real record verdi
close can fold) and attestation (an operator affirms the mechanism's
queryable store is a real residence, not a log an engineer has to go
find by hand).

## AC-4

A superseded spec's terminal state is legible at both levels, story and
feature predecessors alike. "Legible" is the operative word: the
frontmatter status flip already exists at the story rung (round 5's
D-12); this AC requires that the flip is visible where an operator
actually looks — the board, `verdi matrix`, dex — at that rung, and that
an equivalent, currently-nonexistent mechanism exists and is equally
visible at the feature rung, closing the gap 02 §Kind registry names by
name: "a superseded feature predecessor's status remains governed by the
rung-4 cascade machinery for now — its terminal-state question is carried
to round 6."

Evidence: behavioral (an exerciser confirms both a superseded story spec
and a superseded feature spec render their terminal state, unambiguously,
on every surface that renders specs) and attestation (an operator
affirms that finding a superseded predecessor's status no longer requires
reading raw frontmatter).

## DC-1

Authoritative evidence comes only from the repo's own CI (the
verdi-evidence job), never local regeneration — for verdi itself exactly
as for any consumer. This is constitution 4 made concrete for the
self-hosted arena: verdi's specs have always said gates consume
authoritative evidence only, and this repo has always, in fact, gated on
local/advisory evidence and fixture bundles because there was no CI
producing anything else. That was an honest gap while nothing in the
build attempted true closure — it stops being honest the moment `close`
exists and reaches for a shortcut. This decision forecloses the
shortcut: the GitHub Actions workflow this round wires up is the trust
root, full stop, with no "but locally regenerated is close enough"
fallback for verdi's own self-hosted stories. This is why the
`remote-and-ci` stub exists as its own story rather than being folded
into `close-verb` — the trust root has to exist and be exercised for
real before anything downstream can honestly claim to consume it.

## DC-2

The round-6 tracker is the hermetic fake provider; real Jira is a config
change, not a code change. 04's provider port already ships a fake
provider plus a contract-test suite every adapter — fake and Jira alike —
must pass, and `rollup --publish` is already proven against both. This
round is not about proving the Jira adapter again; it is about proving
that `verdi close` actually reaches the publish step and that a published
rollup is readable on the other end (ac-2). Using the fake provider keeps
that proof hermetic and fast without weakening it: switching the tracker
of record to real Jira, when that day comes, is `verdi.yaml`'s
`providers.jira` block and nothing else — no new code path, because the
adapter boundary was already the thing 04 proved.

## OQ-1

Which runtime-evidence mechanism — scheduled probe, OTel-derived check,
or trace ingest — and what does its (story, AC)-queryable store look
like? This is the ratified hard constraint carried forward verbatim from
03 §Runtime evidence residence and 00 §OQ-2: the mechanism is left open,
but whichever one is chosen must satisfy the queryable-by-(story, AC)
requirement before `verdi close` can be said to handle runtime evidence
honestly. This is the round-6 spike's target — the `runtime-evidence`
stub below is expected to be realized in part by a timeboxed spike
answering this question before the mechanism itself is built, rather
than the choice being made silently inside an implementation story.
