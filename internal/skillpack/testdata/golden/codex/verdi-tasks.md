---
name: verdi-tasks
description: Read a spec's tasks document — plan, evidence, and readiness — and turn it into a work list for the human without writing anything to the store.
---
<!-- verdi:generated-skill host=codex skill=tasks -->
<!-- verdi:engine-digest sha256:d805fb1ae60f08b6a2db8f122cd01f2c332314942b619debe5f6538bcb2e4856 -->
<!-- verdi:template-digest sha256:cb3a5342c874baef401f93ce793d5ce5b63f42c030ef4527c38f86442d5baf43 -->
<!-- verdi:render-commit 0123456789abcdef0123456789abcdef01234567 -->
<!-- verdi: this is a generated skill; edits here never change verdi, and any difference is reported as drift by `verdi harness check` until this file is regenerated with `verdi harness render`. -->

# verdi-tasks

Use this skill when the user asks what to work on next for a spec. It reads and never writes: no `mutate_draft`, no `add_annotation`, no import.

## Steps

1. Call `get_document` with `ref` `spec/<slug>` and `kind` `tasks`, adding `proposed` true when the spec is a draft on its design branch (omit it for an accepted spec).
2. From the plan section, list each stub with the criteria it covers. From the evidence section, note each criterion's evidence state and what is still unproven. From the readiness section, list the concerns that need attention, blocking ones first; if readiness was not supplied for this render, say so.
3. Present a work list in this order: blocking readiness concerns; criteria whose evidence table row has "—" in its Detail column (nothing implements or evidences them yet); criteria whose Detail column names an unsatisfied evidence kind ("<kind> unsatisfied"); then everything else, keeping each criterion's State column word as the document prints it. Quote ids so the human can find each item on the board.
4. If the human wants any of it changed, hand off to verdi-clarify or verdi-plan; this skill writes nothing.

```verdi-sequence
call get_document kind=tasks proposed=true
show
```
