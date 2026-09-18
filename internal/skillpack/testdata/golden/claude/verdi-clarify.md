---
name: verdi-clarify
description: Surface what a draft spec still leaves unproven — readiness concerns and unclaimed open questions — and propose one decision or one research stub at a time through mutate_draft, showing each proposal before it is written.
---
<!-- verdi:generated-skill host=claude skill=clarify -->
<!-- verdi:engine-digest sha256:d805fb1ae60f08b6a2db8f122cd01f2c332314942b619debe5f6538bcb2e4856 -->
<!-- verdi:template-digest sha256:a7ce44f42ed22f9abf986ff690957edc1d109e9b06b993316f64426b362e75d4 -->
<!-- verdi:render-commit 0123456789abcdef0123456789abcdef01234567 -->
<!-- verdi: this is a generated skill; edits here never change verdi, and any difference is reported as drift by `verdi harness check` until this file is regenerated with `verdi harness render`. -->

# verdi-clarify

Use this skill on a draft spec on its design branch when the user asks what is still unclear, unresolved, or blocking. It reads the rendered spec document and writes only through `mutate_draft`, one operation per call, and only after the human has seen the proposal.

## Steps

1. Call `get_design_context` with the draft's ref (`spec/<slug>`). Keep `identity` (`checkout`, `branch`, `head`) and `current_draft` (the exact spec bytes and their digest): every `mutate_draft` call must carry them as `expected`, `base_spec_b64`, and `base_digest`.
2. Call `get_document` with `ref` `spec/<slug>`, `kind` `spec`, and `proposed` true (the working-tree draft on this branch, never the accepted bytes). Read three sections:
   - **Readiness.** Each concern is listed with its state, timing, blocking flag, summary, and witnesses. Collect every concern whose state is not proven. If the section says "Readiness was not supplied for this render.", say so to the human and continue with the next two sections.
   - **Open questions.** A question is unclaimed when its line ends with "unclaimed; blocks acceptance until a … claims it or a decision answers it." Collect them.
   - **Decisions.** Read them so a proposal never duplicates a ratified decision.
3. For each unclaimed question, in the document's order, prepare exactly one proposal:
   - a decision, when the human can answer now: `{"op":"add-decision","id":"<next dc-id>","text":"<the answer, with its rationale>","anchor":"<id>"}`; or
   - a research stub that claims the question, when an investigation is needed first: `{"op":"add-stub","slug":"<kebab-slug>","spike":true,"resolves":["<oq-id>"]}` (the store may display this class under its own word; use the word the document uses).
4. Show the human the proposal exactly as it will be sent: the question it answers or claims and the operation JSON. Ask for confirmation.
5. On confirmation, call `mutate_draft` with `harness` (your harness id), optional `session`, `schema` `verdi.draftmutation/v1`, `spec`, `base_digest`, `base_spec_b64`, `expected`, and `operations` holding that one operation. On a stale-base refusal, repeat from step 1; never resend against a guessed base.
6. Repeat steps 4–5 for the next question. Then, for each unproven readiness concern that a decision could settle, offer one `add-decision` the same way; a concern whose witness is missing evidence is reported, not decided.
7. Finish by listing what was written (operation, resulting id or slug) and what remains open.

## Never

Never batch several operations into one `mutate_draft` call in this skill. Never edit `.verdi/specs/…` directly. Never remove a question a decision has not answered.

```verdi-sequence
call get_design_context
call get_document kind=spec proposed=true
loop
show
confirm
call mutate_draft operations=1
end
```
