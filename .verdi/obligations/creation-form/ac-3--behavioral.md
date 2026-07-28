---
id: obligation/creation-form--ac-3--behavioral
kind: obligation
title: "A real browser on the vocab-rename store: form labels speak display words; the submitted spec lands correct-class, right-branch, TODO-free where filled"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/creation-form" }
frozen: { at: 2026-07-21, commit: cbd7eba9edb7770d0c4a10bd881b831c2feb48ba }
---
# A real browser on the vocab-rename store: form labels speak display words; the submitted spec lands correct-class, right-branch, TODO-free where filled

The behavioral evidence must show a Playwright spec under `verdi/e2e/`
(the next free test number at authoring time) driving the vocabulary-
renamed fixture store's sealed feature wall in a real browser: the
creation affordance and the opened dialog render the store's own display
words (the renamed class word appears as visible label text; the bare
class ids do not appear as visible label text anywhere in the dialog),
the form refuses to submit while a statement field is empty, and — after
filling the name and the statement fields and choosing an acceptance
criterion — submission produces a receipt naming the new `design/<name>`
branch. The landed artifact is then verified OUTSIDE the browser's own
claims, through the harness's read-only inspection window into the
fixture store's git state: the spec exists on exactly that branch,
strict-decodes with `class: story`, and carries no `TODO` in any
position whose form field was filled, while an unfilled field's
placeholder default survives as disclosed. Green in CI's e2e step, as
part of `make verify`.
