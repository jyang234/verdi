---
id: spec/runtime-evidence
kind: spec
title: "Runtime Evidence"
owners: [platform-team]
class: story
status: accepted-pending-build
story: jira:VERDI-3
problem: { text: "runtime is the one evidence kind with only a decoder and a fixture (I-? / OQ-2): `verdi.evidence/v1` parses `kind: runtime` and the fold renders it `pending`, but nothing emits or queries one, so a story declaring `evidence: [runtime]` can never fold to evidenced and `verdi close` cannot honestly claim to handle it. true-closure#ac-3 requires every declared kind — runtime included — to have a producing mechanism queryable by (story, AC) at close time (03 §Runtime evidence residence).", anchor: "#problem" }
outcome: { text: "a scheduled-probe runtime mechanism emits `kind: runtime` evidence records keyed by (story, AC), `source: ci`, carried in the derived tree and queryable at close time — so a story that declares runtime evidence folds to evidenced from a real record, exactly as static/behavioral do.", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a scheduled probe emits a `kind: runtime` evidence record keyed by (story, AC) with `provenance.source: ci`, carried in the derived tree and queryable by (story, AC) at close time — rollup.json is its first durable residence (03 §Runtime evidence residence, the ratified hard constraint)", evidence: [static, behavioral], anchor: "#ac-1" }
  - { id: ac-2, text: "`verdi close` and the fold consume a runtime record exactly as they consume static/behavioral: a story declaring `evidence: [runtime]` folds from pending to evidenced once a matching `source: ci` runtime record is present, proven end to end hermetically", evidence: [static, behavioral], anchor: "#ac-2" }
links:
  - { type: implements, ref: "spec/true-closure#ac-3" }
  - { type: resolves, ref: "spec/true-closure#oq-1" }
decisions:
  - { id: dc-1, text: "mechanism = scheduled probe (owner-resolved oq-1), the smallest reversible option: a probe running as a scheduled CI job emits a `kind: runtime` record keyed by (story, AC); no OTel pipeline or trace-ingest infrastructure is stood up — those stay open as future producers behind the same seam (03 §Pluggable evidence)", anchor: "#dc-1" }
  - { id: dc-2, text: "runtime records ride the derived tree (a `runtime.json` per owning-spec key, a sibling of verdicts.json) so `verdi sync`'s forge fetch carries them through the existing `forge.DerivedTree` seam — no new forge-port method; rollup.json is their first durable residence in the corpus (03)", anchor: "#dc-2" }
  - { id: dc-3, text: "authoritative provenance holds identically: the probe stamps `source: ci` only in genuine CI (D6-10); a local probe run stamps `source: local` and is folded only under --preview — verdi itself has no live service to probe, so the mechanism is proven queryable on a fixture story, and a real service plugs in its own probe behind this seam", anchor: "#dc-3" }
constraints:
  - { id: co-1, text: "no network in any test: the probe emit, the (story, AC) query, and the fold-to-evidenced are exercised hermetically over a fixture runtime record and a fixture story declaring runtime, via the httptest forge double", anchor: "#co-1" }
  - { id: co-2, text: "queryable-by-(story, AC)-at-close is the ratified hard constraint carried verbatim from 03 §Runtime evidence residence and true-closure oq-1; the mechanism satisfies it or the story is not done", anchor: "#co-2" }
frozen: { at: 2026-07-13, commit: f9b1597affa00a6570a1f1e28763372d462fe5b6, stub_matched: true }
---
# Runtime Evidence

## Problem

Runtime is the one evidence kind with only a decoder and a fixture. The
`verdi.evidence/v1` decoder accepts `kind: runtime` and the fold renders a
declared-but-unevidenced runtime AC as `pending`, but nothing ever emits a
runtime record and nothing queries one. So a story that declares `evidence:
[runtime]` can never fold to evidenced, and `verdi close` cannot honestly
claim to handle the kind. true-closure#ac-3 requires every declared evidence
kind — runtime included — to have a producing mechanism queryable by (story,
AC) at close time (03 §Runtime evidence residence).

## Outcome

A scheduled-probe runtime mechanism emits `kind: runtime` records keyed by
(story, AC), `source: ci`, carried in the derived tree and queryable at close
time. A story that declares runtime evidence folds to evidenced from a real
record, exactly as static and behavioral do. verdi itself has no live service
to probe, so the mechanism is proven queryable on a fixture; a real service
plugs its own probe in behind the same seam (03 §Pluggable evidence).

## AC-1

A scheduled probe emits a `kind: runtime` evidence record keyed by (story,
AC), `provenance.source: ci`, carried in the derived tree and queryable by
(story, AC) at close time. rollup.json is its first durable residence — the
ratified hard constraint (03 §Runtime evidence residence). Evidence: static
(the record schema + binding are declared, strict-decoded) and behavioral (a
probe run produces a well-formed (story, AC)-keyed record a query returns).

## AC-2

`verdi close` and the fold consume a runtime record exactly as they consume
static and behavioral: a story declaring `evidence: [runtime]` folds from
`pending` to `evidenced` once a matching `source: ci` runtime record is
present. Proven end to end hermetically over a fixture story and a fixture
runtime record. Evidence: static + behavioral.

## DC-1

Scheduled probe, per the owner's resolution of oq-1 and the round-6 protocol's
spike. It is the smallest reversible mechanism: a probe running as a scheduled
CI job emits a `kind: runtime` record keyed by (story, AC). No OTel pipeline
or trace-ingest is stood up; both stay open as future producers behind the
same binding seam (03 §Pluggable evidence). The choice is made here in the
spec, not silently inside an implementation.

## DC-2

Runtime records ride the derived tree — a `runtime.json` under each owning-
spec key, a sibling of `verdicts.json` — so `verdi sync`'s forge fetch carries
them through the existing `forge.DerivedTree` seam (round-6 keying fix, 01 §Ref
slugging), with no new forge-port method. rollup.json is their first durable
residence in the corpus at close, satisfying 03's residence rule.

## DC-3

Authoritative provenance holds identically to static/behavioral: the probe
stamps `source: ci` only in genuine CI (the D6-10 discipline); a local probe
run stamps `source: local` and is folded only under `--preview`. Because verdi
has no live service of its own to probe, the mechanism is proven **queryable
on a fixture story** declaring runtime; a real service supplies its own probe
behind this seam. Honest scope: this story delivers the mechanism and its
queryability, not verdi runtime evidence there is nothing to produce.

## CO-1

No network in any test. The probe emit, the (story, AC) query, and the
fold-to-evidenced are exercised hermetically over a fixture runtime record and
a fixture story that declares runtime, via the httptest forge double — the
same discipline the rest of the evidence path is tested under.

## CO-2

Queryable-by-(story, AC)-at-close is the ratified hard constraint, carried
verbatim from 03 §Runtime evidence residence and true-closure oq-1. The
mechanism satisfies it or the story is not done: `verdi close` must be able to
ask "give me the runtime records for (this story, this AC)" and get them.
