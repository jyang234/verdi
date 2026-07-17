---
id: spec/attest-helper
kind: spec
title: "Attest Helper"
owners: [platform-team]
class: story
status: closed
story: jira:VERDI-29
problem: { text: "attest-helper is the mechanical half of spec/closure-ergonomics ac-2: today an operator authors an attestation entirely by hand — inventing the path, inventing the slug, inventing the frontmatter — and one wrong slug (the story-ref RefSlug, not the spec name; D6-16/D6-18's own witness, corrected once already in 08-revision-notes.md's round-6 entry) silently folds as `absent`, indistinguishable from never having attested at all. There is also no tooling boundary today preventing a future helper from fabricating the claim itself, which dc-2 of the parent feature forbids outright: verdi may write structure, never the human's word.", anchor: problem }
outcome: { text: "a new top-level verb, `verdi attest <story-ref> <ac-id>`, scaffolds a correctly-slugged, correctly-placed attestation skeleton — frontmatter, a `verifies` edge, and an explicit, machine-checkable unauthored marker in place of a claim — and refuses outright rather than overwrite an existing human record or scaffold a nonexistent (story, AC) pair. A companion lint rule (VL-022, the next free rule number this story's own research found) makes a misfiled attestation a named, witness-carrying refusal instead of a silent fold-time `absent`. The scaffold is provably not-yet-evidence: the fold treats an unauthored scaffold exactly as it treats a missing file, until the operator replaces the marker with their own claim.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "given a (story-ref, ac-id) naming a declared acceptance criterion of an active class: story spec, `verdi attest` writes a new attestation file at the exact slugged path the fold reads, with a strict-decodable frontmatter (id, kind, title, owners, schema, a bare verifies edge to the target spec, a frozen stamp) and a body whose entire content is the fixed unauthored marker plus instructional prose that names the (story, AC) and tells the operator the pre-filled frozen stamp is a convenience they update to the commit actually verified against (dc-2, ADJ-30) — the claim itself is never generated, defaulted, or templated with claim-shaped prose", evidence: [static, behavioral, attestation], anchor: ac-1 }
  - { id: ac-2, text: "the verb refuses with the verdict discipline (exit 1) in exactly two cases — the (story, AC) pair does not exist (unresolvable story-ref, or a resolved spec that is not class: story, or an undeclared ac-id), or an attestation already exists at the exact path the fold reads — using an atomic create-only write so a file appearing between the check and the write is never silently overwritten; every other failure is operational (exit 2)", evidence: [static, behavioral, attestation], anchor: ac-2 }
  - { id: ac-3, text: "a new named lint rule, VL-022, refuses (never a silent absent at fold time) any attestation whose verifies edge does not resolve to the (story, AC) implied by its own on-disk path and slug — unresolvable target, wrong class, undeclared AC, or a story-ref-slug/path disagreement are each named with the offending value; scoped to attestations that carry a verifies edge at all (dc-4 pins why)", evidence: [static, attestation], anchor: ac-3 }
  - { id: ac-4, text: "the scaffold ac-1 writes round-trips strict decode and validate at the exact path the fold reads, before any claim is ever authored — the verb self-checks the bytes it is about to write and refuses with an internal operational error rather than ever leaving a malformed file on disk", evidence: [static, behavioral, attestation], anchor: ac-4 }
links:
  - { type: implements, ref: "spec/closure-ergonomics#ac-2" }
decisions:
  - { id: dc-1, text: "verb spelling and its ratification dependency: a NEW top-level verb, `verdi attest <story-ref> <ac-id>` (two positional arguments, no flags), recommended over a mode of an existing verb — design/board scaffold OTHER kinds and neither reads as 'write an attestation'; matrix/rollup are read-only. This is exactly the case the plan's global constraints flag ('any new verb requires the ratification task, 3.R'): landing this story requires one ratification PR moving cmd/verdi/dispatch.go's verbPhase map, docs/design/specs/05-surfaces.md §CLI's table, and internal/specalign/verbs_test.go's inV0 inventory together, plus a 08-revision-notes.md entry (mirroring the dc-7/PR #51 precedent). Judgment call, flagged for seam review: `verdi board attest` was considered and rejected — board's one existing subcommand, commit, is spec-editing, and attest neither edits a spec nor needs a board/server context; a bare top-level verb matches close/gate/audit's own precedent better than nesting under an unrelated one", anchor: dc-1 }
  - { id: dc-2, text: "scaffold content and write discipline: frontmatter is exactly id: attestation/<storySlug>--<acID> (I-6), kind: attestation, a mechanically-derived identifier-shaped title (never claim-shaped prose), owners copied verbatim from the resolved story spec (structure, never invented, never [unassigned]), schema: verdi.attestation/v1, links: [{ type: verifies, ref: spec/<resolved-name> }] (a bare ref — 02 §Link taxonomy's closed edge vocabulary permits nothing else on a non-implements/resolves/supersedes/exempts/depends-on edge), and frozen: { at: <today>, commit: <HEAD at scaffold time> }. Judgment call, flagged for seam review: attestations carry no draft state (frozen is unconditionally required), so there is no schema-legal way to write a strict-decodable, unauthored attestation without SOME frozen stamp; this pins the only available choice (HEAD-at-scaffold-time, exactly how accept and stub-instantiate already stamp their own scaffolds) over inventing a new draft state, which would be an out-of-scope 02 §Kind registry change. Per ADJ-30, the pre-filled frozen stamp is a CONVENIENCE matching the store's existing attestation convention — it names the tree the claim was verified against (witness: attestation/disclosure-legibility--ac-1 stamps 6a3465b, a commit predating the file itself), never the file's own commit — and the OPERATOR updates frozen.commit to the commit actually verified against when authoring the claim; it is part of the human record, legally mutable until first commit (VL-010 binds only committed frozen artifacts), and the scaffold's instructional prose says so explicitly. The verb writes ONLY this one file and commits nothing (unlike design start/stub-instantiate, which commit immediately): an attestation is authored once before its first commit, not a multi-commit design surface", anchor: dc-2 }
  - { id: dc-3, text: "the unauthored-marker mechanism: a single fixed sentinel line, an HTML comment invisible under markdown rendering and vanishingly unlikely to collide with genuine claim prose, defined once as an exported constant so the scaffold writer and every fold reader share one literal. Detection is a raw substring check over the whole file (no frontmatter/body split needed): present => unauthored; absent with the file existing => authored; no file => absent. This three-way state is exposed by a new function every current consumer of the existing existence-only check must switch to, treating ONLY the authored state as satisfied (parent dc-2: not foldable until authored) — the existing existence-only function is untouched for any caller that genuinely only needs raw existence. Three call sites this story's own research found currently read pure existence as attested and must all move together, or an unauthored scaffold would fold as evidenced at whichever one is left behind while correctly reading as absent everywhere else: the story fold, the feature outcome fold, and the board's empty-slot badge. The unauthored state is NOT a new fold-visible status (parent dc-1: gate semantics unchanged, no new pass path) — it collapses to exactly the same not-satisfied outcome absence already produces; the only difference is disclosure, which a caller (the sibling close-preflight story, cross-story seam, co-3) can render more precisely than an undifferentiated absent", anchor: dc-3 }
  - { id: dc-4, text: "VL-022's scope and the grandfather question: the rule fires ONLY on attestations that carry a verifies link, never on one that omits it entirely. Deliberate, disclosed scope limit: every attestation in the store as of this contract predates this feature and carries no verifies edge at all, including files 08-revision-notes.md's round-6 closure-status-flip entry describes as deliberately preserved, immutable, mis-slugged historical record — kept precisely because a frozen artifact must not be touched. A rule resolving the claim by any OTHER means (e.g. reverse-searching the corpus for a matching RefSlug) would need an enumerated grandfather-baseline map to avoid newly failing every pre-existing attestation on its first run, exactly the problem VL-020's own baseline map solved one rule number ago, the harder way. Gating on verifies-presence needs no such map: every file this rule ever examines is one the sanctioned helper wrote, so every pre-existing, hand-authored attestation is out of scope by construction. The residual gap — hand-authoring or hand-moving an attestation without the helper and without a verifies edge of its own — is disclosed, not silently accepted: such a file already bypasses every guardrail this story adds, and catching a mis-slug with no self-declared claim to check against is a strictly harder problem this story does not attempt. Mirrors VL-021's own 'optional field, checked only when present' precedent exactly", anchor: dc-4 }
  - { id: dc-5, text: "exit discipline and scope boundary: verdict (exit 1) for both ac-2 refusal cases, deliberately grouping 'story-ref does not resolve' under the same verdict-not-operational treatment as 'ac-id undeclared' even though the shared resolution seam this verb reuses is treated as operational (exit 2) by its other caller. Disclosed divergence, not an oversight: that other caller is a pure reporting verb for which an unresolvable ref is a usage error, while co-2 names the attest scaffold directly, in the same sentence as preflight's own verdict-not-error posture — a nonexistent (story, AC) pair is exactly the kind of meaningful, expected 'no' co-2 already treats as a verdict on this surface. Scope boundary: targets STORY attestations only — a resolved spec of any other class is refused under the 'pair does not exist' verdict. Feature-level outcome-attestations (hand-authored today) are an explicit non-goal: broadening the helper to scaffold them is a reasonable future extension, not required by the parent ac-2 text this story implements, which is framed as '(story, AC)' throughout", anchor: dc-5 }
constraints:
  - { id: co-1, text: "no network in any test: every test this story adds is hermetic — fixturegit-backed for the verb, in-package Snapshot fixtures for the lint rule — mirroring the existing test harnesses these packages already use", anchor: co-1 }
  - { id: co-2, text: "exit discipline (0 clean / 1 verdict / 2 operational, dc-5's exact mapping) and mutation scope: the verb writes exactly one file to the working tree and commits nothing; it never touches any other file, git ref, or index entry", anchor: co-2 }
  - { id: co-3, text: "cross-story seam boundary, disclosed rather than assumed away: this story's deliverable is the SIGNAL (the three-way attestation state), not any disclosure wording. The sibling close-preflight story is the surface that renders 'scaffolded but unauthored' to an operator; this story guarantees only that the signal exists, is correct, and is exported for that story to consume", anchor: co-3 }
frozen: { at: 2026-07-16, commit: e606a109dbc28ea08cc86265c4fa2dd026f8373a, stub_matched: true }
---
# Attest Helper

## Problem

Closing a story is the most manual, error-prone stretch of the lifecycle
(spec/closure-ergonomics), and attestation authoring is its least
tool-assisted corner. An operator hand-invents the path
(`.verdi/attestations/<slug>/<ac-id>.md`), hand-invents the slug (which must
be `RefSlug` of the story's own tracker ref, not the spec's directory name —
a distinction with no tooling to enforce it), and hand-writes the
frontmatter. One wrong slug does not error: the fold's `AttestationExists`
is a bare `os.Stat` at the path it expects, so a misfiled attestation simply
reads as `absent` — indistinguishable from an operator who never attested
at all (D6-16, D6-18; corrected once already for `remote-and-ci`'s own
attestations, per `08-revision-notes.md`'s round-6 entry, by adding
correctly-slugged files alongside the frozen, immutable, mis-slugged
originals).

Nothing about today's manual flow is dangerous by itself — but the moment a
*helper* verb exists to make this easier, a new risk appears: a helper that
is even slightly too generous (templating a plausible-sounding claim,
defaulting a body, "helpfully" filling in prose) would silently fabricate a
human oracle's record, exactly what spec/closure-ergonomics dc-2 forbids.
This story is the mechanical half of that feature's ac-2: scaffold the
structure, refuse to scaffold nonsense, and make misplacement a loud,
named failure instead of a quiet one.

## Outcome

A new top-level verb, `verdi attest <story-ref> <ac-id>`, given a (story,
AC) pair, writes a correctly-slugged, correctly-placed attestation skeleton
— frontmatter complete except for the claim, which is left in an
explicit, machine-recognizable **unauthored** state. The verb refuses
outright, never silently degrading, when the pair does not exist or an
attestation already exists at that path. A new lint rule, VL-022, closes
the enforcement gap D6-18 exposed: an attestation whose own declared
target disagrees with where it actually lives is now a named refusal, not
a silent fold-time `absent`. And the scaffold's unauthored state is wired
into the fold itself, so a not-yet-authored scaffold can never be mistaken
for real evidence.

## AC-1

Given a (story-ref, ac-id) naming a declared acceptance criterion of an
active `class: story` spec, `verdi attest <story-ref> <ac-id>` writes a new
file at `.verdi/attestations/<RefSlug(story.Story)>/<ac-id>.md` — the exact
path `internal/evidence`'s fold already reads (I-6/I-31) — with this exact
frontmatter shape:

```yaml
---
id: attestation/<storySlug>--<acID>
kind: attestation
title: "unauthored attestation scaffold: <story-ref> <ac-id>"
owners: [<copied verbatim from the resolved story spec's own owners:>]
schema: verdi.attestation/v1
links:
  - { type: verifies, ref: "spec/<resolved-spec-name>" }
frozen: { at: <today, YYYY-MM-DD>, commit: <git HEAD at scaffold time, full sha> }
---
```

and a body whose entire content is the fixed unauthored marker (dc-3)
followed by instructional prose, e.g.:

```
<!-- verdi:attestation-unauthored -->
This attestation was scaffolded by `verdi attest` for <story-ref> <ac-id>
and has not been authored. Replace this entire paragraph, and delete the
marker comment above, with your own first-person account of what you
verified, how, and why this acceptance criterion is satisfied. Until the
marker above is removed, this file folds as absent, with disclosure — it
is not evidence of anything.

The `frozen.commit` stamped above is a convenience: it was pre-filled with
the repository HEAD when this scaffold was written. By the store's
attestation convention that field names the tree your claim was verified
against — not this file's own commit — so set it to the exact commit you
actually reviewed when you author your claim. The stamp is yours to
correct: nothing here is frozen until this file's first commit (VL-010
binds only committed frozen artifacts), so updating it in this same
authoring pass is always legitimate.
```

Title and owners are structure copied or mechanically derived from
identifiers already on hand — never claim-shaped prose (parent dc-2): the
verb states WHAT the file is for and WHO owns it, never WHAT WAS VERIFIED.
Static register: `internal/evidence/attestations_test.go` table-driven
tests over the scaffold-rendering function (frontmatter shape, marker
presence, owners/title derivation) and the marker constant/detection
function. Behavioral register: `cmd/verdi/attest_test.go`,
`TestRunAttest_Happy`, driving the verb's testable core against a
fixturegit-backed store fixture and asserting the written file's exact
path, frontmatter fields, and body marker.

## AC-2

The verb refuses with the verdict discipline (exit 1, not exit 0, not a
partial write) in exactly two cases:

- **the (story, AC) pair does not exist**: `<story-ref>` does not resolve
  via the shared two-form contract (I-30, `internal/storyresolve.Resolve`)
  to an active spec at all; the resolved spec's `class` is not `story`
  (dc-5's scope boundary); or the resolved spec's `acceptance_criteria`
  does not declare `<ac-id>`.
- **the attestation already exists**: a file already sits at the exact
  path the fold reads for this (story, AC) — dc-2's "never overwrite a
  human record" made mechanical, and mechanically race-safe: the write
  uses `O_CREATE|O_EXCL` (mirroring the writer-lock's own atomic idiom,
  I-12), so a file that appears between the pre-check and the write is
  caught by the OS, never silently clobbered.

Every other failure — no store root, an unreadable or malformed spec file,
a git or filesystem error — is operational (exit 2), per co-2. Static
register: table-driven unit tests over the pair-existence check
(unresolvable ref, wrong class, undeclared AC) and the exists-check.
Behavioral register: `cmd/verdi/attest_test.go`,
`TestRunAttest_RefusesUnknownStoryRef`,
`TestRunAttest_RefusesWrongClass`, `TestRunAttest_RefusesUndeclaredAC`,
`TestRunAttest_RefusesAlreadyExists` — each asserting exit 1 and that the
working tree is byte-for-byte unchanged.

## AC-3

VL-022 (`internal/lint/vl022.go`) refuses any attestation-kind document
that carries a `links` entry of type `verifies` (dc-4 pins why the rule's
scope stops exactly there) whose target does not resolve to the (story,
AC) implied by the attestation's own on-disk path and compound id. Mirrors
`vl019.go`'s own `badVerifiesTarget` pattern (an obligation's twin check),
extended with the one genuinely new piece attestations need that
obligations don't: a slug-derivation step, since an attestation's path
segment is `RefSlug(target.Story)` rather than the target's own directory
name. Refused, each naming the offending value:

- the `verifies` ref does not parse, carries a fragment, or does not
  resolve to a spec in the committed zone;
- the resolved target's `class` is not `story`;
- the target's `acceptance_criteria` does not declare the AC named by the
  attestation's own id/path;
- `store.RefSlug(target.Story)` does not equal the attestation's own
  directory segment — the D6-18 class of bug (a spec-name slug substituted
  for the story-ref slug, or vice versa) made a witness-carrying refusal
  instead of a silent `absent` at fold time.

Static register: `internal/lint/vl022_test.go`, table-driven, including a
misplaced fixture (a `verifies` edge whose target's `RefSlug(story)`
disagrees with the fixture's own directory) alongside clean/well-slugged
and no-`verifies`-edge (out-of-scope, no finding) cases — mirroring
`vl019_test.go`/`vl021_test.go`'s own fixture-corpus style. No behavioral
(e2e/built-binary) register: mirrors VL-019/VL-020/VL-021's own precedent
of package-level-only coverage for a lint rule.

## AC-4

The scaffold AC-1 writes round-trips: read back from the exact path the
fold reads, it strict-decodes and validates cleanly as `kind: attestation`
frontmatter (`internal/artifact.DecodeAttestation`) — before any claim is
ever authored, i.e. while the unauthored marker is still present. The verb
self-checks the exact bytes it is about to write (mirroring `design
start`'s and stub-instantiate's own pre-write self-validation, CLAUDE.md:
"never fake success") and refuses with an internal-error operational exit
(2) rather than ever leaving a malformed file on disk. Static register:
a unit test asserting the scaffold-rendering function's output always
self-validates. Behavioral register: `cmd/verdi/attest_test.go`,
`TestRunAttest_ScaffoldRoundTrips` — writes a scaffold, reads the file back
from disk at the fold's own expected path, and asserts
`artifact.DecodeAttestation` succeeds against it byte-for-byte.

## DC-1

Verb spelling and its ratification dependency. This contract recommends a
NEW top-level verb, `verdi attest <story-ref> <ac-id>` (two positional
arguments, no flags), over a mode of an existing verb: `design`/`board`
scaffold OTHER kinds (specs) and neither reads naturally as "write an
attestation"; `matrix`/`rollup` are read-only report verbs with no write
path at all. A new top-level verb is the smallest-surprise choice — but it
is exactly the case the plan's global constraints flag: "verb/tool
inventories stay consistent or `spec-align` fails ... any new verb
requires the ratification task (task 3.R)." This contract records that
dependency rather than assuming it away: landing this story requires ONE
ratification PR (plan task 3.R) that moves `cmd/verdi/dispatch.go`'s
`verbPhase` map, `docs/design/specs/05-surfaces.md` §CLI's table, and
`internal/specalign/verbs_test.go`'s `inV0` inventory together, plus a
`docs/design/specs/08-revision-notes.md` entry recording the ratification
(mirroring the dc-7/PR #51 precedent this plan itself cites).

**Judgment call, flagged for seam review:** `verdi board attest` was
considered and rejected. `board`'s one existing subcommand, `commit`, is
spec-editing (a board-key-to-design-branch ritual); attest neither edits a
spec nor requires a board/server context — it can run from a bare
checkout, no `verdi serve` needed. A bare top-level verb matches
`close`/`gate`/`audit`'s own precedent (a focused, single-purpose ritual
verb, not nested under a loosely-related one) better than forcing it under
`board`.

## DC-2

Scaffold content and write discipline. The written frontmatter is exactly
AC-1's worked example: `id: attestation/<storySlug>--<acID>` (I-6's
compound name), `kind: attestation`, a mechanically-derived,
identifier-shaped `title` (never claim-shaped prose — parent dc-2),
`owners` copied verbatim from the resolved story spec's own `owners:`
(structure, never invented, never the scaffold's own placeholder
`[unassigned]`), `schema: verdi.attestation/v1`, a single `links` entry of
type `verifies` naming the resolved spec as a bare ref (no fragment — 02
§Link taxonomy's closed edge vocabulary permits nothing else on a
non-`implements`/`resolves`/`supersedes`/`exempts`/`depends-on` edge, so
this is the only legal shape regardless of preference), and a `frozen`
stamp.

**Judgment call, flagged for seam review — settled by ADJ-30:**
attestations carry no draft state — `AttestationFrontmatter.Validate`
requires `Frozen` unconditionally — so there is no schema-legal way to
write a strict-decodable, unauthored attestation without SOME frozen stamp
(AC-4 demands the scaffold round-trip Validate cleanly). This contract
pins the only available choice: `frozen.at` is today's date and
`frozen.commit` is the git HEAD at scaffold time — exactly how `verdi
accept` and stub-instantiate already stamp their own scaffolds (a declared,
deterministic stamp, not wall-clock or randomness affecting content).

ADJ-30 settles what that pre-filled stamp MEANS, refining an earlier
framing that read it as the file's own (unknowable) future commit. The
stamp is a **convenience** that matches the store's existing attestation
convention: an attestation's `frozen.commit` names the tree the claim was
verified against, never the file's own commit. The store already proves
this is the convention — `attestation/disclosure-legibility--ac-1` stamps
`6a3465b`, a commit that predates that file's own existence, because
`6a3465b` is the tree the operator reviewed, not where the file happened to
land. The scaffold therefore pre-fills HEAD-at-scaffold-time only as a
starting point; the **operator** updates `frozen.commit` to the commit they
actually verified against at the moment they author the claim. This is
legitimate on two grounds. First, which tree was verified is the epistemic
half of the record, and the epistemic half is the human's to own (parent
dc-2's split: verdi writes structure, the human writes — and here, vouches
for — the substance). Second, the stamp is legally mutable until the file's
first commit: VL-010 binds only COMMITTED frozen artifacts, so correcting
`frozen.commit` in the same authoring pass that removes the unauthored
marker touches nothing frozen. This is the identical trust posture a
hand-authored attestation's `frozen.commit` has always carried — a
reference point the human vouches for, never a mechanically-verified
self-reference — and AC-1's instructional body text tells the operator this
explicitly, in those terms.

