---
id: spec/judge-ergonomics
kind: spec
title: "Judge Ergonomics"
owners: [platform-team]
class: story
status: draft
story: jira:VERDI-P2-1
problem: { text: "every judge-backed verb — align's real LLM judge exchange, and close's internal freeze-align — runs 6-7 minutes, past an agent's foreground patience and past what a background monitor can survive (a monitor dies with the turn; nothing can resume a stopped agent from a completion event alone); X-8 witnessed this five times in one round (a builder idling silently on a port-poll, two builders self-catching hung tail-f monitors, a Fable builder and two fix agents all parking on align/verify awaiting an event that never resumes them), and nothing in the verb's own contract today tells a caller where to look or how long to wait before giving up and resuming later", anchor: problem }
outcome: { text: "align prints its report path as the first line of stdout before the judge subprocess ever runs and writes the report atomically at completion through the existing atomicfile seam, so no partial content is ever observable at that path mid-run; a new --wait[=seconds] flag polls internally up to a bound and exits 2 with the report path already printed on expiry, never a silent hang; the contract lives once in the shared align engine so close's internal freeze-align inherits it rather than carrying a second, divergent implementation", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "align prints its report path as the first line of stdout before the judge subprocess ever runs, and the report is written through the existing atomicfile seam at completion — a reader polling the path observes either nothing yet or the finished report, never partial or truncated content", evidence: [behavioral], anchor: ac-1 }
  - { id: ac-2, text: "align supports a bounded --wait[=seconds] flag: against a judge that completes within the bound, the verb blocks internally and exits 0 normally once the report is ready; against a judge that does not complete within the bound, the verb exits 2 with the report path already printed, never a silent hang past the bound", evidence: [behavioral], anchor: ac-2 }
  - { id: ac-3, text: "the --wait contract is implemented once in the shared align engine: close's internal freeze-align inherits the identical first-line-path, atomic-write, and bounded --wait behavior from the same engine hook align uses, proven by exercising freeze-align's own bounded wait/expiry path directly rather than only align's — no per-verb reimplementation exists to diverge from it", evidence: [behavioral], anchor: ac-3 }
links:
  - { type: implements, ref: "spec/ritual-integrity#ac-1" }
---
# Judge Ergonomics

## Problem

Every judge-backed verb in this ritual — `align`'s real LLM judge exchange
and `close`'s internal freeze-align, which shares the same judge call
underneath — runs six to seven minutes, well past an agent's foreground
patience and past what a background monitor can survive: a background
monitor dies with the turn it was started in, and nothing can resume a
stopped agent from a completion event alone. This is not a hypothetical
risk; it is `spec/ritual-integrity`'s X-8 finding, witnessed five separate
times in a single round — a builder idling silently on a port-poll, two
builders self-catching their own hung `tail -f` monitors, a Fable builder
and two fix agents all parking on `align`/`verify` awaiting an event that
never arrives to resume them. Every occurrence cost real time and required
a human or controller nudge to recover. The round's workaround — chained
bounded foreground waits, a process discipline every dispatch must now
state up front — is exactly that: a discipline imposed from outside the
tool, not a property the tool itself offers. Nothing in a judge-backed
verb's own contract today tells a caller where the report will land or how
long is reasonable to wait before giving up and resuming later; a caller
who cannot poll forever has no cheap way to check in on progress.

## Outcome

`align` prints its report path as the first line of stdout, before the
judge subprocess ever runs, so a caller — human or agent — always has a
filesystem location to watch without parsing anything else the verb
prints. The report is written through the existing `atomicfile` seam at
completion, so no partial or truncated content is ever observable at that
path mid-run: a reader polling the path sees either nothing yet or the
finished report, never a half-written one. A new `--wait[=seconds]` flag
turns that watchability into a bounded verb behavior: the verb polls
internally, on the caller's behalf, up to the given bound (or a sane
default), and on expiry exits 2 *with the report path already printed* —
never a silent hang, and never a caller left guessing where to look. The
contract is implemented once, in the shared align engine, so `close`'s
internal freeze-align inherits it rather than carrying a second,
divergent implementation; any future judge-backed verb gets the contract
by construction rather than by a per-verb reimplementation someone has to
remember to write.

## Ac 1

`align` prints its report path as the first line of stdout before the
judge subprocess is ever invoked — a caller never has to wait for the
judge exchange to learn where to look. The report itself is written
through the existing `internal/atomicfile` seam (`verdi/internal/align/
judge.go`, `judged.go`) at completion, the same primitive every other
corpus write in this repo shares: a reader polling the printed path at any
point mid-run observes either no file yet or the complete, final report —
never a partial write. This must hold against `align.go`'s own
`loadExistingReport` (`align.go:329`), the reader the sentinel discipline
has to agree with, so a caller re-reading the same path a waiting agent is
watching sees the identical guarantee.

## Ac 2

`align` accepts a bounded `--wait[=seconds]` flag and polls internally on
the caller's behalf rather than returning immediately. Driven against the
canned-judge fake (`internal/align/judged_test.go`'s pattern): a
fast-completing judge under `--wait` causes the verb to block until the
report is ready, then exit 0 normally, the report already on disk. A
judge that does not complete within the given bound (or the sane default
when the flag carries no explicit value) causes the verb to exit 2 — not
1, since this is an operational timeout, not a verdict — with the report
path already printed as line 1, so the caller always knows where to
resume watching. No caller is ever left parked on an open-ended wait: the
bound is enforced internally by the verb itself, not by the caller's own
polling discipline.

## Ac 3

The `--wait` contract — first-line report path, atomic write, bounded
internal polling — is implemented exactly once, in the shared align
engine that both `align` and `close`'s internal freeze-align call through,
rather than twice. This is proven directly, not by inference from align's
own tests: a test exercises `close`'s freeze-align path specifically,
against the same canned-judge fake, confirming the identical first-line
path, the identical atomic-write guarantee, and the identical bounded
`--wait`/exit-2-with-path-on-expiry behavior — so a future third
judge-backed verb inherits the contract by calling the same engine hook,
with no separate implementation left to drift out of step.
