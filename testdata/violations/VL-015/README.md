# testdata/violations/VL-015

Skeleton overlays for VL-015 (supersession manifest completeness and
fidelity — 02 §Lint rules), landed in V1-P1 alongside the round-four object
model; VL-015 itself is not implemented until V1-P2, so no lint test
consumes these yet (V1-P1's exit criteria only requires that the v2
contract surface **decodes** — see `internal/artifact/v2fixture_test.go`).
Each is a full copy of `spec/loan-workflow-v2`
(`testdata/corpus/.verdi/specs/active/loan-workflow-v2/spec.md`) with
exactly one injected VL-015 defect, matching the "one minimal overlay per
rule that trips exactly that rule" discipline (`PLAN.md §4`).

- `carried-byte-drift/` — `supersession.carried` still lists `co-1`, but
  this twin's `co-1` text differs from `spec/loan-workflow` v1's frozen
  `co-1` text (`must not add new synchronous cross-service calls`) — VL-015's
  "every carried object's content is byte-identical to its predecessor"
  negative case.
- `unclassified-object/` — `supersession.carried` is empty and `co-1`
  appears in no other bucket (`amended`/`amended_advisory`/`removed`), so
  a real predecessor object (`co-1`, present unchanged in v1) is left
  entirely unclassified — VL-015's "every v1 object classified exactly
  once" negative case.
