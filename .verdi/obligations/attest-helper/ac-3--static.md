---
id: obligation/attest-helper--ac-3--static
kind: obligation
title: "VL-022's table-driven tests include a mis-slug witness, a clean well-slugged fixture, and an out-of-scope no-verifies fixture"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/attest-helper" }
frozen: { at: 2026-07-16, commit: a42d24e2ed017bcf7fa839417755b98b90bb0f34 }
---
# VL-022's table-driven tests include a mis-slug witness, a clean well-slugged fixture, and an out-of-scope no-verifies fixture

The static evidence must show `internal/lint/vl022_test.go`, table-driven
over in-package `Snapshot` fixtures (mirroring `vl019_test.go`/
`vl021_test.go`, never a real git checkout — co-1), covering at least: a
misplaced fixture whose `verifies` target's `store.RefSlug(target.Story)`
disagrees with the fixture's own directory segment (the D6-18 class), which
must produce a finding naming the offending value; a clean, well-slugged
attestation, which must produce no finding; and an attestation that carries
no `verifies` edge at all, which must produce no finding (dc-4's disclosed
scope limit — the rule fires only on attestations that carry the edge). The
evidence must show every refusal path (unparseable/fragment/unresolvable
target, non-story class, undeclared AC, slug/path disagreement) naming its
offending value, never a bare or silent absence.