The verb writes ONLY this one file to the working tree and does **not**
commit it — unlike `design start`/stub-instantiate, which commit their own
scaffolds immediately. An attestation is one file meant to be authored
once, in place, before its first commit; it is not a multi-commit design
surface the way a draft spec is. Auto-committing a scaffold that AC-1's
own construction guarantees is not-yet-evidence would either force an
immediate follow-up amend or litter history with a guaranteed-throwaway
commit. This satisfies co-2's "mutates nothing beyond the files it exists
to write" under its narrowest reading: the working tree gains one new
file; nothing else on disk or in git changes.

## DC-3

The unauthored-marker mechanism (AC-1's own concrete pin, elaborated).

The marker is a single, fixed, byte-exact sentinel line —
`<!-- verdi:attestation-unauthored -->` — an HTML comment: invisible were
the body ever rendered as markdown, trivially greppable, and vanishingly
unlikely to collide with genuine first-person claim prose. It is defined
once as an exported constant, `evidence.UnauthoredAttestationMarker`
(`internal/evidence/attestations.go`), so the scaffold writer (`cmd/verdi`)
and every fold reader (`internal/evidence`, `internal/wallbadge`) share one
literal — never copy-pasted (CLAUDE.md).

Detection is a raw substring check over the file's whole byte content —
deliberately not a frontmatter/body split, since the marker can only ever
appear where the scaffold put it, in the body, and a substring check is
simplest and needs no coupling to frontmatter-parsing code:

- marker present ⇒ `AttestationUnauthored`
- file exists, marker absent ⇒ `AttestationAuthored`
- no file at the path ⇒ `AttestationAbsent`

This three-way `evidence.AttestationState` (a new exported type,
`internal/evidence/attestations.go`) is surfaced by a new function,
`LoadAttestationState(storeRoot, storySlug, acID string) (AttestationState, error)`.
Every current consumer of the existing existence-only `AttestationExists`
must switch to it, treating ONLY `AttestationAuthored` as satisfied (parent
dc-2: "the scaffold is not foldable until the operator has authored the
claim") — `AttestationExists` itself is left untouched, same signature,
same existence-only semantics, for any caller that genuinely only needs
raw existence (its own doc comment already disclaims decoding content; this
story does not weaken that contract, it adds a sibling).

This story's own research found exactly three call sites reading pure
existence as "attested" today, all of which must move together — leaving
even one on the old semantics would let an unauthored scaffold fold as
evidenced there while correctly reading as absent everywhere else, an
inconsistency this decision forecloses rather than permits by omission:

- `internal/evidence/fold.go:78` — the story fold (this story's own direct
  concern, since `verdi attest` writes story attestations)
- `internal/evidence/featurefold.go:135` — the feature outcome fold
- `internal/wallbadge/emptyslot.go:132` — the board's empty-slot badge

`AttestationUnauthored` is **not** a new fold-visible status of its own
(parent dc-1: closure-gate semantics are unchanged, no new pass path) — it
collapses to exactly the same not-satisfied outcome `AttestationAbsent`
already produces, everywhere the fold computes evidenced/pending/no-signal.
The only difference is disclosure: a caller rendering a human-facing reason
— the sibling close-preflight story's own surface, cross-story seam, CO-3
— can now say "scaffolded but not yet authored" instead of the same
undifferentiated "absent" a never-touched AC would also get. This story
does not itself change what any of those three callers RENDER; it only
makes the richer signal available for close-preflight to consume, per its
own contract.

The operator's authoring is one pass over one still-uncommitted file:
remove the marker, write the claim, and — per dc-2 and ADJ-30 — set
`frozen.commit` to the tree actually verified against (the pre-filled
HEAD-at-scaffold-time is only a convenience; VL-010 binds solely committed
frozen artifacts, so nothing here is frozen until that first commit). The
machine supplies the structure; the operator supplies every word of the
claim and the one fact — which tree was verified — that only they can
vouch for. This is dc-3's "human owns the substance" theme applied to the
stamp as well as the claim.

Test files this decision obligates, beyond AC-1's own:
`internal/evidence/attestations_test.go` (table-driven: absent, unauthored,
authored, over both a real marker-bearing fixture and a hand-written
authored one) and updates to `internal/evidence/fold_test.go`,
`internal/evidence/featurefold_test.go`, and
`internal/wallbadge/emptyslot_test.go` proving an unauthored scaffold
folds/badges exactly as absence would at each of the three call sites.

## DC-4

VL-022's scope and the grandfather question.

The rule fires ONLY on attestations that carry a `verifies` link at all —
never on one that omits it entirely. This is a deliberate, disclosed scope
limit, not an oversight.

Every attestation in the store as of this contract's authoring predates
this feature and carries no `verifies` edge — including files
`08-revision-notes.md`'s "Round 6 — closure status flip" entry describes
as deliberately preserved, immutable, mis-slugged historical record
(the corrected pair sits alongside them at the RIGHT slug; the originals
remain because a frozen artifact must not be touched, VL-010). A rule that
resolved the (story, AC) claim by any OTHER means — for instance,
reverse-searching the corpus for a story spec whose own `RefSlug(story)`
happens to match the attestation's directory name — would need an
enumerated grandfather-baseline map to avoid newly failing every one of
those pre-existing files the very first time it runs: exactly the "a new
requirement would break every pre-existing corpus artifact" problem
`internal/lint/vl020.go`'s own `obligationGateBaseline` solved one rule
number ago, the harder way (a maintained, named exemption list).

Gating on verifies-presence instead needs no such map at all: every file
this rule ever examines is one the sanctioned helper wrote (DC-2 always
writes the edge), so every pre-existing, hand-authored attestation is out
of this rule's scope by construction — zero enumerated exceptions to
maintain or later shrink.

The residual gap is disclosed, not silently accepted: a human
hand-authoring or hand-moving an attestation file WITHOUT going through
`verdi attest`, and without adding a `verifies` edge of their own, escapes
this rule. Such a file already bypasses every guardrail this story adds;
catching an unclaimed mis-slug with no self-declared claim to check
against is a strictly harder problem this story does not attempt to solve.
This mirrors `vl021.go`'s own precedent exactly: "the OPTIONAL
`derived_from.source_digest` is format-checked the same way ONLY when
present — its absence is never a finding."

## DC-5

Exit discipline and scope boundary.

`verdi attest` follows co-2's 0/1/2 split precisely: exit 1 (verdict) for
BOTH of AC-2's refusal cases — deliberately grouping "story-ref does not
resolve" under the same verdict-not-operational treatment as "ac-id
undeclared" and "wrong class", even though the shared resolution seam this
verb reuses (`internal/storyresolve.Resolve`) is treated as OPERATIONAL
(exit 2) by its other existing caller, `verdi matrix`
(`cmd/verdi/matrix.go`).

This is a disclosed divergence, not an inconsistency overlooked: `matrix`
is a pure reporting verb, for which an unresolvable ref is closer to a
usage/precondition error — it cannot do its one job at all. `attest` is
explicitly brought under co-2's own umbrella by the parent spec's own
text: "Preflight **and the attest scaffold** mutate nothing beyond the
files they exist to write" — CO-2 names the attest scaffold directly, in
the same sentence as preflight's own verdict-not-error posture. A
nonexistent (story, AC) pair is exactly the kind of meaningful,
expected-to-happen "no" co-2 already treats as a verdict everywhere else on
this surface (preflight's own unmet conditions), so treating it as a
verdict here — rather than mechanically inheriting matrix's exit-2 posture
for the identical resolution failure — is the more faithful reading of the
parent contract, not a mere stylistic choice.

Scope boundary: `verdi attest` targets STORY attestations only. A resolved
spec whose `class` is anything other than `story` — including `class:
feature`, reachable via the same story-ref argument form per
`storyresolve.Resolve`'s own two-form contract, which does not
discriminate by class — is refused under the "pair does not exist" verdict
(exit 1), read as "no STORY exists to attest an AC against."
Feature-level outcome-attestations (R4-I-11's `<feature-slug>--<ac-id>`
compound, hand-authored today, e.g. `disclosure-legibility`,
`diagram-proposals`) are an explicit non-goal of this story: broadening the
helper to scaffold them is a reasonable future extension, not required by
the parent `ac-2` text this story implements, which is framed as "(story,
AC)" throughout, never "(story-or-feature, AC)".

## CO-1

No network in any test. `cmd/verdi/attest_test.go`'s cases are
fixturegit-backed (mirroring `cmd/verdi/design_test.go`'s and
`cmd/verdi/close_test.go`'s own harness exactly: a real, local, hermetic
git repository, no network, no exec of any real upstream tool).
`internal/lint/vl022_test.go`'s cases are in-package `Snapshot` fixtures
(mirroring `vl019_test.go`/`vl021_test.go`), never a real git checkout.

## CO-2

Exit discipline (0 clean / 1 verdict / 2 operational — DC-5's exact
mapping) and mutation scope: the verb writes exactly one file to the
working tree and commits nothing; it never touches any other file, any
git ref, or any git index entry. A refused invocation (either AC-2 case)
leaves the working tree byte-for-byte as it found it.

## CO-3

Cross-story seam boundary, disclosed rather than assumed away: this
story's deliverable is the SIGNAL (`evidence.AttestationState`'s
three-way distinction, DC-3), not any particular disclosure wording. The
sibling close-preflight story (spec/closure-ergonomics ac-1, a mode of
`verdi close`) is the surface that renders "scaffolded but not yet
authored" to an operator; this story guarantees only that the signal
exists, is correct, and is exported (`LoadAttestationState`) for that
story to consume. The orchestrator's own wave-end review names this exact
seam (attest-helper ↔ close-preflight, attestation path/slug grammar) —
this constraint is this story's half of it.
