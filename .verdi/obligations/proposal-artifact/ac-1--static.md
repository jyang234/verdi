---
id: obligation/proposal-artifact--ac-1--static
kind: obligation
title: "DiagramFrontmatter declares class/scope/derived_from and a class-conditioned status enum"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/proposal-artifact" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# DiagramFrontmatter declares class/scope/derived_from and a class-conditioned status enum

The static evidence must show that `internal/artifact/diagram.go`'s
`DiagramFrontmatter` struct declares `Class string` (`yaml:"class,omitempty"`),
`Scope string` (`yaml:"scope,omitempty"`), and
`DerivedFrom *DiagramDerivedFrom` (`yaml:"derived_from,omitempty"`, with
`DiagramDerivedFrom{Ref, Digest string}`), and that `Validate` branches on
`Class`: a distinct `proposalStatuses` map admitting only `proposed` and
`accepted` governs `class: proposal`, while a diagram with `Class == ""`
keeps the existing `diagramStatuses{active, superseded}` map and
`requireFrozen(fm.Frozen, false, ...)` call unchanged (byte-identical
behavior for every pre-existing incumbent-diagram fixture). Every field
strict-decodes through the single `internal/artifact` seam
(`DecodeStrict`/`KnownFields(true)`) — no second decode path.
