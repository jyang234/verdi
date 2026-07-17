---
id: obligation/model-digest--ac-3--behavioral
kind: obligation
title: "Fixture-gate and decode-compatibility tests prove artifacts stamped before the model field existed decode unchanged and no committed fixture regenerates"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/model-digest" }
frozen: { at: 2026-07-17, commit: b7a0cf9801ea852fcc1f4801da11ee115f6ffc41 }
---
# Fixture-gate and decode-compatibility tests prove artifacts stamped before the model field existed decode unchanged and no committed fixture regenerates

The behavioral evidence must show `make fixture` (`go test -race
./internal/fixturegit/... ./internal/corpus/... ./internal/svcfixcanned/...`)
and `make lint-store` (`verdi lint` then `verdi model check`) passing
unmodified — no test file, testdata fixture, or Makefile target touched to
make either pass — proving by omission that this story's diff never
intersects either gate's inputs. It must also show a decode test drawing
on at least one pre-existing committed generated artifact that predates
this story — e.g. `.verdi/specs/active/scoping-canvas/decision-conflict-
report.md`, whose committed `provenance:` block already carries no
`model:` line — proving it still decodes cleanly through its type's
existing decoder (`artifact.DecodeDecisionConflict`/`DecodeDeviation`/
`DecodeDiagramSweep`/`DecodeBoard`) with `Provenance.Model` reading back as
the empty string, never a decode error. Where a hand-render counterpart
already exists for the type (`align.RenderMarkdown`,
`RenderDecisionMarkdown`, `RenderDiagramSweepMarkdown`), it must further
show decoding such a pre-existing artifact and re-rendering it reproduces
the exact original bytes — no `model:` line introduced into an artifact
whose source never had one. Green in CI's test step, with the
fixture/lint-store gates passing by being left untouched.
