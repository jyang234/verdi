---
id: spec/creation-form
kind: spec
title: "Creation Form"
owners: [platform-team]
class: story
status: closed
story: jira:VERDI-P2-7
problem: { text: "the board can only create a story through a declared stub's one-click instantiate, and every spec it scaffolds carries generic TODO placeholders because no creation surface reads the class template's own placeholders as a field contract (the ADJ-65 asymmetry's board half, guide 5.3's D-1 contract unrealized); commit-to-design is still a third, hand-rolled strings.Builder spec producer that ignores a store's own .verdi/templates/ overrides entirely (ledger L-M12), so a team's feature template override changes design start's scaffolds while board-committed feature specs silently keep the hard-coded Go shape; and a vocabulary-renamed store's board has no creation form at all whose labels could speak its display words", anchor: problem }
outcome: { text: "internal/designscaffold gains the one placeholder-enumeration API — Fields(tmpl []byte) ([]Field, error), ordered field descriptors straight from a template's own parse tree — and the board grows a creation form generated from those descriptors: submitted values render through the same shared designscaffold producer every other creation surface calls, inherit stub-instantiate's self-validate + CheckClass posture so a form-submitted spec can never round-trip as the wrong class, and land as a scaffold commit on a fresh design/<name> branch with the serving checkout untouched; a vocabulary-renamed store's form speaks that store's display words and the created spec is TODO-free wherever a field was actually filled; commit-to-design switches to the identical producer — a store's feature-template override is honored there for the first time, byte-stable for every input the old producer already handled — discharging L-M12", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "internal/designscaffold gains the placeholder-enumeration API Fields(tmpl []byte) ([]Field, error) — each Field a {Name, Kind} descriptor, in first-reference template order, deduplicated, enumerating exactly the template positions rendered against the top-level ScaffoldData value (a range or with body's relative fields are the iterated element's, never enumerated); enumeration over the embedded canonical templates yields the pinned D-1 field sets (story.md: Ref, Title, Owners, StoryRef, Spike, Problem, Outcome, Links; feature.md: Ref, Title, Owners, StoryRef, Problem, Outcome), enumeration over a store OVERRIDE template yields THAT template's own referenced fields (the L-M12 property that makes custom template sets reach the form), and a template referencing a placeholder outside the ScaffoldData contract fails closed naming it — mirroring Render's own missingkey=error posture rather than offering a form field whose submission cannot render", evidence: [behavioral], anchor: ac-1 }
  - { id: ac-2, text: "a new create action lands beside stub-instantiate on POST /board/spec/{name}/api/{action}: it scaffolds a story spec from form-submitted values keyed by the enumerated descriptors of the story class's own resolved template (a store's .verdi/templates/ override winning over the embedded canonical, exactly as LoadTemplate already resolves), renders through the shared designscaffold producer, self-validates (SplitFrontmatter + DecodeSpec) and asserts CheckClass(story) before any git plumbing runs — inheriting stub-instantiate's post-render validation so a misconfigured template binding fails closed server-side — then lands exactly one scaffold commit on a fresh design/<name> branch cut via plumbing, the serving checkout's HEAD, working tree, and index untouched; implements links bind to caller-chosen declared acceptance criteria of the feature wall (at least one required, each validated against the projection); the action shares stub-instantiate's own guards (feature-class wall at accepted-pending-build) and refuses, naming the fact, on a taken branch or spec name, an unknown value key, or an undeclared acceptance criterion", evidence: [behavioral], anchor: ac-2 }
  - { id: ac-3, text: "the board's creation form UI is generated from the same descriptors ac-2's action validates against — one field contract, both halves — and proven in a real browser on the vocabulary-renamed fixture store: the sealed feature wall's creation affordance and the form dialog speak the store's own display words (the renamed class words appear; the bare class ids never render as visible label text), statement fields refuse to submit empty, and the submitted spec lands with class story, on branch design/<name>, TODO-free in every position whose field was actually filled while unfilled fields keep their disclosed placeholder defaults", evidence: [behavioral], anchor: ac-3 }
  - { id: ac-4, text: "commit-to-design renders through the shared designscaffold producer instead of its private strings.Builder: the feature class's declared Class.Template filename resolves a store's own .verdi/templates/ override — honored on this path for the first time, discharging L-M12's third-producer divergence, with CheckClass(feature) asserted post-render — and, absent an override, falls back to the embedded commit-to-design canonical template whose render is BYTE-IDENTICAL to the retired hand-rolled output for every input the old producer already handled, pinned by the existing TestScaffoldSpec_BytePin fixture kept byte-for-byte unchanged", evidence: [behavioral], anchor: ac-4 }
links:
  - { type: implements, ref: "spec/creation-surfaces#ac-2" }
frozen: { at: 2026-07-21, commit: 04e6c0ff27a3644516b3aad6667c108c7c1af1b4, stub_matched: true }
---
# Creation Form

## Problem

The board's only creation path is stub-instantiate: a feature wall's
declared stub becomes a story scaffold in one click, but a free story —
one the feature's stubs did not plan — cannot be born from the board at
all, and everything any surface scaffolds arrives stuffed with generic
`TODO:` placeholders. The cause is structural: no creation surface reads
the class template's `{{ .Placeholder }}` slots as a field contract
(guide 5.3, Appendix D-1), so nothing can *ask* for a problem statement
before the artifact exists — the exact asymmetry ADJ-65 harvested seven
frictions from.

