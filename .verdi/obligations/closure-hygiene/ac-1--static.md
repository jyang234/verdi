---
id: obligation/closure-hygiene--ac-1--static
kind: obligation
title: "The status-vs-git-reality scan is a total three-outcome function in a new internal/residue package"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/closure-hygiene" }
frozen: { at: 2026-07-20, commit: f8c298d3ad712ead9c108d707a10c49547a440ce }
---
# The status-vs-git-reality scan is a total three-outcome function in a new internal/residue package

The static evidence must show a new `internal/residue` package (dc-1)
implements the scan as a total function over every active-zone spec,
returning exactly one of {pattern-a fires, pattern-b fires, neither} —
never an unreachable fourth path. `status: superseded` specs are excluded
before either pattern's logic runs (dc-2), a check that happens first, not
a state that merely happens never to match either pattern's conditions.

Pattern (a), for a `status: accepted-pending-build` spec `<name>`,
requires all three of: a local `close/<name>` branch exists
(`gitx.HasLocalBranch`), it is unmerged into the default branch
(`gitx.IsAncestor` false against `gitx.DefaultBranch`'s tip), and that
branch's own tip tree already contains `.verdi/specs/archive/<name>/spec.md`
— read via git plumbing against the branch tip (`gitx.LsTree`/`gitx.Show`),
never a working-tree file check, since the archive move exists only on the
branch's own unmerged commits.

Pattern (b), for a `class: feature` `status: accepted-pending-build` spec,
requires every declared `stubs[].slug` to resolve to an on-disk
`.verdi/specs/archive/<slug>/spec.md` carrying `status: closed`; one
unrealized or non-closed stub is enough to keep the pattern from firing.

It must also show `cmd/verdi/audit.go`'s `runAudit` invokes this scan as a
third, independent pass alongside the existing exemption/spec-stale
passes, and that only a pattern-a finding sets the run's `flagged` bool
(dc-3) — a pattern-b finding is appended to the section's own output but
never reaches the exit-code path.
