# testdata/violations/VL-018

Skeleton overlay for VL-018 (`layout.json` positions keys resolve to real
object ids — 02 §Lint rules, §Record schemas "Board layout"), landed in
V1-P1 alongside `verdi.boardlayout/v1`; VL-018 itself is not implemented
until V1-P2, so no lint test consumes this yet — `BoardLayout.Validate()`
(V1-P1) only checks each key's *shape*, not cross-file resolution against
the sibling spec's declared objects, matching this phase's "pure types,
not yet lint-checked for ... dangling-key correctness" posture.

- `.verdi/specs/active/accepted-pending-build/spec.md` — a copy of the
  corpus fixture, declaring `ac-1`, `ac-2`, `ac-3`, `co-1`, `dc-1`, `dc-2`.
- `.verdi/specs/active/accepted-pending-build/layout.json` — `positions`
  carries `ac-99`, which is not one of that spec's declared object ids —
  VL-018's dangling-key negative case.
