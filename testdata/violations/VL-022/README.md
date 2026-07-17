# testdata/violations/VL-022

VL-022 (an attestation's `verifies` edge must resolve to the (story, AC)
implied by its own on-disk path and compound id — spec/attest-helper AC-3,
spec/closure-ergonomics AC-2's enforcement half), scoped to attestations
that carry a `verifies` edge at all (DC-4's disclosed grandfather-avoiding
scope limit). Mirrors `vl019.go`'s own `badVerifiesTarget` pattern,
extended with a slug-derivation step: an attestation's path segment is
`store.RefSlug(target.Story)`, not the target's own directory name (the
D6-18 class of bug this rule closes).

All three cases below share one ad hoc story spec,
`.verdi/specs/active/vl-022-story/spec.md` (`story: jira:VL022-1`, RefSlug
`jira-vl022-1`, declaring `ac-1`) — self-contained, not the golden
corpus's own `spec/stale-decline`/`spec/borrower-update-api`, so this
rule's fixtures need no cross-reference to the shared corpus.

- `misslug/.verdi/attestations/vl-022-story/ac-1.md` — the primary
  witness: id and path both name `vl-022-story` (VL-011's own id/path
  agreement is satisfied), but the `verifies` target's own story-ref slug
  is `jira-vl022-1`, not `vl-022-story` — VL-022's headline refusal,
  naming both disagreeing values.
- `clean/.verdi/attestations/jira-vl022-1/ac-1.md` — the positive
  complement: directory `jira-vl022-1` agrees with
  `store.RefSlug("jira:VL022-1")`; every VL-022 check passes; no finding.
- `no-verifies/.verdi/attestations/vl-022-story/ac-1.md` — DC-4's scope
  limit: no `verifies` edge at all, and (deliberately) the same
  wrong-looking directory `misslug/` uses — VL-022 stays silent regardless,
  proving the rule is gated on verifies-PRESENCE, not on inferring slug
  correctness by any other means.

VL-022's other refusal shapes (an unresolvable verifies target; a target
resolving to a non-STORY class, e.g. the golden corpus's own
`spec/stale-decline`; a target that does not declare the id's own AC; a
fragment-bearing verifies edge) are covered by ad hoc overlays in
`vl022_test.go` rather than additional testdata directories here, mirroring
`vl019_test.go`'s own precedent — no new corpus surface needed per
scenario. Two of those ad hoc cases (undeclared AC, fragment form) need
`vl-022-story`'s own spec present without either attestation fixture
above — `story-only/.verdi/specs/active/vl-022-story/spec.md` supplies
just the spec, so `buildLintRepo`'s per-argument overlay-directory
composition (each argument must itself be store-root-shaped) can chain it
alongside a one-off ad hoc attestation.
