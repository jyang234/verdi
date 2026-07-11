# testdata/violations/VL-017

Skeleton overlay for VL-017 (open-question stickies resolved-or-carried —
02 §Lint rules), landed in V1-P1 alongside the annotation-type extensions;
VL-017 itself is not implemented until a later phase, so no lint test
consumes this yet. 02 §Lint rules requires an unresolved open-question
annotation to be either `status: resolved` or "explicitly carried as a
declared open-question object on the spec" — the artifact-contract spec
(02) does not otherwise define an `open_questions:` frontmatter block
alongside `acceptance_criteria:`/`constraints:`/`decisions:` (§Object
model), so this fixture models the annotation half only; the "declared
open-question object" carrying mechanism is left for whichever phase
implements VL-017 to define (flagged in the V1-P1 phase report as a
spec-gap candidate for that phase's invention ledger).

- `.verdi/specs/active/open-question-story/spec.md` — a draft story spec
  with no declared mechanism carrying the open question below.
- `mutable/annotations/spec--open-question-story.jsonl` — a single
  `type: question`, `status: open` annotation targeting that spec, neither
  resolved nor (there being no such block) carried.
