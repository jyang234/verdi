---
id: obligation/instruction-conformance--ac-5--behavioral
kind: obligation
title: "go test ./internal/specalign/... (equivalently make spec-align, and therefore make verify), actually run against this repo's own real tree rather than a fixture, exits 0 with zero AC-1-3 findings, reachable only after the build phase disposes of the SKILL.md violation"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/instruction-conformance" }
frozen: { at: 2026-07-19, commit: b4bbc327614200071401da65706b23119fbabd2d }
---
# go test ./internal/specalign/... (equivalently make spec-align, and therefore make verify), actually run against this repo's own real tree rather than a fixture, exits 0 with zero AC-1-3 findings, reachable only after the build phase disposes of the SKILL.md violation

The behavioral evidence must show `go test ./internal/specalign/...` —
equivalently `make spec-align`, and therefore `make verify` — actually
invoked against this repository's own committed `.claude/skills/*/SKILL.md`
and root `CLAUDE.md`, never a synthetic fixture standing in for them, and
observed to exit 0 with AC-1 through AC-3's checks together reporting zero
findings, captured in CI's test step. This story's own outcome text
discloses that this exact command fails red against this repo's pre-build
tree today — naming `.claude/skills/commit-to-design/SKILL.md` as the
offense — "which is the point"; this obligation is satisfied only by the
post-disposition green run (after the build phase applies DC-4's
retirement or DC-3's rewrite-to-disclose alternative to that file), never
by a run against a tree that was never capable of exhibiting the red
direction the AC's own text names. A run that is green only because the
new checks were never wired into the default `go test` invocation is not
this obligation's proof — see the sibling `ac-5--static` obligation for
the wiring/reachability half of this same claim.
