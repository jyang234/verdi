# testdata/violations

One minimal DATA overlay per lint rule VL-001..VL-014 (02 §Lint rules),
each a small set of files that, layered onto `testdata/corpus/`, violates
exactly that rule. **Data only** — `artifactlint`, the engine that
consumes these and asserts "rule id equality, not just failure"
(PLAN.md §4), lives in `internal/lint` (phase 4). Every overlay's files
are rooted at the store root (`.gitattributes` at the top; everything
else under `.verdi/`) so a test harness can layer an overlay directory
straight onto a built corpus checkout by copying paths verbatim — no
per-overlay placement logic needed.

Design note (phase 4): `internal/lint`'s VL-001 fires only on
`artifact.DecodeStrict` failure (frontmatter absence, unknown fields, the
restricted dialect) — the *syntactic* half of "decodes strictly against
kind schema". The *semantic* half (required fields, enum membership,
cross-field agreement) is independently re-checked by each rule that owns
it (VL-002/004/005/006/008/009/011/014), reading the raw decoded struct
directly rather than calling a kind's `Validate()`. This is what lets
overlays like VL-006's and two of VL-014's (see below) decode successfully
under `DecodeStrict` yet still trip their own specific rule, even though
`internal/artifact`'s full `Decode<Kind>` (`DecodeStrict` + `Validate()`)
would independently reject the same file as defense in depth.

| Overlay | Rule | Files | Expected finding |
|---|---|---|---|
| `VL-001/.verdi/adr/vl-001-unknown-field.md` | VL-001 | 1 | unknown top-level field `bogus_field` fails KnownFields |
| `VL-001/.verdi/adr/vl-001-anchor.md` | VL-001 | 1 | YAML anchor `&o` fails the restricted dialect |
| `VL-001/.verdi/adr/vl-001-alias.md` | VL-001 | 1 | YAML anchor+alias `&d`/`*d` fails the restricted dialect |
| `VL-001/.verdi/adr/vl-001-custom-tag.md` | VL-001 | 1 | custom tag `!weird` fails the restricted dialect |
| `VL-001/.verdi/adr/missing-frontmatter.md` | VL-001 | 1 | no `---` delimiters at all — frontmatter absent |
| `VL-002/path-mismatch/` | VL-002 | 1 | `id: spec/actual-name` disagrees with dir `wrong-dir-name` |
| `VL-002/duplicate-ref/` | VL-002 | 2 | both files declare `id: adr/vl-002-duplicate` |
| `VL-003/dangling-link/.../vl-003-dangling-link.md` | VL-003 | 1 | `links[0].ref` names no artifact in the committed zone |
| `VL-003/dangling-pin/.../spec.md` | VL-003 | 1 | `context[0]`'s commit is not real git history |
| `VL-004/.../spec.md` | VL-004 | 1 | `status: draft` layered onto the default branch |
| `VL-005/.../spec.md` | VL-005 | 1 | two `type: story` links (plus the scalar `story:` field) |
| `VL-006/.../spec.md` | VL-006 | 1 | `acceptance_criteria[0].evidence` is empty |
| `VL-007/.verdi/scratchpad.txt` | VL-007 | 1 | unrecognized top-level entry directly under `.verdi/` |
| `VL-008/.../spec.md` | VL-008 | 1 | `provenance:` present, no `frozen:`, not `gated_generated`-allowlisted |
| `VL-009/.../vl-009-bad-frozen.md` | VL-009 | 1 | `frozen.commit` well-formed but not real git history |
| `VL-010/before/` + `VL-010/after/` | VL-010 | 2 | body text differs across commits at the same `frozen.commit` |
| `VL-011/.../ac-1.md` | VL-011 | 1 | path `story-9999/ac-1.md` disagrees with `id: ...story-1482--ac-3` |
| `VL-012/.gitattributes` | VL-012 | 1 | missing `gitlab-generated`/`linguist-generated` attribute lines |
| `VL-013/.verdi/data/...` | VL-013 | 1 | a file under `data/` present in the overlay (simulates `git add -f`) |
| `VL-014/missing-sticky/` | VL-014 | 2 | board.json has 2 stickies, `dispositions:` covers only 1 |
| `VL-014/dangling-disposition/` | VL-014 | 2 | a disposition names a sticky id absent from board.json |
| `VL-014/incorporated-without-where/.../spec.md` | VL-014 | 1 | `incorporated` disposition has no `where` |
| `VL-014/contradicted-without-note/.../spec.md` | VL-014 | 1 | `contradicted` disposition has no `note` |
| `VL-014/unresolvable-where-anchor/` | VL-014 | 1 | `where: "#does-not-exist"` names no heading in the spec body |

VL-001's overlays cover both halves of the rule: strict decode (unknown
field) and the restricted dialect (anchor/alias/custom-tag, PLAN.md I-1).
VL-014's overlays cover the I-5 hardened edges named in PLAN.md §4:
missing sticky, dangling disposition, `incorporated` without `where`,
`contradicted` without `note`, and an unresolvable `where` anchor.

Phase 4 note: the five `VL-001/*.md`, `VL-002/duplicate-ref/*.md`,
`VL-003/*.md`, and `VL-006/no-evidence-kind.md` files originally
authored in phase 2 sat at the overlay directory's top level rather than
at their real store-relative path; phase 4 relocated them under
`.verdi/...` (and, for the two spec-kind overlays, split into a
`<name>/spec.md` directory) so the layering convention above is uniform
across every rule. Content unchanged.
