---
id: spec/disposition-verb
kind: spec
title: "Disposition Verb"
owners: [platform-team]
class: story
status: accepted-pending-build
story: jira:VERDI-30
problem: { text: "recording a reviewer's decision on a deviation-report finding has no verb: every round-6 disposition (D6-25) was a hand-edit of deviation-report.md's frontmatter, typing a disposition and note into the findings: block directly, with no validation before the write, no refusal for an unknown or already-dispositioned finding, no protection against editing an already-frozen report, and no guarantee the finding's rendered line in the report's own markdown body stays in agreement with what the frontmatter now says. The disposition layer this story binds to (spec/closure-ergonomics dc-4) is deliberately kept outside the report's integrity digest so that recording a decision can never invalidate it — but nothing today makes that property, or any of the others a verb would give for free, true by construction rather than by care.", anchor: problem }
outcome: { text: "a new verdi disposition verb records a reviewer's decision (the finding, the decision, and the rationale) into a deviation report's living disposition layer, in place, leaving its digest and integrity untouched and independently reverifiable; keeps the report's human-legible body in agreement with what it just wrote; refuses, as named verdicts, every unsafe request (an unknown finding, a re-disposition without an explicit --amend, any write to an already-frozen report); and survives verdi align --freeze byte-for-byte through the already-landed FreezeInPlace path (PR #99) and its D6-24 keep-genuine guard (PR #101). The previously de-facto hand-edit flow is retired from architecture-and-journeys.md's closure-ritual narrative: after this story, that document names verdi disposition as the only sanctioned way to record one.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "verdi disposition <spec-ref> <finding-id> <fixed|accepted-deviation> --rationale <text> writes exactly one disposition into <spec-ref>'s LIVING deviation-report.md (a report carrying no frozen: stamp): the named finding's frontmatter entry gains disposition: <decision> and note: <rationale> (internal/artifact's Finding.Disposition/.Note, deviation.go), and its corresponding rendered line in the report's markdown body is updated to the same decision and rationale, so the human-legible prose and the machine record agree. <decision> accepts only the two values the existing schema already knows, fixed and accepted-deviation (FindingDisposition, deviation.go); the verb defines no new vocabulary. Every OTHER finding, and the report's digest:, integrity:, and judge_integrity: fields, are carried over byte-for-byte: align.ComputeDigest recomputed over the same computed section still equals the stored digest value, and, when a genuine judged exchange is present, align.VerifyIntegrity still succeeds — the write is invisible to either verifier.", evidence: [behavioral], anchor: ac-1 }
  - { id: ac-2, text: "The verb refuses, each as a named verdict (exit 1) rather than a silent or partial write, on: a finding-id absent from <spec-ref>'s current findings: list; a finding that already carries a disposition unless --amend is given (and, symmetrically, --amend on a finding with no existing disposition, since there is nothing to amend); and a <spec-ref> whose deviation-report.md already carries a frozen: stamp, since a frozen report is immutable to every verb, this one included (mirroring align.FreezeInPlace's own precondition, freeze.go). A genuinely operational failure, no deviation-report.md at <spec-ref>'s path at all, or one that fails strict decode, exits 2, distinct from the three verdicts above.", evidence: [behavioral], anchor: ac-2 }
  - { id: ac-3, text: "A finding dispositioned by this verb, once every finding in the report is dispositioned, survives verdi align --freeze byte-for-byte: align.FreezeInPlace (PR #99) stamps the living, verb-written report verbatim, proven end to end against the D6-24 keep-genuine fix's drifting-judge harness (T.1, PR #101, mirroring TestRunAlign_FreezePreservesDispositions in cmd/verdi/align_test.go) so a judge run that would produce different content on a hypothetical regeneration is shown to have no path into the frozen output.", evidence: [behavioral, attestation], anchor: ac-3 }
  - { id: ac-4, text: "After this story, verdi/docs/architecture-and-journeys.md's closure-ritual narrative, currently silent on how a finding gets dispositioned (every finding gets dispositioned, section D.5, names no mechanism), names verdi disposition as the only sanctioned way to record one; no sentence in that document describes or implies hand-editing a deviation report's disposition fields as an accepted practice.", evidence: [static], anchor: ac-4 }
links:
  - { type: implements, ref: "spec/closure-ergonomics#ac-3" }
decisions:
  - { id: dc-1, text: "A new top-level verb, verdi disposition <spec-ref> <finding-id> <fixed|accepted-deviation> --rationale <text>, matching the schema's own FindingDisposition vocabulary (deviation.go) and mirroring verdi accept's shape: a small, single-purpose, spec-ref-scoped write verb. This is the CLI-surface addition task 3.R's ratification PR must carry (05 §CLI's table names no disposition row today; cmd/verdi/dispatch.go's verbPhase names no disposition key). Rejected alternatives: a mode of verdi align (align --disposition ..., mirroring dc-5's close-preflight precedent) — rejected because align's job is to regenerate (Compute plus judge) or freeze the whole report, and grafting a single-finding mutate path onto the same verb conflates two different mechanics single responsibility keeps apart, and risks the exact regeneration hazard T.1 fixed if a future change ever wires the new flag through the regenerate path instead of a true surgical edit; a subcommand (verdi align disposition ...) — rejected because align is an established flag-based verb, not a subcommand dispatcher, and grafting subcommand syntax onto it would be an inconsistent shape within the same verb; folding it into verdi close (close --disposition ...) — rejected because disposal happens mid-build, routinely long before close is ever attempted (the merge gate itself requires a fully-dispositioned report), misplacing the verb at the wrong point in the lifecycle purely because both concern closure eventually; a write path on verdi gate — rejected because gate is a read-only verdict verb over existing state and must never become a mutator. This is a narrower reading of dc-5's verb-economy motive, not a departure from its letter: dc-5's own text scopes its one-verb consolidation to ac-1's preflight specifically (No new verb is added for ac-1); it says nothing about ac-3's disposition-recording, which close's own scope (eligibility, freeze, rollup, archive-move on an already-fully-dispositioned report) never touched even before this story — stretching dc-5's preflight-specific consolidation onto a verb it never named would be the actual overreach.", anchor: dc-1 }
  - { id: dc-2, text: "The verb's write mechanics mirror align.FreezeInPlace's own discipline exactly: value-copy, never mutate the decoded original; self-validate before write; never fake success. Decode the report (artifact.DecodeDeviation), copy it, set only the target finding's Disposition and Note, Validate the copy, then re-render via align.RenderMarkdown, deviation-report's own deterministic, hand-rolled re-renderer and its established serialization path, never a generic yaml.Marshal, never internal/artifact/splice, which is scoped to spec.md only per its own package doc. The verb additionally keeps the report's markdown body in agreement with the frontmatter write: the target finding's rendered bullet line (the disposition-labeled line align.RenderBody's renderFindings produces) is updated to the same decision and rationale, while every other line, including the Boundary-diff and Diagram-alignment subsections the verb has no data to regenerate (neither is stored in the frontmatter; align.FreezeInPlace's own doc comment is explicit that the digest's inputs cannot be recomputed from a decoded report alone), is carried over byte-for-byte. This is a judgment call flagged for review: the parent ac-3 speaks only to the frontmatter disposition layer, but internal/dex/storyaxis.go's renderQuartetDeviation renders both a table built fresh from frontmatter and the report's raw body prose on the same archived page — a frontmatter-only write would leave a permanently frozen, publicly rendered record where a table cell reads accepted-deviation a few lines above a bullet still reading UNDISPOSITIONED for the same finding. Keeping the two in agreement is the smaller-surface way to avoid shipping that self-contradiction, without requiring the verb to re-run Compute, which needs the toolchain runner, not a local, offline, single-field write, to regenerate sections it cannot safely touch. This body update never touches digest- or integrity-covered content: both hashes are computed over (covers, finding id/kind/text, baseline diffs) and the persisted judge stdin/result bytes only (internal/align/verify.go's digestInput and artifact.JudgeIntegrity) — the rendered body has never been, and is not made by this story to become, an input to either; grepping internal/align for any hash/digest call reached from RenderMarkdown, RenderBody, or renderFindings turns up none.", anchor: dc-2 }
  - { id: dc-3, text: "Exit discipline follows the parent's co-2 verbatim: every verb keeps 0 clean, 1 verdict, 2 operational, and unmet conditions are a verdict, not an error. The three refusals ac-2 names are verdicts about the report's own state (an unresolvable finding, a disposition collision, an immutable frozen record), not environment failures, so each exits 1 and names the offending finding id or state explicitly, never a silent absence. --amend is required to change an existing disposition and refused when there is nothing to amend, a deliberate symmetry so the flag is never a no-op rubber stamp, chosen over either always allowing silent overwrite (risks silently discarding a prior reviewer's rationale) or never allowing amendment at all (would force a return to hand-editing for the one case, a reviewer's own mistake, a verb exists to remove). An amended disposition fully replaces the prior decision and rationale; the superseded text is not retained inside the artifact itself, ordinary git history is the audit trail, exactly as it already is for a hand-edited report today.", anchor: dc-3 }
  - { id: dc-4, text: "<spec-ref> is an explicit, required positional argument (spec/<name>) resolved against the current checkout's .verdi/specs/active/<name>/deviation-report.md; the verb never fetches or switches branches, and never infers the target from the checked-out branch the way verdi align does (align's own header comment: no story/spec argument, matching 05 §CLI's table). This is a deliberate difference from align's own convention, chosen for the same reason verdi rollup, verdi matrix, and verdi verify-artifact all name their target explicitly rather than inferring it: a mutating, scriptable, audit-sensitive command should say in its own invocation exactly what it is about to change, not depend on background checkout state the operator must remember. Scope: this verb operates on deviation-report.md (build-branch reports, schema verdi.deviation/v1) only. decision-conflict-report.md (design-branch mode, schema verdi.decisionconflict/v1) carries a different, four-value disposition vocabulary (ConflictDisposition: superseded, exempt, rejected, no-conflict; decisionconflict.go is explicit this is not a reuse of FindingDisposition) and is out of scope for this story, matching the parent's own ac-3 text, a deviation-report finding; decision-conflict dispositions remain hand-edited until a future story takes them on, named here rather than silently folded in or silently dropped.", anchor: dc-4 }
  - { id: dc-5, text: "The verb never calls align.Compute, align.PreserveDispositions, or the judge; it is a pure, local, offline read-mutate-write over a report that already exists. A subsequent ordinary verdi align regeneration (non-freeze) still carries the verb-written disposition forward correctly through the existing Identity and PreserveDispositions machinery (internal/align/identity.go) unmodified, because Identity hashes only Kind, ID, and Text and deliberately excludes Disposition and Note; the verb's write is invisible to, and therefore never breaks, that carry-forward rule.", anchor: dc-5 }
constraints:
  - { id: co-1, text: "No network in any test (feature co-1): the verb is exercised entirely on fixture deviation-report.md files and fixturegit repos; no real judge, no real forge, no real upstream toolchain exec.", anchor: co-1 }
  - { id: co-2, text: "The verb never recomputes or rewrites digest:, integrity:, or judge_integrity:; it is not a producer of computed or judged content, align.ComputeDigest and the judge integrity hash are never invoked by this verb, it only ever narrows a write to the fields ac-1 names. This is the mechanical reason ac-1's digest-validity property holds, and it is the same discipline align.FreezeInPlace already applies to the same struct for the same reason.", anchor: co-2 }
  - { id: co-3, text: "A frozen deviation-report.md is immutable to every verb converging on it, this one included. No flag, including --amend, ever overrides ac-2's frozen-report refusal; the only way to change a frozen report is the one that already exists, there is none, freezing is terminal, per align.FreezeInPlace's own doc comment: a frozen report is immutable.", anchor: co-3 }
frozen: { at: 2026-07-16, commit: d2ecf50f0e6f8a3163692abce22fe55de7adf3c2, stub_matched: true }
---
# Disposition Verb

## Problem

Recording a reviewer's decision on a deviation-report finding has no verb.
Every round-6 disposition (D6-25) was a hand-edit of `deviation-report.md`'s
frontmatter — typing a `disposition:`/`note:` fragment into the `findings:`
block directly, with no validation before the write, no refusal for a
finding that does not exist or is already dispositioned, no protection
against editing an already-frozen report, and no guarantee the finding's
rendered line in the report's own markdown body stays in agreement with
what the frontmatter now says.

The disposition layer this story binds to (spec/closure-ergonomics dc-4) is
deliberately kept outside the report's integrity digest, so that recording
a decision can never invalidate it. But nothing today makes that property —
or any of the others a verb would give for free — true by construction
rather than by the hand-editor's care.

## Outcome

A new `verdi disposition` verb records a reviewer's decision — the finding,
the decision, and the rationale — into a deviation report's living
disposition layer, in place, leaving its digest and integrity untouched and
independently reverifiable. It keeps the report's human-legible body in
agreement with what it just wrote. It refuses, as named verdicts, every
unsafe request: an unknown finding, a re-disposition without an explicit
`--amend`, any write to an already-frozen report. And a disposition it
records survives `verdi align --freeze` byte-for-byte, through the
already-landed `FreezeInPlace` path (PR #99) and its D6-24 keep-genuine
guard (PR #101).

The previously de-facto hand-edit flow is retired from
`architecture-and-journeys.md`'s closure-ritual narrative: after this
story, that document names `verdi disposition` as the only sanctioned way
to record one.

## AC-1

`verdi disposition <spec-ref> <finding-id> <fixed|accepted-deviation>
--rationale <text>` writes exactly one disposition into `<spec-ref>`'s
LIVING `deviation-report.md` (a report carrying no `frozen:` stamp): the
named finding's frontmatter entry gains `disposition: <decision>` and
`note: <rationale>` (`internal/artifact`'s `Finding.Disposition`/`.Note`,
deviation.go), and its corresponding rendered line in the report's
markdown body is updated to the same decision and rationale, so the
human-legible prose and the machine record agree.

`<decision>` accepts only the two values the existing schema already
knows — `fixed` and `accepted-deviation` (`FindingDisposition`,
deviation.go) — the verb defines no new vocabulary. Every OTHER finding,
and the report's `digest:`, `integrity:`, and `judge_integrity:` fields,
are carried over byte-for-byte: `align.ComputeDigest` recomputed over the
same computed section still equals the stored digest value, and, when a
genuine judged exchange is present, `align.VerifyIntegrity` still
succeeds — the write is invisible to either verifier.

## AC-2

The verb refuses, each as a named verdict (exit 1) rather than a silent or
partial write, on:

- a `<finding-id>` absent from `<spec-ref>`'s current `findings:` list;
- a finding that already carries a disposition, unless `--amend` is given
  — and, symmetrically, `--amend` on a finding with no existing
  disposition, since there is nothing to amend;
- a `<spec-ref>` whose `deviation-report.md` already carries a `frozen:`
  stamp — a frozen report is immutable to every verb, this one included
  (mirroring `align.FreezeInPlace`'s own precondition, freeze.go).

A genuinely operational failure — no `deviation-report.md` at
`<spec-ref>`'s path at all, or one that fails strict decode — exits 2,
distinct from the three verdicts above.

## AC-3

A finding dispositioned by this verb, once every finding in the report is
dispositioned, survives `verdi align --freeze` byte-for-byte:
`align.FreezeInPlace` (PR #99) stamps the living, verb-written report
verbatim, proven end to end against the D6-24 keep-genuine fix's
drifting-judge harness (T.1, PR #101, mirroring
`TestRunAlign_FreezePreservesDispositions` in `cmd/verdi/align_test.go`)
so a judge run that would produce different content on a hypothetical
regeneration is shown to have no path into the frozen output.

## AC-4

After this story, `verdi/docs/architecture-and-journeys.md`'s
closure-ritual narrative — currently silent on how a finding gets
dispositioned ("every finding gets dispositioned", §D.5, names no
mechanism) — names `verdi disposition` as the only sanctioned way to
record one; no sentence in that document describes or implies
hand-editing a deviation report's disposition fields as an accepted
practice.

## DC-1

A new top-level verb, `verdi disposition <spec-ref> <finding-id>
<fixed|accepted-deviation> --rationale <text>`, matching the schema's own
`FindingDisposition` vocabulary (deviation.go) and mirroring `verdi
accept`'s shape: a small, single-purpose, spec-ref-scoped write verb. This
is the CLI-surface addition task 3.R's ratification PR must carry (05
§CLI's table names no `disposition` row today; `cmd/verdi/dispatch.go`'s
`verbPhase` names no `disposition` key).

Rejected alternatives:

- **A mode of `verdi align`** (`align --disposition ...`, mirroring dc-5's
  close-preflight precedent) — rejected because align's job is to
  regenerate (Compute plus judge) or freeze the whole report; grafting a
  single-finding mutate path onto the same verb conflates two different
  mechanics single responsibility keeps apart, and risks the exact
  regeneration hazard T.1 fixed if a future change ever wires the new
  flag through the regenerate path instead of a true surgical edit.
- **A subcommand** (`verdi align disposition ...`) — rejected because
  `align` is an established flag-based verb, not a subcommand dispatcher,
  and grafting subcommand syntax onto it would be an inconsistent shape
  within the same verb.
- **Folding it into `verdi close`** (`close --disposition ...`) —
  rejected because disposal happens mid-build, routinely long before
  close is ever attempted (the merge gate itself requires a
  fully-dispositioned report), misplacing the verb at the wrong point in
  the lifecycle purely because both concern closure eventually.
- **A write path on `verdi gate`** — rejected because gate is a
  read-only verdict verb over existing state and must never become a
  mutator.

This is a narrower reading of dc-5's verb-economy motive, not a departure
from its letter: dc-5's own text scopes its one-verb consolidation to
ac-1's preflight specifically ("No new verb is added for ac-1"); it says
nothing about ac-3's disposition-recording, which `close`'s own scope
(eligibility, freeze, rollup, archive-move on an already-fully-dispositioned
report) never touched even before this story — stretching dc-5's
preflight-specific consolidation onto a verb it never named would be the
actual overreach.

## DC-2

The verb's write mechanics mirror `align.FreezeInPlace`'s own discipline
exactly: value-copy, never mutate the decoded original; self-validate
before write; never fake success. Decode the report
(`artifact.DecodeDeviation`), copy it, set only the target finding's
Disposition and Note, `Validate` the copy, then re-render via
`align.RenderMarkdown` — deviation-report's own deterministic, hand-rolled
re-renderer and its established serialization path, never a generic
`yaml.Marshal`, never `internal/artifact/splice`, which is scoped to
spec.md only per its own package doc.

The verb additionally keeps the report's markdown body in agreement with
the frontmatter write: the target finding's rendered bullet line (the
disposition-labeled line `align.RenderBody`'s `renderFindings` produces)
is updated to the same decision and rationale, while every other line —
including the Boundary-diff and Diagram-alignment subsections the verb
has no data to regenerate (neither is stored in the frontmatter;
`align.FreezeInPlace`'s own doc comment is explicit that the digest's
inputs cannot be recomputed from a decoded report alone) — is carried
over byte-for-byte.

This is a judgment call flagged for review: the parent ac-3 speaks only
to the frontmatter disposition layer, but `internal/dex/storyaxis.go`'s
`renderQuartetDeviation` renders both a table built fresh from frontmatter
AND the report's raw body prose on the same archived page — a
frontmatter-only write would leave a permanently frozen, publicly
rendered record where a table cell reads `accepted-deviation` a few lines
above a bullet still reading `UNDISPOSITIONED` for the same finding.
Keeping the two in agreement is the smaller-surface way to avoid shipping
that self-contradiction, without requiring the verb to re-run Compute
(which needs the toolchain runner, not a local, offline, single-field
write) to regenerate sections it cannot safely touch.

This body update never touches digest- or integrity-covered content:
both hashes are computed over `(covers, finding id/kind/text, baseline
diffs)` and the persisted judge stdin/result bytes only
(`internal/align/verify.go`'s `digestInput` and `artifact.JudgeIntegrity`)
— the rendered body has never been, and is not made by this story to
become, an input to either. Grepping `internal/align` for any
hash/digest call reached from `RenderMarkdown`, `RenderBody`, or
`renderFindings` turns up none.

## DC-3

Exit discipline follows the parent's co-2 verbatim: every verb keeps 0
clean / 1 verdict / 2 operational, and unmet conditions are a verdict, not
an error. The three refusals ac-2 names are verdicts about the report's
own state (an unresolvable finding, a disposition collision, an immutable
frozen record), not environment failures, so each exits 1 and names the
offending finding id or state explicitly — never a silent absence.

`--amend` is required to change an EXISTING disposition and refused when
there is nothing to amend — a deliberate symmetry so the flag is never a
no-op rubber stamp, chosen over either always allowing silent overwrite
(risks silently discarding a prior reviewer's rationale) or never allowing
amendment at all (would force a return to hand-editing for the one case —
a reviewer's own mistake — a verb exists to remove). An amended
disposition fully replaces the prior decision and rationale; the
superseded text is not retained inside the artifact itself — ordinary git
history is the audit trail, exactly as it already is for a hand-edited
report today.

## DC-4

`<spec-ref>` is an explicit, required positional argument (`spec/<name>`)
resolved against the current checkout's
`.verdi/specs/active/<name>/deviation-report.md`; the verb never fetches
or switches branches, and never infers the target from the checked-out
branch the way `verdi align` does (align's own header comment: "no
story/spec argument, matching 05 §CLI's table"). This is a deliberate
difference from align's own convention, chosen for the same reason `verdi
rollup`, `verdi matrix`, and `verdi verify-artifact` all name their
target explicitly rather than inferring it: a mutating, scriptable,
audit-sensitive command should say in its own invocation exactly what it
is about to change, not depend on background checkout state the operator
must remember.

Scope: this verb operates on `deviation-report.md` (build-branch reports,
schema `verdi.deviation/v1`) only. `decision-conflict-report.md`
(design-branch mode, schema `verdi.decisionconflict/v1`) carries a
DIFFERENT, four-value disposition vocabulary (`ConflictDisposition`:
superseded/exempt/rejected/no-conflict; decisionconflict.go is explicit
this is not a reuse of `FindingDisposition`) and is out of scope for this
story, matching the parent's own ac-3 text ("a deviation-report finding")
— decision-conflict dispositions remain hand-edited until a future story
takes them on, named here rather than silently folded in or silently
dropped.

## DC-5

The verb never calls `align.Compute`, `align.PreserveDispositions`, or the
judge — it is a pure, local, offline read-mutate-write over a report that
already exists. A subsequent ordinary `verdi align` regeneration
(non-freeze) still carries the verb-written disposition forward correctly
through the existing `Identity`/`PreserveDispositions` machinery
(internal/align/identity.go) unmodified, because `Identity` hashes only
`(Kind, ID, Text)` and deliberately excludes `Disposition`/`Note` — the
verb's write is invisible to, and therefore never breaks, that
carry-forward rule.

## CO-1

No network in any test (feature co-1): the verb is exercised entirely on
fixture `deviation-report.md` files and fixturegit repos; no real judge,
no real forge, no real upstream toolchain exec.

## CO-2

The verb never recomputes or rewrites `digest:`, `integrity:`, or
`judge_integrity:` — it is not a producer of computed or judged content,
`align.ComputeDigest` and the judge integrity hash are never invoked by
this verb, it only ever narrows a write to the fields ac-1 names. This is
the mechanical reason ac-1's digest-validity property holds, and it is
the same discipline `align.FreezeInPlace` already applies to the same
struct for the same reason.

## CO-3

A frozen `deviation-report.md` is immutable to every verb converging on
it — this one included. No flag, including `--amend`, ever overrides
ac-2's frozen-report refusal; the only way to change a frozen report is
the one that already exists — there is none, freezing is terminal, per
`align.FreezeInPlace`'s own doc comment: "a frozen report is immutable."
