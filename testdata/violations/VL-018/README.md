# testdata/violations/VL-018

VL-018 (`layout.json` positions keys resolve to real object ids — 02 §Lint
rules, §Record schemas "Board layout"), implemented at V1-P2.

- `.verdi/specs/active/vl-018-dangling-key/spec.md` — a minimal new-class
  feature spec declaring `ac-1` and `co-1`, each with a resolving anchor.
  Simplified at V1-P2 from V1-P1's original skeleton (a full copy of the
  `escrow-autopay` corpus fixture, complete with a `frozen:` stamp
  citing a commit that is not real git history in this package's own
  fixturegit-built test repos) down to the minimum this rule's own
  resolution logic needs, avoiding an unrelated VL-009 finding. `ac-1`
  declares `attestation` among its evidence kinds (L-M14 remedy 1,
  internal/lint/vl006.go's checkFeatureACAttestation) — this fixture is
  status: draft/unfrozen, so it is not otherwise grandfathered and would
  trip an unrelated VL-006 finding without it.
- `.verdi/specs/active/vl-018-dangling-key/layout.json` — `positions`
  carries `ac-99`, which is not one of that spec's declared object ids —
  VL-018's dangling-key negative case.
