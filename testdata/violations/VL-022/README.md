# testdata/violations/VL-022

VL-022 (an attestation's `verifies` edge must resolve to the (target, AC)
implied by its own on-disk path and compound id — spec/attest-helper AC-3,
spec/closure-ergonomics AC-2's enforcement half), scoped to attestations
that carry a `verifies` edge at all (DC-4's disclosed grandfather-avoiding
scope limit). Mirrors `vl019.go`'s own `badVerifiesTarget` pattern,
extended with a slug-derivation step whose derivation is CLASS-scoped: for
a class: story target the attestation's path segment is
`store.RefSlug(target.Story)`, not the target's own directory name (the
D6-18 class of bug this rule closes); for a class: feature target
(R-RR2-2) it is the feature's own name (the `Name` of
`artifact.ParseRef(target.ID)`, the exact segment `evidence.FoldFeature`'s
caller passes as `FeatureSlug`) — and the mis-slug half applies only to
an AC that DECLARES the `attestation` evidence kind, an AC that does not
being outside every consumer and therefore skipped.

The story-scoped cases below all turn on one ad hoc story spec,
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

The two feature-scoped cases (R-RR2-2) share their own ad hoc feature
spec, `.verdi/specs/active/vl-022-feature/spec.md` (`class: feature`, no
`story:` at all, declaring `ac-1` with `evidence: [attestation]` and
`ac-2` with `evidence: [static]`) — again self-contained, so the feature
rule's fixtures never lean on the golden corpus either.

- `feature-misslug/.verdi/attestations/wrong-name/ac-1.md` — the feature
  half's primary witness: the attestation's own directory/id segment is
  `wrong-name`, but its `verifies` target's own name is
  `vl-022-feature`, and `ac-1` DECLARES the attestation kind — one
  refusal, naming both disagreeing values and the feature. Its sibling
  `wrong-name/ac-2.md` sits at the same wrong directory but verifies
  `ac-2`, which declares no attestation kind at all: no finding, so the
  kind boundary is checked independently of slug agreement.
- `feature-undeclared-kind/.verdi/attestations/vl-022-feature/ac-2.md` —
  the boundary on its own, isolated from slug correctness: the directory
  `vl-022-feature` AGREES with the feature's own name, and `ac-2`
  declares no attestation kind, so no fold reads this path and VL-022
  stays silent (the showcase's own `jira-loan-1482--ac-2` is the live
  example this fixture stands in for). It also supplies the feature spec
  alone for the ad hoc undeclared-AC overlay in `vl022_test.go`.

VL-022's other refusal shapes (an unresolvable verifies target; a target
resolving to a class VL-022 does not track — component, say, which is
skipped, not refused; a target that does not declare the id's own AC,
for either tracked class; a fragment-bearing verifies edge) are covered
by ad hoc overlays in
`vl022_test.go` rather than additional testdata directories here, mirroring
`vl019_test.go`'s own precedent — no new corpus surface needed per
scenario. Two of those ad hoc cases (undeclared AC, fragment form) need
`vl-022-story`'s own spec present without either attestation fixture
above — `story-only/.verdi/specs/active/vl-022-story/spec.md` supplies
just the spec, so `buildLintRepo`'s per-argument overlay-directory
composition (each argument must itself be store-root-shaped) can chain it
alongside a one-off ad hoc attestation.
