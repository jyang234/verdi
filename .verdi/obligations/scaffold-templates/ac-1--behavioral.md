---
id: obligation/scaffold-templates--ac-1--behavioral
kind: obligation
title: "Equivalence tests prove every embedded canonical template decodes to the same SpecFrontmatter fields as the retired Feature/Story string builders, for every class and the spike variant"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/scaffold-templates" }
frozen: { at: 2026-07-17, commit: 88f8a0a6236e71f403d768f45140718624b8cd2c }
---
# Equivalence tests prove every embedded canonical template decodes to the same SpecFrontmatter fields as the retired Feature/Story string builders, for every class and the spike variant

The behavioral evidence must show Go tests in `internal/designscaffold`
(extending `designscaffold_test.go`'s existing `TestFeature`/
`TestStory_Plain`/`TestStory_Spike` convention) proving that rendering
each embedded canonical template — `feature.md` and `story.md`, including
story's own spike variant — through the new template-rendering path and
decoding the result via `artifact.SplitFrontmatter` + `artifact.DecodeSpec`
produces `SpecFrontmatter` values field-equal (Class, Story, Problem,
Outcome, AcceptanceCriteria, Stubs, Links) to what the retired `Feature`/
`Story` string-builder functions produce for the identical inputs — one
case per class plus one for the spike variant, checked on decoded fields,
never a byte comparison of the rendered markdown. It must also show that
once every case passes, the retired `fmt.Sprintf`/`strings.Builder` bodies
of `Feature` and `Story` are deleted from `designscaffold.go`, so the
equivalence tests are proof the template path fully replaces them rather
than a parallel path kept alongside it. Green in CI's test step.