`commit-to-design` is worse than asymmetric: it is a third spec producer.
`internal/commitdesign`'s `scaffoldSpec` is a `strings.Builder` body that
mints a `class: feature` spec without ever consulting the resolved
model's `Class.Template` or `designscaffold.LoadTemplate` — so a store
that overrides `.verdi/templates/feature.md` changes `design start`'s
scaffolds while board-committed feature specs silently keep the
hard-coded Go shape. That divergence is ratified as ledger L-M12 and
assigned here.

And a store that renames its vocabulary (`story` → "Workstream") has no
creation form whose labels could speak those words: the display chain is
wired, but there is no form to route through it.

## Outcome

One field contract, every consumer. `internal/designscaffold` gains the
placeholder-enumeration API, and the board's new creation form is
generated from it: labels resolved through the store's display words,
values rendered through the same shared producer as `design start`,
stub-instantiate, and — from this story on — `commit-to-design`, whose
store-override honoring is byte-stable for every input the old producer
handled. A form-submitted spec self-validates and passes `CheckClass`
before any git plumbing runs, lands on its own `design/<name>` branch,
and is TODO-free wherever the author actually answered the form.

## Produced interface (consumed by the CLI interview, plan Task 12)

```go
// Field is one ordered creation-surface input descriptor, enumerated
// from a class template's own placeholders (guide 5.3 / D-1).
type Field struct {
    Name string    // the ScaffoldData field the template references, e.g. "Title"
    Kind FieldKind // how a creation surface sources the value
}

type FieldKind string

const (
    FieldIdentity   FieldKind = "identity"   // derived from the new spec's own name (Ref)
    FieldInput      FieldKind = "input"      // a single-line input (Title, Owners, StoryRef)
    FieldStatement  FieldKind = "statement"  // a multiline statement (Problem, Outcome)
    FieldStructural FieldKind = "structural" // derived from creation context (Spike, Links, ParentRef)
)

// Fields enumerates tmpl's placeholders as ordered field descriptors.
func Fields(tmpl []byte) ([]Field, error)
```

Order is first-reference document order, deduplicated. Only fields
evaluated against the top-level `ScaffoldData` value enumerate: a
`{{range .Links}}` pipeline contributes `Links` itself, while the range
body's `{{.Type}}`/`{{.Ref}}` are the iterated element's own fields and
are skipped — the walk changes context exactly where `text/template`'s
dot does.

## Ac 1

The enumeration is the D-1 contract made mechanical: parse the template,
walk its tree, and report — in first-reference order — every field a
creation surface must supply. The embedded canonical templates pin the
two standard sets; a store override template yields *its* referenced
fields, which is what makes a custom template set actually reach the
form a user fills in. A placeholder outside the `ScaffoldData` contract
(the guide's aspirational `custom:` placeholders, e.g. `{{.Runbook}}`)
fails closed by name: `Render`'s `missingkey=error` posture over a
struct means such a template cannot render today, so enumeration
refusing it honestly — rather than growing a form field whose submission
would explode — is the disclosed v1 boundary. The guide's
custom-placeholder aspiration stays roadmap, not silently half-built.

## Ac 2

The action is stub-instantiate's sibling and inherits its whole safety
posture: the same wall guards (feature class, `accepted-pending-build` —
implementations build accepted specs only), the same
self-validate-then-`CheckClass` gate before a single git object is
written, and the same pure-plumbing branch cut (`WriteBlob` →
`BuildTreeWithFile` → `CommitTree` → create-only `UpdateRef`) that never
moves the serving checkout. What is new is the input shape: values keyed
by the enumerated descriptors (unknown keys refuse by name — the form
and the action share one contract, so a drifted client fails loudly),
and implements edges chosen by the author from the wall's own declared
acceptance criteria — real coverage claims, at least one, each validated
against the projection, never design start's placeholder edge. Unfilled
fields fall back to the same disclosed placeholder defaults every other
scaffold consumer uses; the receipt names what remains to fill.

## Ac 3

The form is the descriptors, rendered: the dialog's fields are generated
server-side from the same enumeration the action validates against, so
the two halves cannot drift. Display discipline follows the wall's
existing rule (vocabulary.go's enumeration rule): label prose resolves
through the store's display words; identity values — the branch name,
the spec ref, testids, API field names — stay bare. Proven in a real
browser against the vocab-rename fixture store: the affordance speaks
the renamed class word, the created spec lands on `design/<name>` with
`class: story`, and every position whose field was filled carries the
author's words with no `TODO` residue.

## Ac 4

The L-M12 discharge, with the parity pin the frozen feature AC demands.
Byte-stability and override-honoring cannot both hold over the embedded
canonical `feature.md` — the legacy commit-to-design shape predates
`problem:`/`outcome:` and carries `context:`/`dispositions:` blocks
`feature.md` has no slots for — so the switch resolves in two layers: a
store's own override of the feature class's declared `Class.Template`
wins (the exact file L-M12's witness named as silently ignored), and the
no-override fallback is a new embedded commit-to-design canonical
template whose render reproduces the retired `strings.Builder` output
byte-for-byte, proven by the existing byte-pin fixture kept unchanged.
`ScaffoldData` grows the content-carrying fields the ratified ledger
entry predicted (`Pins`, `Dispositions`); `CheckClass(feature)` guards
the render exactly as every other consumer already guards its own.
