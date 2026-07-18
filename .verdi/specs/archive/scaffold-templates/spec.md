---
id: spec/scaffold-templates
kind: spec
title: "Scaffold Templates"
owners: [platform-team]
class: story
status: closed
story: jira:VERDI-31
problem: { text: "spec scaffolds are Go string builders (seam S9, internal/designscaffold): changing what a new spec looks like is a Go change and recompile; teams cannot add sections or fields, and strict decode rejects any team-specific frontmatter outright, so there is no sanctioned extension surface for spec content at all", anchor: problem }
outcome: { text: "designscaffold renders from template files — a store's .verdi/templates/<class>.md overriding an embedded canonical set that reproduces today's scaffolds exactly — selected via the resolved model's Class.Template for both call sites (design start and the board's stub-instantiate); a custom: opaque frontmatter namespace decodes, survives re-emit, and renders, with the YAML dialect wall still enforced inside it (operating-model dc-2); and verdi model check round-trips every template through instantiate-then-strict-decode so a broken template fails at check time naming the template, never at first use", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "the embedded canonical templates render scaffolds decode-equivalent to the retired string builders for every class and the spike variant, proven by equivalence tests on decoded fields", evidence: [behavioral], anchor: ac-1 }
  - { id: ac-2, text: "a store template override with an added body section and a custom: field scaffolds new specs accordingly, the custom content survives strict decode and canonical re-emit, and an anchor/dialect violation inside custom: still fails closed", evidence: [behavioral], anchor: ac-2 }
  - { id: ac-3, text: "verdi model check instantiates and strict-decodes every resolved template (store overrides and embedded), failing closed naming the offending template, proven by driving the built binary", evidence: [behavioral], anchor: ac-3 }
links:
  - { type: implements, ref: "spec/operating-model#ac-3" }
frozen: { at: 2026-07-17, commit: 5e7677b5bd7ddd0c6a7c9adc94733c37c49fc6a9, stub_matched: true }
---
# Scaffold Templates

## Problem

Every scaffolded spec's shape — the frontmatter fields it carries, the
section headings its body opens with — is Go source, not data.
`internal/designscaffold` (seam S9) is the one place both consumers route
through: `Feature` (`fmt.Sprintf` over one literal format string) and
`Story` (`strings.Builder` plus `fmt.Fprintf`, covering the spike variant
too) render scaffolds baked into the package itself. `cmd/verdi/design.go`'s
`design start` and the workbench's `stub-instantiate` board action
(`internal/workbench/boardspecapi.go`) call the same two functions rather
than re-implementing them independently — the cross-subsystem duplication
model-schema closed at the model-*description* layer has no counterpart
here. But a singular seam is not an extensible one: changing what a new
spec looks like — adding a section, renaming a heading, adding a field —
is a Go source change to `designscaffold.go` and a recompile, exactly like
every other hard-coded surface the extensibility audit named.

