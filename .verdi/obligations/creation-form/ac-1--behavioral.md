---
id: obligation/creation-form--ac-1--behavioral
kind: obligation
title: "Fields enumerates a template's placeholders as ordered descriptors — embedded sets pinned, override sets its own, unknown placeholders fail closed"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/creation-form" }
frozen: { at: 2026-07-21, commit: cbd7eba9edb7770d0c4a10bd881b831c2feb48ba }
---
# Fields enumerates a template's placeholders as ordered descriptors — embedded sets pinned, override sets its own, unknown placeholders fail closed

The behavioral evidence must show table-driven tests in
`internal/designscaffold` (a `fields_test.go` sibling of the new API)
pinning all four legs of the contract. First: `Fields` over the embedded
canonical `templates/story.md` returns exactly the descriptors
`Ref, Title, Owners, StoryRef, Spike, Problem, Outcome, Links` in that
order, and over `templates/feature.md` exactly
`Ref, Title, Owners, StoryRef, Problem, Outcome` — the pinned D-1 sets,
asserted as full ordered slices (names AND kinds), never a subset
membership check. Second: enumeration over a store OVERRIDE template
that references fewer fields, in a different order, returns THAT
template's own fields in ITS order — proving the form's field source
follows the resolved template, not a hardcoded set. Third: a template
whose range body references the iterated element's own fields
(`{{range .Links}}{{.Type}}{{.Ref}}{{end}}`) contributes `Links` alone —
the dot-context rule asserted directly. Fourth, the negative pin: a
template referencing a placeholder outside the ScaffoldData contract
(e.g. `{{.Runbook}}`) returns an error naming that placeholder, and a
syntactically broken template returns a parse error — never a silently
truncated descriptor list. Green in CI's test step, as part of
`make verify`.
