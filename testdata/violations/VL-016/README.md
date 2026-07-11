# testdata/violations/VL-016

VL-016 (spike path fence — 02 §Lint rules, 01 §Store manifest
`spike_paths:`), implemented at V1-P2. Two overlay variants, both diffed
from the commit *before* the spike spec exists (VL-016 needs the diff
itself to touch the spike's own spec directory — that is the "is this
branch a spike build branch" signal this phase's implementation uses, see
vl016.go's doc comment):

- `only-spike-dir/.verdi/specs/active/borrower-update-mobile-spike/spec.md`
  — the spike alone (`spike: true`, a `resolves` edge, no `implements`
  edges). Used as the clean/happy-path overlay: a diff touching only the
  spike's own directory never fires VL-016 regardless of `spike_paths:`.
  Nested under its own subdirectory (rather than sitting directly under
  this README) so a test using it as a single overlay dir does not also
  pick up this README.md itself as a spurious diffed path.
- `touched-outside-fence/` — a full copy of the same spike spec (so the
  diff still touches the spike's own directory) plus
  `internal/production/should-not-be-touched.go`, a path outside both the
  spike's own directory and any `spike_paths:` allowlist entry — VL-016's
  negative case.
