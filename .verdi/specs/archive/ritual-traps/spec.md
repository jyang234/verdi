---
id: spec/ritual-traps
kind: spec
title: "Ritual Traps"
owners: [platform-team]
class: story
status: closed
story: jira:VERDI-P2-4
problem: { text: "three independently-witnessed small defects tax every design series that touches them: ResolveAnchor slugifies a heading's text via SlugifyHeading but never transforms the frontmatter anchor: value before comparing, so anchor: AC-1 silently fails to resolve against ## AC-1 unless the author already knows, from unwritten convention, to write every anchor in lowercase (X-1); align's finding-id construction doubles the judged- prefix (judged-judged-...) on certain regeneration paths, a tool defect whose fix must not disturb archived reports whose dispositions already reference the doubled ids; and VL-003 validates a bindings entry's bare AC id against its owning spec's declared criteria but never resolves a fragment-qualified spec/<name>#<ac-id> entry against the named spec's own ACs at all, so a typo'd AC id in such an entry passes lint with no finding — and this gap is deeper than the parent feature's design knew: the root verdi.bindings.yaml (this very file every story in this design series appends to) is never discovered as a lint-checkable Service at all, because no .flowmap.yaml exists at the module root (D6-4), so checkBindings validates nothing in it today, bare or fragment-qualified — the fragment-check fix cannot fire on the very file it is meant to protect without a root-discovery path first (chronicle P2-3(b))", anchor: problem }
outcome: { text: "three independently-witnessed traps close with pins proving they stay closed: ResolveAnchor slugifies both the heading side and the frontmatter anchor: value through the same SlugifyHeading transform, so anchor resolution is symmetric regardless of case; a freshly minted judged finding id carries exactly one judged- prefix while an archived report fixture carrying the old doubled judged-judged- form still decodes and round-trips completely untouched, since the fix is prospective-only; and VL-003 gains both a root-discovery path making the repository's own root verdi.bindings.yaml a checked target for the first time, and a fragment-qualified cross-check resolving spec/<name>#<ac-id> against the named spec's own declared acceptance criteria, so a typo'd AC id inside a fragment-qualified entry reds lint by name instead of passing silently — proven on the real root verdi.bindings.yaml this design series itself has been appending fragment-qualified entries to", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "ResolveAnchor slugifies both a heading's text and the frontmatter anchor: value through the same SlugifyHeading transform, so anchor: AC-1 resolves against ## AC-1 regardless of case (X-1's exact witness); a negative pin proves the previously-failing mixed-case resolve now succeeds, and a case that already matched exactly continues to resolve unchanged", evidence: [behavioral], anchor: ac-1 }
  - { id: ac-2, text: "align's judged finding-id construction is fixed prospectively only: a freshly minted judged finding id carries exactly one judged- prefix, while a fixture standing in for an already-archived report carrying the old doubled judged-judged- form still decodes and round-trips completely untouched, since real archived dispositions reference those ids exactly as originally minted", evidence: [behavioral], anchor: ac-2 }
  - { id: ac-3, text: "VL-003 gains a root-discovery path making the repository's own root verdi.bindings.yaml — sibling of .verdi/, not nested inside any .flowmap.yaml service root, per its own documented D6-4 rationale — a checked lint target for the first time; today checkBindings iterates only discovered Services, and since no .flowmap.yaml exists at the module root, the root bindings file is invisible to it and nothing in it, bare or fragment-qualified, is validated (chronicle P2-3(b)) — without this, ac-4's fragment-check fix cannot fire on the very file it exists to protect", evidence: [behavioral], anchor: ac-3 }
  - { id: ac-4, text: "VL-003 cross-checks a fragment-qualified verdi.bindings.yaml entry (spec/<name>#<ac-id>) against the NAMED spec's own declared acceptance criteria, not only (as today) a bare ac-<slug> entry against the bindings file's own primary spec — a typo'd AC id inside a fragment-qualified entry reds lint by name; proven on the real root verdi.bindings.yaml, whose fragment-qualified entries this design series itself has been authoring, once ac-3 makes that file a checked target", evidence: [behavioral], anchor: ac-4 }
links:
  - { type: implements, ref: "spec/ritual-integrity#ac-4" }
frozen: { at: 2026-07-20, commit: 931e3b40be1d375d297b08128443c37e46c93bbd, stub_matched: true }
---
# Ritual Traps

## Problem

