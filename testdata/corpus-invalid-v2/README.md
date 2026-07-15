# testdata/corpus-invalid-v2

Decode-failure twins for the v1-P1 round-four contract surface (object
model, story class, edges, reaffirmation, board layout), mirroring
`testdata/corpus-invalid/`'s v0 convention: each file is a small variant of
a real `testdata/corpus/` v2 fixture with exactly one injected defect,
proven in `internal/artifact/v2fixture_test.go` to fail loudly.

| File | Base fixture | Defect | Expected failure |
|---|---|---|---|
| `feature-unknown-field.md` | `escrow-autopay/spec.md` | unknown top-level field `bogus_extra_field` | KnownFields(true) rejection naming the field |
| `story-unknown-field.md` | `borrower-update-api/spec.md` | unknown top-level field `bogus_extra_field` | KnownFields(true) rejection naming the field |
| `layout-unknown-field.json` | `escrow-autopay/layout.json` | unknown field `bogus_extra_field` | `encoding/json` `DisallowUnknownFields` rejection |
| `reaffirmation-unknown-field.md` | `reaffirmations/jira-loan-1483/ac-1.md` | unknown top-level field `bogus_extra_field` | KnownFields(true) rejection naming the field |
| `feature-mismatched-anchor.md` | `escrow-autopay/spec.md` | `ac-2`'s `anchor:` renamed to `#nonexistent-heading` | `ResolveObjectAnchors` fails naming the anchor rule |
