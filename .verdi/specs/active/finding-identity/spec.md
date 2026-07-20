---
id: spec/finding-identity
kind: spec
title: "Finding Identity"
owners: [platform-team]
class: story
status: draft
story: jira:VERDI-P2-2
problem: { text: "regenerating a judge-backed align report today discards every prior disposition wholesale, because internal/align/identity.go's Identity deliberately folds Kind+ID+Text into the disposition-carry hash — a load-bearing, fail-closed design guarding against a stale disposition surviving a real verdict or witness change; but a JUDGED finding's Text is the judge's own prose, which varies run to run even when nothing about the underlying issue changed, so the same fail-closed design discards every judged disposition on every regeneration, not just the ones that actually changed, and X-18 showed the second-order cost directly — a discarded disposition is simply re-recorded and re-accepted on the next pass, so the identical standing adjudication consumes budget against the spec-stale deviations threshold every time the report regenerates, and a feature can be blocked from closing by its own settled history repeating itself", anchor: problem }
outcome: { text: "a judged finding's rule/boundary-derived slug becomes an untrusted hint for regeneration, never a trusted identity key: a regenerated finding whose slug matches a prior dispositioned finding is pre-filled as a candidate — the old ruling and old text shown beside the new text — and AllDispositioned stays false until a human confirms each candidate individually as a working-tree edit at the covering head; the frozen computed-identity rule in identity.go is unchanged byte-for-byte, a confirmed reaffirmation carries carried-from: <covers-sha> excluded from the report digest and omitempty, a finding dispositioned before but absent from a fresh run lands in not-resurfaced: persisted until a human marks it fixed, and the spec-stale budget counts unique accepted-deviation identities across findings: union not-resurfaced: while the feature-close budget unions the implementing stories' archived reports with the feature's own", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "a regenerated judged finding whose slug matches a finding the prior report already dispositioned is presented as a candidate — the old ruling and the old text rendered beside the new text — never silently carried and never silently discarded; AllDispositioned stays false until a human confirms each candidate individually as a working-tree edit at the covering head, exactly the discipline X-16 already established for fresh findings", evidence: [behavioral], anchor: ac-1 }
  - { id: ac-2, text: "the frozen computed-identity rule (Kind+ID+Text, identity.go) is unchanged byte-for-byte — the existing holds-to-violated negative test keeps passing unmodified — and an escalation under a stable slug (a low-confidence cosmetic ruling followed by a high-confidence real regression at the identical slug) presents both texts side by side rather than silently inheriting the old ruling; a confirmed reaffirmation carries carried-from: <covers-sha> on the disposition, excluded from the report digest (VerifyDigest unaffected on every existing frozen archive) and omitempty (every old fixture keeps decoding unchanged)", evidence: [behavioral], anchor: ac-2 }
  - { id: ac-3, text: "a finding dispositioned in a prior report but absent from a fresh run lands in a not-resurfaced: section, persisted across further regenerations until a human explicitly marks it fixed; the spec-stale budget counts unique accepted-deviation identities across findings: union not-resurfaced:, proven by a judge-re-roll replay in which a finding fails to reproduce and the accepted-deviation count stays exactly unchanged — the X-18 laundering drain closed at the mechanism that caused it", evidence: [behavioral], anchor: ac-3 }
  - { id: ac-4, text: "the feature-close budget is a union over every implementing story's archived report plus the feature's own report, proven against the true X-18 shape (an accepted deviation recorded in one story's archived report counts exactly once, never zero and never twice, in the feature-close union); two findings that land on the same slug within a single report is disclosed as a judge-contract-violation finding in its own right, never silently deduplicated into one", evidence: [behavioral], anchor: ac-4 }
links:
  - { type: implements, ref: "spec/ritual-integrity#ac-2" }
---
# Finding Identity

## Problem

`internal/align/identity.go`'s `Identity` folds `Kind+ID+Text` into the
hash that decides whether a prior disposition survives report
regeneration — deliberately, so that a real verdict or witness change
voids a stale disposition rather than silently carrying it forward. This
is a load-bearing, fail-closed design, and it is not wrong for a
computed or conflict finding, whose `Text` is deterministic. But a
`JUDGED` finding's `Text` is the judge's own free-form prose, and two
runs of a real LLM judge exchange over the identical underlying issue
essentially never produce byte-identical text — so the very design that
protects against a stale disposition discards *every* judged disposition
on *every* regeneration, whether or not anything about the issue actually
changed. X-18 named the second-order cost directly: a discarded
disposition is not simply lost, it is re-recorded and re-accepted on the
very next pass, so the identical, already-settled adjudication consumes
fresh budget against the spec-stale deviations threshold each time the
report regenerates. A feature's own history, repeating itself through no
fault of anyone's judgment, can block that feature from ever closing.