Three independently-witnessed small defects tax every design series that
touches them, and one of them is deeper than the parent feature's own
design knew when it was written. `ResolveAnchor`
(`internal/artifact/object.go`) lowercases a heading's text via
`SlugifyHeading` but never applies the same transform to the frontmatter
`anchor:` value before comparing the two — so `anchor: AC-1` silently
fails to resolve against `## AC-1` unless the author already knows, purely
from unwritten convention, to write every anchor in lowercase (X-1).
Align's finding-id construction independently doubles the `judged-`
prefix into `judged-judged-...` on certain regeneration paths — a tool
defect discovered mid-round whose fix must not disturb any archived
report whose dispositions already reference the doubled ids exactly as
minted. And `VL-003` validates a bindings entry's *bare* AC id against its
owning spec's own declared criteria, but never resolves a
*fragment-qualified* `spec/<name>#<ac-id>` entry against the *named*
spec's own ACs at all — a typo'd AC id inside such an entry passes lint
with no finding. This last gap is larger than the parent feature spec's
own design knew: the repository's own root `verdi.bindings.yaml` — the
very file this whole design series appends fragment-qualified entries
to, story by story — is never discovered as a lint-checkable Service at
all, because no `.flowmap.yaml` exists at the module root (D6-4's
documented rationale: the verdi repo is not a flowmap service of itself).
`checkBindings` iterates only discovered Services, so today it validates
*nothing* in the root bindings file, bare or fragment-qualified. Fixing
only the fragment-resolution logic, without also giving `VL-003` a path
to discover the root file in the first place, would land a fix that
cannot fire on the one file it exists to protect — the parent AC's own
behavioral letter already forces this correction (chronicle P2-3(b)).

## Outcome

All three traps close with pins proving they stay closed. `ResolveAnchor`
slugifies both the heading side and the frontmatter `anchor:` value
through the identical `SlugifyHeading` transform, so anchor resolution is
symmetric regardless of case — closing X-1 outright. Align's judged
finding-id construction is fixed prospectively only: a freshly minted
judged finding id carries exactly one `judged-` prefix, while a fixture
standing in for an already-archived report carrying the old doubled
`judged-judged-...` form still decodes and round-trips completely
untouched, because real archived dispositions reference those exact,
originally-minted ids and silently renumbering them would break every
one. `VL-003` gains two things together, not one: a root-discovery path
that makes the repository's own root `verdi.bindings.yaml` a checked lint
target for the first time ever, and a fragment-qualified cross-check that
resolves `spec/<name>#<ac-id>` against the *named* spec's own declared
acceptance criteria. Landed together, a typo'd AC id inside a
fragment-qualified entry in the very file this design series has been
appending to reds lint by name — proven for real, on the real root
bindings file, not only on a synthetic fixture standing in for it.

## Ac 1

`ResolveAnchor` is extended to slugify the frontmatter `anchor:` value
through the same `SlugifyHeading` transform already applied to heading
text (`internal/lint/headings.go` is the existing consumer this mirrors),
so `anchor: AC-1` resolves against a `## AC-1` heading regardless of
case. A negative pin proves the previously-failing mixed-case case now
succeeds (X-1's exact witness, reproduced as a fixture), and an
already-matching lowercase case continues to resolve exactly as before —
this is a resolve-*more* direction, never a resolve-less one, so nothing
that worked yesterday can stop working today.

## Ac 2

Align's judged finding-id construction, which on certain regeneration
paths doubles the `judged-` prefix into `judged-judged-...`, is fixed so
that a freshly minted judged finding id carries exactly one `judged-`
prefix. This is fixed *prospectively only*: a fixture standing in for an
already-archived report that carries the old doubled `judged-judged-...`
form must still decode and round-trip completely untouched, proven by a
committed fixture exercising exactly that shape — because real archived
dispositions reference those ids exactly as originally minted, and
silently renumbering them on read would break every existing
disposition's own identity.

## Ac 3

`VL-003` gains a root-discovery path so the repository's own root
`verdi.bindings.yaml` becomes a checked lint target for the first time.
Today's `checkBindings` (`internal/lint/vl003.go`) iterates
`in.Snapshot.Services` only, and a Service is discovered from a
`.flowmap.yaml` file; since none exists at the module root — this
repository is deliberately not a flowmap service of itself, per the root
bindings file's own documented D6-4 rationale — the root file is
structurally invisible to `checkBindings` today, and nothing in it is
validated, bare or fragment-qualified. This is proven directly: a test
constructs the equivalent of today's shape (a root bindings file with no
`.flowmap.yaml` at the module root) and shows a deliberately-wrong bare
AC id in it passes lint silently *before* this story's fix, then reds
*after* — the exact absence this story closes, not merely asserted but
demonstrated red-to-green. Without this leg, `ac-4`'s fragment-check
fix would be unreachable on the one file it exists to protect.

## Ac 4

`VL-003` gains a check it does not have today: cross-checking a
fragment-qualified `verdi.bindings.yaml` entry (`spec/<name>#<ac-id>`)
against the *named* spec's own declared acceptance criteria — not only,
as today, a bare `ac-<slug>` entry against the bindings file's own
primary spec. Proven on the real root `verdi.bindings.yaml`, once `ac-3`
makes it a checked target: a fixture introduces a fragment-qualified
entry naming a typo'd AC id (e.g. `spec/some-story#ac-9` where that story
declares no `ac-9`) and asserts lint reds by name, naming both the
offending entry and the target spec. A companion case with a *correct*
fragment-qualified entry — the exact shape this design series' own
bindings appends already are — must continue to pass, proving the check
is additive and does not regress the real entries already landed by
`judge-ergonomics`, `finding-identity`, and `evidence-resilience`.
