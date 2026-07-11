# Spike S8 — zoned-incremental board layout determinism

Prototype: `/Users/johnyang/.claude/jobs/f8ad4a26/tmp/spike-s8/`
(`layout.go`, `layout_test.go`; module `spike-s8`, stdlib only, no verdi or
verdi-go imports). `go build ./...`, `go vet ./...`, `gofmt -l .` (empty),
`go test ./...`, and `go test -race ./...` all pass.

## Algorithm sketch

- **Inputs**: `[]Object{Kind, ID, DocOrder}` + `map[string]Position{X,Y}`
  (the "stored" positions, i.e. current `layout.json`). **Output**:
  `Layout{Schema, Positions}`, canonical-JSON-marshaled.
- **Zones**: closed 5-value enum in a fixed order — attribute, ac,
  constraint, decision, story (05-surfaces.md's own listing order). Each
  zone owns a disjoint vertical band (`zoneBand = 100000`px, generous
  headroom so no realistic zone's row count can spill into the next zone's
  band).
- **Ordering within a zone**: bucket objects by `Kind`, then sort each
  bucket by `(DocOrder, ID)` ascending — `ID` is a plain byte-wise
  (ordinal, case-sensitive) string comparison used only as a tiebreak.
  Because the sort key depends only on the objects' own fields, this step
  is invariant to the order the caller built the input slice in.
- **Placement**: objects with a `stored` entry pass through **verbatim** —
  copied byte-for-byte, never inspected for collisions, never "fixed."
  Objects without one are placed in a zone-local row-major grid (4
  columns, fixed cell size) at the next free slot at or after that zone's
  slot high-water mark, skipping (without ever decrementing) any slot
  whose pixel is already occupied by some stored position.
- **High-water mark / freed-slot policy** (the recommended option,
  implemented and tested): a zone's starting counter is computed by
  scanning **every** entry in the `stored` map — including entries for
  objects no longer present in the current object list ("orphans") — for
  positions that align exactly to that zone's grid, and taking the max
  slot index found. The counter then only increases while placing that
  zone's unstored objects. Consequence: **a slot is never reused within a
  revision, as long as `layout.json` entries are never pruned when their
  object is deleted.** This pushes a policy requirement onto the caller
  (V1-P6's layout-generation driver / whatever prunes `layout.json`):
  orphaned position entries for deleted objects must be left in place, not
  cleaned up. This is consistent with what 02-artifact-contract.md already
  says about VL-018: it "checks its keys resolve to real object IDs; it
  never gates otherwise" — i.e. orphaned keys are already tolerated by the
  lint, so this policy costs nothing new. `TestFreedSlot_PolicyContrast`
  demonstrates the failure mode if a caller prunes anyway: the high-water
  mark drops and a new object silently reuses the freed slot.
- **Canonical output**: reimplemented the repo's `internal/canonjson`
  convention locally (stdlib-only, no import): marshal → decode through
  `json.Number` → recursive re-encode with object keys sorted
  (`sort.Strings`), HTML escaping disabled, single trailing `\n`. Verified
  byte-for-byte against a literal expected string in
  `TestMarshalCanonical_SortedKeysNoEscapeTrailingNewline`.

## Property-test results (all green, `go test ./... -v` and `-race`)

| Test | Property | Result |
|---|---|---|
| `TestDeterministic_RunTwice` | (1) same input → byte-identical output, run twice | PASS |
| `TestDeterministic_ShuffledInputOrder` | (4) invariant to object-slice and positions-map construction order, 20 random shuffle trials | PASS |
| `TestIncremental_AddOneOfEachKind` | (3) adding one new object per zone leaves every prior object's position byte/value-identical; new objects placed with no collisions | PASS |
| `TestRemoval_SurvivorsUnchanged_FreedSlotNotReused` | (2)+(3) removing an object leaves survivors' stored positions unchanged, and the freed slot is *not* reused by a subsequently added object (orphan entry retained) | PASS |
| `TestFreedSlot_PolicyContrast` | negative demonstration: if orphaned entries **are** pruned, freed-slot reuse *does* happen — documents why the policy matters | PASS (reproduces the anti-pattern on purpose) |
| `TestEdgeCase_IDsDifferByCaseOnly` | two same-kind objects whose IDs differ only by case get distinct, stable positions; ordinal tiebreak is stable across reruns | PASS |
| `TestEdgeCase_StoredPositionCollisionKeptVerbatim` | two stored positions at the identical coordinate are both kept as-is (never "fixed"); a new object placed afterward is routed around the collision point | PASS |
| `TestEdgeCase_UnknownKindFailsClosed` | an object of a kind outside the 5-value enum makes `Generate` return an error rather than placing it in an overflow zone | PASS |
| `TestMarshalCanonical_SortedKeysNoEscapeTrailingNewline` | canonical JSON shape matches the repo's `canonjson` posture exactly | PASS |

**Verdict: all four binding properties hold under every table-driven case exercised, including the freed-slot edge case, with no wall-clock/randomness/map-iteration dependence anywhere in the algorithm.**

## Edge-case decisions and rationale

- **Case-only-differing IDs**: treated as fully distinct objects (map keys
  are exact strings; no case-folding). Ordering tiebreak is byte-wise
  ordinal comparison (`a.ID < b.ID` in Go, which is a plain byte
  comparison) — deterministic regardless of locale/collation settings,
  matching CLAUDE.md's "no wall-clock or randomness" spirit extended to
  "no locale-dependence" for string ordering.
- **Stored-position collision**: both kept verbatim. Per
  05-surfaces.md, "Stored coordinates are never moved by generation" —
  there is no carve-out for the case where two stored coordinates happen
  to coincide (e.g. from two independent manual drags, or a copy/paste of
  a subtree before positions were re-assigned). Generation is not in the
  business of resolving overlaps; that's a workbench UI concern (drag to
  separate), not a layout-generation concern.
- **Unknown kind → fail closed** (chosen over an overflow zone): consistent
  with CLAUDE.md's "unknown enum values fail closed" and the object-kind
  enum being closed per 02-artifact-contract.md's kind registry / VL-003's
  "unknown types fail closed" posture for edge types. An overflow zone
  would silently launder a decode-time bug (e.g. a typo'd kind, or a kind
  added to the schema but not yet taught to the layout algorithm) into a
  rendering artifact instead of surfacing it as an error at generation
  time.
- **Freed-slot reuse**: not reused within a revision (see high-water-mark
  design above). Chosen because the alternative (compacting/reusing gaps)
  makes a later object's position depend on the *history* of prior
  deletions rather than purely on current stored state + doc order,
  which is a strictly harder invariant to keep byte-identical across
  runs and violates the spirit of property (3) the moment two objects
  are deleted and one re-added in a different order.

## Binding constraints carried forward to V1-P6

1. Generation must be a pure function of `(objects, stored positions)` —
   no wall clock, no RNG, no unsorted map iteration feeding into output
   order or key selection anywhere in the real implementation (parser
   output order, frontmatter map iteration, etc. all need the same
   sort-before-use discipline this spike used for its input slice/map).
2. Any object present in the `positions` map passes through untouched,
   full stop — no "auto-fix" of overlaps, no snapping, no re-flow, even
   when two stored positions collide.
3. `layout.json` entries for deleted objects must **not** be pruned by
   whatever writes the file (autosave, acceptance-freeze, or a future
   `board` CLI verb) — retaining orphaned entries is what makes the
   freed-slot policy hold. This is a new, spike-derived requirement on the
   V1-P6 writer, not previously called out in 02-artifact-contract.md;
   worth either a line in that spec or an invention-ledger entry
   (PLAN.md §7) if V1-P6 wants a different policy (e.g. an explicit
   `freedSlots` high-water counter field in the schema instead of relying
   on orphan-entry survival — that would be a schema change, not an
   algorithm change, and is out of scope for this spike to decide).
4. Canonical output convention: mirror `internal/canonjson` exactly
   (marshal → `json.Number`-preserving decode → sorted-key recursive
   re-encode → HTML-escaping disabled → single trailing `\n`). V1-P6
   should import the real `internal/canonjson` package rather than
   reimplementing it (this spike only reimplemented it because it must
   not depend on the verdi module).
5. Zone order and grid geometry (column count, cell size, band height) are
   **not** binding — 05-surfaces.md only binds the zoning-by-kind +
   doc/ID-order + next-free-slot + stored-never-moves properties, not the
   pixel layout. V1-P6 is free to pick different geometry as long as the
   four numbered properties at the top of this doc hold.
