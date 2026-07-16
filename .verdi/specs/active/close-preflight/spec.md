---
id: spec/close-preflight
kind: spec
title: "Close Preflight"
owners: [platform-team]
class: story
status: draft
story: jira:VERDI-28
problem: { text: "Closing a story or feature has exactly one way to learn whether the closure gate holds: run the real, mutating `verdi close <ref>` and see what happens. When the gate fails, close does already print a per-condition PASS/FAIL breakdown (cmd/verdi/closuregate.go's runClosureGate, cmd/verdi/closuregatefeature.go's runFeatureClosureGate) - but for the single most common failure, an AC that is not yet evidenced, that breakdown collapses an entire story's worth of missing artifacts into one static line, \"story is not eligible (not every AC is evidenced or waived)\" (closuregate.go:91), or, for a feature, a bare per-AC status list with no path attached (closuregatefeature.go:148) - never the exact attestation path or derived-evidence directory the fold is actually reading. And if the gate WOULD have passed, there is no way to find that out without the real run proceeding straight through cutting a closure branch, freezing the alignment report, moving the whole quartet to specs/archive/, committing, and publishing the rollup to the configured tracker (close.go's own doc comment, steps 3-5) - an operator who only meant to check status has no way to stop that, and a publish to a real tracker is not undoable. Close is also gated behind a CI-only refusal (close.go:113-120) that exists solely to protect the publish step, but today blocks even a harmless status check from running locally without --force-local.", anchor: problem }
outcome: { text: "verdi close --preflight <ref> - a mode of the existing verb (dc-1/ADJ-23), never a new one - evaluates the exact same closure-gate conditions a real close would, for a story or a feature alike (dc-2, dc-3: the shared runClosureGate/runFeatureClosureGate functions), and stops there: it never cuts a branch, never freezes anything, never writes or publishes anything. Where the gate's existing per-condition breakdown is already itemized enough (spec-stale, pending-supersession, stub reconciliation, implementing-stories-closed), the preflight surfaces it unchanged; where it is not (an AC's own evidenced/pending/violated/no-signal status), the preflight renders the missing per-kind detail - the exact artifact and the exact path the fold reads for it - straight from the same fold result close's own gate already computes. It runs anywhere, in CI or on a plain laptop, without the publish guard in its way, because it never reaches the step that guard protects. Running it before a real close makes a subsequent close's refusal (closure-ergonomics co-3) something the operator already knew about, in detail, beforehand - never a surprise, and never a risk of an accidental real closure.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "For a named story or feature spec (dc-3), `verdi close --preflight <ref>` reports every condition a real `verdi close <ref>` would refuse on today - story: eligibility (every AC evidenced or waived over authoritative evidence), no unresolved spec-stale flag, no unresolved pending-supersession flag; feature: the same two plus every feature AC evidenced (incl. the outcome floor), stub reconciliation not blocked, every implementing story closed - using the identical evaluation functions close itself calls (dc-2), never a re-derived verdict. Each unmet AC names its missing evidence kind and the exact on-disk path the fold reads for it (dc-4, dc-7) - and for the attestation kind, distinguishes an absent attestation (no file at that path; remedy: scaffold it with `verdi attest`) from a scaffolded-but-unauthored one (a scaffold present at that exact path still carrying the unauthored sentinel; remedy: author the claim), naming the same exact path either way (dc-7, consuming spec/attest-helper dc-3's three-way attestation state; ADJ-33 seam mandate); spec-stale names its finding id(s) and the deviation-report.md path; pending-supersession names the touched object id(s) and open MR id(s) (dc-4: no local artifact applies there); stub-reconciliation names the unreconciled slug(s); the implementing-stories condition names the still-open story ref(s). Additionally (dc-1, closing judged-dcj-2 of this story's own design-mode judge sweep): whenever preflight runs outside a detected CI environment without --force-local, it also discloses, once and informationally, that a real close run under those same conditions would separately refuse at the CI-only/--force-local publish guard regardless of the gate verdict above - so a ready preflight is never mistaken for 'a real close will succeed right now, from this exact environment'.", evidence: [behavioral, attestation], anchor: ac-1 }
  - { id: ac-2, text: "Every `--preflight` run keeps the exit discipline byte-for-byte - 0 every applicable condition holds, 1 at least one is unmet (a verdict, never an error), 2 only a genuine operational failure (dc-5) - and in every one of the three outcomes nothing on disk changes and no external call with a side effect (a tracker publish) is ever made: `--preflight` is dispatched before close's CI-only/--force-local publish guard (close.go:113-120), not behind it, since that guard exists solely to protect a publish step preflight never reaches (dc-1).", evidence: [behavioral, attestation], anchor: ac-2 }
  - { id: ac-3, text: "The agreement property (closure-ergonomics co-3) is checkable, not aspirational: for each defect class named in ac-1, a fixture store constructed with exactly that one defect produces --preflight's matching disclosure AND a subsequent real `verdi close` on the byte-identical fixture refuses for exactly that same reason - proving the two cannot drift because they share the same evaluation functions (dc-2). A fixture with every condition satisfied reports ready (exit 0) and then closes successfully via a real, unmodified `verdi close` run on that same fixture.", evidence: [behavioral, attestation], anchor: ac-3 }
links:
  - { type: implements, ref: "spec/closure-ergonomics#ac-1" }
decisions:
  - { id: dc-1, text: "--preflight is a bare mode-selecting switch on the existing verdi close verb (verdi close --preflight <jira:STORY-KEY | spec/name>; closure-ergonomics dc-5/ADJ-23: no new verb, no 05 CLI inventory change), parsed the same order-independent way --force-local already is (close.go's cmdClose arg loop) - the ref stays the sole positional argument. --preflight is dispatched BEFORE the CI-only/--force-local publish guard (close.go:113-120), not conditioned by it: that guard exists solely to gate the publish step (04 Semantics: \"PublishRollup runs in CI only\"), and --preflight never reaches a publish call at all (ac-2), so subjecting it to the same refusal would make the verb's only side-effect-free, anywhere-runnable mode unusable from a plain local checkout without an unrelated escape hatch. This story's own design-mode judge sweep caught a real gap in that framing (judged-dcj-2, confidence 0.40): bypassing the guard entirely means a locally-run, --force-local-less preflight could report ready while a real close from that same checkout would still refuse AT the guard, which co-3 forbids being a surprise. Closed, not disclaimed: --preflight ALSO prints the guard's own condition (ac-1's added clause) whenever it is outside CI without --force-local, so the one refusal reason --preflight itself is exempt from reaching is still a refusal reason it names. A second, re-swept judge pass then caught a follow-on risk in that very fix (confidence 0.45): a hand-duplicated restatement of the guard's condition, printed from a second call site, would itself be the small second implementation dc-2 argues against, free to drift if the guard's own condition ever changes. Closed the same way dc-2 closes everything else: the disclosure's own check MUST be the identical boolean evaluation the guard performs (lint.ReadCIEnv().InCI and the --force-local flag, close.go:113-114) called directly, never a re-derived or hand-typed restatement of it - one predicate, two print sites, not two predicates.", anchor: dc-1 }
  - { id: dc-2, text: "Preflight and close share the SAME evaluation functions, never a second implementation: for a story, runClosureGate (closuregate.go); for a feature, runFeatureClosureGate (closuregatefeature.go) - the identical functions close.go:170-172/closefeature.go:91 already call as their own first gating step, and both are already pure with respect to the store (nothing on disk changes until AFTER they return ok=true; the mutating tail - branch cut, align freeze, archive, commit, publish - is entirely downstream of that return). --preflight calls the same function(s) and stops there. Where ac-1 requires more detail than these functions' own coarse Reason strings carry today (condition 1's eligibility: closuregate.go:91 collapses a whole story to one static line; closuregatefeature.go:148 lists per-AC status but still never a path), the preflight renders that detail directly from the SAME evidence.StoryResult/evidence.FeatureResult these functions already compute internally, never by re-deriving eligibility through any independent path - a rendering difference, not a second verdict-computing implementation. A third judge pass flagged this same render/refusal split on its own terms (confidence 0.30): parent dc-5's \"one code path\"/co-3-structural claim is upheld at the verdict layer but not at the rendering layer (the report is strictly richer text than a real refusal for the identical failure), and no supersedes/narrowing edge exists against closure-ergonomics#dc-5 for that gap (ADJ-26: no exempts edges this round) - flagged explicitly here, not merely asserted, for the seam review to adjudicate whether dc-5's wording was meant to bind renderings too. ADJ-33 adjudicated this (render-split): ACCEPTED - dc-5's one-code-path is structural verdict agreement via the shared evaluation functions, not a binding on rendering; richer preflight rendering satisfies co-3 by construction, and enriching close's own refusal text is optional build polish, not obligated. The tension carries no supersedes/exempts edge (ADJ-26 forbids them this round), so its align-sweep finding is hand-dispositioned no-conflict per ADJ-33. The preflight's scope is bounded to these gate conditions alone (03 Gates' closure gate; closure-ergonomics dc-1's byte-for-byte enumeration) - it does not attempt to predict a later ritual-step failure once the gate holds (align-freeze, the archive commit itself), which verdi align already lets an operator rehearse independently; folding ritual-step failures into ac-1 would blur \"the closure gate\" with \"every reason close could ever exit non-zero\", which closure-ergonomics dc-1 does not ask for. This story's own design-mode judge sweep flagged this narrowing as undeclared against parent dc-5's broader \"one verb owns the ritual and its rehearsal\" wording (judged-dcj-3, confidence 0.30, no supersedes/narrowing edge on closure-ergonomics#dc-5): correct, and left undeclared at the edge-graph level on purpose - ADJ-26 rules out exempts edges for this round (D6-27 is deferred, needing its own ratified spec), so this narrowing is disclosed here in prose, as a judgment call for the seam review to ratify or reject, rather than encoded as an edge the round does not permit. ADJ-33 ratified it (ritual-step boundary): ACCEPTED - preflight rehearses everything close REFUSES on (the gate conditions plus the disclosed CI publish guard); freeze/archive/publish are executions, not refusals, so bounding preflight to the gate is no genuine conflict with dc-5's rehearsal wording. Hand-dispositioned no-conflict per ADJ-33.", anchor: dc-2 }
  - { id: dc-3, text: "--preflight covers BOTH scopes, though ac-1's parent text (closure-ergonomics ac-1) names only \"a named story\": the feature five-condition AND (closure-ergonomics dc-1) is included because it rides the same shared enumeration cheaply - runFeatureClosureGate already exists, is already pure until its own caller mutates, and is already the exact function a real verdi close <feature-spec> calls first (closefeature.go:91). Covering it costs one more dispatch on the resolved spec's Class, mirroring runClose's own \"if spec.Class == artifact.ClassFeature\" branch (close.go:170) - not a second mechanism. Recorded here as a deliberate widening of ac-1's literal story-only framing, flagged for the seam review to ratify or narrow back to story-only. This story's own design-mode judge sweep caught exactly this tension (judged-dcj-1, confidence 0.55): the widening contradicts closure-ergonomics#ac-1's literal text with no supersedes/exempts edge declaring it. Correct, and left undeclared at the edge-graph level on purpose, not by oversight - ADJ-26 rules out exempts edges for this round (D6-27, the exempts-edge tiering question, is deferred to its own ratified spec), so a story cannot legally declare the edge the judge is asking for even if it wanted to; prose disclosure plus an explicit flag for seam adjudication is the correct, round-scoped substitute. ADJ-33 ratified the widening (feature-scope inclusion): ACCEPTED AND RATIFIED - parent ac-1's 'a named story' is the minimum, not a ceiling; the shared runFeatureClosureGate already exists and Phase 4's feature closes need co-3 as much as story closes. No genuine undeclared conflict with ac-1; its align-sweep finding is hand-dispositioned no-conflict per ADJ-33.", anchor: dc-3 }
  - { id: dc-4, text: "The preflight's disclosure granularity is exactly what the shared fold can see today, never invented further: AttestationExists (internal/evidence/attestations.go:19-32) is existence-only at the one correct path - a never-authored attestation and one authored at the wrong path both read as \"not found at the required path\", and naming that exact path is the fully actionable remedy either way. After spec/attest-helper lands its three-way attestation state (its dc-3, the state's owner: evidence.LoadAttestationState over the exported unauthored sentinel), the preflight reads THAT signal rather than the bare existence check, so a scaffold present at the correct path but still unauthored is named distinctly from a truly-absent attestation (dc-7, the ADJ-33 seam mandate) - the wrong-path case still folds as absent, unchanged. A mis-slugged attestation's own wrong-path problem is separately caught by the pre-existing verdi lint VL-011 rule (internal/lint/vl011.go:62-65), already run upstream of close by the ordinary lint-store gate; this story does not duplicate that check. Symmetrically, evidence records carry no distinct \"stale\" concept in the fold today (internal/evidence/records.go's ancestor filter, records.go:90-96, silently drops a non-ancestor record exactly like an absent one) - the preflight discloses the derived-tree root probed (.verdi/data/derived/<ref-slug>/, foldload.go:36) and, from data LoadRecordsWithSources already computes at no extra cost, names any found-but-excluded (non-ancestor) commit directory explicitly - the closest honest rendering of \"stale\" the current data model supports, added because it is free, not because it changes any verdict. A third judge pass raised this at low confidence (0.15) against a DIFFERENT parent decision, closure-ergonomics dc-3 (sync's ancestor-authority framing): considered and not acted on - dc-3 is scoped to verdi sync's bundle-fetch question, not to a diagnostic rendering this same decision's own text already says changes no verdict; stretching dc-3's authority framing to cover an unrelated disclosure is a category mismatch, disclosed here rather than silently dropped so the seam review can overrule it. ADJ-33 adjudicated this (excluded-record): REJECTED as a category mismatch - parent dc-3 is scoped to sync's bundle fetch, not to a zero-verdict diagnostic rendering - upholding this decision's own reading; its align-sweep finding is hand-dispositioned rejected per ADJ-33.", anchor: dc-4 }
  - { id: dc-5, text: "Preflight draws no new line between verdict (1) and operational (2): it inherits exactly the one the shared gate functions already draw. An absent artifact is always a verdict (e.g. Eligible=false, closuregate.go:88-91); a genuine I/O, decode, or transport error is always operational (any err these functions return propagates as exit 2). The easy-to-miss case: a nil forge (none configured/reachable) is not an error for pending-supersession - it is a disclosed-unproven condition that does not by itself fail the gate (closuregate.go:158-169, Disclosed: true) - while a forge that IS configured/reachable but genuinely errors when called is operational (closuregate.go:181-184, the LoadPendingSupersessionCandidates err != nil branch). Preflight preserves this exact three-way split (disclosed / verdict / operational); it never collapses \"no forge\" and \"forge errored\" into one case.", anchor: dc-5 }
  - { id: dc-6, text: "Feature-scope outcome-floor attestations key by the feature spec's own NAME (specRef.Name), never by store.RefSlug(spec.Story) the way story-scope attestations do (internal/evidence/featurefold.go:65-74's FeatureSlug doc comment; closefeature.go's foldFeature passing specRef.Name, not a story-slug helper) - the two slugs coincide only by accident. The build must use the same FeatureSlug convention for the feature-scope preflight's attestation-path disclosure, never the story-scope StorySlug helper - named explicitly here since it is the single easiest correctness mistake a fold-reusing implementation could make.", anchor: dc-6 }
  - { id: dc-7, text: "Attestation-unauthored disclosure - the cross-story seam with spec/attest-helper, mandated by ADJ-33's all-seven Wave A seam review. spec/attest-helper (its dc-3, the state's owner) introduces a three-way attestation state - absent (no file at the path the fold reads), unauthored-scaffold (a `verdi attest` scaffold present at that path but still carrying the fixed sentinel line `<!-- verdi:attestation-unauthored -->`, exported as evidence.UnauthoredAttestationMarker), and authored (file present, sentinel removed) - surfaced by a new evidence.LoadAttestationState that every fold call site switches to, counting ONLY authored as satisfied. attest-helper collapses unauthored to the SAME not-satisfied outcome absence already produces (its dc-3; parent closure-ergonomics dc-1: no new fold-visible pass path) and EXPLICITLY assigns the RENDERING of the absent-vs-unauthored distinction to THIS story (attest-helper dc-3/co-3: its deliverable is the SIGNAL, not any disclosure wording; the sibling close-preflight story is the surface that renders scaffolded-but-unauthored). So this story's ac-1 missing-evidence disclosure, for the attestation kind, tells two cases apart at the same exact path (dc-4; dc-6's FeatureSlug convention for the feature outcome floor): absent -> name the path and say scaffold it with `verdi attest`; unauthored-scaffold -> name the same path and say a scaffold is present but the claim is unauthored (sentinel present), author it. This changes NO verdict - it is exactly the disclosure-only, verdict-identical render-split ADJ-33 accepts for dc-2/dc-5 (the fold's pass/fail is structurally unchanged, unauthored == not-satisfied; only the human-facing reason gets more precise), so it introduces no new pass path and no undeclared conflict with closure-ergonomics dc-1. Build ordering (ADJ-33): the fold-state change - LoadAttestationState, the AttestationState type, the exported sentinel, and switching the existing fold call sites - is spec/attest-helper's to BUILD; this story CONSUMES that exported signal and owns only the preflight rendering of it. Both stories build in sub-wave B2 concurrently, so this story's build depends on attest-helper's LoadAttestationState landing; recorded here so the fold change and its disclosure land coherently across the two B2 builds.", anchor: dc-7 }
constraints:
  - { id: co-1, text: "No network in any test: every --preflight fixture is a fixturegit store (testdata/, deterministic); a forge is exercised only through the same httptest-backed fake double runClosureGate's own existing tests use (closuregate_test.go), including both directions of dc-5's disclosed-vs-operational forge split, never a live network call.", anchor: co-1 }
---
# Close Preflight

## Problem

Closing a story or feature has exactly one way to learn whether the closure
gate holds today: run the real, mutating `verdi close <ref>` and see what
happens. When the gate fails, close does already print a per-condition
PASS/FAIL breakdown (`runClosureGate`, cmd/verdi/closuregate.go; `verdi
close <feature-spec>`'s `runFeatureClosureGate`, closuregatefeature.go) —
but for the single most common failure, an AC that is not yet evidenced,
that breakdown collapses an entire story's worth of missing artifacts into
one static line, `"story is not eligible (not every AC is evidenced or
waived)"` (closuregate.go:91), or, for a feature, a bare per-AC status list
with no path attached (closuregatefeature.go:148) — never the exact
attestation path or derived-evidence directory the fold is actually
reading.

And if the gate would have passed, there is no way to find that out
without the real run proceeding straight through cutting a closure branch,
freezing the alignment report, moving the whole quartet to
`specs/archive/`, committing, and publishing the rollup to the configured
tracker (close.go's own doc comment, steps 3–5). An operator who only
meant to check status has no way to stop that, and a publish to a real
(non-fake) tracker is not undoable — nor is finding your own checkout
suddenly switched onto a fresh `close/<name>` branch you did not ask for.
Close is also gated behind a CI-only refusal (close.go:113-120) that exists
solely to protect the publish step, but today blocks even a harmless
status check from running locally without `--force-local`.

## Outcome

`verdi close --preflight <ref>` — a mode of the existing verb (dc-1;
closure-ergonomics dc-5/ADJ-23), never a new one — evaluates the exact same
closure-gate conditions a real close would, for a story or a feature alike
(dc-2, dc-3: the shared `runClosureGate`/`runFeatureClosureGate`
functions), and stops there: it never cuts a branch, never freezes
anything, never writes or publishes anything. Where the gate's existing
per-condition breakdown is already itemized enough (spec-stale,
pending-supersession, stub reconciliation, implementing-stories-closed),
the preflight surfaces it unchanged; where it is not (an AC's own
evidenced/pending/violated/no-signal status), the preflight renders the
missing per-kind detail — the exact artifact and the exact path the fold
reads for it — straight from the same fold result close's own gate already
computes.

It runs anywhere, in CI or on a plain laptop, without the publish guard in
its way, because it never reaches the step that guard protects. Running it
before a real close makes a subsequent close's refusal (closure-ergonomics
co-3) something the operator already knew about, in detail, beforehand —
never a surprise, and never a risk of an accidental real closure.

## AC-1

For a named story or feature spec (dc-3), `verdi close --preflight <ref>`
reports every condition a real `verdi close <ref>` would refuse on today.
For a story, that is the three closure-gate conditions (closuregate.go):
(1) story eligibility — every AC evidenced or waived, over authoritative
(`source: ci`) evidence only; (2) no unresolved spec-stale flag; (3) no
unresolved pending-supersession flag. For a feature, that is the
five-condition AND (closuregatefeature.go): the same (2)/(3) plus (1) every
feature AC evidenced, including the mandatory outcome floor; stub
reconciliation not blocked; every implementing story closed. Every
condition is evaluated through the identical functions close itself calls
(dc-2) — never a re-derived verdict that could disagree with the real one.

Per condition, the disclosure names the exact artifact and path wherever
one genuinely exists (dc-4): an unmet AC names its missing evidence kind(s)
and — attestation: `.verdi/attestations/<story-slug>/<ac-id>.md` (feature
outcome floor: `.verdi/attestations/<feature-spec-name>/<ac-id>.md`, dc-6), and for that attestation kind the disclosure
distinguishes an **absent** attestation (no file at that path — "scaffold it
with `verdi attest`") from a **scaffolded-but-unauthored** one (a `verdi
attest` scaffold sitting at that exact path but still carrying the unauthored
sentinel — "a scaffold is present but the claim is unauthored; author it"),
the same exact path named in both cases (dc-7, consuming spec/attest-helper
dc-3's three-way attestation state per the ADJ-33 seam mandate);
static/behavioral/runtime: the derived-tree root
`.verdi/data/derived/<ref-slug>/`, plus any found-but-excluded
(non-ancestor) commit directory named explicitly. Spec-stale names its
own-text finding id(s), the accepted-deviation count against its
threshold, and the `deviation-report.md` path. Pending-supersession names
the touched object id(s) and the open MR id(s) — no local artifact applies
there (dc-4), so no path is fabricated for it. Stub-reconciliation names
the unreconciled stub slug(s). The implementing-stories condition names the
still-open story ref(s).

Additionally (dc-1): whenever `--preflight` runs outside a detected CI
environment without `--force-local`, it also discloses — once,
informationally, never as a failing condition — that a real `close` run
under those same conditions would separately refuse at the CI-only/
`--force-local` publish guard, regardless of the gate verdict above. This
closes a gap this story's own design-mode judge sweep found
(judged-dcj-2): without it, a "ready" preflight run locally without
`--force-local` could be followed by a real close's surprise refusal at a
guard `--preflight` itself never needs to hit — but must still name, so a
ready verdict is never mistaken for "a real close will succeed right now,
from this exact environment."

Evidence: behavioral (a Go test drives the
preflight over one fixture per defect class and asserts the exact disclosure
text, artifact kind, and path) + attestation (the operator affirms, having
read the merged diff, that every named path is produced by the real
path-construction helpers — attestations.go, foldload.go, records.go — never
hand-typed or re-derived independently, and that no second eligibility
computation exists anywhere in the diff).

## AC-2

Every `verdi close --preflight <ref>` run keeps the exit discipline
byte-for-byte: 0 when every applicable condition holds (ready to close), 1
when at least one condition is unmet (a verdict, never an error), 2 only
for a genuine operational failure — an unresolvable ref, an unreadable
store or manifest, a decode error, or a configured-but-erroring forge
(dc-5) — never for a merely-absent artifact, which is always a verdict.
`--preflight` introduces no new distinction between verdict and
operational beyond the one close's own gate functions already draw (dc-5).

In every one of the three outcomes (ready, unmet, operational error),
nothing on disk changes: no branch is cut, no file is written, no commit is
made, and no external call with a side effect (a tracker publish) is ever
made. `--preflight` is dispatched before close's CI-only/`--force-local`
publish guard (close.go:113-120), not behind it — that guard exists solely
to protect a publish step preflight never reaches (dc-1), so a real
`--preflight` run needs neither a CI environment nor `--force-local` to
execute. Evidence: behavioral (a Go test exercises all three exit codes
over dedicated fixtures, plus a working-tree diff/`git status` assertion
proving zero mutation after each; a further test proves `--preflight`
succeeds outside any CI environment and without `--force-local`, and never
prints the force-local escape-hatch warning) + attestation (the operator
affirms, having read the merged diff, that no code path reachable under
`--preflight` performs a write — `os.WriteFile`, a git mutation, or a
provider publish call — under any fixture, including a ready one).

## AC-3

The agreement property (closure-ergonomics co-3) is checkable, not
aspirational. For each defect class named in AC-1 — an unmet AC per
evidence kind (no signal, pending, violated), a flagged spec-stale finding,
an open pending-supersession MR, an unreconciled stub, an open
implementing story — a fixture store constructed with exactly that one
defect produces `--preflight`'s matching disclosure, and a subsequent real
`verdi close` run on the byte-identical fixture refuses for exactly that
same reason, never a different one. This holds because the two share the
same evaluation functions (dc-2), so they cannot silently drift apart.

A fixture store with every condition satisfied reports ready (`--preflight`
exits 0) and then closes successfully via a real, unmodified `verdi close`
run on that same fixture (exit 0, the quartet archived). Evidence:
behavioral (one Go test per defect class runs BOTH halves of the pair in
the same test — `--preflight`'s disclosure, then a real `close` on the
identical fixture, asserting the refusal reason matches; a further test
runs the ready-fixture pair end to end through a real close) + attestation
(the operator affirms, having read the merged diff and the fixture test
file(s), that every pair is exercised end to end in one test rather than
asserted against two independently hand-maintained expectations that could
drift apart).

## DC-1

`--preflight` is a bare mode-selecting switch on the existing `verdi close`
verb — `verdi close --preflight <jira:STORY-KEY | spec/name>`
(closure-ergonomics dc-5/ADJ-23: no new verb, so no 05 §CLI inventory
change rides with this story) — parsed the same order-independent way
`--force-local` already is (close.go's `cmdClose` arg loop): the ref stays
the sole positional argument, unchanged from ordinary `close <ref>` usage.

`--preflight` is dispatched BEFORE the CI-only/`--force-local` publish
guard (close.go:113-120), not conditioned by it: that guard exists solely
to gate the publish step (04 §Semantics: "PublishRollup runs in CI only"),
and `--preflight` never reaches a publish call at all (AC-2), so subjecting
it to the same refusal would make the verb's only side-effect-free,
anywhere-runnable mode unusable from a plain local checkout without an
unrelated escape hatch — directly undermining the story's own value.

This story's own design-mode judge sweep caught a real gap in that framing
(judged-dcj-2, confidence 0.40): bypassing the guard entirely means a
locally-run, `--force-local`-less preflight could report ready while a real
close from that same checkout would still refuse AT the guard — which
closure-ergonomics co-3 forbids being a surprise. Closed, not disclaimed:
`--preflight` also prints the guard's own condition (AC-1's added clause)
whenever it runs outside CI without `--force-local`, so the one refusal
reason `--preflight` itself is exempt from reaching is still a refusal
reason it names.

A second, re-swept judge pass then caught a follow-on risk in that very
fix (confidence 0.45): a hand-duplicated restatement of the guard's
condition, printed from a second call site, would itself be the small
second implementation DC-2 argues against — free to drift if the guard's
own condition ever changes, since nothing structural would force the two
to agree. Closed the same way DC-2 closes everything else: the
disclosure's own check must be the identical boolean evaluation the guard
performs (`lint.ReadCIEnv().InCI` and the `--force-local` flag,
close.go:113-114) called directly, never a re-derived or hand-typed
restatement of it — one predicate, two print sites, not two predicates.

## DC-2

Preflight and close share the SAME evaluation functions, never a second
implementation: for a story, `runClosureGate` (closuregate.go); for a
feature, `runFeatureClosureGate` (closuregatefeature.go) — the identical
functions close.go:170-172/closefeature.go:91 already call as their own
first gating step. Both are already pure with respect to the store:
nothing on disk changes until AFTER they return `ok=true`; the mutating
tail (branch cut, align freeze, archive, commit, publish) is entirely
downstream of that return. `--preflight` calls the same function(s) and
stops there.

Where AC-1 requires more detail than these functions' own coarse `Reason`
strings carry today — condition 1's eligibility: closuregate.go:91
collapses a whole story to one static line
(`"story is not eligible (not every AC is evidenced or waived)"`);
closuregatefeature.go:148 lists per-AC status (e.g. `ac-1=pending`) but
still never a path — the preflight renders that detail directly from the
SAME `evidence.StoryResult`/`evidence.FeatureResult` these functions
already compute internally, never by re-deriving eligibility through any
independent path. This is a rendering difference, not a second
verdict-computing implementation, so the preflight's verdict structurally
cannot drift from what a real close would do.

A third judge pass flagged this same render/refusal split on its own
terms (confidence 0.30, distinct from judged-dcj-3 below): parent DC-5
states "the preflight report and close's refusal are one code path and
their required agreement (co-3) is structural" — under this decision the
shared path covers verdict computation only, so the report's TEXT and a
real refusal's text are guaranteed to differ in richness for the identical
failure. Fair, and — like the two tensions below — recorded explicitly
here as a flagged narrowing of DC-5's "one code path" wording, not merely
asserted and left implicit: co-3's structural guarantee is upheld at the
verdict layer (the pass/fail can never disagree, this decision's whole
point); it is NOT upheld at the rendering layer, and no supersedes/
narrowing edge against closure-ergonomics#dc-5 exists for that (ADJ-26:
no exempts edges this round). ADJ-33 adjudicated it
(render-split): **ACCEPTED** — DC-5's "one code path" binds structural
verdict agreement through the shared evaluation functions, not the rendering
layer; richer preflight rendering satisfies co-3 by construction, and
enriching close's own refusal text is optional build polish, not obligated.
No supersedes/exempts edge is added (ADJ-26 forbids them this round), so the
align sweep's finding for this tension is hand-dispositioned `no-conflict`
per ADJ-33.

The preflight's scope is bounded to these gate conditions alone (03
§Gates' closure gate; closure-ergonomics dc-1's byte-for-byte enumeration)
— it does not attempt to predict a later ritual-step failure once the gate
holds (align-freeze, the archive commit itself), which `verdi align`
already lets an operator rehearse independently. Folding ritual-step
failures into AC-1 would blur "the closure gate" with "every reason close
could ever exit non-zero", which closure-ergonomics dc-1 does not ask for
— disclosed here as a bounded, deliberate exclusion, not an oversight.

This story's own design-mode judge sweep flagged this narrowing as
undeclared against the parent's broader wording (judged-dcj-3, confidence
0.30): parent dc-5 frames the mode as "one verb owns the ritual and its
rehearsal", and if "rehearsal" is read to cover the whole ritual rather
than only the gate, this decision narrows it with no supersedes/narrowing
edge against closure-ergonomics#dc-5. Correct, and left undeclared at the
edge-graph level on purpose, not by oversight: ADJ-26 rules out exempts
edges for this round (D6-27, the exempts-edge tiering question, is
deferred to its own ratified spec), so this narrowing is disclosed here in
prose, as a judgment call for the seam review to ratify or reject, rather
than encoded as an edge this round does not permit. ADJ-33 ratified it
(ritual-step boundary): **ACCEPTED** — preflight rehearses exactly what close
*refuses* on (the gate conditions plus the disclosed CI publish guard);
freeze, archive, and publish are executions, not refusals, so bounding
preflight to the gate is no genuine conflict with dc-5's "one verb owns the
ritual and its rehearsal." The align sweep's finding for this tension is
hand-dispositioned `no-conflict` per ADJ-33.

## DC-3

`--preflight` covers BOTH scopes, though AC-1's parent text
(closure-ergonomics ac-1) names only "a named story": the feature
five-condition AND (closure-ergonomics dc-1) is included because it rides
the same shared enumeration cheaply. `runFeatureClosureGate` already
exists, is already pure until its own caller mutates, and is already the
exact function a real `verdi close <feature-spec>` calls first
(closefeature.go:91). Covering it costs one more dispatch on the resolved
spec's `Class`, mirroring `runClose`'s own
`if spec.Class == artifact.ClassFeature` branch (close.go:170) — not a
second mechanism.

Recorded here as a deliberate widening of AC-1's literal story-only
framing (the architect's own guidance for this story: include it if it
rides the shared enumeration cheaply, else disclose it as a bounded
residual) — flagged prominently for the seam review to ratify, or narrow
back to story-only, before build.

This story's own design-mode judge sweep caught exactly this tension
(judged-dcj-1, confidence 0.55): the widening contradicts
closure-ergonomics#ac-1's literal "a named story" text, and no
supersedes/exempts edge against that AC declares the widening in the edge
graph. Correct, and left undeclared at the edge-graph level on purpose,
not by oversight: ADJ-26 rules out exempts edges for this round (D6-27,
the exempts-edge tiering question, is deferred to its own ratified spec)
— a story cannot legally declare the edge the judge is asking for even if
it wanted to, so prose disclosure plus an explicit flag for seam
adjudication (above) is the correct, round-scoped substitute, not a
shortcut. ADJ-33 ratified the widening (feature-scope inclusion): **ACCEPTED
AND RATIFIED** — parent ac-1's "a named story" is the minimum, not a ceiling;
the shared `runFeatureClosureGate` already exists and Phase 4's feature closes
need co-3 as much as story closes. The align sweep's finding for this tension
is hand-dispositioned `no-conflict` per ADJ-33.

## DC-4

The preflight's disclosure granularity is exactly what the shared fold can
see today, never invented further. `AttestationExists`
(internal/evidence/attestations.go:19-32) is existence-only at the one
correct path — a never-authored attestation and one authored at the wrong
path both read as "not found at the required path", and naming that exact
path is the fully actionable remedy either way: put a file there. A
mis-slugged attestation's own wrong-path problem is separately caught by
the pre-existing `verdi lint` VL-011 rule (internal/lint/vl011.go:62-65),
already run upstream of close by the ordinary lint-store gate in CI; this
story does not duplicate that check inside the fold.

Once spec/attest-helper lands its three-way attestation state (its dc-3, the
state's owner — `evidence.LoadAttestationState` reading the exported
unauthored sentinel `<!-- verdi:attestation-unauthored -->`), the preflight
reads *that* signal in place of the bare existence check, so a scaffold
sitting at the correct path but still unauthored is disclosed distinctly from
a truly-absent attestation (dc-7, the ADJ-33 seam mandate) — the same exact
path named either way, and the wrong-path case still folding as absent,
unchanged.

Symmetrically, evidence records carry no distinct "stale" concept in the
fold today: `internal/evidence/records.go`'s ancestor filter
(records.go:90-96) silently drops a record whose commit is not the
resolved commit or a real ancestor of it, exactly like an absent record.
The preflight discloses the derived-tree root probed
(`.verdi/data/derived/<ref-slug>/`, foldload.go:36) and, from data
`LoadRecordsWithSources` already computes at no extra cost, names any
found-but-excluded (non-ancestor) commit directory explicitly — the
closest honest rendering of "stale" the current data model supports, added
because it is free (the manifest is already computed and today only
discarded), not because it changes any verdict.

A third judge pass raised this at low confidence (0.15) against a
DIFFERENT parent decision, closure-ergonomics dc-3 ("where the two
surfaces would disagree, the fold's rule is authoritative and sync
conforms to it"), reading naming an excluded commit as a second,
undeclared account of the fold's resolution rule. Considered and not
acted on: dc-3 is scoped to `verdi sync`'s bundle-fetch/ancestor-walk
question (a different surface entirely — fetching bundles from a forge),
not to a diagnostic rendering that this same decision's own text already
states changes no verdict; stretching dc-3's single-authority framing to
cover an unrelated disclosure is, on inspection, a category mismatch
rather than a real tension. Disclosed rather than silently dropped, so
the seam review can overrule this reading if it disagrees. ADJ-33 adjudicated
it (excluded-record): **REJECTED** as a category mismatch — parent dc-3 is
scoped to `verdi sync`'s bundle fetch, not to a zero-verdict diagnostic
rendering — upholding this decision's own reading. The align sweep's finding
for this tension is hand-dispositioned `rejected` per ADJ-33.

## DC-5

Preflight draws no new line between verdict (1) and operational (2): it
inherits exactly the one the shared gate functions already draw. An absent
artifact is always a verdict (e.g. `Eligible=false`, closuregate.go:88-91);
a genuine I/O, decode, or transport error is always operational (any `err`
these functions return propagates as exit 2, mirroring close.go's own
`if err != nil { return 2 }` pattern throughout).

The easy-to-miss case: a nil forge (none configured/reachable) is NOT an
error for pending-supersession — it is a disclosed-unproven condition that
does not by itself fail the gate (closuregate.go:158-169, `Disclosed:
true`) — while a forge that IS configured/reachable but genuinely errors
when called is operational (closuregate.go:181-184, the
`LoadPendingSupersessionCandidates` `err != nil` branch). Preflight
preserves this exact three-way split (disclosed / verdict / operational);
it never collapses "no forge" and "forge errored" into one case.

## DC-6

Feature-scope outcome-floor attestations key by the feature spec's own
NAME (`specRef.Name`), never by `store.RefSlug(spec.Story)` the way
story-scope attestations do (internal/evidence/featurefold.go:65-74's
`FeatureSlug` doc comment; closefeature.go's `foldFeature` passing
`specRef.Name`, not a story-slug helper) — the two slugs coincide only by
accident (a feature whose `story:` field happens to slug identically to
its own spec name).

The build must use the same `FeatureSlug` convention for the feature-scope
preflight's attestation-path disclosure, never the story-scope `StorySlug`
helper — named explicitly here since it is the single easiest correctness
mistake a fold-reusing implementation could make, and a wrong path in a
disclosure is exactly the kind of defect this story exists to eliminate.

## DC-7

Attestation-unauthored disclosure — the cross-story seam with
spec/attest-helper, mandated by ADJ-33's all-seven Wave A seam review.

spec/attest-helper (its **dc-3**, the state's owner) introduces a three-way
attestation state, replacing the fold's existence-only read:

- **absent** — no file at the path the fold reads;
- **unauthored-scaffold** — a `verdi attest` scaffold sits at that path but
  still carries the fixed sentinel line `<!-- verdi:attestation-unauthored -->`
  (exported once as `evidence.UnauthoredAttestationMarker`);
- **authored** — the file is present and the sentinel is gone.

The signal is surfaced by a new `evidence.LoadAttestationState`, which every
fold call site switches to, counting **only** `authored` as satisfied.
attest-helper collapses `unauthored` to the *same* not-satisfied outcome
absence already produces (its dc-3; parent closure-ergonomics dc-1: no new
fold-visible pass path) and **explicitly assigns the rendering** of the
absent-vs-unauthored distinction to this story (attest-helper dc-3/co-3: its
deliverable is the SIGNAL, not any disclosure wording; the sibling
close-preflight story is the surface that renders scaffolded-but-unauthored).

So this story's AC-1 missing-evidence disclosure, for the attestation kind,
tells two cases apart at the *same exact path* (dc-4; dc-6's `FeatureSlug`
convention for the feature outcome floor):

- **absent** → name the path and say "scaffold it with `verdi attest`";
- **unauthored-scaffold** → name the same path and say "a scaffold is present
  but the claim is unauthored (sentinel present) — author it".

This changes **no verdict**: it is exactly the disclosure-only,
verdict-identical render-split ADJ-33 accepts for dc-2/dc-5 above (the fold's
pass/fail is structurally unchanged — `unauthored == not-satisfied` — only the
human-facing reason gets more precise), so it introduces no new pass path and
no undeclared conflict with closure-ergonomics dc-1.

**Build ordering (ADJ-33):** the fold-state change — `LoadAttestationState`,
the `AttestationState` type, the exported sentinel, and switching the existing
fold call sites — is spec/attest-helper's to *build*; this story *consumes*
that exported signal and owns only the preflight rendering of it. Both stories
build in sub-wave B2 concurrently, so this story's build depends on
attest-helper's `LoadAttestationState` landing; recorded here so the fold
change and its disclosure land coherently across the two B2 builds.

## CO-1

No network in any test: every `--preflight` fixture is a fixturegit store
(testdata/, deterministic); a forge is exercised only through the same
httptest-backed fake double `runClosureGate`'s own existing tests use
(closuregate_test.go), including both directions of DC-5's
disclosed-vs-operational forge split, never a live network call.
