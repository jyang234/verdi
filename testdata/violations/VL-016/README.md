# testdata/violations/VL-016

Skeleton overlay for VL-016 (spike path fence — 02 §Lint rules, 01 §Store
manifest `spike_paths:`), landed in V1-P1 alongside the spike variant;
VL-016 itself (and the build-branch-diff mechanism it checks) is not
implemented until a later phase, so no lint test consumes this yet.

- `.verdi/specs/active/borrower-update-mobile-spike/spec.md` — a copy of
  the corpus's spike fixture (`spec: true`, `resolves` edge, no
  `implements` edges).
- `touched-outside-fence/internal/production/should-not-be-touched.go` — a
  marker file standing in for "a second copy of the same spike's build
  branch diff touching a path outside `spike_paths:`" (V1-P1 brief §4): a
  path a real spike build branch's diff should never touch once VL-016
  fences it against a repo's configured `spike_paths:` allowlist.
