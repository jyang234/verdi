---
name: verdi-plan
description: Read a spec's plan document and propose, one at a time through mutate_draft, a stub for each acceptance criterion nothing covers yet.
---
<!-- verdi:generated-skill host=claude skill=plan -->
<!-- verdi:engine-digest sha256:d805fb1ae60f08b6a2db8f122cd01f2c332314942b619debe5f6538bcb2e4856 -->
<!-- verdi:template-digest sha256:d12f0a521c6bdc6e77cb85b07214c209caaa714cac2d5f1e031023574c1e0ede -->
<!-- verdi:render-commit 0123456789abcdef0123456789abcdef01234567 -->
<!-- verdi: this is a generated skill; edits here never change verdi, and any difference is reported as drift by `verdi harness check` until this file is regenerated with `verdi harness render`. -->

# verdi-plan

Use this skill when a draft spec has acceptance criteria that no stub covers and the user wants the plan filled in. The plan document is a projection of the spec's own `stubs` block; coverage is computed from it, never declared.

## Steps

1. Call `get_design_context` with `spec/<slug>`. Keep `identity` (`checkout`, `branch`, `head`): every `mutate_draft` call carries exactly those three as `expected`. Then read the draft's bytes from `<checkout>/.verdi/specs/active/<slug>/spec.md`: `base_spec_b64` is their standard base64, and `base_digest` is `sha256:` followed by the lowercase hex SHA-256 of those exact bytes. Re-read them before every call; a stale base is refused.
2. Call `get_document` with `ref` `spec/<slug>`, `kind` `plan`, and `proposed` true, and show the human the plan as it stands. The plan document lists only what is planned, so call `get_document` again with `kind` `spec` and `proposed` true: in its criteria section, an uncovered criterion's coverage line reads "not yet planned."; a covered one reads "covered by …"; "not computed for this render." means the facts were unavailable, in which case say so and stop. Collect the uncovered criterion ids in document order.
3. For each uncovered criterion, prepare exactly one stub: `{"op":"add-stub","slug":"<kebab-slug describing the deliverable>","acceptance_criteria":["<ac-id>"]}`. A stub may cover several criteria when they are one deliverable; say why.
4. Show the human the criterion text and the operation JSON. Ask for confirmation.
5. On confirmation, call `mutate_draft` with `harness` (your harness id), optional `session`, `schema` `verdi.draftmutation/v1`, `spec`, `base_digest`, `base_spec_b64`, `expected`, and `operations` holding that one operation. On a stale-base refusal, repeat from step 1; never resend against a guessed base.
6. When every criterion is covered, call `get_document` with `kind` `plan` and `proposed` true once more and show the human the plan section as it now reads.

```verdi-sequence
call get_design_context
call get_document kind=plan proposed=true
call get_document kind=spec proposed=true
loop
show
confirm
call mutate_draft operations=1
end
call get_document kind=plan proposed=true
```