## Outcome

A judged finding's slug — derived from the rule or boundary the finding
attacks, never from the judge's prose itself — becomes an **untrusted
hint** for regeneration, never a trusted identity key. When `align`
regenerates a report and a fresh finding's slug matches a finding the
prior report already dispositioned, the fresh finding is pre-filled as a
**candidate**: the old ruling and the old text rendered beside the new
text, so a human sees exactly what changed before deciding anything.
`AllDispositioned` stays false until a human confirms each candidate
individually, as a working-tree edit at the report's covering head —
exactly the same discipline X-16 already established for fresh findings,
extended rather than replaced. The frozen computed-identity rule in
`identity.go` is unchanged byte-for-byte: the existing holds-to-violated
negative test keeps passing unmodified, because this story's slug-primary
matching branches on `Kind == FindingJudged` only. A confirmed
reaffirmation gains `carried-from: <covers-sha>` provenance, excluded
from the report digest (so `VerifyDigest` is unaffected on every existing
frozen archive) and `omitempty` (so every old fixture keeps decoding
as-is). A finding dispositioned before but absent from a fresh run lands
in a `not-resurfaced:` section, persisted across further regenerations
until a human explicitly marks it fixed. The spec-stale budget counts
unique accepted-deviation identities across `findings:` union
`not-resurfaced:` — closing the X-18 judge-re-roll laundering drain
outright — while the feature-close budget unions every implementing
story's archived report with the feature's own, the actual cross-report
mechanism X-18's own postmortem named as needed.

## Ac 1

When `align` regenerates a report and a fresh judged finding's slug
matches a finding the prior report already dispositioned, the fresh
finding is pre-filled as a **candidate**, never silently carried and
never silently discarded: the old ruling and the old text are rendered
beside the new text, so a human sees precisely what the judge's prose
changed before making any decision. A candidate is explicitly not a
disposition — `AllDispositioned` stays false until a human confirms each
candidate individually, and confirmation is a working-tree edit at the
report's covering head, exactly the discipline X-16 already established
for fresh findings rather than a new, second confirmation mechanism.
Driven against the canned-judge fake across a same-slug, reworded-text
regeneration.

## Ac 2

The frozen computed-identity rule in `internal/align/identity.go` — the
content hash over `Kind+ID+Text`, deliberately fail-closed so a verdict
or witness change voids a stale disposition — is unchanged byte-for-byte:
the existing holds-to-violated negative test keeps passing unmodified,
because the slug-primary matching this story adds branches on
`Kind == FindingJudged` only. An escalation under a stable slug — a
low-confidence cosmetic ruling followed, on a later run, by a
high-confidence real regression landing at the identical slug — presents
both texts side by side rather than silently inheriting the old, wrong
ruling; the human must choose, every time. A confirmed reaffirmation
gains `carried-from: <covers-sha>` on the disposition — a schema-additive
field on `internal/artifact/deviation.go` — excluded from the report
digest (a frozen-report fixture proves `VerifyDigest` is unaffected) and
`omitempty` (an old fixture without the field still decodes unchanged).

## Ac 3

A finding that was dispositioned in a prior report but that a fresh judge
run simply does not re-emit lands in a `not-resurfaced:` section — never
treated as resolved, since a non-reproducible judge failing to re-emit a
finding proves nothing about whether the underlying issue is actually
fixed. That section persists across further regenerations until a human
explicitly marks the finding fixed. The spec-stale deviations budget
counts unique accepted-deviation identities across `findings:` union
`not-resurfaced:` — proven by the exact laundering replay X-18 witnessed:
re-run the judge so a previously-dispositioned finding does not
reproduce, and assert the accepted-deviation count is unchanged, not
decremented. `not-resurfaced:` has exactly two consumers, both exercised
here: the disposition pre-fill UI (so a `not-resurfaced` finding that
resurfaces later still pre-fills as a candidate) and the deviations
counterweight.

## Ac 4

The feature-close budget is a union over every implementing story's
*archived* report plus the feature's own report — the actual
cross-report mechanism X-18's own postmortem named as needed, proven
against the true shape X-18 witnessed: an accepted deviation recorded in
one story's archived report counts exactly once toward the feature-close
budget, never zero (silently dropped because the feature's own report
never reproduced it) and never twice (double-counted across the story and
feature reports independently). Two findings that land on the same slug
within a single report — a genuine judge-contract violation, since a
rule/boundary-derived slug is defined to be a stable per-finding-class
identifier within one run — is disclosed as its own finding, never
silently deduplicated into one entry that would hide which of the two the
human actually dispositioned.