The gap runs deeper than "no template file to edit." `internal/artifact`'s
single strict-decode seam enforces `KnownFields(true)` on every spec
(CLAUDE.md's own house rule), so even a team willing to hand-edit a
freshly scaffolded spec cannot add a field of its own — a bare
`custom_field: ...` in frontmatter fails decode outright, the identical
fail-closed posture that makes strict decode valuable everywhere else
becomes an absolute wall here. There is no sanctioned extension surface
for spec content at all: not a template a team can override, not a
namespace strict decode will tolerate. A team that wants its specs to
carry an extra "rollout plan" section or a tracking field of its own has
no path to either that doesn't mean forking `internal/designscaffold` and
`internal/artifact` both.

## Outcome

`designscaffold` stops building strings and starts rendering templates. A
store may drop a `.verdi/templates/<class>.md` file — `feature.md`,
`story.md` — that overrides an embedded canonical template of the same
name; the embedded canonical templates render byte-for-byte the same
scaffolds today's `Feature`/`Story` string builders produce, so a store
with no `templates/` directory at all changes nothing (the same "absence
changes nothing" posture operating-model's own embedded `canonical.yaml`
already established for the model manifest itself). Which template file
governs a given scaffold is no longer a Go `switch` on class name: both
call sites — `design start` and stub-instantiate — resolve it through the
same seam, reading `Class.Template` off the store's already-resolved
`model.Model` (the field model-schema's kernel already requires non-empty
per class, `internal/model/model.go`), so template selection and
model-schema's own class declarations are the same fact read twice, not
two separate facts to keep in sync.

A new `custom:` top-level frontmatter key becomes a team's sanctioned
extension point: `KnownFields` at the frontmatter level stops rejecting it
outright, but nothing inside it is a free pass — the strict-decode seam's
YAML dialect wall (anchor/alias/tag rejection) still applies inside
`custom:`, exactly the posture operating-model's own dc-2 already settled
for `model.yaml`'s `custom:` namespace ("loosening later is additive,
tightening would break stores"). A template that populates `custom:`
renders it, a spec that carries it decodes without error, and canonical
re-emit round-trips it unchanged — the field survives the identical
strict-decode-then-re-emit cycle every other frontmatter field already
does today.

Because a template is now something a store can get wrong, `verdi model
check` is the seam that catches it before a scaffold does: it instantiates
every template the resolved model can reach — every store override and
every embedded fallback — and strict-decodes the result, exactly like a
real scaffold consumer would. A broken template (malformed template
syntax, or output that fails strict decode) fails `model check` closed,
naming the offending template file, rather than surfacing for the first
time as a confusing decode error on someone's first `design start` after
the store's templates changed underneath them.

## Ac 1

The embedded canonical set replaces `Feature` and `Story` (including the
spike variant `Story` already renders) one class at a time, and
*equivalence* — not byte-identity — is what gets proven: a
template-rendered scaffold and its retired string-builder equivalent, both
run through `artifact.SplitFrontmatter` + `artifact.DecodeSpec`, must
decode to the same `SpecFrontmatter` fields. `designscaffold_test.go`'s
existing `TestFeature`/`TestStory_Plain`/`TestStory_Spike`-style assertions
— Class, Story, Problem, Outcome, AcceptanceCriteria, Stubs, Links — are
the precedent this equivalence check extends, not a byte comparison of the
rendered markdown itself. "Every class" means every class the resolved
model actually declares a template for today — `feature` and `story`,
matching `internal/model/canonical.go`'s own disclosed scope: `component`
carries "no scaffold/template anywhere in the code today" per that file's
own comment, and stays out of scope here for the identical, already-
ratified reason, not a new omission this story invents. Once every case
passes, the retired `fmt.Sprintf`/`strings.Builder` bodies of `Feature` and
`Story` are deleted — proving equivalence is what makes deleting them,
rather than merely adding a parallel path beside them, safe.

## Ac 2

A store template override is the whole point made concrete: a
`.verdi/templates/story.md` that adds a body section (say, a "Rollout
Plan" heading) and a `custom:` frontmatter field with a real value
scaffolds every subsequent `design start`/stub-instantiate story spec
carrying both, in place of the embedded canonical `story.md`. The custom
field is not merely written once — it must survive the same
strict-decode-then-canonical-re-emit round trip a spec already goes
through today (the identical property AC-1's equivalence tests check for
every other field), so a scaffolded spec's `custom:` content is provably
not silently dropped or mangled by decode/re-emit. And the escape hatch
has a floor: a `custom:` block that smuggles in a YAML anchor, alias, or
tag still fails closed at decode — operating-model dc-2's dialect wall
extended to this namespace verbatim, proven by a violation fixture shaped
like `internal/model`'s own catalog of one-fixture-per-kernel-rule
violations (model-schema ac-1's precedent).

## Ac 3

`verdi model check` (`cmd/verdi/model.go`) already resolves a store's
model manifest and gives it 0/1/2 exit discipline (model-schema ac-3);
this AC is that check learning about templates, the one genuinely new
surface this story adds to it. For every class the resolved model
declares, `model check` renders the class's own resolved template — the
store override if one exists at the path the model's `Class.Template`
filename implies under `.verdi/templates/`, the embedded canonical
fallback otherwise — with placeholder data, then strict-decodes the result
exactly as a real scaffold consumer would. A template that fails to render
or that renders content failing strict decode fails `model check` closed,
and the printed error names the specific template file at fault — never a
bare "model.yaml invalid" — so a store that ships a broken template
override learns it at `check` time, wired into `make verify`'s
`lint-store` step exactly like model-schema's own frontier/kernel checks,
rather than at the moment some future `design start` silently produces a
spec nobody can decode. Proven by tests driving the real built binary,
mirroring model-schema ac-3's own `cmd/verdi/model_test.go` convention
(real built-binary end-to-end tests, never a package-internal unit
standing in for one).
